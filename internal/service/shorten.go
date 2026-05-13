package service

import (
	"errors"
	"math/rand"
	"net/url"

	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
)

type URLStorage interface {
	Get(key string) (string, bool)
	Set(key string, value string) error
}

type ShortenService struct {
	storage  URLStorage
	producer *Producer
	baseURL  string
}

type ShortenResult struct {
	ID       string
	ShortURL string
}

func NewShortenService(s URLStorage, p *Producer, baseURL string) *ShortenService {
	return &ShortenService{
		storage:  s,
		producer: p,
		baseURL:  baseURL,
	}
}

func (s *ShortenService) GetOriginalURL(id string) (string, bool) {
	return s.storage.Get(id)
}

func (s *ShortenService) CreateShortURL(originURL string) (string, error) {
	for {
		id := generateID()

		err := s.storage.Set(id, originURL)
		if errors.Is(err, storage.ErrDuplicateKey) {
			continue
		}

		if err != nil {
			return "", err
		}

		res, err := url.JoinPath(s.baseURL, id)
		if err != nil {
			return "", err
		}

		err = s.producer.WriteEvent(&Event{
			ShortURL:    id,
			OriginalURL: originURL,
		})

		if err != nil {
			return "", err
		}

		return res, nil
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
