package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			name:   "wrong method",
			method: http.MethodGet,
			body:   "https://test.com",
			host:   "localhost:8080",
			want: want{
				statusCode: http.StatusBadRequest,
				shortURL:   "",
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
			storage = map[string]string{}

			body := strings.NewReader(tt.body)
			r := httptest.NewRequest(tt.method, "/", body)
			r.Host = tt.host

			w := httptest.NewRecorder()

			SetShortUrl(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)

			if tt.want.shortURL != "" {
				assert.Equal(t, tt.want.shortURL, w.Body.String())
				assert.Equal(t, tt.body, storage["abc123"])
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
			storage = map[string]string{
				"abc123": "https://test.com",
			}

			r := httptest.NewRequest(http.MethodGet, "/"+tt.id, nil)

			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", tt.id)

			r = r.WithContext(
				context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx),
			)

			w := httptest.NewRecorder()

			GetUrlById(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)
			assert.Equal(t, tt.want.originURL, res.Header.Get("Location"))
		})
	}
}
