package grpcserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	shortenerpb "github.com/dmitrymack/go-url-shortener.git/api/proto"
	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const testBaseURL = "http://localhost:8080"

func userContext(userID string) context.Context {
	return context.WithValue(context.Background(), contextkeys.UserIDContextKey, userID)
}

func TestShortenURL_Success(t *testing.T) {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	resp, err := s.ShortenURL(userContext("user1"), &shortenerpb.URLShortenRequest{Url: strPtr("https://example.com")})

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp.GetResult(), testBaseURL+"/"))

	id := strings.TrimPrefix(resp.GetResult(), testBaseURL+"/")
	value, err := svc.GetOriginalURL(id)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value)
}

func TestShortenURL_EmptyURL(t *testing.T) {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	_, err := s.ShortenURL(userContext("user1"), &shortenerpb.URLShortenRequest{Url: strPtr("")})

	assertCode(t, err, codes.InvalidArgument)
}

func TestShortenURL_DuplicateOriginalURL(t *testing.T) {
	store := &mockURLStorage{
		SetFn: func(ctx context.Context, key, value, userID string) (string, error) {
			return "existingID", storage.ErrDuplicateOriginalURL
		},
	}
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	resp, err := s.ShortenURL(userContext("user1"), &shortenerpb.URLShortenRequest{Url: strPtr("https://example.com")})

	require.NoError(t, err, "a duplicate URL is not an RPC error: the response has no field to flag it, so it just returns the existing link")
	assert.Equal(t, testBaseURL+"/existingID", resp.GetResult())
}

func TestShortenURL_StorageError(t *testing.T) {
	store := &mockURLStorage{
		SetFn: func(ctx context.Context, key, value, userID string) (string, error) {
			return "", errors.New("storage unavailable")
		},
	}
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	_, err := s.ShortenURL(userContext("user1"), &shortenerpb.URLShortenRequest{Url: strPtr("https://example.com")})

	assertCode(t, err, codes.Internal)
	assertNoErrLeak(t, err, "storage unavailable")
}

func TestExpandURL_Success(t *testing.T) {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	_, err := store.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)

	s := NewServer(svc, nil, zap.NewNop())
	resp, err := s.ExpandURL(context.Background(), &shortenerpb.URLExpandRequest{Id: strPtr("abc123")})

	require.NoError(t, err)
	assert.Equal(t, "https://example.com", resp.GetResult())
}

func TestExpandURL_NotFound(t *testing.T) {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	_, err := s.ExpandURL(context.Background(), &shortenerpb.URLExpandRequest{Id: strPtr("unknown")})

	assertCode(t, err, codes.NotFound)
}

func TestExpandURL_Deleted(t *testing.T) {
	store := &mockURLStorage{
		GetFn: func(key string) (string, error) {
			return "", storage.ErrDeleted
		},
	}
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	_, err := s.ExpandURL(context.Background(), &shortenerpb.URLExpandRequest{Id: strPtr("abc123")})

	assertCode(t, err, codes.NotFound)
}

func TestExpandURL_StorageError(t *testing.T) {
	store := &mockURLStorage{
		GetFn: func(key string) (string, error) {
			return "", errors.New("connection reset by peer")
		},
	}
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	_, err := s.ExpandURL(context.Background(), &shortenerpb.URLExpandRequest{Id: strPtr("abc123")})

	assertCode(t, err, codes.Internal, "a real storage failure must not look like a missing link (NotFound)")
	assertNoErrLeak(t, err, "connection reset by peer")
}

func TestListUserURLs_Success(t *testing.T) {
	store := &mockURLStorage{
		GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
			return []storage.URLRecord{
				{ID: "abc123", OriginURL: "https://example.com"},
				{ID: "def456", OriginURL: "https://example.org"},
			}, nil
		},
	}
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	resp, err := s.ListUserURLs(userContext("user1"), &emptypb.Empty{})

	require.NoError(t, err)
	require.Len(t, resp.GetUrl(), 2)
	assert.Equal(t, testBaseURL+"/abc123", resp.GetUrl()[0].GetShortUrl())
	assert.Equal(t, "https://example.com", resp.GetUrl()[0].GetOriginalUrl())
	assert.Equal(t, testBaseURL+"/def456", resp.GetUrl()[1].GetShortUrl())
}

func TestListUserURLs_StorageError(t *testing.T) {
	store := &mockURLStorage{
		GetUrlsByUserFn: func(userID string) ([]storage.URLRecord, error) {
			return nil, errors.New("storage unavailable")
		},
	}
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	_, err := s.ListUserURLs(userContext("user1"), &emptypb.Empty{})

	assertCode(t, err, codes.Internal)
	assertNoErrLeak(t, err, "storage unavailable")
}

func TestNotify_NoAuditorIsNoop(t *testing.T) {
	store := storage.NewStorage()
	svc := shortenService.NewShortenService(store, testBaseURL, zap.NewNop())
	s := NewServer(svc, nil, zap.NewNop())

	assert.NotPanics(t, func() {
		s.notify(context.Background(), "shorten", "https://example.com")
	})
}

func assertCode(t *testing.T, err error, want codes.Code, msgAndArgs ...any) {
	t.Helper()

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "expected a gRPC status error")
	assert.Equal(t, want, st.Code(), msgAndArgs...)
}

// assertNoErrLeak fails if the gRPC status message contains raw, an
// internal detail that statusFromStorageErr must keep server-side.
func assertNoErrLeak(t *testing.T, err error, raw string) {
	t.Helper()

	st, ok := status.FromError(err)
	require.True(t, ok, "expected a gRPC status error")
	assert.NotContains(t, st.Message(), raw)
}

// strPtr is a tiny local alias for *string, saving a "google.golang.org/protobuf/proto"
// import for the single helper this test file needs.
func strPtr(s string) *string { return &s }
