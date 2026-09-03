// Package service contains the business logic of the URL shortener service:
// short identifier generation, creating and looking up links, and
// asynchronous deletion of a user's links.
package service

//go:generate go run github.com/dmitrymack/go-url-shortener.git/cmd/reset github.com/dmitrymack/go-url-shortener.git/...

import (
	"context"
	"errors"
	"math/rand"
	"net/url"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"go.uber.org/zap"
)

// URLStorage is the storage interface used by ShortenService. It is
// satisfied by storage.Storage (in-memory), storage.FileStorage, and
// storage.Postgres.
type URLStorage interface {
	Get(key string) (string, error)
	Set(ctx context.Context, key string, value string, userID string) (string, error)
	SetBatch(ctx context.Context, batchItems []storage.URLRecord, userID string) error
	SetDeletedBatch(ctx context.Context, keys []string, userID string) error

	GetUrlsByUser(userID string) ([]storage.URLRecord, error)
}

// DeleteTask is a job for asynchronously deleting a user's links, submitted
// to the queue via ShortenService.EnqueueDelete.
//
// generate:reset
type DeleteTask struct {
	UserID string
	IDs    []string
}

// ShortenService implements creating, looking up, and deleting short links
// on top of an arbitrary URLStorage.
type ShortenService struct {
	storage     URLStorage
	baseURL     string
	deleteQueue chan DeleteTask
	logger      *zap.SugaredLogger
}

// NewShortenService creates a ShortenService over storage s. baseURL is the
// prefix prepended to a generated identifier when forming a short link.
// logger is used by StartDeleteWorker to report deletion errors.
func NewShortenService(s URLStorage, baseURL string, logger *zap.Logger) *ShortenService {
	return &ShortenService{
		storage:     s,
		baseURL:     baseURL,
		deleteQueue: make(chan DeleteTask, 100),
		logger:      logger.Sugar(),
	}
}

// GetOriginalURL returns the original URL stored under the short
// identifier id.
func (s *ShortenService) GetOriginalURL(id string) (string, error) {
	return s.storage.Get(id)
}

// GetUrlsByUser returns all links created by userID, with fully formed
// short URLs (baseURL + identifier).
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

// CreateShortURL creates a short link for originURL, retrying identifier
// generation on a key collision. If the URL was already shortened before,
// it returns the existing short link and storage.ErrDuplicateOriginalURL.
func (s *ShortenService) CreateShortURL(ctx context.Context, originURL string) (string, error) {
	userID := ctx.Value(contextkeys.UserIDContextKey).(string)

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

// CreateBatchShortURL creates short links for the set of originURLs with a
// single batch call to the storage and returns the corresponding short URLs
// in the same order as the input originURLs.
func (s *ShortenService) CreateBatchShortURL(ctx context.Context, originURLs []string) ([]string, error) {
	userID := ctx.Value(contextkeys.UserIDContextKey).(string)
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

// SetDeletedBatch synchronously marks links keys, owned by userID, as
// deleted.
func (s *ShortenService) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	return s.storage.SetDeletedBatch(ctx, keys, userID)
}

// StartDeleteWorker starts a background goroutine that drains the queue of
// deletion jobs submitted via EnqueueDelete and marks the corresponding
// links as deleted in the storage.
func (s *ShortenService) StartDeleteWorker() {
	go func() {
		for task := range s.deleteQueue {
			if err := s.storage.SetDeletedBatch(context.Background(), task.IDs, task.UserID); err != nil {
				s.logger.Errorln("delete batch failed", "error", err)
			}
		}
	}()
}

// generateID returns a random alphanumeric identifier of fixed length for
// use as a short URL.
func generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	b := make([]byte, length)

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

// EnqueueDelete puts task on the asynchronous deletion queue, drained by the
// worker started via StartDeleteWorker.
func (s *ShortenService) EnqueueDelete(task DeleteTask) {
	s.deleteQueue <- task
}
