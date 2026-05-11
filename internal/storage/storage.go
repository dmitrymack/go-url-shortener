package storage

import "errors"

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

func (s *Storage) Set(key string, value string) error {
	if _, ok := s.data[key]; ok {
		return ErrDuplicateKey
	}

	s.data[key] = value
	return nil
}
