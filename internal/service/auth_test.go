package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildJWTString_GetUserID_RoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	tokenString, err := BuildJWTString("user1")
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	userID := GetUserID(tokenString)
	assert.Equal(t, "user1", userID)
}

func TestGetUserID_InvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	assert.Empty(t, GetUserID("not-a-valid-token"))
}

func TestGetUserID_WrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	tokenString, err := BuildJWTString("user1")
	require.NoError(t, err)

	t.Setenv("JWT_SECRET", "different-secret")
	assert.Empty(t, GetUserID(tokenString))
}
