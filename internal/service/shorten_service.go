package service

import (
	"math/rand"

	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
)

type ShortenService struct {
	storage storage.URLStorage
	baseURL string
}

func NewShortenService(s storage.URLStorage, baseURL string) *ShortenService {
	return &ShortenService{
		storage: s,
		baseURL: baseURL,
	}
}

func (s *ShortenService) GetOriginalURL(id string) (string, bool) {
	return s.storage.Get(id)
}

func (s *ShortenService) CreateShortURL(originURL string) string {
	for {
		id := generateID()

		if _, exists := s.storage.Get(id); !exists {
			s.storage.Set(id, originURL)
			return s.baseURL + "/" + id
		}
	}
}

func generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	b := make([]byte, length)

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}
