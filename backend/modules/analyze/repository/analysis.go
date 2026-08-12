package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/pgvector/pgvector-go"

	"swap-chain/modules/analyze/model"
	"swap-chain/modules/analyze/service"
	"swap-chain/shared/db"
)

type Analysis struct {
	queries *db.Queries
}

func NewPostgreSQLAnalysis(queries *db.Queries) (*Analysis, error) {
	if queries == nil {
		return nil, fmt.Errorf("postgres analysis repository init: 'queries' is required")
	}

	return &Analysis{queries: queries}, nil
}

func (r *Analysis) GetItemForAnalysis(ctx context.Context, itemID int64) (model.AnalysisItem, error) {
	row, err := r.queries.GetItemForAnalysis(ctx, itemID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.AnalysisItem{}, model.ErrItemNotFound
	case err != nil:
		return model.AnalysisItem{}, fmt.Errorf("get item %d for analysis: %w", itemID, err)
	}

	actualStatus := mapItemStatus(row.Status)
	if actualStatus != model.ItemStatusAnalyzing {
		return model.AnalysisItem{}, &model.ItemStatusError{
			ItemID:   itemID,
			Expected: model.ItemStatusAnalyzing,
			Actual:   actualStatus,
		}
	}

	return mapAnalysisItem(row), nil
}

func (r *Analysis) CompleteItemAnalysis(ctx context.Context, result model.AnalysisResult) (bool, error) {
	rowsAffected, err := r.queries.CompleteItemAnalysis(ctx, mapAnalysisResult(result))
	if err != nil {
		return false, fmt.Errorf("complete item %d analysis: %w", result.ItemID, err)
	}
	if rowsAffected > 1 {
		return false, fmt.Errorf("complete item %d analysis: unexpected affected rows count %d", result.ItemID, rowsAffected)
	}

	return rowsAffected == 1, nil
}

func (r *Analysis) FindCategories(
	ctx context.Context,
	embedding []float32,
	undefinedCategoryID int32,
) ([]model.CategoryCandidate, error) {
	embeddingVector := pgvector.NewVector(embedding)
	rows, err := r.queries.FindCategory(ctx, db.FindCategoryParams{
		EmbeddingLocal:      &embeddingVector,
		UndefinedCategoryID: undefinedCategoryID,
	})
	if err != nil {
		return nil, fmt.Errorf("find categories: %w", err)
	}

	return mapCategoryCandidates(rows), nil
}

func (r *Analysis) CountCategories(ctx context.Context) (int64, error) {
	count, err := r.queries.CountCategories(ctx)
	if err != nil {
		return 0, fmt.Errorf("count categories: %w", err)
	}

	return count, nil
}

func (r *Analysis) CountCategoriesMissingEmbedding(ctx context.Context) (int64, error) {
	count, err := r.queries.CountCategoriesMissingEmbedding(ctx)
	if err != nil {
		return 0, fmt.Errorf("count categories missing embedding: %w", err)
	}

	return count, nil
}

func (r *Analysis) ListCategoriesMissingEmbedding(
	ctx context.Context,
) ([]service.CategoryEmbeddingTarget, error) {
	rows, err := r.queries.ListCategoriesMissingEmbedding(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories missing embedding: %w", err)
	}

	categories := make([]service.CategoryEmbeddingTarget, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, service.CategoryEmbeddingTarget{ID: row.ID, Name: row.Name})
	}

	return categories, nil
}

func (r *Analysis) SetCategoryEmbedding(
	ctx context.Context,
	categoryID int32,
	embedding []float32,
) (bool, error) {
	embeddingVector := pgvector.NewVector(embedding)
	rowsAffected, err := r.queries.SetCategoryEmbedding(ctx, db.SetCategoryEmbeddingParams{
		EmbeddingLocal: &embeddingVector,
		ID:             categoryID,
	})
	if err != nil {
		return false, fmt.Errorf("set category %d embedding: %w", categoryID, err)
	}
	if rowsAffected > 1 {
		return false, fmt.Errorf("set category %d embedding: unexpected affected rows count %d", categoryID, rowsAffected)
	}

	return rowsAffected == 1, nil
}

func (r *Analysis) ClaimStaleAnalyzingItems(
	ctx context.Context,
	staleBefore time.Time,
	batchSize int32,
) ([]int64, error) {
	itemIDs, err := r.queries.ClaimStaleAnalyzingItems(ctx, db.ClaimStaleAnalyzingItemsParams{
		StaleBefore: staleBefore,
		BatchSize:   batchSize,
	})
	if err != nil {
		return nil, fmt.Errorf("claim stale analyzing items: %w", err)
	}

	return itemIDs, nil
}

func mapAnalysisItem(row db.GetItemForAnalysisRow) model.AnalysisItem {
	return model.AnalysisItem{
		ID:               row.ID,
		AnalysisVersion:  row.AnalysisVersion,
		OfferTitle:       row.OfferTitle,
		OfferDescription: nullableString(row.OfferDescription),
	}
}

func mapAnalysisResult(result model.AnalysisResult) db.CompleteItemAnalysisParams {
	offerEmbedding := pgvector.NewVector(result.OfferEmbedding)
	return db.CompleteItemAnalysisParams{
		OfferCategoryID: sql.NullInt32{Int32: result.OfferCategoryID, Valid: true},
		ParamRichness: sql.NullString{
			String: strconv.FormatFloat(result.ParamRichness, 'f', -1, 64),
			Valid:  true,
		},
		IsCategoryManual:    result.IsCategoryManual,
		OfferEmbeddingLocal: &offerEmbedding,
		OfferEmbeddingExternal: &offerEmbedding,
		AnalysisVersion:     result.AnalysisVersion,
		ID:                  result.ItemID,
	}
}

func mapCategoryCandidates(rows []db.FindCategoryRow) []model.CategoryCandidate {
	candidates := make([]model.CategoryCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, model.CategoryCandidate{
			ID:         row.ID,
			Name:       row.Name,
			Similarity: row.Similarity,
		})
	}

	return candidates
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

func mapItemStatus(status db.ItemStatus) model.ItemStatus {
	return model.ItemStatus(status)
}

var (
	_ service.AnalysisRepository          = (*Analysis)(nil)
	_ service.TaggingRepo                 = (*Analysis)(nil)
	_ service.StaleAnalysisRepository     = (*Analysis)(nil)
	_ service.CategoryBootstrapRepository = (*Analysis)(nil)
)

func (r *Analysis) GetItemWishesForAnalysis(ctx context.Context, itemID int64) ([]db.GetItemWishesForAnalysisRow, error) {
	return r.queries.GetItemWishesForAnalysis(ctx, itemID)
}

func (r *Analysis) UpdateItemWishAnalysis(ctx context.Context, params db.UpdateItemWishAnalysisParams) error {
	return r.queries.UpdateItemWishAnalysis(ctx, params)
}
