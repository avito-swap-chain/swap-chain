// Package users implements demo-user registration and lookup.
package users

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrNotFound indicates that no user matches the lookup.
	ErrNotFound = errors.New("user not found")
	// ErrPhoneExists indicates that the canonical phone is already registered.
	ErrPhoneExists = errors.New("phone is already registered")
)

const (
	RoleUser  = "USER"
	RoleAdmin = "ADMIN"
)

// ValidationError contains field-level registration or login errors.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "user validation failed"
}

// User is the transport-independent user representation.
type User struct {
	ID        int64
	Username  string
	Phone     string
	Role      string
	AvatarURL string
	CreatedAt time.Time
}

// CreateInput contains user-provided registration fields.
type CreateInput struct {
	Username string
	Phone    string
}

// UpdateInput contains fields the user is allowed to modify on their own profile.
// Nil pointers are not updated; empty strings are stored as-is (used to clear avatarUrl).
type UpdateInput struct {
	Username  *string
	AvatarURL *string
}

// Service defines user operations required by the HTTP handler.
type Service interface {
	Create(ctx context.Context, input CreateInput) (User, error)
	Get(ctx context.Context, userID int64) (User, error)
	FindByPhone(ctx context.Context, phone string) (User, error)
	Update(ctx context.Context, userID int64, input UpdateInput) (User, error)
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	input.Username = strings.TrimSpace(input.Username)
	phone, err := NormalizePhone(input.Phone)
	if err != nil {
		return CreateInput{}, err
	}
	input.Phone = phone

	fields := make(map[string]string)
	usernameLength := len([]rune(input.Username))
	if usernameLength == 0 {
		fields["username"] = "must not be blank"
	} else if usernameLength > 100 {
		fields["username"] = "must contain at most 100 characters"
	}
	if len(fields) > 0 {
		return CreateInput{}, &ValidationError{Fields: fields}
	}
	return input, nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	fields := make(map[string]string)
	if input.Username != nil {
		trimmed := strings.TrimSpace(*input.Username)
		input.Username = &trimmed
		usernameLength := len([]rune(*input.Username))
		if usernameLength == 0 {
			fields["username"] = "must not be blank"
		} else if usernameLength > 100 {
			fields["username"] = "must contain at most 100 characters"
		}
	}
	if input.AvatarURL != nil {
		trimmed := strings.TrimSpace(*input.AvatarURL)
		input.AvatarURL = &trimmed
	}
	if len(fields) > 0 {
		return UpdateInput{}, &ValidationError{Fields: fields}
	}
	return input, nil
}

// NormalizePhone converts common Russian formatting to a stable E.164-like value.
func NormalizePhone(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	digits := make([]byte, 0, len(raw))
	for index, char := range raw {
		switch {
		case char >= '0' && char <= '9':
			digits = append(digits, byte(char))
		case char == '+' && index == 0:
		case char == ' ' || char == '-' || char == '(' || char == ')':
		default:
			return "", invalidPhone()
		}
	}

	if len(digits) == 10 {
		digits = append([]byte{'7'}, digits...)
	} else if len(digits) == 11 && digits[0] == '8' {
		digits[0] = '7'
	}
	if len(digits) < 10 || len(digits) > 15 || digits[0] == '0' {
		return "", invalidPhone()
	}
	return "+" + string(digits), nil
}

func invalidPhone() error {
	return &ValidationError{Fields: map[string]string{
		"phone": "must contain a valid phone number",
	}}
}
