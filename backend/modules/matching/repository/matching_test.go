package repository

import (
	"database/sql"
	"testing"

	"swap-chain/shared/db"

	pgvector "github.com/pgvector/pgvector-go"
)

func TestMapItemMatch(t *testing.T) {
	offerEmbedding := pgvector.NewVector([]float32{0.1, 0.2})
	wantEmbedding := pgvector.NewVector([]float32{0.3, 0.4})
	row := db.FindSimilarItemsRow{
		ID:                  8,
		OfferTitle:          "Игровая приставка",
		OfferDescription:    sql.NullString{String: "Описание вещи", Valid: true},
		WantDescription:     sql.NullString{String: "Горный велосипед", Valid: true},
		OfferEmbeddingLocal: &offerEmbedding,
		WantEmbeddingLocal:  &wantEmbedding,
		ImageAmount:         sql.NullInt32{Int32: 2, Valid: true},
		ParamRichness:       sql.NullString{String: "0.7", Valid: true},
		QualityScore:        sql.NullString{String: "0.8", Valid: true},
		UserRating:          sql.NullString{String: "4.5", Valid: true},
		UserSuccessRate:     sql.NullString{String: "0.9", Valid: true},
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
