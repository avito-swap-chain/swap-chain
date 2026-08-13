package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"swap-chain/modules/analyze/model"

	"go.uber.org/zap"
)

type CategoryClassifier interface {
	GenerateJSON(ctx context.Context, prompt string) (string, error)
}

type TaggingRepo interface {
	FindCategories(
		ctx context.Context,
		local []float32,
		external []float32,
		undefinedCategoryID int32,
	) ([]model.CategoryCandidate, error)
	ListCategories(ctx context.Context, undefinedCategoryID int32) ([]model.CategoryCandidate, error)
}

type TaggingConfig struct {
	SimilarityThreshold float64
	ConfidenceMargin    float64
	UndefinedCategoryID int32
}

// Tagging сначала классифицирует текст через Flash, затем использует embedding
// и при неоднозначности переводит карточку к ручному выбору.
type Tagging struct {
	classifier CategoryClassifier
	repo       TaggingRepo
	cfg        TaggingConfig
	logger     *zap.Logger
}

func NewTagging(classifier CategoryClassifier, repo TaggingRepo, cfg TaggingConfig, logger *zap.Logger) (*Tagging, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("tagging init: 'tagging repo' is required")
	case cfg.UndefinedCategoryID <= 0:
		return nil, fmt.Errorf("tagging init: 'undefined category ID' must be positive")
	case logger == nil:
		return nil, fmt.Errorf("tagging init: 'logger' is required")
	}

	if math.IsNaN(cfg.SimilarityThreshold) || math.IsInf(cfg.SimilarityThreshold, 0) ||
		cfg.SimilarityThreshold < -1 || cfg.SimilarityThreshold > 1 {
		return nil, fmt.Errorf("tagging init: invalid 'similarity threshold' range %g", cfg.SimilarityThreshold)
	}
	if math.IsNaN(cfg.ConfidenceMargin) || math.IsInf(cfg.ConfidenceMargin, 0) ||
		cfg.ConfidenceMargin < 0 || cfg.ConfidenceMargin > 2 {
		return nil, fmt.Errorf("tagging init: invalid 'confidence margin' %g", cfg.ConfidenceMargin)
	}

	return &Tagging{classifier: classifier, repo: repo, cfg: cfg, logger: logger}, nil
}

// DefineTag реализует цепочку Flash -> embedding -> ручной выбор.
func (s *Tagging) DefineTag(
	ctx context.Context,
	text string,
	local []float32,
	external []float32,
) (*model.CategoryMatch, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("define category: text is empty")
	}
	if len(local) == 0 && len(external) == 0 {
		return nil, fmt.Errorf("define category: embeddings are empty")
	}

	if s.classifier != nil {
		categoryID, err := s.defineByTextModel(ctx, text)
		if err == nil {
			if categoryID == s.cfg.UndefinedCategoryID {
				return &model.CategoryMatch{RequiresInput: true}, nil
			}
			return &model.CategoryMatch{CategoryID: categoryID}, nil
		}
		s.logger.Warn("category classification via text model failed, using embeddings", zap.Error(err))
	}

	return s.defineByCosineComparison(ctx, local, external)
}

func (s *Tagging) defineByTextModel(ctx context.Context, text string) (int32, error) {
	categories, err := s.repo.ListCategories(ctx, s.cfg.UndefinedCategoryID)
	if err != nil {
		return 0, fmt.Errorf("list categories: %w", err)
	}
	if len(categories) == 0 {
		return 0, fmt.Errorf("category list is empty")
	}

	var available strings.Builder
	known := make(map[int32]struct{}, len(categories))
	for _, category := range categories {
		known[category.ID] = struct{}{}
		available.WriteString(strconv.FormatInt(int64(category.ID), 10))
		available.WriteString(": ")
		available.WriteString(category.Name)
		available.WriteByte('\n')
	}
	known[s.cfg.UndefinedCategoryID] = struct{}{}

	raw, err := s.classifier.GenerateJSON(ctx, BuildCategoryDefinitionPrompt(available.String(), "", text))
	if err != nil {
		return 0, fmt.Errorf("generate category: %w", err)
	}
	s.logger.Info("category model response", zap.String("response", raw))
	var response struct {
		CategoryID int32 `json:"category_id"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return 0, fmt.Errorf("decode category response: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return 0, fmt.Errorf("decode category response: %w", err)
	}
	if _, ok := known[response.CategoryID]; !ok {
		return 0, fmt.Errorf("model returned unavailable category %d", response.CategoryID)
	}

	return response.CategoryID, nil
}

func (s *Tagging) defineByCosineComparison(
	ctx context.Context,
	local []float32,
	external []float32,
) (*model.CategoryMatch, error) {
	candidates, err := s.repo.FindCategories(ctx, local, external, s.cfg.UndefinedCategoryID)
	if err != nil {
		return nil, fmt.Errorf("find categories: %w", err)
	}
	if s.isNeedManual(candidates) {
		return &model.CategoryMatch{RequiresInput: true}, nil
	}

	return &model.CategoryMatch{CategoryID: candidates[0].ID}, nil
}

func (s *Tagging) isNeedManual(candidates []model.CategoryCandidate) bool {
	if len(candidates) == 0 || candidates[0].Similarity < s.cfg.SimilarityThreshold {
		return true
	}
	return len(candidates) > 1 && candidates[0].Similarity-candidates[1].Similarity < s.cfg.ConfidenceMargin
}
