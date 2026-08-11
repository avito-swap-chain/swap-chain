package users

import (
	"context"
	"sync"
	"time"
)

// MemoryService is a lightweight adapter used by HTTP tests.
type MemoryService struct {
	mu      sync.RWMutex
	nextID  int64
	users   map[int64]User
	byPhone map[string]int64
}

// NewMemoryService creates an isolated user store and loads optional fixtures.
func NewMemoryService(fixtures ...User) *MemoryService {
	service := &MemoryService{
		nextID:  1,
		users:   make(map[int64]User),
		byPhone: make(map[string]int64),
	}
	for _, user := range fixtures {
		if user.Role == "" {
			user.Role = RoleUser
		}
		service.users[user.ID] = user
		service.byPhone[user.Phone] = user.ID
		if user.ID >= service.nextID {
			service.nextID = user.ID + 1
		}
	}
	return service
}

// Create validates and stores a user in memory.
func (s *MemoryService) Create(_ context.Context, input CreateInput) (User, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return User{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byPhone[normalized.Phone]; exists {
		return User{}, ErrPhoneExists
	}
	user := User{
		ID:        s.nextID,
		Username:  normalized.Username,
		Phone:     normalized.Phone,
		Role:      RoleUser,
		CreatedAt: time.Now().UTC(),
	}
	s.users[user.ID] = user
	s.byPhone[user.Phone] = user.ID
	s.nextID++
	return user, nil
}

// Get returns a user by ID.
func (s *MemoryService) Get(_ context.Context, userID int64) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, exists := s.users[userID]
	if !exists {
		return User{}, ErrNotFound
	}
	return user, nil
}

// FindByPhone returns a user by normalized phone.
func (s *MemoryService) FindByPhone(_ context.Context, phone string) (User, error) {
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID, exists := s.byPhone[normalized]
	if !exists {
		return User{}, ErrNotFound
	}
	return s.users[userID], nil
}

// Update validates and persists profile changes for the given user.
func (s *MemoryService) Update(_ context.Context, userID int64, input UpdateInput) (User, error) {
	normalized, err := normalizeUpdateInput(input)
	if err != nil {
		return User{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	user, exists := s.users[userID]
	if !exists {
		return User{}, ErrNotFound
	}
	if normalized.Username != nil {
		user.Username = *normalized.Username
	}
	if normalized.AvatarURL != nil {
		user.AvatarURL = *normalized.AvatarURL
	}
	s.users[userID] = user
	return user, nil
}
