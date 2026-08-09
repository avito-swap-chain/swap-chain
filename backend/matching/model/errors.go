package model

import (
	"errors"
	"fmt"
)

var (
	ErrVertexAlreadyExists = errors.New("vertex already exists")
	ErrVertexNotFound      = errors.New("vertex not found")
	ErrEdgeAlreadyExists   = errors.New("edge already exists")
	ErrEdgeNotFound        = errors.New("edge not found")
	ErrItemNotFound        = errors.New("item not found")
	ErrItemNotMatchable    = errors.New("item is not in matching status")
)

type ItemStatus string

const (
	ItemStatusUnknown  ItemStatus = ""
	ItemStatusMatching ItemStatus = "MATCHING"
)

// ItemStatusError сообщает, что вещь существует, но её статус не позволяет
// использовать её как источник для поиска цепочек.
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
	return ErrItemNotMatchable
}
