package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const testBaseURL = "http://localhost:8080"

func userContext(userID string) context.Context {
	return context.WithValue(context.Background(), contextkeys.UserIDContextKey, userID)
}

func TestCreateShortURL_Success(t *testing.T) {
	store := storage.NewStorage()
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	shortURL, err := s.CreateShortURL(userContext("user1"), "https://example.com")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(shortURL, testBaseURL+"/"))

	id := strings.TrimPrefix(shortURL, testBaseURL+"/")
	assert.NotEmpty(t, id)

	value, err := s.GetOriginalURL(id)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value)
}

func TestCreateShortURL_RetriesOnKeyCollision(t *testing.T) {
	calls := 0
	store := &mockStorage{
		SetFn: func(ctx context.Context, key, value, userID string) (string, error) {
			calls++
			if calls == 1 {
				return "", storage.ErrDuplicateKey
			}
			return key, nil
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	shortURL, err := s.CreateShortURL(userContext("user1"), "https://example.com")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(shortURL, testBaseURL+"/"))
	assert.Equal(t, 2, calls, "Set should have been retried once after the collision")
}

func TestCreateShortURL_DuplicateOriginalURL(t *testing.T) {
	store := &mockStorage{
		SetFn: func(ctx context.Context, key, value, userID string) (string, error) {
			return "existingID", storage.ErrDuplicateOriginalURL
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	shortURL, err := s.CreateShortURL(userContext("user1"), "https://example.com")

	assert.ErrorIs(t, err, storage.ErrDuplicateOriginalURL)
	assert.Equal(t, testBaseURL+"/existingID", shortURL)
}

func TestCreateShortURL_StorageError(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	store := &mockStorage{
		SetFn: func(ctx context.Context, key, value, userID string) (string, error) {
			return "", wantErr
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	shortURL, err := s.CreateShortURL(userContext("user1"), "https://example.com")

	assert.ErrorIs(t, err, wantErr)
	assert.Empty(t, shortURL)
}

func TestGetOriginalURL(t *testing.T) {
	store := storage.NewStorage()
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	_, err := store.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)

	value, err := s.GetOriginalURL("abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value)

	_, err = s.GetOriginalURL("unknown")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetUrlsByUser_JoinsBaseURL(t *testing.T) {
	store := &mockStorage{
		GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
			return []storage.URLRecord{
				{ID: "abc123", OriginURL: "https://example.com"},
				{ID: "def456", OriginURL: "https://example.org"},
			}, nil
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	urls, err := s.GetUrlsByUser("user1")

	require.NoError(t, err)
	require.Len(t, urls, 2)
	assert.Equal(t, testBaseURL+"/abc123", urls[0].ID)
	assert.Equal(t, "https://example.com", urls[0].OriginURL)
	assert.Equal(t, testBaseURL+"/def456", urls[1].ID)
}

func TestGetUrlsByUser_StorageError(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	store := &mockStorage{
		GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
			return nil, wantErr
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	urls, err := s.GetUrlsByUser("user1")

	assert.ErrorIs(t, err, wantErr)
	assert.Nil(t, urls)
}

func TestCreateBatchShortURL_Success(t *testing.T) {
	store := storage.NewStorage()
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	origins := []string{"https://example.com", "https://example.org", "https://example.net"}
	shortURLs, err := s.CreateBatchShortURL(userContext("user1"), origins)

	require.NoError(t, err)
	require.Len(t, shortURLs, len(origins))

	for i, shortURL := range shortURLs {
		assert.True(t, strings.HasPrefix(shortURL, testBaseURL+"/"))

		id := strings.TrimPrefix(shortURL, testBaseURL+"/")
		value, err := s.GetOriginalURL(id)
		require.NoError(t, err)
		assert.Equal(t, origins[i], value)
	}
}

func TestCreateBatchShortURL_StorageError(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	store := &mockStorage{
		SetBatchFn: func(ctx context.Context, batchItems []storage.URLRecord, userID string) error {
			return wantErr
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	shortURLs, err := s.CreateBatchShortURL(userContext("user1"), []string{"https://example.com"})

	assert.ErrorIs(t, err, wantErr)
	assert.Nil(t, shortURLs)
}

func TestSetDeletedBatch(t *testing.T) {
	store := storage.NewStorage()
	s := NewShortenService(store, testBaseURL, zap.NewNop())

	_, err := store.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)

	err = s.SetDeletedBatch(context.Background(), []string{"abc123"}, "user1")
	require.NoError(t, err)

	_, err = s.GetOriginalURL("abc123")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestStartDeleteWorker_ProcessesQueuedTasks(t *testing.T) {
	type call struct {
		keys   []string
		userID string
	}
	calls := make(chan call, 1)
	store := &mockStorage{
		SetDeletedBatchFn: func(ctx context.Context, keys []string, userID string) error {
			calls <- call{keys: keys, userID: userID}
			return nil
		},
	}
	s := NewShortenService(store, testBaseURL, zap.NewNop())
	s.StartDeleteWorker()

	s.EnqueueDelete(DeleteTask{UserID: "user1", IDs: []string{"abc123", "def456"}})

	select {
	case got := <-calls:
		assert.Equal(t, "user1", got.userID)
		assert.Equal(t, []string{"abc123", "def456"}, got.keys)
	case <-time.After(time.Second):
		t.Fatal("delete worker did not process the queued task in time")
	}
}

func TestStartDeleteWorker_LogsStorageError(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	done := make(chan struct{}, 1)
	store := &mockStorage{
		SetDeletedBatchFn: func(ctx context.Context, keys []string, userID string) error {
			defer close(done)
			return wantErr
		},
	}

	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	s := NewShortenService(store, testBaseURL, logger)
	s.StartDeleteWorker()

	s.EnqueueDelete(DeleteTask{UserID: "user1", IDs: []string{"abc123"}})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete worker did not process the queued task in time")
	}

	require.Eventually(t, func() bool {
		return logs.Len() > 0
	}, time.Second, time.Millisecond, "expected an error to be logged")
}
