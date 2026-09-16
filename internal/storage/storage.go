// Package storage contains the short link storage implementations:
// in-memory (Storage), file-based (FileStorage), and PostgreSQL (Postgres).
package storage

//go:generate go run github.com/dmitrymack/go-url-shortener.git/cmd/reset github.com/dmitrymack/go-url-shortener.git/...

import (
	"context"
	"errors"
	"sync"
)

// ErrDuplicateKey is returned when trying to save a value under a short
// identifier that is already taken.
var ErrDuplicateKey = errors.New("duplicate key")

// ErrNotFound is returned when no original URL is found for a short
// identifier.
var ErrNotFound = errors.New("not found")

// ErrDeleted is returned when the link found by identifier is marked as
// deleted.
var ErrDeleted = errors.New("deleted")

// Storage is a simple thread-safe in-memory short link store. Data is not
// persisted across process restarts.
type Storage struct {
	data map[string]string
	mu   sync.RWMutex
}

// NewStorage creates an empty in-memory store.
func NewStorage() *Storage {
	return &Storage{data: make(map[string]string)}
}

// Get returns the original URL stored under key.
func (s *Storage) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

// Set stores value under key. If the key is already taken, it returns
// ErrDuplicateKey and does not overwrite the existing value.
func (s *Storage) Set(ctx context.Context, key string, value string, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		return key, ErrDuplicateKey
	}

	s.data[key] = value
	return key, nil
}

// SetBatch sequentially stores all of batchItems.
func (s *Storage) SetBatch(ctx context.Context, batchItems []URLRecord, userID string) error {
	for _, item := range batchItems {
		if _, err := s.Set(ctx, item.ID, item.OriginURL, userID); err != nil {
			return err
		}
	}

	return nil
}

// GetUrlsByUser is not implemented for the in-memory store, since it does
// not track link ownership.
func (s *Storage) GetUrlsByUser(userID string) ([]URLRecord, error) {
	return nil, errors.New("not implemented")
}

// SetDeletedBatch removes the records for keys.
func (s *Storage) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.data, key)
	}

	return nil
}
