package categories

import (
	"context"
	"errors"
	"fmt"

	"swap-chain/shared/db"
)

var ErrCategoryNotFound = errors.New("category not found")
var ErrUndefinedCategory = errors.New("undefined category is not allowed")

type Category struct {
	ID       int32
	Name     string
	IsSystem bool
}

type Service interface {
	List(ctx context.Context) ([]Category, error)
	ValidateCategory(ctx context.Context, categoryID int32) error
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

func (s *PostgresService) ValidateCategory(ctx context.Context, categoryID int32) error {
	if categoryID == s.undefinedCategoryID {
		return fmt.Errorf("%w: id=%d", ErrUndefinedCategory, categoryID)
	}
	cats, err := s.queries.ListCategories(ctx, s.undefinedCategoryID)
	if err != nil {
		return fmt.Errorf("validate category: %w", err)
	}
	for _, c := range cats {
		if c.ID == categoryID {
			return nil
		}
	}
	return fmt.Errorf("%w: id=%d", ErrCategoryNotFound, categoryID)
}
