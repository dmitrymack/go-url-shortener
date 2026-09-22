package middleware

import (
	"context"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authorizationMetadataKey is the gRPC analog of contextkeys.UserTokenCookieName.
const authorizationMetadataKey = "authorization"

// GRPCAuthInterceptor is the gRPC counterpart of AuthorizerHandler: JWT via
// "authorization" metadata instead of a cookie, issuing one if absent.
func GRPCAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	var token string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(authorizationMetadataKey); len(values) > 0 {
			token = values[0]
		}
	}

	if token == "" {
		userID := generateUUID(16)

		if newToken, err := service.BuildJWTString(userID); err == nil {
			grpc.SetHeader(ctx, metadata.Pairs(authorizationMetadataKey, newToken))
		}

		ctx = context.WithValue(ctx, contextkeys.UserIDContextKey, userID)
		return handler(ctx, req)
	}

	userID := service.GetUserID(token)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	ctx = context.WithValue(ctx, contextkeys.UserIDContextKey, userID)
	return handler(ctx, req)
}
