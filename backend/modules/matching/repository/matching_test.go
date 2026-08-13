package repository

import (
	"database/sql"
	"math"
	"strings"
	"testing"

	"swap-chain/shared/db"

	pgvector "github.com/pgvector/pgvector-go"
)

func TestNewPostgreSQLMatchingRequiresQueries(t *testing.T) {
	if _, err := NewPostgreSQLMatching(nil); err == nil {
		t.Fatal("NewPostgreSQLMatching(nil) error = nil")
	}
}

func TestMapItemMatch(t *testing.T) {
	offerEmbedding := pgvector.NewVector([]float32{0.1, 0.2})
	row := db.FindSimilarItemsRow{
		ID:                  8,
		OfferTitle:          "Игровая приставка",
		OfferDescription:    sql.NullString{String: "Описание вещи", Valid: true},
		OfferEmbeddingLocal: &offerEmbedding,
		ImageAmount:         sql.NullInt32{Int32: 2, Valid: true},
		ParamRichness:       sql.NullString{String: "0.7", Valid: true},
		QualityScore:        sql.NullString{String: "0.8", Valid: true},
		UserRating:          "4.5",
		UserSuccessRate:     "0.9",
		Similarity:          0.91,
	}

	got, err := mapItemMatch(7, row)
	if err != nil {
		t.Fatalf("mapItemMatch() error = %v", err)
	}
	if got.SourceID != 7 || got.TargetItem.ID != 8 || got.Similarity != 0.91 {
		t.Fatalf("mapItemMatch() = %+v", got)
	}
	if got.TargetItem.Meta.ImageAmount != 2 || got.TargetItem.Meta.SuccessRate != 0.9 {
		t.Fatalf("metadata = %+v", got.TargetItem.Meta)
	}
}

func TestMapItemMatchRejectsInvalidNumericMetadata(t *testing.T) {
	row := db.FindSimilarItemsRow{
		ParamRichness:   sql.NullString{String: "not-a-number", Valid: true},
		UserRating:      "4.5",
		UserSuccessRate: "0.9",
	}
	if _, err := mapItemMatch(1, row); err == nil || !strings.Contains(err.Error(), "param_richness") {
		t.Fatalf("mapItemMatch() error = %v", err)
	}
	if _, err := parseFloat("rating", "NaN"); err == nil {
		t.Fatal("parseFloat(NaN) error = nil")
	}
	if got := castSimilarity(float32(0.5)); math.Abs(got-0.5) > 0.000001 {
		t.Fatalf("castSimilarity(float32) = %v", got)
	}
	if got := castSimilarity("unknown"); got != 0 {
		t.Fatalf("castSimilarity(unknown) = %v", got)
	}
}
