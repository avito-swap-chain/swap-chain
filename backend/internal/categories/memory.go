package categories

import (
	"context"
	"fmt"
	"sort"
)

type MemoryService struct {
	categories          []Category
	undefinedCategoryID int32
}

func NewMemoryService(all []Category, undefinedCategoryID int32) *MemoryService {
	return &MemoryService{
		categories:          all,
		undefinedCategoryID: undefinedCategoryID,
	}
}

func (s *MemoryService) List(_ context.Context) ([]Category, error) {
	result := make([]Category, 0, len(s.categories))
	for _, c := range s.categories {
		if c.ID == s.undefinedCategoryID {
			continue
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (s *MemoryService) ValidateCategory(_ context.Context, categoryID int32) error {
	if categoryID == s.undefinedCategoryID {
		return fmt.Errorf("%w: id=%d", ErrUndefinedCategory, categoryID)
	}
	for _, c := range s.categories {
		if c.ID == categoryID {
			return nil
		}
	}
	return fmt.Errorf("%w: id=%d", ErrCategoryNotFound, categoryID)
}
