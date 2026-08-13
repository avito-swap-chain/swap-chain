package service

import (
	"context"
	"fmt"
	"strings"

	"swap-chain/modules/analyze/model"

	"golang.org/x/sync/errgroup"
)

type AnalysisRepository interface {
	GetItemForAnalysis(ctx context.Context, itemID int64) (model.AnalysisItem, error)
	GetItemWishesForAnalysis(ctx context.Context, itemID int64) ([]model.AnalysisWish, error)
	CompleteItemAnalysis(ctx context.Context, result model.AnalysisResult) (bool, error)
}

type ParamRichnessEvaluator interface {
	EvaluateDescription(ctx context.Context, description string) (*model.DescriptionScore, error)
}

type CategoryDefiner interface {
	DefineTag(ctx context.Context, text string, local []float32, external []float32) (*model.CategoryMatch, error)
}

type AnalysisVectorizer interface {
	EnrichAndVectorize(ctx context.Context, text string) ([]float32, []float32, error)
}

type CategoryActionNotifier interface {
	NotifyCategoryActionRequired(ctx context.Context, userID int64, itemTitle string, itemID int64)
}

type AnalysisConfig struct {
	Concurrency int
}

// Analysis выполняет вычисления параллельно и сохраняет результат одной
// транзакцией после успешного завершения всего pipeline.
type Analysis struct {
	repo       AnalysisRepository
	scoring    ParamRichnessEvaluator
	tagging    CategoryDefiner
	vectorizer AnalysisVectorizer
	notifier   CategoryActionNotifier
	cfg        AnalysisConfig
}

type analyzedText struct {
	normalized        string
	embeddingLocal    []float32
	embeddingExternal []float32
}

func NewAnalysis(
	repo AnalysisRepository,
	scoring ParamRichnessEvaluator,
	tagging CategoryDefiner,
	vectorizer AnalysisVectorizer,
	notifier CategoryActionNotifier,
	cfg AnalysisConfig,
) (*Analysis, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("analysis init: 'analysis repo' is required")
	case scoring == nil:
		return nil, fmt.Errorf("analysis init: 'param richness evaluator' is required")
	case tagging == nil:
		return nil, fmt.Errorf("analysis init: 'category definer' is required")
	case vectorizer == nil:
		return nil, fmt.Errorf("analysis init: 'analysis vectorizer' is required")
	case notifier == nil:
		return nil, fmt.Errorf("analysis init: 'category action notifier' is required")
	case cfg.Concurrency <= 0:
		return nil, fmt.Errorf("analysis init: 'concurrency' must be positive")
	}

	return &Analysis{
		repo:       repo,
		scoring:    scoring,
		tagging:    tagging,
		vectorizer: vectorizer,
		notifier:   notifier,
		cfg:        cfg,
	}, nil
}

func (s *Analysis) AnalyzeItem(ctx context.Context, itemID int64) error {
	item, err := s.repo.GetItemForAnalysis(ctx, itemID)
	if err != nil {
		return fmt.Errorf("get item %d for analysis: %w", itemID, err)
	}
	wishes, err := s.repo.GetItemWishesForAnalysis(ctx, itemID)
	if err != nil {
		return fmt.Errorf("get wishes for item %d: %w", itemID, err)
	}
	if len(wishes) == 0 {
		return fmt.Errorf("analyze item %d: wishes are empty", itemID)
	}
	if item.OfferCategoryID == nil || !item.OfferCategoryIsManual {
		return fmt.Errorf("analyze item %d: %w", itemID, model.ErrOfferCategoryRequired)
	}

	offerDescription := normalizeText(item.OfferDescription)
	if offerDescription == "" {
		return fmt.Errorf("analyze item %d: offer description is empty", itemID)
	}

	offer := analyzedText{normalized: normalizeText(item.OfferTitle, offerDescription)}
	var descriptionScore *model.DescriptionScore
	wishResults := make([]model.WishAnalysisResult, len(wishes))

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.cfg.Concurrency)

	group.Go(func() error {
		result, err := s.scoring.EvaluateDescription(groupCtx, offerDescription)
		if err != nil {
			return fmt.Errorf("score item %d description: %w", itemID, err)
		}
		if result == nil {
			return fmt.Errorf("score item %d description: evaluator returned nil result", itemID)
		}
		if result.ParamRichness < 0 || result.ParamRichness > 1 {
			return fmt.Errorf("score item %d description: param richness %.4f is outside [0,1]", itemID, result.ParamRichness)
		}
		descriptionScore = result
		return nil
	})

	group.Go(func() error {
		var err error
		offer.embeddingLocal, offer.embeddingExternal, err = s.vectorizer.EnrichAndVectorize(groupCtx, offer.normalized)
		if err != nil {
			return fmt.Errorf("vectorize item %d offer: %w", itemID, err)
		}
		return nil
	})

	for index, wish := range wishes {
		index, wish := index, wish
		group.Go(func() error {
			normalized := normalizeText(wish.Description)
			if normalized == "" {
				return fmt.Errorf("analyze wish %d: description is empty", wish.ID)
			}
			local, external, err := s.vectorizer.EnrichAndVectorize(groupCtx, normalized)
			if err != nil {
				return fmt.Errorf("vectorize wish %d: %w", wish.ID, err)
			}

			result := model.WishAnalysisResult{
				ID:                    wish.ID,
				CategoryIsManual:      wish.CategoryIsManual,
				WantEmbeddingLocal:    local,
				WantEmbeddingExternal: external,
			}
			if wish.CategoryIsManual && wish.CategoryID != nil {
				result.CategoryID = copyInt32(wish.CategoryID)
				wishResults[index] = result
				return nil
			}

			category, err := s.tagging.DefineTag(groupCtx, normalized, local, external)
			if err != nil {
				return fmt.Errorf("define wish %d category: %w", wish.ID, err)
			}
			if category == nil {
				return fmt.Errorf("define wish %d category: category definer returned nil result", wish.ID)
			}
			if category.RequiresInput {
			} else {
				result.CategoryID = int32Pointer(category.CategoryID)
			}
			wishResults[index] = result
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	result := model.AnalysisResult{
		ItemID:                    item.ID,
		AnalysisVersion:           item.AnalysisVersion,
		OfferCategoryID:           copyInt32(item.OfferCategoryID),
		OfferCategoryIsManual:     true,
		ParamRichness:             descriptionScore.ParamRichness,
		OfferEmbeddingLocal:       offer.embeddingLocal,
		OfferEmbeddingExternal:    offer.embeddingExternal,
		Wishes:                    wishResults,
	}
	for _, wish := range wishResults {
		if wish.CategoryID == nil {
			result.RequiresCategoryInput = true
			break
		}
	}

	updated, err := s.repo.CompleteItemAnalysis(ctx, result)
	if err != nil {
		return fmt.Errorf("complete item %d analysis: %w", itemID, err)
	}
	if !updated {
		return fmt.Errorf("complete item %d analysis: %w", itemID, model.ErrAnalysisStateChanged)
	}
	if result.RequiresCategoryInput {
		s.notifier.NotifyCategoryActionRequired(ctx, item.UserID, item.OfferTitle, item.ID)
	}

	return nil
}

func normalizeText(parts ...string) string {
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func int32Pointer(value int32) *int32 {
	return &value
}

func copyInt32(value *int32) *int32 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}
