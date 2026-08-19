package service

import (
	"context"
	"strings"
	"testing"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
)

func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateID()
	}
}

func BenchmarkCreateShortURL(b *testing.B) {
	store := storage.NewStorage()
	s := NewShortenService(store, "http://localhost:8080")
	ctx := context.WithValue(context.Background(), contextKeys.UserIDContextKey, "bench_user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.CreateShortURL(ctx, "https://example.com/some/long/path"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	store := storage.NewStorage()
	s := NewShortenService(store, "http://localhost:8080")
	ctx := context.WithValue(context.Background(), contextKeys.UserIDContextKey, "bench_user")

	shortURL, err := s.CreateShortURL(ctx, "https://example.com/some/long/path")
	if err != nil {
		b.Fatal(err)
	}
	id := strings.TrimPrefix(shortURL, "http://localhost:8080/")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetOriginalURL(id); err != nil {
			b.Fatal(err)
		}
	}
}
