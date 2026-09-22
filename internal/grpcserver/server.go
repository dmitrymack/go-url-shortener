// Package grpcserver implements the gRPC counterpart of internal/handler:
// the same ShortenService, exposed as shortenerpb.ShortenerServiceServer.
package grpcserver

import (
	"context"
	"errors"

	shortenerpb "github.com/dmitrymack/go-url-shortener.git/api/proto"
	"github.com/dmitrymack/go-url-shortener.git/internal/audit"
	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server is a facade over ShortenService for the ShortenerService gRPC
// service — the same logic internal/handler.Handler uses over HTTP.
type Server struct {
	shortenerpb.UnimplementedShortenerServiceServer

	service *service.ShortenService
	auditor audit.Publisher
	logger  *zap.SugaredLogger
}

// NewServer creates a Server backed by svc. auditor may be nil to disable
// auditing, matching handler.NewHandler.
func NewServer(svc *service.ShortenService, auditor audit.Publisher, logger *zap.Logger) *Server {
	return &Server{service: svc, auditor: auditor, logger: logger.Sugar()}
}

// ShortenURL creates a short link for the given URL. A pre-existing link
// is returned as-is, with no error: an error would discard the body.
func (s *Server) ShortenURL(ctx context.Context, in *shortenerpb.URLShortenRequest) (*shortenerpb.URLShortenResponse, error) {
	if in.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	shortURL, err := s.service.CreateShortURL(ctx, in.GetUrl())
	if err != nil && !errors.Is(err, storage.ErrDuplicateOriginalURL) {
		return nil, s.statusFromStorageErr(err)
	}

	s.notify(ctx, audit.ActionShorten, in.GetUrl())

	return &shortenerpb.URLShortenResponse{Result: proto.String(shortURL)}, nil
}

// ExpandURL resolves a short identifier to its original URL.
func (s *Server) ExpandURL(ctx context.Context, in *shortenerpb.URLExpandRequest) (*shortenerpb.URLExpandResponse, error) {
	originalURL, err := s.service.GetOriginalURL(in.GetId())
	if err != nil {
		return nil, s.statusFromStorageErr(err)
	}

	s.notify(ctx, audit.ActionFollow, originalURL)

	return &shortenerpb.URLExpandResponse{Result: proto.String(originalURL)}, nil
}

// ListUserURLs returns every link created by the caller, identified via
// middleware.GRPCAuthInterceptor.
func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*shortenerpb.UserURLsResponse, error) {
	userID, ok := contextkeys.UserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	urls, err := s.service.GetUrlsByUser(userID)
	if err != nil {
		return nil, s.statusFromStorageErr(err)
	}

	resp := &shortenerpb.UserURLsResponse{Url: make([]*shortenerpb.URLData, 0, len(urls))}
	for _, u := range urls {
		resp.Url = append(resp.Url, &shortenerpb.URLData{
			ShortUrl:    proto.String(u.ID),
			OriginalUrl: proto.String(u.OriginURL),
		})
	}

	return resp, nil
}

// notify sends an audit event, if auditing is enabled.
func (s *Server) notify(ctx context.Context, action, url string) {
	audit.NotifyFromContext(ctx, s.auditor, action, url)
}

// statusFromStorageErr maps err to a client-safe status: anything but a
// known sentinel is logged and reported as a bare "internal error".
func (s *Server) statusFromStorageErr(err error) error {
	switch {
	case errors.Is(err, storage.ErrDeleted):
		return status.Error(codes.NotFound, "link deleted")
	case errors.Is(err, storage.ErrNotFound):
		return status.Error(codes.NotFound, "link not found")
	default:
		s.logger.Errorw("storage error", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}
