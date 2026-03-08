package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kh0anh/quantflow/config"
	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/pkg/response"
)

// UpdateProfileRequest is the JSON body expected by PUT /api/v1/account/profile.
// Schema source: api.yaml §components/schemas/UpdateProfileRequest.
type UpdateProfileRequest struct {
	CurrentPassword string `json:"current_password"`
	NewUsername     string `json:"new_username"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// AccountHandler groups all account-management HTTP handlers (WBS 2.1.6).
type AccountHandler struct {
	accountLogic *logic.AccountLogic
	cfg          *config.Config
}

// NewAccountHandler constructs an AccountHandler with its dependencies.
func NewAccountHandler(accountLogic *logic.AccountLogic, cfg *config.Config) *AccountHandler {
	return &AccountHandler{accountLogic: accountLogic, cfg: cfg}
}

// UpdateProfile handles PUT /api/v1/account/profile.
//
// Validation order (fast-fail, mirrors api.yaml error definitions):
//  1. Decode JSON body.
//  2. current_password is required.
//  3. At least one of new_username / new_password must be provided.
//  4. If new_password is provided, confirm_password must match.
//
// On success the handler performs Force Logout — identical cookie-clearing
// pattern to POST /auth/logout — per SRS FR-ACCESS-02 / UC-02 Business Rule 2.
func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	// current_password is always mandatory.
	if req.CurrentPassword == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "current_password is required.")
		return
	}

	// At least one change must be requested.
	if req.NewUsername == "" && req.NewPassword == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_CHANGE_FIELDS", "Please provide at least new_username or new_password.")
		return
	}

	// If a new password is supplied, confirmation must match (UC-02 E2).
	if req.NewPassword != "" && req.NewPassword != req.ConfirmPassword {
		response.Error(w, http.StatusBadRequest, "PASSWORD_MISMATCH", "The confirmation password does not match.")
		return
	}

	// Retrieve authenticated user identity from the JWT claims injected by JWTAuth middleware.
	claims, _ := appMiddleware.ClaimsFromContext(r.Context())

	input := logic.UpdateProfileInput{
		UserID:          claims.UserID,
		CurrentPassword: req.CurrentPassword,
		NewUsername:     req.NewUsername,
		NewPassword:     req.NewPassword,
	}

	if err := h.accountLogic.UpdateProfile(r.Context(), input); err != nil {
		switch {
		case errors.Is(err, logic.ErrInvalidCurrentPassword):
			response.Error(w, http.StatusUnauthorized, "INVALID_CURRENT_PASSWORD", "Current password is incorrect. Please try again.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
		}
		return
	}

	// Force Logout — clear the session cookie (Max-Age=0) so the old JWT is
	// invalidated client-side immediately (SRS FR-ACCESS-02, UC-02 BR-2).
	// Pattern matches POST /auth/logout (WBS 2.1.2).
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName, // shared constant defined in auth_handler.go
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // net/http emits Max-Age=0 on the wire (RFC 6265 §5.3)
		HttpOnly: true,
		Secure:   h.cfg.GoEnv == "production",
		SameSite: http.SameSiteLaxMode,
	})

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Update successful. Please log in again.",
	})
}
