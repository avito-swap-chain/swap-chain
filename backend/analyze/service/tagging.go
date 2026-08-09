package service

import (
	"context"
	"fmt"
	"math"

	"swap-chain/analyze/model"
)

type TaggingRepo interface {
	FindCategories(
		ctx context.Context,
		embedding []float32,
		undefinedCategoryID int32,
	) ([]model.CategoryCandidate, error)
}

type TaggingConfig struct {
	SimilarityThreshold float64 // порог, не достигнув который, включается ручное тегирование
	ConfidenceMargin    float64 // минимальный отрыв топ-1 от топ-2 категории для уверенности
	UndefinedCategoryID int32   // fallback-категория, если уверенно определить категорию не удалось
}

type Tagging struct {
	repo TaggingRepo
	cfg  TaggingConfig
}

func NewTagging(repo TaggingRepo, cfg TaggingConfig) (*Tagging, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("tagging init: 'tagging repo' is required")
	case cfg.UndefinedCategoryID <= 0:
		return nil, fmt.Errorf("tagging init: 'undefined category ID' must be positive")
	}

	if math.IsNaN(cfg.SimilarityThreshold) || math.IsInf(cfg.SimilarityThreshold, 0) ||
		cfg.SimilarityThreshold < -1 || cfg.SimilarityThreshold > 1 {
		return nil, fmt.Errorf("tagging init: invalid 'similarity threshold' range %g", cfg.SimilarityThreshold)
	}
	if math.IsNaN(cfg.ConfidenceMargin) || math.IsInf(cfg.ConfidenceMargin, 0) ||
		cfg.ConfidenceMargin < 0 || cfg.ConfidenceMargin > 2 {
		return nil, fmt.Errorf("tagging init: invalid 'confidence margin' %g", cfg.ConfidenceMargin)
	}

	return &Tagging{
		repo: repo,
		cfg:  cfg,
	}, nil
}

// DefineTag определяет наиболее подходящую категорию по готовому embedding.
// Вектор рассчитывает Analysis и переиспользует его для matching.
func (s *Tagging) DefineTag(ctx context.Context, embedding []float32) (*model.CategoryMatch, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("define category: embedding is empty")
	}

	rows, err := s.repo.FindCategories(ctx, embedding, s.cfg.UndefinedCategoryID)
	if err != nil {
		return nil, fmt.Errorf("find category error: %w", err)
	}
	isManual, topCategory := s.isNeedManual(rows)
	if isManual {
		match := &model.CategoryMatch{
			CategoryID: s.cfg.UndefinedCategoryID,
			IsManual:   true,
		}
		if topCategory != nil {
			match.Confidence = topCategory.Similarity
		}

		return match, nil
	}

	return &model.CategoryMatch{
		CategoryID: topCategory.ID,
		Confidence: topCategory.Similarity,
		IsManual:   false,
	}, nil
}

func (s *Tagging) isNeedManual(rows []model.CategoryCandidate) (bool, *model.CategoryCandidate) {
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
