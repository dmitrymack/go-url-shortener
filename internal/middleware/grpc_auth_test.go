package middleware

import (
	"context"
	"testing"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeServerStream records headers grpc.SetHeader sends through it, so
// the "issue a new user" path can be tested without a real connection.
type fakeServerStream struct {
	header metadata.MD
}

func (f *fakeServerStream) Method() string { return "/shortener.ShortenerService/ListUserURLs" }

func (f *fakeServerStream) SetHeader(md metadata.MD) error {
	f.header = metadata.Join(f.header, md)
	return nil
}

func (f *fakeServerStream) SendHeader(md metadata.MD) error { return f.SetHeader(md) }

func (f *fakeServerStream) SetTrailer(metadata.MD) error { return nil }

func TestGRPCAuthInterceptor_NoToken_IssuesNewUser(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	stream := &fakeServerStream{}
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), stream)

	var gotUserID string
	handler := func(ctx context.Context, req any) (any, error) {
		gotUserID, _ = ctx.Value(contextkeys.UserIDContextKey).(string)
		return "ok", nil
	}

	resp, err := GRPCAuthInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.NotEmpty(t, gotUserID)

	tokens := stream.header.Get(authorizationMetadataKey)
	require.Len(t, tokens, 1)
	assert.Equal(t, gotUserID, service.GetUserID(tokens[0]), "the returned token must carry the same userID put into the request context")
}

func TestGRPCAuthInterceptor_ValidToken_PassesUserID(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := service.BuildJWTString("user1")
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authorizationMetadataKey, token))

	var gotUserID string
	handler := func(ctx context.Context, req any) (any, error) {
		gotUserID, _ = ctx.Value(contextkeys.UserIDContextKey).(string)
		return "ok", nil
	}

	resp, err := GRPCAuthInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Equal(t, "user1", gotUserID)
}

func TestGRPCAuthInterceptor_InvalidToken_Unauthenticated(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authorizationMetadataKey, "garbage"))

	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	}

	_, err := GRPCAuthInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.False(t, handlerCalled, "handler must not be called for an invalid token")
}
