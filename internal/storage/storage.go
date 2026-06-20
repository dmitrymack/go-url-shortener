package storage

import (
	"context"
	"errors"
	"sync"
)

var ErrDuplicateKey = errors.New("duplicate key")
var ErrNotFound = errors.New("not found")
var ErrDeleted = errors.New("deleted")

type Storage struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewStorage() *Storage {
	return &Storage{data: make(map[string]string)}
}

func (s *Storage) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *Storage) Set(ctx context.Context, key string, value string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		return key, ErrDuplicateKey
	}

	s.data[key] = value
	return key, nil
}

func (s *Storage) SetBatch(ctx context.Context, batchItems []URLRecord) error {
	for _, item := range batchItems {
		if _, err := s.Set(ctx, item.ID, item.OriginURL); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) GetUrlsByUser(userID string) ([]URLRecord, error) {
	return nil, errors.New("not implemented")
}

func (s *Storage) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.data, key)
	}

	return nil
}
