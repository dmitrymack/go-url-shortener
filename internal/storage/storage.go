package storage

type URLStorage interface {
	Get(key string) (string, bool)
	Set(key string, value string)
}

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

func (s *Storage) Set(key string, value string) {
	s.data[key] = value
}
