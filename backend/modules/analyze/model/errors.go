package model

import (
	"errors"
	"fmt"
)

var (
	ErrItemNotFound          = errors.New("item not found")
	ErrItemNotAnalyzable     = errors.New("item is not in analyzing status")
	ErrAnalysisStateChanged  = errors.New("item is no longer analyzing")
	ErrOfferCategoryRequired = errors.New("manual offer category is required")
	ErrEmptyChoices          = errors.New("empty choices")
)

type ItemStatus string

const (
	ItemStatusUnknown        ItemStatus = ""
	ItemStatusAnalyzing      ItemStatus = "ANALYZING"
	ItemStatusActionRequired ItemStatus = "ACTION_REQUIRED"
	ItemStatusMatching       ItemStatus = "MATCHING"
	ItemStatusLocked         ItemStatus = "LOCKED"
)

// ItemStatusError сообщает, что вещь существует, но её статус не позволяет
// запустить анализ. Actual позволяет отличить уже завершённый анализ от
// временно заблокированной вещи без разбора текста ошибки.
type ItemStatusError struct {
	ItemID   int64
	Expected ItemStatus
	Actual   ItemStatus
}

func (e *ItemStatusError) Error() string {
	actual := string(e.Actual)
	if e.Actual == ItemStatusUnknown {
		actual = "UNKNOWN"
	}

	return fmt.Sprintf("item %d has status %s, expected %s", e.ItemID, actual, e.Expected)
}

func (e *ItemStatusError) Unwrap() error {
	return ErrItemNotAnalyzable
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gigachat api error: status=%d, message=%s", e.StatusCode, e.Message)
}
