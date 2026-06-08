package middleware

import (
	"context"
	"errors"
	"math/rand"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
)

func AuthorizerHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := (*http.Request).Cookie(r, contextKeys.UserTokenCookieName)

		if errors.Is(err, http.ErrNoCookie) {
			userID := generateUUID(16)

			token, _ := service.BuildJWTString(userID)
			http.SetCookie(w, &http.Cookie{
				Name:  contextKeys.UserTokenCookieName,
				Value: token,
			})

			ctx := context.WithValue(r.Context(), contextKeys.UserIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userID := service.GetUserID(cookie.Value)
		if userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), contextKeys.UserIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateUUID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}
