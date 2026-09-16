package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizerHandler_NoCookie_IssuesNewUser(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = r.Context().Value(contextkeys.UserIDContextKey).(string)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	AuthorizerHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.NotEmpty(t, gotUserID)

	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, string(contextkeys.UserTokenCookieName), cookies[0].Name)
	assert.Equal(t, gotUserID, service.GetUserID(cookies[0].Value), "the issued cookie must carry the same userID put into the request context")
}

func TestAuthorizerHandler_ValidCookie_PassesUserID(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := service.BuildJWTString("user1")
	require.NoError(t, err)

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = r.Context().Value(contextkeys.UserIDContextKey).(string)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: string(contextkeys.UserTokenCookieName), Value: token})
	w := httptest.NewRecorder()

	AuthorizerHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "user1", gotUserID)
	assert.Empty(t, res.Cookies(), "a valid existing cookie should not be reissued")
}

func TestAuthorizerHandler_InvalidCookie_Unauthorized(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: string(contextkeys.UserTokenCookieName), Value: "garbage"})
	w := httptest.NewRecorder()

	AuthorizerHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	assert.False(t, nextCalled, "next must not be called for an invalid token")
}

func TestGenerateUUID(t *testing.T) {
	a := generateUUID(16)
	b := generateUUID(16)

	assert.Len(t, a, 16)
	assert.Len(t, b, 16)
	assert.NotEqual(t, a, b, "two generated UUIDs should not collide")
}
