package middleware

import (
	"context"
	"net/http"

	"github.com/kh0anh/quantflow/config"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
	"github.com/kh0anh/quantflow/pkg/response"
)

// contextKey is an unexported type for context keys in this package.
// Using a typed key prevents collisions with keys from other packages.
type contextKey string

// claimsCtxKey is the context key under which *pkgcrypto.Claims is stored.
const claimsCtxKey contextKey = "jwt_claims"

// tokenCookieName matches the cookie name used by auth_handler.go.
const tokenCookieName = "token"

// JWTAuth returns a Chi-compatible middleware that protects a route group.
//
// Flow:
//  1. Read the HttpOnly cookie named "token".
//  2. Verify the JWT signature and expiry via pkgcrypto.ParseToken().
//  3. On failure → respond 401 and halt the handler chain.
//  4. On success → store *Claims in context and call next handler.
//
// WBS 2.1.5: "Read cookie token - verify signature"
func JWTAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(tokenCookieName)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
				return
			}

			claims, err := pkgcrypto.ParseToken(cookie.Value, cfg.JWTSecret)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "SESSION_EXPIRED", "Session has expired. Please log in again.")
				return
			}

			// Inject verified claims into request context for downstream handlers.
			ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves the verified *pkgcrypto.Claims injected by JWTAuth.
// Returns (nil, false) if the middleware was not applied or claims are absent.
func ClaimsFromContext(ctx context.Context) (*pkgcrypto.Claims, bool) {
	claims, ok := ctx.Value(claimsCtxKey).(*pkgcrypto.Claims)
	return claims, ok
}
