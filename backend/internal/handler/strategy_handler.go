package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/pkg/response"
)

// StrategyHandler groups all /strategies HTTP handlers (WBS 2.3.1-2.3.7).
// Additional handler methods will be added in tasks 2.3.2–2.3.7.
type StrategyHandler struct {
	strategyLogic *logic.StrategyLogic
}

// NewStrategyHandler constructs a StrategyHandler with its dependencies.
func NewStrategyHandler(strategyLogic *logic.StrategyLogic) *StrategyHandler {
	return &StrategyHandler{strategyLogic: strategyLogic}
}

// List handles GET /api/v1/strategies.
//
// Query parameters (all optional):
//   - page   int  default=1,  min=1         — 1-based page number
//   - limit  int  default=20, min=1, max=100 — records per page
//   - search string                          — case-insensitive ILIKE on name
//
// Flow (WBS 2.3.1, api.yaml §GET /strategies):
//  1. Extract verified user identity from JWT Claims.
//  2. Parse and validate query params; apply defaults via NewListStrategiesInput.
//  3. Delegate to StrategyLogic.ListStrategies.
//  4. Return 200 { data: [...], pagination: {...} }.
//
// Success → 200  { data: []StrategySummary, pagination: PagePagination }
// Auth ✗  → 401  UNAUTHORIZED  (handled by JWTAuth middleware before this)
// Server ✗ → 500 INTERNAL_ERROR
func (h *StrategyHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	// Parse query params — use defaults on parse failure or zero values.
	page := parseIntParam(r, "page", 1)
	limit := parseIntParam(r, "limit", 20)
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	input := logic.NewListStrategiesInput(page, limit, search)

	out, err := h.strategyLogic.ListStrategies(r.Context(), claims.UserID, input)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Lỗi kỹ thuật. Vui lòng thử lại sau.")
		return
	}

	response.JSON(w, http.StatusOK, out)
}

// parseIntParam reads a query parameter as an integer.
// Returns defaultVal when the parameter is absent or cannot be parsed.
func parseIntParam(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return v
}
