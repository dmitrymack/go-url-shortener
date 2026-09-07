package handler

import (
	"context"

	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
)

// mockURLStorage is a service.URLStorage test double: each method delegates
// to the corresponding func field. It exists for the handful of behaviors
// the real storage.Storage/FileStorage can't be made to exhibit on demand
// (GetUrlsByUser actually returning records — both real in-memory stores
// return "not implemented" — and arbitrary storage errors); everywhere else
// tests use storage.NewStorage() directly.
type mockURLStorage struct {
	GetFn             func(key string) (string, error)
	SetFn             func(ctx context.Context, key, value, userID string) (string, error)
	SetBatchFn        func(ctx context.Context, batchItems []storage.URLRecord, userID string) error
	SetDeletedBatchFn func(ctx context.Context, keys []string, userID string) error
	GetUrlsByUserFn   func(userID string) ([]storage.URLRecord, error)
}

func (m *mockURLStorage) Get(key string) (string, error) {
	return m.GetFn(key)
}

func (m *mockURLStorage) Set(ctx context.Context, key, value, userID string) (string, error) {
	return m.SetFn(ctx, key, value, userID)
}

func (m *mockURLStorage) SetBatch(ctx context.Context, batchItems []storage.URLRecord, userID string) error {
	return m.SetBatchFn(ctx, batchItems, userID)
}

func (m *mockURLStorage) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	return m.SetDeletedBatchFn(ctx, keys, userID)
}

func (m *mockURLStorage) GetUrlsByUser(userID string) ([]storage.URLRecord, error) {
	return m.GetUrlsByUserFn(userID)
}
