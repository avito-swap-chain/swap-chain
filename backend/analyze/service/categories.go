package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/pgvector/pgvector-go"

	"swap-chain/shared/db"
)

const categoryEmbeddingDimensions = 1024

// CategoryBootstrapRepository stores embeddings for the versioned category catalogue.
type CategoryBootstrapRepository interface {
	CountCategories(ctx context.Context) (int64, error)
	CountCategoriesMissingEmbedding(ctx context.Context) (int64, error)
	ListCategoriesMissingEmbedding(ctx context.Context) ([]db.ListCategoriesMissingEmbeddingRow, error)
	SetCategoryEmbedding(ctx context.Context, arg db.SetCategoryEmbeddingParams) (int64, error)
}

// CategoryBootstrap fills category embeddings using the same model as item analysis.
type CategoryBootstrap struct {
	repo       CategoryBootstrapRepository
	vectorizer AnalysisVectorizer
}

// NewCategoryBootstrap creates the category bootstrap workflow.
func NewCategoryBootstrap(repo CategoryBootstrapRepository, vectorizer AnalysisVectorizer) (*CategoryBootstrap, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("category bootstrap: repository is required")
	case vectorizer == nil:
		return nil, fmt.Errorf("category bootstrap: vectorizer is required")
	default:
		return &CategoryBootstrap{repo: repo, vectorizer: vectorizer}, nil
	}
}

// Bootstrap generates missing category embeddings in deterministic ID order.
func (bootstrap *CategoryBootstrap) Bootstrap(ctx context.Context) error {
	categories, err := bootstrap.repo.ListCategoriesMissingEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("list categories missing embeddings: %w", err)
	}
	if len(categories) == 0 {
		return bootstrap.Ready(ctx)
	}

	for _, category := range categories {
		embedding, vectorErr := bootstrap.vectorizer.Vectorize(ctx, categoryEmbeddingText(category.Name))
		if vectorErr != nil {
			return fmt.Errorf("vectorize category %d %q: %w", category.ID, category.Name, vectorErr)
		}
		if len(embedding) != categoryEmbeddingDimensions {
			return fmt.Errorf(
				"vectorize category %d %q: embedding has %d dimensions, want %d",
				category.ID,
				category.Name,
				len(embedding),
				categoryEmbeddingDimensions,
			)
		}

		updated, updateErr := bootstrap.repo.SetCategoryEmbedding(ctx, db.SetCategoryEmbeddingParams{
			EmbeddingLocal: pgvector.NewVector(embedding),
			ID:             category.ID,
		})
		if updateErr != nil {
			return fmt.Errorf("store category %d %q embedding: %w", category.ID, category.Name, updateErr)
		}
		if updated != 1 {
			return fmt.Errorf("store category %d %q embedding: updated %d rows, want 1", category.ID, category.Name, updated)
		}
	}

	return bootstrap.Ready(ctx)
}

// Ready verifies that the catalogue exists and every category has an embedding.
func (bootstrap *CategoryBootstrap) Ready(ctx context.Context) error {
	total, err := bootstrap.repo.CountCategories(ctx)
	if err != nil {
		return fmt.Errorf("count categories: %w", err)
	}
	if total == 0 {
		return fmt.Errorf("category catalogue is empty")
	}
	missing, err := bootstrap.repo.CountCategoriesMissingEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("count categories missing embeddings: %w", err)
	}
	if missing != 0 {
		return fmt.Errorf("category embeddings are incomplete: %d of %d missing", missing, total)
	}
	return nil
}

func categoryEmbeddingText(name string) string {
	return "Категория товара для обмена: " + strings.TrimSpace(name)
}
