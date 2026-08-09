package repository

import (
	"database/sql"
	"testing"

	"swap-chain/shared/db"

	pgvector "github.com/pgvector/pgvector-go"
)

func TestMapItemMatch(t *testing.T) {
	row := db.FindSimilarItemsRow{
		ID:               8,
		OfferTitle:       "Игровая приставка",
		OfferDescription: sql.NullString{String: "Описание вещи", Valid: true},
		WantDescription:  sql.NullString{String: "Горный велосипед", Valid: true},
		OfferEmbedding:   pgvector.NewVector([]float32{0.1, 0.2}),
		WantEmbedding:    pgvector.NewVector([]float32{0.3, 0.4}),
		ImageAmount:      2,
		ParamRichness:    0.7,
		QualityScore:     0.8,
		UserRating:       4.5,
		UserSuccessRate:  0.9,
		Similarity:       0.91,
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
