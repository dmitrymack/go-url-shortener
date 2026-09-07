package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSetBatchURL(t *testing.T) {
	t.Run("positive test", func(t *testing.T) {
		store := storage.NewStorage()
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		body := strings.NewReader(`[
			{"correlation_id":"1","original_url":"https://example.com/one"},
			{"correlation_id":"2","original_url":"https://example.com/two"}
		]`)
		r := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", body)
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "batch_user")
		w := httptest.NewRecorder()

		h.SetBatchURL(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusCreated, res.StatusCode)
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

		var resp []BatchResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&resp))
		require.Len(t, resp, 2)
		assert.Equal(t, "1", resp[0].CorrelationID)
		assert.Equal(t, "2", resp[1].CorrelationID)
	})

	t.Run("invalid json", func(t *testing.T) {
		store := storage.NewStorage()
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(`[{`))
		w := httptest.NewRecorder()

		h.SetBatchURL(w, r)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("storage error", func(t *testing.T) {
		store := &mockURLStorage{
			SetBatchFn: func(ctx context.Context, batchItems []storage.URLRecord, userID string) error {
				return errors.New("storage unavailable")
			},
		}
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		body := strings.NewReader(`[{"correlation_id":"1","original_url":"https://example.com"}]`)
		r := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", body)
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "batch_user")
		w := httptest.NewRecorder()

		h.SetBatchURL(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	})
}

func TestGetUserURLS(t *testing.T) {
	t.Run("positive test", func(t *testing.T) {
		store := &mockURLStorage{
			GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
				return []storage.URLRecord{{ID: "abc123", OriginURL: "https://example.com"}}, nil
			},
		}
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "urls_user")
		w := httptest.NewRecorder()

		h.GetUserURLS(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

		var resp []storage.URLRecord
		require.NoError(t, json.NewDecoder(res.Body).Decode(&resp))
		require.Len(t, resp, 1)
		assert.Equal(t, "http://localhost:8080/abc123", resp[0].ID)
	})

	t.Run("no urls", func(t *testing.T) {
		store := &mockURLStorage{
			GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
				return nil, nil
			},
		}
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "urls_user")
		w := httptest.NewRecorder()

		h.GetUserURLS(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusNoContent, res.StatusCode)
	})

	t.Run("storage error", func(t *testing.T) {
		store := &mockURLStorage{
			GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
				return nil, errors.New("storage unavailable")
			},
		}
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "urls_user")
		w := httptest.NewRecorder()

		h.GetUserURLS(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	})
}

func TestDeleteUserUrls(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		store := storage.NewStorage()
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(`[`))
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "delete_user")
		w := httptest.NewRecorder()

		h.DeleteUserUrls(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("accepted and processed asynchronously", func(t *testing.T) {
		type call struct {
			keys   []string
			userID string
		}
		calls := make(chan call, 1)
		store := &mockURLStorage{
			SetDeletedBatchFn: func(ctx context.Context, keys []string, userID string) error {
				calls <- call{keys: keys, userID: userID}
				return nil
			},
		}
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		service.StartDeleteWorker()
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(`["abc123","def456"]`))
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "delete_user")
		w := httptest.NewRecorder()

		h.DeleteUserUrls(w, r.WithContext(ctx))

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusAccepted, res.StatusCode)

		select {
		case got := <-calls:
			assert.Equal(t, "delete_user", got.userID)
			assert.Equal(t, []string{"abc123", "def456"}, got.keys)
		case <-time.After(time.Second):
			t.Errorf("delete worker did not process the enqueued task in time")
			return
		}
	})
}

func TestPingDatabase(t *testing.T) {
	t.Run("database available", func(t *testing.T) {
		store := storage.NewStorage()
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()

		h.PingDatabase(w, r)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("database ping error", func(t *testing.T) {
		store := storage.NewStorage()
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, &MockDB{PingErr: errors.New("connection refused")}, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()

		h.PingDatabase(w, r)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	})

	t.Run("no database configured", func(t *testing.T) {
		store := storage.NewStorage()
		service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
		h := NewHandler(service, nil, nil, zap.NewNop())

		r := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()

		h.PingDatabase(w, r)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	})
}

func TestAuditEvent_NoAuditorIsNoop(t *testing.T) {
	store := storage.NewStorage()
	service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
	h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

	assert.NotPanics(t, func() {
		h.auditEvent(context.Background(), "shorten", "https://example.com")
	})
}
