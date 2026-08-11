package categories

import (
	"context"
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
