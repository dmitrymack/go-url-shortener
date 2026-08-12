package service

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net/url"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
)

type URLStorage interface {
	Get(key string) (string, error)
	Set(ctx context.Context, key string, value string, userID string) (string, error)
	SetBatch(ctx context.Context, batchItems []storage.URLRecord, userID string) error
	SetDeletedBatch(ctx context.Context, keys []string, userID string) error

	GetUrlsByUser(userID string) ([]storage.URLRecord, error)
}

type DeleteTask struct {
	UserID string
	IDs    []string
}

type ShortenService struct {
	storage     URLStorage
	baseURL     string
	deleteQueue chan DeleteTask
}

func NewShortenService(s URLStorage, baseURL string) *ShortenService {
	return &ShortenService{
		storage:     s,
		baseURL:     baseURL,
		deleteQueue: make(chan DeleteTask, 100),
	}
}

func (s *ShortenService) GetOriginalURL(id string) (string, error) {
	return s.storage.Get(id)
}

func (s *ShortenService) GetUrlsByUser(userID string) ([]storage.URLRecord, error) {
	urls, err := s.storage.GetUrlsByUser(userID)
	if err != nil {
		return nil, err
	}

	for i := range urls {
		urls[i].ID, err = url.JoinPath(
			s.baseURL,
			urls[i].ID,
		)

		if err != nil {
			return nil, err
		}
	}

	return urls, nil
}

func (s *ShortenService) CreateShortURL(ctx context.Context, originURL string) (string, error) {
	userID := ctx.Value(contextKeys.UserIDContextKey).(string)

	for {
		id := generateID()

		id, err := s.storage.Set(ctx, id, originURL, userID)
		if errors.Is(err, storage.ErrDuplicateKey) {
			continue
		}

		if errors.Is(err, storage.ErrDuplicateOriginalURL) {
			res, err := url.JoinPath(s.baseURL, id)
			if err != nil {
				return "", err
			}

			return res, storage.ErrDuplicateOriginalURL
		}

		if err != nil {
			return "", err
		}

		res, err := url.JoinPath(s.baseURL, id)
		if err != nil {
			return "", err
		}

		return res, nil
	}
}

func (s *ShortenService) CreateBatchShortURL(ctx context.Context, originURLs []string) ([]string, error) {
	userID := ctx.Value(contextKeys.UserIDContextKey).(string)
	items := make([]storage.URLRecord, 0, len(originURLs))
	res := make([]string, 0, len(originURLs))

	for _, originURL := range originURLs {
		id := generateID()

		items = append(items, storage.URLRecord{
			ID:        id,
			OriginURL: originURL,
		})

		shortURL, err := url.JoinPath(s.baseURL, id)
		if err != nil {
			return make([]string, 0), err
		}

		res = append(res, shortURL)
	}

	err := s.storage.SetBatch(ctx, items, userID)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *ShortenService) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	return s.storage.SetDeletedBatch(ctx, keys, userID)
}

func (s *ShortenService) StartDeleteWorker() {
	go func() {
		for task := range s.deleteQueue {
			if err := s.storage.SetDeletedBatch(context.Background(), task.IDs, task.UserID); err != nil {
				log.Println(err)
			}
		}
	}()
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

func (s *ShortenService) EnqueueDelete(task DeleteTask) {
	s.deleteQueue <- task
}
