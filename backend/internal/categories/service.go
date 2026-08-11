package categories

import (
	"context"
	"fmt"

	"swap-chain/shared/db"
)

type Category struct {
	ID       int32
	Name     string
	IsSystem bool
}

type Service interface {
	List(ctx context.Context) ([]Category, error)
}

type PostgresService struct {
	queries             *db.Queries
	undefinedCategoryID int32
}

func NewPostgresService(queries *db.Queries, undefinedCategoryID int32) *PostgresService {
	return &PostgresService{
		queries:             queries,
		undefinedCategoryID: undefinedCategoryID,
	}
}

func (s *PostgresService) List(ctx context.Context) ([]Category, error) {
	rows, err := s.queries.ListCategories(ctx, s.undefinedCategoryID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	result := make([]Category, 0, len(rows))
	for _, row := range rows {
		result = append(result, Category{
			ID:       row.ID,
			Name:     row.Name,
			IsSystem: row.IsSystem,
		})
	}
	return result, nil
}
