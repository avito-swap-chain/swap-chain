package categories

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryServiceListsOnlyUserCategoriesExcludingUndefined(t *testing.T) {
	service := NewMemoryService([]Category{
		{ID: 1, Name: "Электроника", IsSystem: true},
		{ID: 2, Name: "Бытовая техника", IsSystem: true},
		{ID: 46, Name: "Прочее", IsSystem: false},
		{ID: 47, Name: "Другое", IsSystem: true},
	}, 47)

	cats, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(cats) != 3 {
		t.Fatalf("got %d categories, want 3", len(cats))
	}
	for _, c := range cats {
		if c.ID == 47 {
			t.Fatalf("undefined category id=47 was not excluded")
		}
	}
	if cats[0].ID != 1 || cats[1].ID != 2 || cats[2].ID != 46 {
		t.Fatalf("wrong order: %+v", cats)
	}
}

func TestMemoryServiceValidatesUserFacingCategory(t *testing.T) {
	service := NewMemoryService([]Category{{ID: 2, Name: "Электроника"}, {ID: 47, Name: "Другое"}}, 47)
	if err := service.ValidateCategory(context.Background(), 2); err != nil {
		t.Fatalf("ValidateCategory(2) error = %v", err)
	}
	if err := service.ValidateCategory(context.Background(), 47); !errors.Is(err, ErrUndefinedCategory) {
		t.Fatalf("ValidateCategory(47) error = %v", err)
	}
	if err := service.ValidateCategory(context.Background(), 999); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("ValidateCategory(999) error = %v", err)
	}
}

func TestMemoryServiceReturnsEmptyWhenOnlyUndefined(t *testing.T) {
	service := NewMemoryService([]Category{
		{ID: 47, Name: "Другое", IsSystem: true},
	}, 47)

	cats, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(cats) != 0 {
		t.Fatalf("got %d categories, want 0", len(cats))
	}
}
