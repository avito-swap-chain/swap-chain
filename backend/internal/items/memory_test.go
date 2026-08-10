package items

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryServiceWithdrawsItemWithoutDeletingIt(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		WantDescription:  "Телефон",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := service.Update(context.Background(), 7, created.ID, UpdateInput{Withdraw: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != "WITHDRAWN" || updated.WantDescription != "" {
		t.Fatalf("withdrawn item = %+v, want WITHDRAWN with empty wish", updated)
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
		WantDescription:  "Телефон",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.items[created.ID] = Item{ID: created.ID, UserID: 7, Status: "WITHDRAWN"}

	want := "Игровой телефон"
	updated, err := service.Update(context.Background(), 7, created.ID, UpdateInput{WantDescription: &want})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != "ANALYZING" || updated.WantDescription != want {
		t.Fatalf("updated item = %+v, want ANALYZING with new wish", updated)
	}
}

func TestMemoryServiceRefusesLockedOrForeignItemUpdate(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Горный велосипед",
		WantDescription:  "Телефон",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	description := "Новое описание"
	if _, err := service.Update(context.Background(), 8, created.ID, UpdateInput{OfferDescription: &description}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign Update() error = %v, want ErrForbidden", err)
	}
	service.mu.Lock()
	item := service.items[created.ID]
	item.Status = "LOCKED"
	service.items[created.ID] = item
	service.mu.Unlock()
	if _, err := service.Update(context.Background(), 7, created.ID, UpdateInput{OfferDescription: &description}); !errors.Is(err, ErrConflict) {
		t.Fatalf("locked Update() error = %v, want ErrConflict", err)
	}
}
