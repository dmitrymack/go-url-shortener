package service

import (
	"context"
	"strings"
	"testing"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"go.uber.org/zap"
)

func BenchmarkGenerateID(b *testing.B) {
	for b.Loop() {
		generateID()
	}
}

func BenchmarkCreateShortURL(b *testing.B) {
	store := storage.NewStorage()
	s := NewShortenService(store, "http://localhost:8080", zap.NewNop())
	ctx := context.WithValue(context.Background(), contextKeys.UserIDContextKey, "bench_user")

	for b.Loop() {
		if _, err := s.CreateShortURL(ctx, "https://example.com/some/long/path"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	store := storage.NewStorage()
	s := NewShortenService(store, "http://localhost:8080", zap.NewNop())
	ctx := context.WithValue(context.Background(), contextKeys.UserIDContextKey, "bench_user")

	shortURL, err := s.CreateShortURL(ctx, "https://example.com/some/long/path")
	if err != nil {
		b.Fatal(err)
	}
	id := strings.TrimPrefix(shortURL, "http://localhost:8080/")

	for b.Loop() {
		if _, err := s.GetOriginalURL(id); err != nil {
			b.Fatal(err)
		}
	}
}
