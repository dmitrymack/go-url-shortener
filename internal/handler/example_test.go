package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
)

// ExampleHandler_SetShortUrl demonstrates POST / — shortening a URL passed
// as plain text.
func ExampleHandler_SetShortUrl() {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, "http://localhost:8080")
	h := NewHandler(svc, nil, nil)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	ctx := context.WithValue(r.Context(), contextKeys.UserIDContextKey, "example_user")
	w := httptest.NewRecorder()

	h.SetShortUrl(w, r.WithContext(ctx))

	res := w.Result()
	defer res.Body.Close()

	fmt.Println(res.StatusCode)
	// Output:
	// 201
}

// ExampleHandler_SetShortUrlByJSON demonstrates POST /api/shorten —
// shortening a URL passed as JSON.
func ExampleHandler_SetShortUrlByJSON() {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, "http://localhost:8080")
	h := NewHandler(svc, nil, nil)

	body := strings.NewReader(`{"url":"https://example.com"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	r.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(r.Context(), contextKeys.UserIDContextKey, "example_user")
	w := httptest.NewRecorder()

	h.SetShortUrlByJSON(w, r.WithContext(ctx))

	res := w.Result()
	defer res.Body.Close()

	var resp ResponseObject
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(res.StatusCode, strings.HasPrefix(resp.Result, "http://localhost:8080/"))
	// Output:
	// 201 true
}

// ExampleHandler_GetUrlById demonstrates GET /{id} — following a short link
// and getting a redirect to the original URL.
func ExampleHandler_GetUrlById() {
	store := storage.NewStorage()
	store.Set(context.Background(), "abc12345", "https://example.com", "example_user")
	svc := shortenService.NewShortenService(store, "http://localhost:8080")
	h := NewHandler(svc, nil, nil)

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc12345")

	r := httptest.NewRequest(http.MethodGet, "/abc12345", nil)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx)
	w := httptest.NewRecorder()

	h.GetUrlById(w, r.WithContext(ctx))

	res := w.Result()
	defer res.Body.Close()

	fmt.Println(res.StatusCode, res.Header.Get("Location"))
	// Output:
	// 307 https://example.com
}

// ExampleHandler_SetBatchURL demonstrates POST /api/shorten/batch — batch
// shortening several URLs in a single request.
func ExampleHandler_SetBatchURL() {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, "http://localhost:8080")
	h := NewHandler(svc, nil, nil)

	body := strings.NewReader(`[
		{"correlation_id":"1","original_url":"https://example.com/one"},
		{"correlation_id":"2","original_url":"https://example.com/two"}
	]`)
	r := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", body)
	ctx := context.WithValue(r.Context(), contextKeys.UserIDContextKey, "example_user")
	w := httptest.NewRecorder()

	h.SetBatchURL(w, r.WithContext(ctx))

	res := w.Result()
	defer res.Body.Close()

	var resp []BatchResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(res.StatusCode, len(resp), resp[0].CorrelationID, resp[1].CorrelationID)
	// Output:
	// 201 2 1 2
}

// exampleUserURLStorage augments the in-memory storage.Storage with a real
// GetUrlsByUser implementation (the in-memory store doesn't implement it) —
// used only by the ExampleHandler_GetUserURLS example.
type exampleUserURLStorage struct {
	*storage.Storage
	records []storage.URLRecord
}

func (s *exampleUserURLStorage) Set(ctx context.Context, key, value, userID string) (string, error) {
	key, err := s.Storage.Set(ctx, key, value, userID)
	if err == nil {
		s.records = append(s.records, storage.URLRecord{ID: key, OriginURL: value})
	}
	return key, err
}

func (s *exampleUserURLStorage) GetUrlsByUser(userID string) ([]storage.URLRecord, error) {
	return s.records, nil
}

// ExampleHandler_GetUserURLS demonstrates GET /api/user/urls — fetching all
// short links created by the current user.
func ExampleHandler_GetUserURLS() {
	store := &exampleUserURLStorage{Storage: storage.NewStorage()}
	svc := shortenService.NewShortenService(store, "http://localhost:8080")
	h := NewHandler(svc, nil, nil)

	ctx := context.WithValue(context.Background(), contextKeys.UserIDContextKey, "example_user")
	if _, err := svc.CreateShortURL(ctx, "https://example.com"); err != nil {
		fmt.Println(err)
		return
	}

	r := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	w := httptest.NewRecorder()

	h.GetUserURLS(w, r.WithContext(ctx))

	res := w.Result()
	defer res.Body.Close()

	var resp []storage.URLRecord
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(res.StatusCode, len(resp), resp[0].OriginURL)
	// Output:
	// 200 1 https://example.com
}
