package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/pkg/response"
)

// saveApiKeysRequest is the JSON body expected by POST /exchange/api-keys.
// secret_key is Write-Only — it is NEVER echoed back in any response.
type saveApiKeysRequest struct {
	Exchange  string `json:"exchange"`
	ApiKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

// ApiKeyHandler groups all /exchange/api-keys HTTP handlers (WBS 2.2.1-2.2.3).
// Additional handler methods (Get, Delete) will be added in tasks 2.2.2 and 2.2.3.
type ApiKeyHandler struct {
	apiKeyLogic *logic.ApiKeyLogic
}

// NewApiKeyHandler constructs an ApiKeyHandler with its dependencies.
func NewApiKeyHandler(apiKeyLogic *logic.ApiKeyLogic) *ApiKeyHandler {
	return &ApiKeyHandler{apiKeyLogic: apiKeyLogic}
}

// Save handles POST /api/v1/exchange/api-keys.
//
// Full flow (SRS UC-03, WBS 2.2.1):
//  1. Decode and validate JSON body.
//  2. Extract user identity from JWT Claims injected by JWTAuth middleware.
//  3. Delegate to ApiKeyLogic.SaveApiKey:
//     format-check → Binance ping-verify → AES-256-GCM encrypt → DB upsert.
//  4. Return 201 with ApiKeyInfo (api_key_masked; secret_key never returned).
//
// Success   → 201  ApiKeyInfo (api_key_masked, status, created_at)
// Bad body  → 400  INVALID_KEY_FORMAT
// Binance ✗ → 422  EXCHANGE_VALIDATION_FAILED
// Server  ✗ → 500  INTERNAL_ERROR
func (h *ApiKeyHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req saveApiKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	// Trim whitespace before validation — prevents accidental spaces from being
	// forwarded to Binance or stored in the database.
	req.ApiKey = strings.TrimSpace(req.ApiKey)
	req.SecretKey = strings.TrimSpace(req.SecretKey)

	if req.ApiKey == "" || req.SecretKey == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_KEY_FORMAT", "Please verify the API Key format.")
		return
	}

	// Default exchange name to "Binance" when the field is omitted (api.yaml default).
	if strings.TrimSpace(req.Exchange) == "" {
		req.Exchange = "Binance"
	}

	// Extract verified user identity from the JWT Claims injected by JWTAuth.
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	saved, err := h.apiKeyLogic.SaveApiKey(r.Context(), claims.UserID, logic.SaveApiKeyInput{
		Exchange:  req.Exchange,
		ApiKey:    req.ApiKey,
		SecretKey: req.SecretKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrInvalidKeyFormat):
			response.Error(w, http.StatusBadRequest, "INVALID_KEY_FORMAT", "Please verify the API Key format.")
		case errors.Is(err, logic.ErrExchangeValidationFailed):
			response.Error(w, http.StatusUnprocessableEntity, "EXCHANGE_VALIDATION_FAILED",
				"API Key is invalid or lacks Futures Trading permission.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
		}
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"message": "Exchange connected successfully.",
		"data":    saved.ToInfo(),
	})
}
