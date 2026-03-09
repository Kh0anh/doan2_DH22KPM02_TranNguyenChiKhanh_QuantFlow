package handler

import (
	"encoding/json"
	"errors"
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
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
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

// createStrategyRequest is the JSON body expected by POST /strategies.
// logic_json is kept as json.RawMessage so it is forwarded to the logic layer
// without double-serialisation and validated there using encoding/json.
type createStrategyRequest struct {
	Name      string          `json:"name"`
	LogicJSON json.RawMessage `json:"logic_json"`
	Status    string          `json:"status"`
}

// Create handles POST /api/v1/strategies.
//
// Flow (WBS 2.3.2, api.yaml §POST /strategies, SRS FR-DESIGN-11):
//  1. Extract JWT claims from context.
//  2. Decode JSON body — name and logic_json are required.
//  3. Delegate to StrategyLogic.CreateStrategy (validates event_on_candle block,
//     normalises status, atomically inserts strategy + version_number=1).
//  4. Return 201 with { message, data: StrategyCreated }.
//
// Success              → 201  { message, data: StrategyCreated }
// Missing event block  → 400  MISSING_EVENT_TRIGGER
// Malformed JSON       → 400  INVALID_JSON_STRUCTURE
// Auth ✗               → 401  UNAUTHORIZED
// Server ✗             → 500  INTERNAL_ERROR
func (h *StrategyHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	var req createStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		return
	}

	if strings.TrimSpace(req.Name) == "" || len(req.LogicJSON) == 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		return
	}

	created, err := h.strategyLogic.CreateStrategy(r.Context(), claims.UserID, logic.CreateStrategyInput{
		Name:      req.Name,
		LogicJSON: req.LogicJSON,
		Status:    req.Status,
	})
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrMissingEventTrigger):
			response.Error(w, http.StatusBadRequest, "MISSING_EVENT_TRIGGER", "Strategy must contain an Event Trigger block.")
		case errors.Is(err, logic.ErrInvalidJSONStructure):
			response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"message": "Strategy saved successfully.",
		"data":    created,
	})
}
