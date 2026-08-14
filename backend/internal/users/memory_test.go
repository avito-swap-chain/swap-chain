package users

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryServiceUserLifecycle(t *testing.T) {
	service := NewMemoryService()
	created, err := service.Create(context.Background(), CreateInput{Username: "  Никита  ", Phone: "8 (999) 123-45-67"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Username != "Никита" || created.Phone != "+79991234567" || created.Role != RoleUser {
		t.Fatalf("created user = %+v", created)
	}

	byPhone, err := service.FindByPhone(context.Background(), "+7 999 123-45-67")
	if err != nil || byPhone.ID != created.ID {
		t.Fatalf("FindByPhone() = %+v, %v", byPhone, err)
	}

	username, avatar := "  Николай ", " /api/v1/media/avatar.png "
	updated, err := service.Update(context.Background(), created.ID, UpdateInput{Username: &username, AvatarURL: &avatar})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Username != "Николай" || updated.AvatarURL != "/api/v1/media/avatar.png" {
		t.Fatalf("updated user = %+v", updated)
	}
}

func TestMemoryServiceRejectsDuplicateAndInvalidChanges(t *testing.T) {
	service := NewMemoryService()
	if _, err := service.Create(context.Background(), CreateInput{Username: "Первый", Phone: "+79991234567"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), CreateInput{Username: "Второй", Phone: "89991234567"}); !errors.Is(err, ErrPhoneExists) {
		t.Fatalf("duplicate Create() error = %v", err)
	}

	blank := "   "
	var validationError *ValidationError
	if _, err := service.Update(context.Background(), 1, UpdateInput{Username: &blank}); !errors.As(err, &validationError) {
		t.Fatalf("blank Update() error = %v", err)
	}
	if _, err := service.Get(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
}
