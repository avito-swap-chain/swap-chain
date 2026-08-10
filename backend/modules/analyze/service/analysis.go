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
	CompleteItemAnalysis(ctx context.Context, result model.AnalysisResult) (bool, error)
}

type ParamRichnessEvaluator interface {
	EvaluateDescription(ctx context.Context, description string) (*model.DescriptionScore, error)
}

type CategoryDefiner interface {
	DefineTag(ctx context.Context, embedding []float32) (*model.CategoryMatch, error)
}

type AnalysisVectorizer interface {
	EnrichAndVectorize(ctx context.Context, text string) ([]float32, error)
}

// Analysis выполняет полный повторяемый сценарий анализа уже созданной вещи.
// Все вычисления происходят до единственной финальной записи в БД.
type Analysis struct {
	repo       AnalysisRepository
	scoring    ParamRichnessEvaluator
	tagging    CategoryDefiner
	vectorizer AnalysisVectorizer
}

type analyzedText struct {
	normalized string
	embedding  []float32
}

const analysisConcurrency = 3

func NewAnalysis(
	repo AnalysisRepository,
	scoring ParamRichnessEvaluator,
	tagging CategoryDefiner,
	vectorizer AnalysisVectorizer,
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
	}

	return &Analysis{
		repo:       repo,
		scoring:    scoring,
		tagging:    tagging,
		vectorizer: vectorizer,
	}, nil
}

func (s *Analysis) AnalyzeItem(ctx context.Context, itemID int64) error {
	item, err := s.repo.GetItemForAnalysis(ctx, itemID)
	if err != nil {
		return fmt.Errorf("get item %d for analysis: %w", itemID, err)
	}

	offerDescription := normalizeText(item.OfferDescription)
	if offerDescription == "" {
		return fmt.Errorf("analyze item %d: offer description is empty", itemID)
	}

	wantDescription := normalizeText(item.WantDescription)
	if wantDescription == "" {
		return fmt.Errorf("analyze item %d: want description is empty", itemID)
	}

	offer := analyzedText{normalized: normalizeText(item.OfferTitle, offerDescription)}
	want := analyzedText{normalized: wantDescription}

	var descriptionScore *model.DescriptionScore
	var offerCategory *model.CategoryMatch
	var wantCategory *model.CategoryMatch

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(analysisConcurrency)

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
		offer.embedding, err = s.vectorizer.EnrichAndVectorize(groupCtx, offer.normalized)
		if err != nil {
			return fmt.Errorf("vectorize item %d offer: %w", itemID, err)
		}

		offerCategory, err = s.tagging.DefineTag(groupCtx, offer.embedding)
		if err != nil {
			return fmt.Errorf("define item %d offer category: %w", itemID, err)
		}
		if offerCategory == nil {
			return fmt.Errorf("define item %d offer category: category definer returned nil result", itemID)
		}

		return nil
	})

	group.Go(func() error {
		var err error
		want.embedding, err = s.vectorizer.EnrichAndVectorize(groupCtx, want.normalized)
		if err != nil {
			return fmt.Errorf("vectorize item %d want: %w", itemID, err)
		}

		wantCategory, err = s.tagging.DefineTag(groupCtx, want.embedding)
		if err != nil {
			return fmt.Errorf("define item %d want category: %w", itemID, err)
		}
		if wantCategory == nil {
			return fmt.Errorf("define item %d want category: category definer returned nil result", itemID)
		}

		return nil
	})

	if err := group.Wait(); err != nil {
		return err
	}

	updated, err := s.repo.CompleteItemAnalysis(ctx, model.AnalysisResult{
		ItemID:           item.ID,
		AnalysisVersion:  item.AnalysisVersion,
		OfferCategoryID:  offerCategory.CategoryID,
		WantCategoryID:   wantCategory.CategoryID,
		ParamRichness:    descriptionScore.ParamRichness,
		IsCategoryManual: offerCategory.IsManual || wantCategory.IsManual,
		OfferEmbedding:   offer.embedding,
		WantEmbedding:    want.embedding,
	})
	if err != nil {
		return fmt.Errorf("complete item %d analysis: %w", itemID, err)
	}
	if !updated {
		return fmt.Errorf("complete item %d analysis: %w", itemID, model.ErrAnalysisStateChanged)
	}

	return nil
}

func normalizeText(parts ...string) string {
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
