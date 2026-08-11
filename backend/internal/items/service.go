// Package items implements item validation and persistence workflows.
package items

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"swap-chain/internal/media"
)

var (
	// ErrNotFound indicates that an item ID is unknown to the current repository.
	ErrNotFound = errors.New("item not found")
	// ErrForbidden indicates an attempt to modify another user's item.
	ErrForbidden = errors.New("item forbidden")
	// ErrConflict indicates that an item is immutable in its current lifecycle state.
	ErrConflict = errors.New("item conflict")
)

// ValidationError contains field-level item validation errors.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "item validation failed"
}

// Item is the transport-independent item representation.
type Item struct {
	ID               int64
	UserID           int64
	OfferTitle       string
	OfferDescription string
	WantDescription  string
	ImageURLs        []string
	Status           string
	OfferCategoryID  *int32
	WantCategoryID   *int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CreateInput contains normalized user-provided item fields.
type CreateInput struct {
	OfferTitle       string
	OfferDescription string
	WantDescription  string
	ImageURLs        []string
}

// UpdateInput contains a partial item change. Withdraw removes the wish and
// makes the item unavailable for matching without deleting its history.
type UpdateInput struct {
	OfferTitle       *string
	OfferDescription *string
	WantDescription  *string
	Withdraw         bool
}

// Service defines item operations required by the HTTP handler.
type Service interface {
	Create(ctx context.Context, userID int64, input CreateInput) (Item, error)
	Update(ctx context.Context, userID, itemID int64, input UpdateInput) (Item, error)
	Get(ctx context.Context, itemID int64) (Item, error)
	ListByUser(ctx context.Context, userID, afterID int64, limit int) ([]Item, *int64, error)
}

func normalizeUpdate(input UpdateInput) UpdateInput {
	if input.OfferTitle != nil {
		value := strings.TrimSpace(*input.OfferTitle)
		input.OfferTitle = &value
	}
	if input.OfferDescription != nil {
		value := strings.TrimSpace(*input.OfferDescription)
		input.OfferDescription = &value
	}
	if input.WantDescription != nil {
		value := strings.TrimSpace(*input.WantDescription)
		input.WantDescription = &value
	}
	return input
}

func validateUpdate(input UpdateInput) error {
	fields := make(map[string]string)
	if input.OfferDescription == nil && input.WantDescription == nil && input.OfferTitle == nil && !input.Withdraw {
		fields["request"] = "must change a description, title or withdraw the item"
	}
	if input.Withdraw && input.WantDescription != nil {
		fields["wantDescription"] = "cannot be changed while withdrawing the item"
	}
	if input.OfferTitle != nil {
		validateText(fields, "offerTitle", *input.OfferTitle, 255)
	}
	if input.OfferDescription != nil {
		validateText(fields, "offerDescription", *input.OfferDescription, 4000)
	}
	if input.WantDescription != nil {
		validateText(fields, "wantDescription", *input.WantDescription, 4000)
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// ParseCursor parses the public pagination cursor.
func ParseCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return value, nil
}

// FormatCursor serializes an item ID as a public pagination cursor.
func FormatCursor(value int64) string {
	return strconv.FormatInt(value, 10)
}

func normalize(input CreateInput) CreateInput {
	input.OfferTitle = strings.TrimSpace(input.OfferTitle)
	input.OfferDescription = strings.TrimSpace(input.OfferDescription)
	input.WantDescription = strings.TrimSpace(input.WantDescription)
	if input.ImageURLs == nil {
		input.ImageURLs = []string{}
	}
	for i := range input.ImageURLs {
		input.ImageURLs[i] = strings.TrimSpace(input.ImageURLs[i])
	}
	return input
}

func validate(input CreateInput) error {
	fields := make(map[string]string)
	validateText(fields, "offerTitle", input.OfferTitle, 255)
	validateText(fields, "offerDescription", input.OfferDescription, 4000)
	validateText(fields, "wantDescription", input.WantDescription, 4000)

	if len(input.ImageURLs) > 10 {
		fields["imageUrls"] = "must contain at most 10 URLs"
	} else {
		for _, rawURL := range input.ImageURLs {
			if media.IsPublicURL(rawURL) {
				continue
			}
			parsed, err := url.ParseRequestURI(rawURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				fields["imageUrls"] = "must contain valid HTTP(S) URLs or API media paths"
				break
			}
		}
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validateText(fields map[string]string, name, value string, maxLength int) {
	length := len([]rune(value))
	if length == 0 {
		fields[name] = "must not be blank"
		return
	}
	if length > maxLength {
		fields[name] = fmt.Sprintf("must contain at most %d characters", maxLength)
	}
}
