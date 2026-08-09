package service

import (
	"context"
	"fmt"

	analyzemodel "swap-chain/analyze/model"
	"swap-chain/shared/db"

	"github.com/pgvector/pgvector-go"
)

type TaggingRepo interface {
	FindCategory(ctx context.Context, embedding pgvector.Vector) ([]db.FindCategoryRow, error)
}

type TaggingConfig struct {
	SimilarityThreshold float64 // порог, не достигнув который, включается ручное тегирование
	ConfidenceMargin    float64 // минимальный отрыв топ-1 от топ-2 категории для уверенности
}

type Tagging struct {
	vectorizer *Vectorizer
	repo       TaggingRepo
	cfg        TaggingConfig
}

func NewTagging(vectorizer *Vectorizer, repo TaggingRepo, cfg TaggingConfig) *Tagging {
	return &Tagging{
		vectorizer: vectorizer,
		repo:       repo,
		cfg:        cfg,
	}
}

// DefineTag определяет наиболее подходящую категорию по текстовому описанию
func (s *Tagging) DefineTag(ctx context.Context, title string, description string) (*analyzemodel.CategoryMatch, error) {
	textForTagging := fmt.Sprintf("%s. %s", title, description)

	vec, err := s.vectorizer.Vectorize(ctx, textForTagging)
	if err != nil {
		return nil, fmt.Errorf("vectorize error: %w", err)
	}

	vecArg := pgvector.NewVector(vec)
	rows, err := s.repo.FindCategory(ctx, vecArg)
	if err != nil {
		return nil, fmt.Errorf("find category error: %w", err)
	}
	isManual, topCategory := s.isNeedManual(rows)
	if topCategory == nil || isManual {
		return &analyzemodel.CategoryMatch{IsManual: true}, nil
	}

	return &analyzemodel.CategoryMatch{
		CategoryID: int(topCategory.ID),
		Confidence: topCategory.Similarity,
		IsManual:   false,
	}, nil
}

func (s *Tagging) isNeedManual(rows []db.FindCategoryRow) (bool, *db.FindCategoryRow) {
	if len(rows) == 0 {
		return true, nil
	}

	topCategory := rows[0]

	if topCategory.Similarity < s.cfg.SimilarityThreshold {
		return true, &topCategory
	}

	if len(rows) >= 2 {
		secondCategory := rows[1]
		diff := topCategory.Similarity - secondCategory.Similarity

		if diff < s.cfg.ConfidenceMargin {
			return true, nil
		}
	}

	return false, &topCategory
}
