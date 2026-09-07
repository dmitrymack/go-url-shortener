// Package middleware contains the HTTP middleware of the URL shortener
// service: cookie-based authorization, request logging, and gzip
// compression.
package middleware

import (
	"context"
	"errors"
	"math/rand"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextkeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
)

// AuthorizerHandler identifies the user via the JWT stored in the
// contextkeys.UserTokenCookieName cookie and puts their identifier into the
// request context under contextkeys.UserIDContextKey. If the cookie is
// missing, it issues a new user: generates a UUID, signs a token, and sets
// the cookie on the response. If the cookie is present but the token is
// invalid, it responds with 401.
func AuthorizerHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(string(contextkeys.UserTokenCookieName))

		if errors.Is(err, http.ErrNoCookie) {
			userID := generateUUID(16)

			token, _ := service.BuildJWTString(userID)
			http.SetCookie(w, &http.Cookie{
				Name:  string(contextkeys.UserTokenCookieName),
				Value: token,
				Path:  "/",
			})

			ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userID := service.GetUserID(cookie.Value)
		if userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), contextkeys.UserIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateUUID returns a random alphanumeric string of the given length for
// use as a new user's identifier.
func generateUUID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}
