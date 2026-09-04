package service

import (
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v4"
	_ "github.com/joho/godotenv/autoload"
)

// Claims is the payload of the authorization JWT: standard JWT fields plus
// the user identifier.
//
// generate:reset
// Note: the embedded jwt.RegisteredClaims has no Reset() method of its
// own, so the generated Reset() only clears UserID.
type Claims struct {
	jwt.RegisteredClaims
	UserID string
}

// BuildJWTString signs and returns a JWT containing userID. The signing
// secret is read from the JWT_SECRET environment variable.
func BuildJWTString(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{},
		UserID:           userID,
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetUserID verifies the signature and validity of the JWT tokenString and
// returns the user identifier it contains. If the token is invalid or
// signed with a different method, it returns an empty string.
func GetUserID(tokenString string) string {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
	if err != nil {
		return ""
	}

	if !token.Valid {
		return ""
	}

	return claims.UserID
}
