package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func BenchmarkSetShortURL(b *testing.B) {
	store := storage.NewStorage()
	service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
	h := NewHandler(service, &MockDB{}, nil, zap.NewNop())
	ctx := context.WithValue(context.Background(), contextkeys.UserIDContextKey, "bench_user")

	for b.Loop() {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/some/long/path")).WithContext(ctx)
		w := httptest.NewRecorder()
		h.SetShortURL(w, r)
	}
}

func BenchmarkSetShortURLByJSON(b *testing.B) {
	store := storage.NewStorage()
	service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
	h := NewHandler(service, &MockDB{}, nil, zap.NewNop())
	ctx := context.WithValue(context.Background(), contextkeys.UserIDContextKey, "bench_user")
	body := `{"url":"https://example.com/some/long/path"}`

	for b.Loop() {
		r := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.SetShortURLByJSON(w, r)
	}
}

func BenchmarkGetURLByID(b *testing.B) {
	store := storage.NewStorage()
	service := shortenService.NewShortenService(store, "http://localhost:8080", zap.NewNop())
	h := NewHandler(service, &MockDB{}, nil, zap.NewNop())

	store.Set(context.Background(), "abc12345", "https://example.com/some/long/path", "bench_user")

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc12345")
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, routeCtx)

	for b.Loop() {
		r := httptest.NewRequest(http.MethodGet, "/abc12345", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		h.GetURLByID(w, r)
	}
}
