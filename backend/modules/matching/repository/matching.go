package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"

	"swap-chain/modules/matching/model"
	"swap-chain/shared/db"
)

type Matching struct {
	queries *db.Queries
}

func NewPostgreSQLMatching(queries *db.Queries) (*Matching, error) {
	if queries == nil {
		return nil, fmt.Errorf("postgres matching repository init: 'queries' is required")
	}

	return &Matching{queries: queries}, nil
}

func (r *Matching) ValidateSourceItem(ctx context.Context, itemID int64) error {
	item, err := r.queries.GetMatchingSourceItem(ctx, itemID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.ErrItemNotFound
	case err != nil:
		return fmt.Errorf("get matching source item %d: %w", itemID, err)
	}

	actualStatus := mapItemStatus(item.Status)
	if actualStatus != model.ItemStatusMatching {
		return &model.ItemStatusError{
			ItemID:   itemID,
			Expected: model.ItemStatusMatching,
			Actual:   actualStatus,
		}
	}

	return nil
}

func (r *Matching) FindSimilarItems(
	ctx context.Context,
	sourceID int64,
	limit int,
) ([]model.ItemMatch, error) {
	if limit <= 0 || int64(limit) > math.MaxInt32 {
		return nil, fmt.Errorf("find similar items: limit %d is outside (0,%d]", limit, int64(math.MaxInt32))
	}

	rows, err := r.queries.FindSimilarItems(ctx, db.FindSimilarItemsParams{
		ItemID: sourceID,
		Limit:  int32(limit),
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

func mapItemMatch(sourceID int64, row db.FindSimilarItemsRow) (model.ItemMatch, error) {
	paramRichness, err := parseNullableFloat("param_richness", row.ParamRichness)
	if err != nil {
		return model.ItemMatch{}, err
	}

	qualityScore, err := parseNullableFloat("quality_score", row.QualityScore)
	if err != nil {
		return model.ItemMatch{}, err
	}

	userRating, err := parseFloat("user_rating", row.UserRating)
	if err != nil {
		return model.ItemMatch{}, err
	}

	userSuccessRate, err := parseFloat("user_success_rate", row.UserSuccessRate)
	if err != nil {
		return model.ItemMatch{}, err
	}

	offerDescription := nullableString(row.OfferDescription)

	var offerVector []float32
	if row.OfferEmbeddingLocal != nil {
		offerVector = row.OfferEmbeddingLocal.Slice()
	}

	return model.ItemMatch{
		SourceID: sourceID,
		TargetItem: model.Item{
			ID:               row.ID,
			OfferTitle:       row.OfferTitle,
			OfferDescription: offerDescription,
			OfferVector:      offerVector,
			Meta: model.ItemMetadata{
				TitleLen:       len([]rune(row.OfferTitle)),
				DescriptionLen: len([]rune(offerDescription)),
				ImageAmount:    int(row.ImageAmount.Int32),
				ParamRichness:  paramRichness,
				QualityScore:   qualityScore,
				UserRating:     userRating,
				SuccessRate:    userSuccessRate,
			},
		},
		Similarity: castSimilarity(row.Similarity),
	}, nil
}

func castSimilarity(sim interface{}) float64 {
	switch v := sim.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	}
	return 0
}

func parseNullableFloat(field string, value sql.NullString) (float64, error) {
	if !value.Valid {
		return 0, nil
	}

	return parseFloat(field, value.String)
}

func parseFloat(field, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s value %q: %w", field, value, err)
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("parse %s value %q: value must be finite", field, value)
	}

	return parsed, nil
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
