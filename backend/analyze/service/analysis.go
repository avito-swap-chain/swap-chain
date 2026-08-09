// Package service implements item enrichment, categorization and vectorization.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pgvector/pgvector-go"

	analyzemodel "swap-chain/analyze/model"
	"swap-chain/shared/db"
)

var (
	// ErrAnalysisStateChanged indicates that an item left ANALYZING before completion.
	ErrAnalysisStateChanged = errors.New("item is no longer analyzing")
)

type analysisRepository interface {
	GetItemForAnalysis(ctx context.Context, id int64) (db.GetItemForAnalysisRow, error)
	CompleteItemAnalysis(ctx context.Context, arg db.CompleteItemAnalysisParams) (int64, error)
}

type paramRichnessEvaluator interface {
	EvaluateDescription(ctx context.Context, description string) (*analyzemodel.DescriptionScore, error)
}

type categoryDefiner interface {
	DefineTag(ctx context.Context, title string, description string) (*analyzemodel.CategoryMatch, error)
}

type analysisVectorizer interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

// Analysis выполняет полный повторяемый сценарий анализа уже созданной вещи.
// Все вычисления происходят до единственной финальной записи в БД.
type Analysis struct {
	repo       analysisRepository
	scoring    paramRichnessEvaluator
	tagging    categoryDefiner
	vectorizer analysisVectorizer
}

// NewAnalysis creates the complete item-analysis pipeline.
func NewAnalysis(
	repo analysisRepository,
	scoring paramRichnessEvaluator,
	tagging categoryDefiner,
	vectorizer analysisVectorizer,
) *Analysis {
	return &Analysis{
		repo:       repo,
		scoring:    scoring,
		tagging:    tagging,
		vectorizer: vectorizer,
	}
}

// AnalyzeItem computes all matching metadata and atomically completes analysis.
func (s *Analysis) AnalyzeItem(ctx context.Context, itemID int64) error {
	item, err := s.repo.GetItemForAnalysis(ctx, itemID)
	if err != nil {
		return fmt.Errorf("get item %d for analysis: %w", itemID, err)
	}

	offerDescription := strings.TrimSpace(item.OfferDescription.String)
	if !item.OfferDescription.Valid || offerDescription == "" {
		return fmt.Errorf("analyze item %d: offer description is empty", itemID)
	}

	wantDescription := strings.TrimSpace(item.WantDescription.String)
	if !item.WantDescription.Valid || wantDescription == "" {
		return fmt.Errorf("analyze item %d: want description is empty", itemID)
	}

	descriptionScore, err := s.scoring.EvaluateDescription(ctx, offerDescription)
	if err != nil {
		return fmt.Errorf("score item %d description: %w", itemID, err)
	}
	if descriptionScore.ParamRichness < 0 || descriptionScore.ParamRichness > 1 {
		return fmt.Errorf("score item %d description: param richness %.4f is outside [0,1]", itemID, descriptionScore.ParamRichness)
	}

	offerCategory, err := s.tagging.DefineTag(ctx, item.OfferTitle, offerDescription)
	if err != nil {
		return fmt.Errorf("define item %d offer category: %w", itemID, err)
	}

	wantCategory, err := s.tagging.DefineTag(ctx, "", wantDescription)
	if err != nil {
		return fmt.Errorf("define item %d want category: %w", itemID, err)
	}

	offerText := strings.TrimSpace(item.OfferTitle + " " + offerDescription)
	offerVector, err := s.vectorizer.Vectorize(ctx, offerText)
	if err != nil {
		return fmt.Errorf("vectorize item %d offer: %w", itemID, err)
	}

	wantVector, err := s.vectorizer.Vectorize(ctx, wantDescription)
	if err != nil {
		return fmt.Errorf("vectorize item %d want: %w", itemID, err)
	}
	offerEmbedding := pgvector.NewVector(offerVector)
	wantEmbedding := pgvector.NewVector(wantVector)

	updated, err := s.repo.CompleteItemAnalysis(ctx, db.CompleteItemAnalysisParams{
		OfferCategoryID: categoryID(offerCategory),
		WantCategoryID:  categoryID(wantCategory),
		ParamRichness: sql.NullString{
			String: strconv.FormatFloat(descriptionScore.ParamRichness, 'f', -1, 64),
			Valid:  true,
		},
		OfferEmbedding:   offerEmbedding,
		WantEmbedding:    wantEmbedding,
		IsCategoryManual: offerCategory.IsManual || wantCategory.IsManual,
		ID:               item.ID,
	})
	if err != nil {
		return fmt.Errorf("complete item %d analysis: %w", itemID, err)
	}
	if updated != 1 {
		return fmt.Errorf("complete item %d analysis: %w", itemID, ErrAnalysisStateChanged)
	}

	return nil
}

func categoryID(match *analyzemodel.CategoryMatch) sql.NullInt32 {
	if match == nil || match.IsManual || match.CategoryID <= 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(match.CategoryID), Valid: true}
}
