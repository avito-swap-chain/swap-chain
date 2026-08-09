// Package repository adapts generated database queries to matching domain models.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"swap-chain/matching/model"
	"swap-chain/shared/db"
)

// Matching is a PostgreSQL-backed matching repository.
type Matching struct {
	queries *db.Queries
}

// NewPostgreSQLMatching creates a matching repository over sqlc queries.
func NewPostgreSQLMatching(queries *db.Queries) *Matching {
	return &Matching{queries: queries}
}

// IsSourceMatchable reports whether an item is ready to participate in matching.
func (r *Matching) IsSourceMatchable(ctx context.Context, itemID int) (bool, error) {
	_, err := r.queries.GetMatchingSourceItem(ctx, int64(itemID))

	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("get matching source item %d: %w", itemID, err)
	}
}

// FindSimilarItems returns cross-owner candidates mapped to domain values.
func (r *Matching) FindSimilarItems(ctx context.Context, sourceID int, limit int) ([]model.ItemMatch, error) {
	if limit <= 0 || int64(limit) > math.MaxInt32 {
		return nil, fmt.Errorf("find similar items: limit %d is outside (0,%d]", limit, int64(math.MaxInt32))
	}

	rows, err := r.queries.FindSimilarItems(ctx, db.FindSimilarItemsParams{
		ID:    int64(sourceID),
		Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("query similar items for source %d: %w", sourceID, err)
	}

	matches := make([]model.ItemMatch, 0, len(rows))
	for _, row := range rows {
		match, err := mapItemMatch(sourceID, row)
		if err != nil {
			return nil, fmt.Errorf("map candidate item %d: %w", row.ID, err)
		}

		matches = append(matches, match)
	}

	return matches, nil
}

func mapItemMatch(sourceID int, row db.FindSimilarItemsRow) (model.ItemMatch, error) {
	offerDescription := nullableString(row.OfferDescription)
	wantDescription := nullableString(row.WantDescription)

	return model.ItemMatch{
		SourceID: sourceID,
		TargetItem: model.Item{
			ID:               int(row.ID),
			OfferTitle:       row.OfferTitle,
			OfferDescription: offerDescription,
			WantDescription:  wantDescription,
			OfferVector:      row.OfferEmbedding.Slice(),
			WantVector:       row.WantEmbedding.Slice(),
			Meta: model.ItemMetadata{
				TitleLen:       len([]rune(row.OfferTitle)),
				DescriptionLen: len([]rune(offerDescription)),
				ImageAmount:    int(row.ImageAmount),
				ParamRichness:  row.ParamRichness,
				QualityScore:   row.QualityScore,
				UserRating:     row.UserRating,
				SuccessRate:    row.UserSuccessRate,
			},
		},
		Similarity: row.Similarity,
	}, nil
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}
