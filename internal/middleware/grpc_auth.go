package middleware

import (
	"context"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authorizationMetadataKey is the gRPC analog of contextkeys.UserTokenCookieName.
const authorizationMetadataKey = "authorization"

// GRPCAuthInterceptor returns the gRPC counterpart of AuthorizerHandler:
// JWT via "authorization" metadata instead of a cookie, issuing one if
// absent. Failing to issue or deliver that token surfaces as an error
// immediately, rather than silently minting a fresh user on every call.
func GRPCAuthInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	sugar := logger.Sugar()

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var token string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(authorizationMetadataKey); len(values) > 0 {
				token = values[0]
			}
		}

		if token == "" {
			userID := generateUUID(16)

			newToken, err := service.BuildJWTString(userID)
			if err != nil {
				sugar.Errorw("failed to issue an auth token", "error", err)
				return nil, status.Error(codes.Internal, "failed to issue an auth token")
			}

			if err := grpc.SetHeader(ctx, metadata.Pairs(authorizationMetadataKey, newToken)); err != nil {
				sugar.Errorw("failed to send an auth token", "error", err)
				return nil, status.Error(codes.Internal, "failed to send an auth token")
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
}
