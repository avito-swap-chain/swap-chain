package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/analyze/model"

	"go.uber.org/zap"
)

type taggingRepoStub struct {
	categories []model.CategoryCandidate
	matches    []model.CategoryCandidate
	findCalls  int
}

func (r *taggingRepoStub) FindCategories(context.Context, []float32, []float32, int32) ([]model.CategoryCandidate, error) {
	r.findCalls++
	return r.matches, nil
}

func (r *taggingRepoStub) ListCategories(context.Context, int32) ([]model.CategoryCandidate, error) {
	return r.categories, nil
}

func newTaggingForTest(t *testing.T, classifier CategoryClassifier, repo *taggingRepoStub) *Tagging {
	t.Helper()
	tagging, err := NewTagging(classifier, repo, TaggingConfig{
		SimilarityThreshold: 0.65,
		ConfidenceMargin:    0.05,
		UndefinedCategoryID: 47,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewTagging() error = %v", err)
	}
	return tagging
}

func TestTaggingUsesTextModelFirst(t *testing.T) {
	repo := &taggingRepoStub{categories: []model.CategoryCandidate{{ID: 1, Name: "Электроника"}}}
	tagging := newTaggingForTest(t, generatorStub{response: `{"category_id":1}`}, repo)

	match, err := tagging.DefineTag(context.Background(), "Телефон", []float32{1}, nil)
	if err != nil {
		t.Fatalf("DefineTag() error = %v", err)
	}
	if match.CategoryID != 1 || match.RequiresInput || repo.findCalls != 0 {
		t.Fatalf("DefineTag() = %+v, embedding calls = %d", match, repo.findCalls)
	}
}

func TestTaggingRequestsInputWhenTextModelReturnsUndefined(t *testing.T) {
	repo := &taggingRepoStub{
		categories: []model.CategoryCandidate{{ID: 1, Name: "Электроника"}},
		matches: []model.CategoryCandidate{{ID: 1, Name: "Электроника", Similarity: 0.99}},
	}
	tagging := newTaggingForTest(t, generatorStub{response: `{"category_id":47}`}, repo)

	match, err := tagging.DefineTag(context.Background(), "Что-нибудь полезное", []float32{1}, nil)
	if err != nil {
		t.Fatalf("DefineTag() error = %v", err)
	}
	if !match.RequiresInput || match.CategoryID != 0 || repo.findCalls != 0 {
		t.Fatalf("DefineTag() = %+v, embedding calls = %d", match, repo.findCalls)
	}
}

func TestTaggingFallsBackToEmbedding(t *testing.T) {
	repo := &taggingRepoStub{
		categories: []model.CategoryCandidate{{ID: 1, Name: "Электроника"}},
		matches: []model.CategoryCandidate{
			{ID: 4, Name: "Спорт", Similarity: 0.90},
			{ID: 9, Name: "Транспорт", Similarity: 0.70},
		},
	}
	tagging := newTaggingForTest(t, generatorStub{err: errors.New("flash unavailable")}, repo)

	match, err := tagging.DefineTag(context.Background(), "Велосипед", []float32{1}, nil)
	if err != nil {
		t.Fatalf("DefineTag() error = %v", err)
	}
	if match.CategoryID != 4 || match.RequiresInput || repo.findCalls != 1 {
		t.Fatalf("DefineTag() = %+v, embedding calls = %d", match, repo.findCalls)
	}
}

func TestTaggingRequestsInputForAmbiguousEmbedding(t *testing.T) {
	repo := &taggingRepoStub{matches: []model.CategoryCandidate{
		{ID: 4, Name: "Спорт", Similarity: 0.80},
		{ID: 9, Name: "Транспорт", Similarity: 0.77},
		{ID: 3, Name: "Дом", Similarity: 0.41},
	}}
	tagging := newTaggingForTest(t, nil, repo)

	match, err := tagging.DefineTag(context.Background(), "Самокат", []float32{1}, nil)
	if err != nil {
		t.Fatalf("DefineTag() error = %v", err)
	}
	if !match.RequiresInput || match.CategoryID != 0 {
		t.Fatalf("DefineTag() = %+v", match)
	}
}
