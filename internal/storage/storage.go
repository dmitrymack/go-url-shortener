package storage

import (
	"context"
	"errors"
)

var ErrDuplicateKey = errors.New("duplicate key")

type Storage struct {
	data map[string]string
}

func NewStorage() *Storage {
	return &Storage{data: make(map[string]string)}
}

func (s *Storage) Get(key string) (string, bool) {
	value, ok := s.data[key]
	return value, ok
}

func (s *Storage) Set(ctx context.Context, key string, value string) (string, error) {
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
