package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUploadStoresDetectedImage(t *testing.T) {
	storage := &fakeStorage{}
	service := NewService(storage, 1024)

	stored, err := service.Upload(context.Background(), bytes.NewReader(tinyPNG()))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if stored.ContentType != "image/png" || stored.Size != int64(len(tinyPNG())) || !strings.HasSuffix(stored.Key, ".png") {
		t.Fatalf("Upload() = %+v", stored)
	}
	if storage.contentType != "image/png" || !bytes.Equal(storage.data, tinyPNG()) {
		t.Fatalf("stored object = type %q bytes %x", storage.contentType, storage.data)
	}
}

func TestUploadRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		max   int64
		want  error
	}{
		{name: "empty", max: 10, want: ErrEmpty},
		{name: "too large", input: tinyPNG(), max: 4, want: ErrTooLarge},
		{name: "unsupported", input: []byte("plain text"), max: 100, want: ErrUnsupportedType},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(&fakeStorage{}, test.max).Upload(context.Background(), bytes.NewReader(test.input))
			if !errors.Is(err, test.want) {
				t.Fatalf("Upload() error = %v, want %v", err, test.want)
			}
		})
	}
}

type fakeStorage struct {
	data        []byte
	contentType string
}

func (s *fakeStorage) Put(_ context.Context, _ string, body io.Reader, _ int64, contentType string) error {
	s.data, _ = io.ReadAll(body)
	s.contentType = contentType
	return nil
}

func (s *fakeStorage) Open(_ context.Context, _ string) (Object, error) {
	return Object{}, ErrNotFound
}

func tinyPNG() []byte {
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52}
}
