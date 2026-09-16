package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockDB is a storage.Database test double. The zero value always succeeds;
// set PingErr to make Ping fail.
type MockDB struct {
	PingErr error
}

func (db *MockDB) Ping(ctx context.Context) error { return db.PingErr }

func (db *MockDB) Close(ctx context.Context) error { return nil }

func TestSetShortURL(t *testing.T) {
	type want struct {
		statusCode int
		shortURL   string
	}

	tests := []struct {
		name   string
		method string
		body   string
		host   string
		want   want
	}{
		{
			name:   "positive test",
			method: http.MethodPost,
			body:   "https://test.com",
			host:   "localhost:8080",
			want: want{
				statusCode: http.StatusCreated,
				shortURL:   "http://localhost:8080/abc123",
			},
		},
		{
			name:   "empty body",
			method: http.MethodPost,
			body:   "",
			host:   "localhost:8080",
			want: want{
				statusCode: http.StatusBadRequest,
				shortURL:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storage.NewStorage()
			mockDB := &MockDB{}
			service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
			h := NewHandler(service, mockDB, nil, zap.NewNop())

			body := strings.NewReader(tt.body)
			r := httptest.NewRequest(tt.method, "/", body)
			r.Host = tt.host

			w := httptest.NewRecorder()
			ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "test_set_user")
			h.SetShortURL(w, r.WithContext(ctx))

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)

			if tt.want.statusCode == http.StatusCreated {
				shortURL := w.Body.String()

				assert.True(t, strings.HasPrefix(shortURL, "http://localhost:8080/"))

				id := strings.TrimPrefix(shortURL, "http://localhost:8080/")
				assert.NotEmpty(t, id)

				value, err := service.GetOriginalURL(id)

				assert.True(t, err == nil)
				assert.Equal(t, tt.body, value)
			}
		})
	}
}

func TestGetURLByID(t *testing.T) {
	type want struct {
		statusCode int
		originURL  string
	}

	tests := []struct {
		name   string
		id     string
		userID string
		want   want
	}{
		{
			name:   "positive test",
			id:     "abc123",
			userID: "user_abc123",
			want: want{
				statusCode: http.StatusTemporaryRedirect,
				originURL:  "https://test.com",
			},
		},
		{
			name:   "url not found",
			id:     "unknown",
			userID: "user_unknown",
			want: want{
				statusCode: http.StatusBadRequest,
				originURL:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storage.NewStorage()
			if tt.want.originURL != "" {
				store.Set(context.Background(), tt.id, tt.want.originURL, tt.userID)
			}
			mockDB := &MockDB{}
			service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
			h := NewHandler(service, mockDB, nil, zap.NewNop())

			r := httptest.NewRequest(http.MethodGet, "/"+tt.id, nil)

			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", tt.id)

			r = r.WithContext(
				context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx),
			)

			w := httptest.NewRecorder()

			h.GetURLByID(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)
			assert.Equal(t, tt.want.originURL, res.Header.Get("Location"))
		})
	}
}

func TestSetShortURLByJSON(t *testing.T) {
	type want struct {
		statusCode int
	}

	tests := []struct {
		name string
		body string
		host string
		want want
	}{
		{
			name: "positive test",
			body: `{"url":"https://test.com"}`,
			host: "localhost:8080",
			want: want{
				statusCode: http.StatusCreated,
			},
		},
		{
			name: "empty url field",
			body: `{"url":""}`,
			host: "localhost:8080",
			want: want{
				statusCode: http.StatusBadRequest,
			},
		},
		{
			name: "invalid json",
			body: `{"url":"https://test.com"`,
			host: "localhost:8080",
			want: want{
				statusCode: http.StatusBadRequest,
			},
		},
		{
			name: "wrong field name",
			body: `{"url111":"https://test.com"}`,
			host: "localhost:8080",
			want: want{
				statusCode: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storage.NewStorage()
			mockDB := &MockDB{}
			service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
			h := NewHandler(service, mockDB, nil, zap.NewNop())

			body := strings.NewReader(tt.body)
			r := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
			r.Host = tt.host
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, "test_set_json_user")
			h.SetShortURLByJSON(w, r.WithContext(ctx))

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)

			if tt.want.statusCode == http.StatusCreated {
				assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

				var respObj ResponseObject

				err := json.NewDecoder(res.Body).Decode(&respObj)
				require.NoError(t, err)

				assert.True(t, strings.HasPrefix(respObj.Result, "http://localhost:8080/"))

				id := strings.TrimPrefix(respObj.Result, "http://localhost:8080/")

				assert.NotEmpty(t, id)

				value, err := service.GetOriginalURL(id)

				assert.True(t, err == nil)
				assert.Equal(t, "https://test.com", value)
			}
		})
	}
}
