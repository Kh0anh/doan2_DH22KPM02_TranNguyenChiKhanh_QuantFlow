package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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

// Get handles GET /api/v1/strategies/{id}.
//
// Flow (WBS 2.3.3, api.yaml §GET /strategies/{id}):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter via chi.URLParam.
//  3. Delegate to StrategyLogic.GetStrategy (ownership check + active bot lookup).
//  4. Return 200 { data: StrategyDetail }.
//     If active bots exist, warning and active_bot_ids are included automatically.
//
// Success      → 200  { data: StrategyDetail }
// Not found    → 404  STRATEGY_NOT_FOUND
// Auth ✗       → 401  UNAUTHORIZED
// Server ✗     → 500  INTERNAL_ERROR
func (h *StrategyHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	id := chi.URLParam(r, "id")

	detail, err := h.strategyLogic.GetStrategy(r.Context(), claims.UserID, id)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrStrategyNotFound):
			response.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "Strategy not found.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": detail,
	})
}

// updateStrategyRequest is the JSON body expected by PUT /strategies/{id}.
// All fields are optional — omitted fields retain their current values.
// logic_json is kept as json.RawMessage to avoid double-serialisation.
type updateStrategyRequest struct {
	Name      string          `json:"name"`
	LogicJSON json.RawMessage `json:"logic_json"`
	Status    string          `json:"status"`
}

// Update handles PUT /api/v1/strategies/{id}.
//
// Flow (WBS 2.3.4, api.yaml §PUT /strategies/{id}):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter.
//  3. Decode JSON body (all fields optional).
//  4. Delegate to StrategyLogic.UpdateStrategy — validates logic_json when
//     provided, atomically bumps version_number, updates strategy metadata.
//  5. Return 200 { message, data: StrategyUpdated }.
//     If Running bots exist, data.warning is populated automatically.
//
// Success              → 200  { message, data: StrategyUpdated }
// Missing event block  → 400  MISSING_EVENT_TRIGGER
// Malformed JSON       → 400  INVALID_JSON_STRUCTURE
// Not found / no owner → 404  STRATEGY_NOT_FOUND
// Auth ✗               → 401  UNAUTHORIZED
// Server ✗             → 500  INTERNAL_ERROR
func (h *StrategyHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	id := chi.URLParam(r, "id")

	var req updateStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		return
	}

	updated, err := h.strategyLogic.UpdateStrategy(r.Context(), claims.UserID, id, logic.UpdateStrategyInput{
		Name:      req.Name,
		LogicJSON: req.LogicJSON,
		Status:    req.Status,
	})
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrStrategyNotFound):
			response.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "Strategy not found.")
		case errors.Is(err, logic.ErrMissingEventTrigger):
			response.Error(w, http.StatusBadRequest, "MISSING_EVENT_TRIGGER", "Strategy must contain an Event Trigger block.")
		case errors.Is(err, logic.ErrInvalidJSONStructure):
			response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message": "Strategy updated successfully.",
		"data":    updated,
	})
}

// Delete handles DELETE /api/v1/strategies/{id}.
//
// Flow (WBS 2.3.5, api.yaml §DELETE /strategies/{id}):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter.
//  3. Delegate to StrategyLogic.DeleteStrategy.
//  4. On Running bots — return 409 with custom body including active_bot_ids.
//  5. On success — return 200 { message }.
//
// Success              → 200  { message }
// Running bots exist   → 409  STRATEGY_IN_USE  (includes active_bot_ids in error)
// Not found / no owner → 404  STRATEGY_NOT_FOUND
// Auth ✗               → 401  UNAUTHORIZED
// Server ✗             → 500  INTERNAL_ERROR
func (h *StrategyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	id := chi.URLParam(r, "id")

	botIDs, err := h.strategyLogic.DeleteStrategy(r.Context(), claims.UserID, id)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrStrategyInUse):
			// 409 uses a non-standard extended error body per api.yaml
			// (active_bot_ids nested inside error object).
			response.JSON(w, http.StatusConflict, map[string]any{
				"error": map[string]any{
					"code":           "STRATEGY_IN_USE",
					"message":        "Strategy is being used by running Bot(s). Please stop all Bots before deleting.",
					"active_bot_ids": botIDs,
				},
			})
		case errors.Is(err, logic.ErrStrategyNotFound):
			response.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "Strategy not found.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message": "Strategy deleted successfully.",
	})
}

// importStrategyRequest is the JSON body expected by POST /strategies/import.
// Mirrors api.yaml §ImportStrategyRequest.
type importStrategyRequest struct {
	Name      string          `json:"name"`
	LogicJSON json.RawMessage `json:"logic_json"`
}

// Import handles POST /api/v1/strategies/import.
//
// Flow (WBS 2.3.6, api.yaml §POST /strategies/import, SRS FR-DESIGN-13):
//  1. Extract JWT claims from context.
//  2. Decode JSON body → importStrategyRequest.
//  3. Delegate to StrategyLogic.ImportStrategy.
//  4. Both ErrInvalidJSONStructure and ErrMissingEventTrigger → 400 INVALID_JSON_STRUCTURE.
//  5. Success → 201 { message, data: StrategyCreated }.
//
// Success              → 201  { message, data: StrategyCreated }
// Invalid JSON         → 400  INVALID_JSON_STRUCTURE
// Auth ✗               → 401  UNAUTHORIZED
// Server ✗             → 500  INTERNAL_ERROR
func (h *StrategyHandler) Import(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	var req importStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the file format and try again.")
		return
	}

	input := logic.ImportStrategyInput{
		Name:      req.Name,
		LogicJSON: req.LogicJSON,
	}

	created, err := h.strategyLogic.ImportStrategy(r.Context(), claims.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrInvalidJSONStructure), errors.Is(err, logic.ErrMissingEventTrigger):
			response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the file format and try again.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"message": "Strategy imported successfully.",
		"data":    created,
	})
}
