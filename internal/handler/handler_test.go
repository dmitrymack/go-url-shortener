package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestSetShortUrl(t *testing.T) {
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
			service := shortenService.NewShortenService(store, "http://localhost:8080")
			h := NewHandler(service)

			body := strings.NewReader(tt.body)
			r := httptest.NewRequest(tt.method, "/", body)
			r.Host = tt.host

			w := httptest.NewRecorder()

			h.SetShortUrl(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)

			if tt.want.statusCode == http.StatusCreated {
				shortURL := w.Body.String()

				assert.True(t, strings.HasPrefix(shortURL, "http://localhost:8080/"))

				id := strings.TrimPrefix(shortURL, "http://localhost:8080/")
				assert.NotEmpty(t, id)

				value, ok := service.GetOriginalURL(id)

				assert.True(t, ok)
				assert.Equal(t, tt.body, value)
			}
		})
	}
}

func TestGetUrlById(t *testing.T) {
	type want struct {
		statusCode int
		originURL  string
	}

	tests := []struct {
		name string
		id   string
		want want
	}{
		{
			name: "positive test",
			id:   "abc123",
			want: want{
				statusCode: http.StatusTemporaryRedirect,
				originURL:  "https://test.com",
			},
		},
		{
			name: "url not found",
			id:   "unknown",
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
				store.Set(tt.id, tt.want.originURL)
			}
			service := shortenService.NewShortenService(store, "http://localhost:8080")
			h := NewHandler(service)

			r := httptest.NewRequest(http.MethodGet, "/"+tt.id, nil)

			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", tt.id)

			r = r.WithContext(
				context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx),
			)

			w := httptest.NewRecorder()

			h.GetUrlById(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)
			assert.Equal(t, tt.want.originURL, res.Header.Get("Location"))
		})
	}
}
