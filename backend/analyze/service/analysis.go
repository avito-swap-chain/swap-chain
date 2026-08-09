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
	ErrManualCategoryRequired = errors.New("manual category selection required")
	ErrAnalysisStateChanged   = errors.New("item is no longer analyzing")
)

type AnalysisRepository interface {
	GetItemForAnalysis(ctx context.Context, id int64) (db.GetItemForAnalysisRow, error)
	CompleteItemAnalysis(ctx context.Context, arg db.CompleteItemAnalysisParams) (int64, error)
}

type ParamRichnessEvaluator interface {
	EvaluateDescription(ctx context.Context, description string) (*analyzemodel.DescriptionScore, error)
}

type CategoryDefiner interface {
	DefineTag(ctx context.Context, title string, description string) (*analyzemodel.CategoryMatch, error)
}

type AnalysisVectorizer interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

// Analysis выполняет полный повторяемый сценарий анализа уже созданной вещи.
// Все вычисления происходят до единственной финальной записи в БД.
type Analysis struct {
	repo       AnalysisRepository
	scoring    ParamRichnessEvaluator
	tagging    CategoryDefiner
	vectorizer AnalysisVectorizer
}

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
	if offerCategory.IsManual {
		return fmt.Errorf("define item %d offer category: %w", itemID, ErrManualCategoryRequired)
	}

	wantCategory, err := s.tagging.DefineTag(ctx, "", wantDescription)
	if err != nil {
		return fmt.Errorf("define item %d want category: %w", itemID, err)
	}
	if wantCategory.IsManual {
		return fmt.Errorf("define item %d want category: %w", itemID, ErrManualCategoryRequired)
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
		OfferCategoryID: sql.NullInt32{Int32: int32(offerCategory.CategoryID), Valid: true},
		WantCategoryID:  sql.NullInt32{Int32: int32(wantCategory.CategoryID), Valid: true},
		ParamRichness: sql.NullString{
			String: strconv.FormatFloat(descriptionScore.ParamRichness, 'f', -1, 64),
			Valid:  true,
		},
		OfferEmbeddingLocal: offerEmbedding,
		WantEmbeddingLocal:  wantEmbedding,
		ID:                  item.ID,
	})
	if err != nil {
		return fmt.Errorf("complete item %d analysis: %w", itemID, err)
	}
	if updated != 1 {
		return fmt.Errorf("complete item %d analysis: %w", itemID, ErrAnalysisStateChanged)
	}

	return nil
}
