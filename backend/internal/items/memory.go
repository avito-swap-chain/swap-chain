package items

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryService is a lightweight adapter used by HTTP tests.
type MemoryService struct {
	mu      sync.RWMutex
	nextID  int64
	items   map[int64]Item
	publish PublishEvent
}

// NewMemoryService creates an isolated item store.
func NewMemoryService(publish ...PublishEvent) *MemoryService {
	var publisher PublishEvent
	if len(publish) > 0 {
		publisher = publish[0]
	}
	return &MemoryService{nextID: 1, items: make(map[int64]Item), publish: publisher}
}

// Create validates and stores an item in memory.
func (s *MemoryService) Create(_ context.Context, userID int64, input CreateInput) (Item, error) {
	input = normalize(input)
	if err := validate(input); err != nil {
		return Item{}, err
	}

	s.mu.Lock()
	now := time.Now().UTC()
	item := Item{
		ID:               s.nextID,
		UserID:           userID,
		OfferTitle:       input.OfferTitle,
		OfferDescription: input.OfferDescription,
		WantDescription:  input.WantDescription,
		ImageURLs:        append([]string(nil), input.ImageURLs...),
		Status:           "ANALYZING",
		OfferCategoryID:  copyInt32Ptr(input.OfferCategoryID),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.items[item.ID] = item
	s.nextID++
	s.mu.Unlock()

	if s.publish != nil {
		s.publish(item.UserID, "item.created", FormatCursor(item.ID), map[string]any{"status": item.Status})
	}
	return clone(item), nil
}

// Update changes a user's item. A change to matching inputs restarts analysis;
// withdrawal keeps the item in history but removes it from matching.
func (s *MemoryService) Update(_ context.Context, userID, itemID int64, input UpdateInput) (Item, error) {
	input = normalizeUpdate(input)
	if err := validateUpdate(input); err != nil {
		return Item{}, err
	}

	s.mu.Lock()
	item, ok := s.items[itemID]
	if !ok {
		s.mu.Unlock()
		return Item{}, ErrNotFound
	}
	if item.UserID != userID {
		s.mu.Unlock()
		return Item{}, ErrForbidden
	}
	if item.Status == "LOCKED" {
		s.mu.Unlock()
		return Item{}, ErrConflict
	}
	oldTitle := item.OfferTitle
	if input.OfferTitle != nil {
		item.OfferTitle = *input.OfferTitle
	}
	if input.OfferDescription != nil {
		item.OfferDescription = *input.OfferDescription
	}
	if input.OfferCategoryID != nil {
		item.OfferCategoryID = input.OfferCategoryID
	}
	switch {
	case input.Withdraw:
		item.WantDescription = ""
		item.Status = "WITHDRAWN"
	case input.WantDescription != nil:
		item.WantDescription = *input.WantDescription
		item.Status = "ANALYZING"
	case input.OfferDescription != nil && item.Status != "WITHDRAWN":
		item.Status = "ANALYZING"
	case (input.OfferTitle != nil && item.OfferTitle != oldTitle || input.OfferCategoryID != nil) && item.Status != "WITHDRAWN":
		item.Status = "ANALYZING"
	}
	item.UpdatedAt = time.Now().UTC()
	s.items[itemID] = item
	s.mu.Unlock()

	if s.publish != nil {
		s.publish(item.UserID, "item.status.updated", FormatCursor(item.ID), map[string]any{"status": item.Status})
	}
	return clone(item), nil
}

// Get returns one in-memory item by ID.
func (s *MemoryService) Get(_ context.Context, itemID int64) (Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[itemID]
	if !ok {
		return Item{}, ErrNotFound
	}
	return clone(item), nil
}

// ListByUser returns a stable ID-ordered page owned by one user.
func (s *MemoryService) ListByUser(_ context.Context, userID, afterID int64, limit int) ([]Item, *int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]int64, 0, len(s.items))
	for id, item := range s.items {
		if item.UserID == userID && id > afterID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var next *int64
	if len(ids) > limit {
		value := ids[limit-1]
		next = &value
		ids = ids[:limit]
	}
	result := make([]Item, 0, len(ids))
	for _, id := range ids {
		result = append(result, clone(s.items[id]))
	}
	return result, next, nil
}

func clone(item Item) Item {
	item.ImageURLs = append([]string(nil), item.ImageURLs...)
	if item.OfferCategoryID != nil {
		id := *item.OfferCategoryID
		item.OfferCategoryID = &id
	}
	if item.WantCategoryID != nil {
		id := *item.WantCategoryID
		item.WantCategoryID = &id
	}
	return item
}

func copyInt32Ptr(src *int32) *int32 {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
