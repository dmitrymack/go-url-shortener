// Package contextKeys contains typed keys for the request context and
// cookies shared across the service's middleware and handlers.
package contextKeys

// ContextKey is the type of context keys used by the package, to avoid
// collisions with keys from other packages.
type ContextKey string

// CookieKey is the type of cookie names used by the package.
type CookieKey string

// UserTokenCookieName is the name of the cookie holding the authorization JWT.
const UserTokenCookieName CookieKey = "userToken"

// UserIDContextKey is the context key under which middleware.AuthorizerHandler
// puts the current user's identifier.
const UserIDContextKey ContextKey = "userID"
