package service

import (
	"context"
	"fmt"
	"strings"
)

const categoryEmbeddingDimensions = 1024

// CategoryEmbeddingTarget содержит данные категории, необходимые для расчёта embedding.
type CategoryEmbeddingTarget struct {
	ID   int32
	Name string
}

// CategoryBootstrapRepository задаёт хранилище для подготовки справочника категорий.
type CategoryBootstrapRepository interface {
	CountCategories(ctx context.Context) (int64, error)
	CountCategoriesMissingEmbedding(ctx context.Context) (int64, error)
	ListCategoriesMissingEmbedding(ctx context.Context) ([]CategoryEmbeddingTarget, error)
	SetCategoryEmbedding(ctx context.Context, categoryID int32, embedding []float32) (bool, error)
}

// CategoryEmbeddingVectorizer рассчитывает embedding без дополнительного enrichment.
type CategoryEmbeddingVectorizer interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

// CategoryBootstrap заполняет отсутствующие embeddings версионированного справочника категорий.
type CategoryBootstrap struct {
	repo       CategoryBootstrapRepository
	vectorizer CategoryEmbeddingVectorizer
}

// NewCategoryBootstrap создаёт сценарий подготовки справочника категорий.
func NewCategoryBootstrap(
	repo CategoryBootstrapRepository,
	vectorizer CategoryEmbeddingVectorizer,
) (*CategoryBootstrap, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("category bootstrap: repository is required")
	case vectorizer == nil:
		return nil, fmt.Errorf("category bootstrap: vectorizer is required")
	default:
		return &CategoryBootstrap{repo: repo, vectorizer: vectorizer}, nil
	}
}

// Bootstrap рассчитывает embeddings категорий в детерминированном порядке по ID.
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

		updated, updateErr := bootstrap.repo.SetCategoryEmbedding(ctx, category.ID, embedding)
		if updateErr != nil {
			return fmt.Errorf("store category %d %q embedding: %w", category.ID, category.Name, updateErr)
		}
		if !updated {
			return fmt.Errorf("store category %d %q embedding: category was not updated", category.ID, category.Name)
		}
	}

	return bootstrap.Ready(ctx)
}

// Ready проверяет наличие справочника и embeddings у каждой категории.
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
