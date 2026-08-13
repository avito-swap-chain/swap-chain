package service

import (
	"context"
	"strings"
	"testing"
)

func TestCategoryBootstrapFillsMissingEmbeddings(t *testing.T) {
	repo := &categoryBootstrapRepo{
		missing: []CategoryEmbeddingTarget{
			{ID: 1, Name: "Электроника"},
			{ID: 2, Name: "Книги"},
		},
		total: 2,
	}
	vectorizer := categoryVectorizer{vector: make([]float32, categoryEmbeddingDimensions)}
	bootstrap, err := NewCategoryBootstrap(repo, vectorizer, nil)
	if err != nil {
		t.Fatalf("create bootstrap: %v", err)
	}
	if err := bootstrap.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if len(repo.updated) != 2 || repo.updated[0] != 1 || repo.updated[1] != 2 {
		t.Fatalf("updated categories = %v, want [1 2]", repo.updated)
	}
}

func TestCategoryBootstrapRejectsWrongEmbeddingDimensions(t *testing.T) {
	repo := &categoryBootstrapRepo{
		missing: []CategoryEmbeddingTarget{{ID: 1, Name: "Электроника"}},
		total:   1,
	}
	bootstrap, err := NewCategoryBootstrap(repo, categoryVectorizer{vector: []float32{1, 2, 3}}, nil)
	if err != nil {
		t.Fatalf("create bootstrap: %v", err)
	}
	err = bootstrap.Bootstrap(context.Background())
	if err == nil || !strings.Contains(err.Error(), "embedding has 3 dimensions") {
		t.Fatalf("bootstrap error = %v", err)
	}
}

func TestCategoryBootstrapReadinessRequiresPopulatedCatalogue(t *testing.T) {
	bootstrap, err := NewCategoryBootstrap(&categoryBootstrapRepo{}, categoryVectorizer{}, nil)
	if err != nil {
		t.Fatalf("create bootstrap: %v", err)
	}
	if err := bootstrap.Ready(context.Background()); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("ready error = %v", err)
	}
}

type categoryBootstrapRepo struct {
	missing []CategoryEmbeddingTarget
	total   int64
	updated []int32
	err     error
}

func (repo *categoryBootstrapRepo) CountCategories(context.Context) (int64, error) {
	return repo.total, repo.err
}

func (repo *categoryBootstrapRepo) CountCategoriesMissingEmbedding(context.Context) (int64, error) {
	if repo.err != nil {
		return 0, repo.err
	}
	return int64(len(repo.missing) - len(repo.updated)), nil
}

func (repo *categoryBootstrapRepo) ListCategoriesMissingEmbedding(context.Context) ([]CategoryEmbeddingTarget, error) {
	if repo.err != nil {
		return nil, repo.err
	}
	return repo.missing, nil
}

func (repo *categoryBootstrapRepo) SetCategoryEmbedding(
	_ context.Context,
	categoryID int32,
	_ []float32,
	_ []float32,
) (bool, error) {
	if repo.err != nil {
		return false, repo.err
	}
	repo.updated = append(repo.updated, categoryID)
	return true, nil
}

type categoryVectorizer struct {
	vector []float32
	err    error
}

func (vectorizer categoryVectorizer) Vectorize(_ context.Context, _ string) ([]float32, error) {
	if vectorizer.err != nil {
		return nil, vectorizer.err
	}
	return vectorizer.vector, nil
}
