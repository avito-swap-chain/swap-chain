// Package media validates and stores images used by exchange items.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

var (
	// ErrEmpty indicates that the uploaded file has no bytes.
	ErrEmpty = errors.New("image is empty")
	// ErrNotFound indicates that the requested image key does not exist.
	ErrNotFound = errors.New("image not found")
	// ErrTooLarge indicates that the uploaded image exceeds the configured limit.
	ErrTooLarge = errors.New("image is too large")
	// ErrUnsupportedType indicates that the uploaded bytes are not a supported image.
	ErrUnsupportedType = errors.New("unsupported image type")
)

var supportedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var publicURLPattern = regexp.MustCompile(`^/api/v1/media/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)

// PublicURL returns the stable API-relative URL for an object key.
func PublicURL(key string) string {
	return "/api/v1/media/" + key
}

// IsPublicURL reports whether value is a backend-issued media URL.
func IsPublicURL(value string) bool {
	return publicURLPattern.MatchString(value)
}

// Object is a stored image and its public metadata.
type Object struct {
	Key         string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

// Storage is the object-store boundary used by Service.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (Object, error)
}

// Service validates media and stores it under generated immutable keys.
type Service struct {
	storage  Storage
	maxBytes int64
}

// NewService creates a media service with a per-image byte limit.
func NewService(storage Storage, maxBytes int64) *Service {
	return &Service{storage: storage, maxBytes: maxBytes}
}

// Upload validates and stores one JPEG, PNG, or WebP image.
func (s *Service) Upload(ctx context.Context, input io.Reader) (Object, error) {
	data, err := io.ReadAll(io.LimitReader(input, s.maxBytes+1))
	if err != nil {
		return Object{}, fmt.Errorf("read image: %w", err)
	}
	if len(data) == 0 {
		return Object{}, ErrEmpty
	}
	if int64(len(data)) > s.maxBytes {
		return Object{}, ErrTooLarge
	}

	contentType := http.DetectContentType(data)
	extension, supported := supportedTypes[contentType]
	if !supported {
		return Object{}, ErrUnsupportedType
	}

	key := uuid.NewString() + extension
	if err := s.storage.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return Object{}, fmt.Errorf("store image: %w", err)
	}

	return Object{Key: key, ContentType: contentType, Size: int64(len(data))}, nil
}

// Open returns a stored image stream and metadata.
func (s *Service) Open(ctx context.Context, key string) (Object, error) {
	return s.storage.Open(ctx, key)
}
