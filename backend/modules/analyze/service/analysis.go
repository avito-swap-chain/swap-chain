package service

import (
	"context"
	"fmt"
	"strings"

	"swap-chain/modules/analyze/model"
	"swap-chain/shared/db"
	"github.com/pgvector/pgvector-go"

	"golang.org/x/sync/errgroup"
)

type AnalysisRepository interface {
	GetItemForAnalysis(ctx context.Context, itemID int64) (model.AnalysisItem, error)
	CompleteItemAnalysis(ctx context.Context, result model.AnalysisResult) (bool, error)
	GetItemWishesForAnalysis(ctx context.Context, itemID int64) ([]db.GetItemWishesForAnalysisRow, error)
	UpdateItemWishAnalysis(ctx context.Context, params db.UpdateItemWishAnalysisParams) error
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

type AnalysisConfig struct {
	Concurrency int
}

// Analysis выполняет полный повторяемый сценарий анализа уже созданной вещи.
// Все вычисления происходят до единственной финальной записи в БД.
type Analysis struct {
	repo       AnalysisRepository
	scoring    ParamRichnessEvaluator
	tagging    CategoryDefiner
	vectorizer AnalysisVectorizer
	cfg        AnalysisConfig
}

type analyzedText struct {
	normalized string
	embedding  []float32
}

func NewAnalysis(
	repo AnalysisRepository,
	scoring ParamRichnessEvaluator,
	tagging CategoryDefiner,
	vectorizer AnalysisVectorizer,
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
	}

	return &Analysis{
		repo:       repo,
		scoring:    scoring,
		tagging:    tagging,
		vectorizer: vectorizer,
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

	offerDescription := normalizeText(item.OfferDescription)
	if offerDescription == "" {
		return fmt.Errorf("analyze item %d: offer description is empty", itemID)
	}

	offer := analyzedText{normalized: normalizeText(item.OfferTitle, offerDescription)}

	var descriptionScore *model.DescriptionScore
	var offerCategory *model.CategoryMatch
	isCategoryManual := false

	group, groupCtx := errgroup.WithContext(ctx)
	if s.cfg.Concurrency > 0 {
		group.SetLimit(s.cfg.Concurrency)
	}

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
		
		if offerCategory.IsManual {
			isCategoryManual = true
		}

		return nil
	})

	for _, wish := range wishes {
		wish := wish
		group.Go(func() error {
			wantNormalized := normalizeText(wish.WantDescription)
			if wantNormalized == "" {
				return nil
			}
			embedding, err := s.vectorizer.EnrichAndVectorize(groupCtx, wantNormalized)
			if err != nil {
				return fmt.Errorf("vectorize wish %d: %w", wish.ID, err)
			}
			cat, err := s.tagging.DefineTag(groupCtx, embedding)
			if err != nil {
				return fmt.Errorf("define wish %d category: %w", wish.ID, err)
			}
			if cat == nil {
				return fmt.Errorf("define wish %d category: returned nil result", wish.ID)
			}
			
			if cat.IsManual {
				isCategoryManual = true
			}

			// we need to cast embedding to float32 pgvector type, but since service logic calls repo, we just pass what db types need
			// wait, our interface uses db.UpdateItemWishAnalysisParams, let's use it
			// db.UpdateItemWishAnalysisParams needs want_embedding_local pgvector.Vector etc
			vec := pgvector.NewVector(embedding)
			return s.repo.UpdateItemWishAnalysis(groupCtx, db.UpdateItemWishAnalysisParams{
				WantCategoryID: cat.CategoryID,
				WantEmbeddingLocal: &vec,
				WantEmbeddingExternal: &vec,
				ID: wish.ID,
			})
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	updated, err := s.repo.CompleteItemAnalysis(ctx, model.AnalysisResult{
		ItemID:           item.ID,
		AnalysisVersion:  item.AnalysisVersion,
		OfferCategoryID:  offerCategory.CategoryID,
		ParamRichness:    descriptionScore.ParamRichness,
		IsCategoryManual: isCategoryManual,
		OfferEmbedding:   offer.embedding,
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
