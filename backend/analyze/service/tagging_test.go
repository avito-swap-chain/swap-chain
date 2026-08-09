package service

import (
	"testing"

	"swap-chain/shared/db"
)

func TestTaggingRequiresManualSelectionWithoutCategories(t *testing.T) {
	tagging := &Tagging{cfg: TaggingConfig{SimilarityThreshold: 0.65, ConfidenceMargin: 0.05}}
	manual, category := tagging.isNeedManual(nil)
	if !manual || category != nil {
		t.Fatalf("isNeedManual() = %v, %+v", manual, category)
	}
}

func TestTaggingAcceptsConfidentCategory(t *testing.T) {
	tagging := &Tagging{cfg: TaggingConfig{SimilarityThreshold: 0.65, ConfidenceMargin: 0.05}}
	rows := []db.FindCategoryRow{
		{ID: 2, Similarity: 0.90},
		{ID: 3, Similarity: 0.70},
	}
	manual, category := tagging.isNeedManual(rows)
	if manual || category == nil || category.ID != 2 {
		t.Fatalf("isNeedManual() = %v, %+v", manual, category)
	}
}

func TestTaggingRejectsAmbiguousCategories(t *testing.T) {
	tagging := &Tagging{cfg: TaggingConfig{SimilarityThreshold: 0.65, ConfidenceMargin: 0.05}}
	rows := []db.FindCategoryRow{
		{ID: 2, Similarity: 0.80},
		{ID: 3, Similarity: 0.77},
	}
	manual, _ := tagging.isNeedManual(rows)
	if !manual {
		t.Fatal("isNeedManual() = false, want true")
	}
}
