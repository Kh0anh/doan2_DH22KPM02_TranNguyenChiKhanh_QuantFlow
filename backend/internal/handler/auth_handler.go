package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/kh0anh/quantflow/config"
	"github.com/kh0anh/quantflow/internal/logic"
	pkgcrypto "github.com/kh0anh/quantflow/pkg/crypto"
	"github.com/kh0anh/quantflow/pkg/response"
)

// tokenCookieName is the HttpOnly cookie name carrying the JWT (api.yaml spec).
const tokenCookieName = "token"

// tokenTTL matches the 24-hour session window defined in NFR-SEC-04 and api.yaml.
const tokenTTL = 24 * time.Hour

// LoginRequest is the JSON body expected by POST /auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthHandler groups all authentication-related HTTP handlers.
type AuthHandler struct {
	authLogic *logic.AuthLogic
	cfg       *config.Config
}

// NewAuthHandler constructs an AuthHandler with its dependencies.
func NewAuthHandler(authLogic *logic.AuthLogic, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authLogic: authLogic, cfg: cfg}
}

// Login handles POST /api/v1/auth/login.
//
// Success  → 200 + Set-Cookie (HttpOnly JWT, Max-Age=86400)
// Locked   → 403 ACCOUNT_LOCKED  + locked_until
// Bad creds→ 401 INVALID_CREDENTIALS + remaining_attempts
// Server   → 500 INTERNAL_ERROR
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	// Basic input validation — must not be empty.
	if req.Username == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Username and password are required.")
		return
	}

	user, lockInfo, err := h.authLogic.Login(r.Context(), req.Username, req.Password)

	if err != nil {
		switch {
		case errors.Is(err, logic.ErrAccountLocked):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":         "ACCOUNT_LOCKED",
					"message":      "Too many failed attempts. Please try again after 15 minutes.",
					"locked_until": lockInfo.LockedUntil.Format(time.RFC3339),
				},
			})

		case errors.Is(err, logic.ErrInvalidCredentials):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":               "INVALID_CREDENTIALS",
					"message":            "Incorrect username or password.",
					"remaining_attempts": lockInfo.RemainingAttempts,
				},
			})

		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
		}
		return
	}

	// Generate JWT (HS256).
	tokenStr, err := pkgcrypto.GenerateToken(user.ID, user.Username, h.cfg.JWTSecret, tokenTTL)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to issue session token.")
		return
	}

	// Set HttpOnly cookie (Secure only in production to allow http in dev).
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    tokenStr,
		Path:     "/",
		MaxAge:   int(tokenTTL.Seconds()), // 86400
		HttpOnly: true,
		Secure:   h.cfg.GoEnv == "production",
		SameSite: http.SameSiteLaxMode,
	})

	response.JSON(w, http.StatusOK, map[string]any{
		"message": "Login successful.",
		"data": map[string]any{
			"user": user.ToBasic(),
		},
	})
}

// Logout handles POST /api/v1/auth/logout.
//
// Requires the token cookie to be present (401 if missing).
// Clears the session cookie via Max-Age=0 per RFC6265 (FR-ACCESS-04, api.yaml).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(tokenCookieName); err != nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	// MaxAge=-1 causes net/http to emit `Max-Age=0` on the wire, instructing
	// the browser to delete the cookie immediately (RFC6265 §5.3).
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.GoEnv == "production",
		SameSite: http.SameSiteLaxMode,
	})

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Logout successful.",
	})
}
