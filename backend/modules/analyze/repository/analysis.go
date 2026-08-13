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
	database *sql.DB
	queries  *db.Queries
}

func NewPostgreSQLAnalysis(database *sql.DB, queries *db.Queries) (*Analysis, error) {
	if database == nil {
		return nil, fmt.Errorf("postgres analysis repository init: 'database' is required")
	}
	if queries == nil {
		return nil, fmt.Errorf("postgres analysis repository init: 'queries' is required")
	}

	return &Analysis{database: database, queries: queries}, nil
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
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin complete item %d analysis: %w", result.ItemID, err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := r.queries.WithTx(tx)
	for _, wish := range result.Wishes {
		rowsAffected, updateErr := queries.UpdateItemWishAnalysis(ctx, mapWishAnalysisResult(result.ItemID, wish))
		if updateErr != nil {
			return false, fmt.Errorf("update wish %d analysis: %w", wish.ID, updateErr)
		}
		if rowsAffected != 1 {
			return false, fmt.Errorf("update wish %d analysis: unexpected affected rows count %d", wish.ID, rowsAffected)
		}
	}

	var rowsAffected int64
	if result.RequiresCategoryInput {
		rowsAffected, err = queries.CompleteItemAnalysisActionRequired(ctx, mapActionRequiredResult(result))
	} else {
		if result.OfferCategoryID == nil {
			return false, fmt.Errorf("complete item %d analysis: offer category is missing", result.ItemID)
		}
		for _, wish := range result.Wishes {
			if wish.CategoryID == nil {
				return false, fmt.Errorf("complete item %d analysis: wish %d category is missing", result.ItemID, wish.ID)
			}
		}
		rowsAffected, err = queries.CompleteItemAnalysisMatching(ctx, mapMatchingResult(result))
	}
	if err != nil {
		return false, fmt.Errorf("finalize item %d analysis: %w", result.ItemID, err)
	}
	if rowsAffected > 1 {
		return false, fmt.Errorf("complete item %d analysis: unexpected affected rows count %d", result.ItemID, rowsAffected)
	}
	if rowsAffected == 0 {
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit item %d analysis: %w", result.ItemID, err)
	}

	return true, nil
}

func (r *Analysis) FindCategories(
	ctx context.Context,
	local []float32,
	external []float32,
	undefinedCategoryID int32,
) ([]model.CategoryCandidate, error) {
	var vecLocal *pgvector.Vector
	if local != nil {
		v := pgvector.NewVector(local)
		vecLocal = &v
	}
	var vecExternal *pgvector.Vector
	if external != nil {
		v := pgvector.NewVector(external)
		vecExternal = &v
	}

	rows, err := r.queries.FindCategory(ctx, db.FindCategoryParams{
		EmbeddingLocal:      vecLocal,
		EmbeddingExternal:   vecExternal,
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

func (r *Analysis) ListCategories(ctx context.Context, undefinedCategoryID int32) ([]model.CategoryCandidate, error) {
	rows, err := r.queries.ListCategories(ctx, undefinedCategoryID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	candidates := make([]model.CategoryCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, model.CategoryCandidate{
			ID:   row.ID,
			Name: row.Name,
		})
	}
	return candidates, nil
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
	local []float32,
	external []float32,
) (bool, error) {
	var vecLocal *pgvector.Vector
	if local != nil {
		v := pgvector.NewVector(local)
		vecLocal = &v
	}
	var vecExternal *pgvector.Vector
	if external != nil {
		v := pgvector.NewVector(external)
		vecExternal = &v
	}
	rowsAffected, err := r.queries.SetCategoryEmbedding(ctx, db.SetCategoryEmbeddingParams{
		EmbeddingLocal:    vecLocal,
		EmbeddingExternal: vecExternal,
		ID:                categoryID,
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
		ID:                    row.ID,
		UserID:                row.UserID,
		AnalysisVersion:       row.AnalysisVersion,
		OfferTitle:            row.OfferTitle,
		OfferDescription:      nullableString(row.OfferDescription),
		OfferCategoryID:       nullableInt32(row.OfferCategoryID),
		OfferCategoryIsManual: row.IsCategoryManual,
	}
}

func mapMatchingResult(result model.AnalysisResult) db.CompleteItemAnalysisMatchingParams {
	return db.CompleteItemAnalysisMatchingParams{
		OfferCategoryID:        nullableInt32Value(result.OfferCategoryID),
		ParamRichness:          numeric(result.ParamRichness),
		IsCategoryManual:       result.OfferCategoryIsManual,
		OfferEmbeddingLocal:    vector(result.OfferEmbeddingLocal),
		OfferEmbeddingExternal: vector(result.OfferEmbeddingExternal),
		AnalysisVersion:        result.AnalysisVersion,
		ID:                     result.ItemID,
	}
}

func mapActionRequiredResult(result model.AnalysisResult) db.CompleteItemAnalysisActionRequiredParams {
	return db.CompleteItemAnalysisActionRequiredParams{
		OfferCategoryID:        nullableInt32Value(result.OfferCategoryID),
		ParamRichness:             numeric(result.ParamRichness),
		IsCategoryManual:          result.OfferCategoryIsManual,
		OfferEmbeddingLocal:       vector(result.OfferEmbeddingLocal),
		OfferEmbeddingExternal:    vector(result.OfferEmbeddingExternal),
		AnalysisVersion:           result.AnalysisVersion,
		ID:                        result.ItemID,
	}
}

func mapWishAnalysisResult(itemID int64, result model.WishAnalysisResult) db.UpdateItemWishAnalysisParams {
	return db.UpdateItemWishAnalysisParams{
		WantCategoryID:        nullableInt32Value(result.CategoryID),
		IsCategoryManual:      result.CategoryIsManual,
		WantEmbeddingLocal:    vector(result.WantEmbeddingLocal),
		WantEmbeddingExternal: vector(result.WantEmbeddingExternal),
		ID:                    result.ID,
		ItemID:                itemID,
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

func nullableInt32(value sql.NullInt32) *int32 {
	if !value.Valid {
		return nil
	}
	result := value.Int32
	return &result
}

func nullableInt32Value(value *int32) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: *value, Valid: true}
}

func numeric(value float64) sql.NullString {
	return sql.NullString{String: strconv.FormatFloat(value, 'f', -1, 64), Valid: true}
}

func vector(value []float32) *pgvector.Vector {
	if len(value) == 0 {
		return nil
	}
	result := pgvector.NewVector(value)
	return &result
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

func (r *Analysis) GetItemWishesForAnalysis(ctx context.Context, itemID int64) ([]model.AnalysisWish, error) {
	rows, err := r.queries.GetItemWishesForAnalysis(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("get wishes for item %d: %w", itemID, err)
	}

	wishes := make([]model.AnalysisWish, 0, len(rows))
	for _, row := range rows {
		wishes = append(wishes, model.AnalysisWish{
			ID:               row.ID,
			Description:      row.WantDescription,
			CategoryID:       nullableInt32(row.WantCategoryID),
			CategoryIsManual: row.IsCategoryManual,
		})
	}
	return wishes, nil
}
