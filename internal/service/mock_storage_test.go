package service

import (
	"context"

	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
)

// mockStorage is a URLStorage test double: each method delegates to the
// corresponding func field. It exists for the handful of behaviors that
// the real in-memory storage.Storage can't be made to exhibit on demand
// (forced ID collisions, storage.ErrDuplicateOriginalURL, arbitrary storage
// errors) — everywhere else tests use storage.NewStorage() directly.
// Leaving a field nil that a test path reaches panics loudly rather than
// silently doing the wrong thing.
type mockStorage struct {
	GetFn             func(key string) (string, error)
	SetFn             func(ctx context.Context, key, value, userID string) (string, error)
	SetBatchFn        func(ctx context.Context, batchItems []storage.URLRecord, userID string) error
	SetDeletedBatchFn func(ctx context.Context, keys []string, userID string) error
	GetUrlsByUserFn   func(userID string) ([]storage.URLRecord, error)
}

func (m *mockStorage) Get(key string) (string, error) {
	return m.GetFn(key)
}

func (m *mockStorage) Set(ctx context.Context, key, value, userID string) (string, error) {
	return m.SetFn(ctx, key, value, userID)
}

func (m *mockStorage) SetBatch(ctx context.Context, batchItems []storage.URLRecord, userID string) error {
	return m.SetBatchFn(ctx, batchItems, userID)
}

func (m *mockStorage) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	return m.SetDeletedBatchFn(ctx, keys, userID)
}

func (m *mockStorage) GetUrlsByUser(userID string) ([]storage.URLRecord, error) {
	return m.GetUrlsByUserFn(userID)
}
