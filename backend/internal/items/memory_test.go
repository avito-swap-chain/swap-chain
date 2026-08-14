package items

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMemoryServiceRequiresOfferCategory(t *testing.T) {
	service := NewMemoryService()
	_, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["categoryId"] == "" {
		t.Fatalf("Create() error = %v, want categoryId validation error", err)
	}
}

func TestMemoryServiceWithdrawsItemWithoutDeletingIt(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := service.Update(context.Background(), 7, created.ID, UpdateInput{Withdraw: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != "WITHDRAWN" || len(updated.Wishes) != 0 {
		t.Fatalf("withdrawn item = %+v, want WITHDRAWN with empty wishes", updated)
	}
	stored, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.ID != created.ID || stored.Status != "WITHDRAWN" {
		t.Fatalf("withdrawn item was not retained: %+v", stored)
	}
}

func TestMemoryServiceRestartsAnalysisWhenWishChanges(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.items[created.ID] = Item{ID: created.ID, UserID: 7, Status: "WITHDRAWN"}

	want := "Игровой телефон"
	updated, err := service.Update(context.Background(), 7, created.ID, UpdateInput{Wishes: []string{want}})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != "ANALYZING" || len(updated.Wishes) == 0 || updated.Wishes[0].Description != want {
		t.Fatalf("updated item = %+v, want ANALYZING with new wish", updated)
	}
}

func TestMemoryServiceRefusesTerminalOrForeignItemUpdate(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	description := "Новое описание"
	if _, err := service.Update(context.Background(), 8, created.ID, UpdateInput{OfferDescription: &description}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign Update() error = %v, want ErrForbidden", err)
	}
	for _, status := range []string{"LOCKED", "EXCHANGED"} {
		service.mu.Lock()
		item := service.items[created.ID]
		item.Status = status
		service.items[created.ID] = item
		service.mu.Unlock()
		if _, err := service.Update(context.Background(), 7, created.ID, UpdateInput{OfferDescription: &description}); !errors.Is(err, ErrConflict) {
			t.Fatalf("%s Update() error = %v, want ErrConflict", status, err)
		}
	}
}

func TestMemoryServiceUpdatesOfferTitle(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.items[created.ID] = Item{ID: created.ID, UserID: 7, OfferTitle: "Велосипед", Status: "MATCHING"}

	title := "Самокат"
	updated, err := service.Update(context.Background(), 7, created.ID, UpdateInput{OfferTitle: &title})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.OfferTitle != title {
		t.Fatalf("updated OfferTitle = %q, want %q", updated.OfferTitle, title)
	}
	if updated.Status != "ANALYZING" {
		t.Fatalf("updated status = %q, want ANALYZING", updated.Status)
	}
}

func TestMemoryServiceRejectsBlankOfferTitle(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	title := ""
	_, err = service.Update(context.Background(), 7, created.ID, UpdateInput{OfferTitle: &title})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Update() error = %v, want ValidationError", err)
	}
	if validationError.Fields["offerTitle"] == "" {
		t.Fatalf("expected validation error for offerTitle, got fields = %v", validationError.Fields)
	}
}

func TestMemoryServiceRejectsLongOfferTitle(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	title := strings.Repeat("д", 256)
	_, err = service.Update(context.Background(), 7, created.ID, UpdateInput{OfferTitle: &title})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Update() error = %v, want ValidationError", err)
	}
	if validationError.Fields["offerTitle"] == "" {
		t.Fatalf("expected validation error for offerTitle, got fields = %v", validationError.Fields)
	}
}

func TestMemoryServiceNoOpOnSameOfferTitle(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.items[created.ID] = Item{ID: created.ID, UserID: 7, OfferTitle: "Велосипед", Status: "MATCHING"}

	title := "Велосипед"
	updated, err := service.Update(context.Background(), 7, created.ID, UpdateInput{OfferTitle: &title})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != "MATCHING" {
		t.Fatalf("no-op update changed status to %q, want MATCHING", updated.Status)
	}
}

func TestMemoryServiceForeignOwnerTitleUpdate(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	title := "Чужой"
	_, err = service.Update(context.Background(), 8, created.ID, UpdateInput{OfferTitle: &title})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update() error = %v, want ErrForbidden", err)
	}
}

func TestMemoryServiceLockedItemTitleUpdate(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.mu.Lock()
	item := service.items[created.ID]
	item.Status = "LOCKED"
	service.items[created.ID] = item
	service.mu.Unlock()
	title := "Самокат"
	_, err = service.Update(context.Background(), 7, created.ID, UpdateInput{OfferTitle: &title})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Update() error = %v, want ErrConflict", err)
	}
}

func TestMemoryServiceResolvesRequiredCategories(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Самокат",
		OfferDescription: "Для города",
		Wishes:           []string{"Что-то для дома"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.mu.Lock()
	item := service.items[created.ID]
	item.Status = "ACTION_REQUIRED"
	service.items[created.ID] = item
	service.mu.Unlock()

	resolved, err := service.ResolveCategories(context.Background(), 7, created.ID, CategoryDecisionInput{
		Wishes: []WishCategoryDecision{{
			WishID:     item.Wishes[0].ID,
			CategoryID: 3,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveCategories() error = %v", err)
	}
	if resolved.Status != "MATCHING" || resolved.OfferCategoryID == nil || *resolved.OfferCategoryID != 4 {
		t.Fatalf("resolved item = %+v", resolved)
	}
	if resolved.Wishes[0].CategoryID == nil || *resolved.Wishes[0].CategoryID != 3 {
		t.Fatalf("resolved wish = %+v", resolved.Wishes[0])
	}
}

func TestMemoryServiceRequiresEveryUnresolvedWishDecision(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Самокат",
		OfferDescription: "Для города",
		Wishes:           []string{"Телефон"},
		OfferCategoryID:  int32PointerForTest(4),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.mu.Lock()
	item := service.items[created.ID]
	item.Status = "ACTION_REQUIRED"
	service.items[created.ID] = item
	service.mu.Unlock()

	_, err = service.ResolveCategories(context.Background(), 7, created.ID, CategoryDecisionInput{})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("ResolveCategories() error = %v, want ValidationError", err)
	}
}

func int32PointerForTest(value int32) *int32 {
	return &value
}
