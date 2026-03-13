package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/pkg/response"
)

// BotHandler groups all /bots HTTP handlers (WBS 2.7.5).
type BotHandler struct {
	botLogic *logic.BotLogic
}

// NewBotHandler constructs a BotHandler with its dependencies.
func NewBotHandler(botLogic *logic.BotLogic) *BotHandler {
	return &BotHandler{botLogic: botLogic}
}

// List handles GET /api/v1/bots.
//
// Query parameters (optional):
//   - status string — filter by bot status ("Running", "Stopped", "Error").
//     Empty string = no filter (return all bots).
//
// Flow (WBS 2.7.5, api.yaml §GET /bots):
//  1. Extract verified user identity from JWT Claims.
//  2. Parse optional status query param.
//  3. Delegate to BotLogic.ListBots.
//  4. Return 200 { data: [] }.
//
// Success → 200  { data: []BotSummary }
// Auth ✗  → 401  UNAUTHORIZED (handled by JWTAuth middleware before this)
// Server ✗ → 500 INTERNAL_ERROR
func (h *BotHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	bots, err := h.botLogic.ListBots(r.Context(), claims.UserID, statusFilter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": bots,
	})
}

// createBotRequest is the JSON body expected by POST /bots.
type createBotRequest struct {
	BotName    string `json:"bot_name"`
	StrategyID string `json:"strategy_id"`
	Symbol     string `json:"symbol"`
}

// Create handles POST /api/v1/bots.
//
// Flow (WBS 2.7.5, api.yaml §POST /bots, SRS FR-RUN-05):
//  1. Extract JWT claims from context.
//  2. Decode JSON body — bot_name, strategy_id, symbol are required.
//  3. Delegate to BotLogic.CreateBot (validates strategy & api_key, snapshots version,
//     inserts DB, launches bot goroutine via BotManager.StartBot).
//  4. Return 201 with { message, data: BotCreated }.
//
// Success                    → 201  { message, data: BotCreated }
// Strategy not found         → 404  STRATEGY_NOT_FOUND
// Strategy status != Valid   → 422  STRATEGY_INVALID
// API Key not configured     → 422  EXCHANGE_NOT_CONFIGURED
// Invalid logic JSON         → 422  INVALID_LOGIC_JSON
// Auth ✗                     → 401  UNAUTHORIZED
// Server ✗                   → 500  INTERNAL_ERROR
func (h *BotHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	var req createBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		return
	}

	// Validate required fields — trim and check non-empty.
	if strings.TrimSpace(req.BotName) == "" || strings.TrimSpace(req.StrategyID) == "" || strings.TrimSpace(req.Symbol) == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
		return
	}

	// Enforce bot_name length limit (api.yaml §CreateBotRequest: maxLength 100).
	if len(req.BotName) > 100 {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Bot name exceeds maximum length of 100 characters.")
		return
	}

	input := logic.CreateBotInput{
		BotName:    req.BotName,
		StrategyID: req.StrategyID,
		Symbol:     req.Symbol,
	}

	created, err := h.botLogic.CreateBot(r.Context(), claims.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrBotStrategyNotFound):
			response.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "Strategy not found.")
		case errors.Is(err, logic.ErrBotStrategyInvalid):
			response.Error(w, http.StatusUnprocessableEntity, "STRATEGY_INVALID", "Strategy status is Draft. Please save the strategy with Valid status.")
		case errors.Is(err, logic.ErrBotAPIKeyNotConfigured):
			response.Error(w, http.StatusUnprocessableEntity, "EXCHANGE_NOT_CONFIGURED", "No exchange connection configured. Please set up API Key before creating a bot.")
		case errors.Is(err, logic.ErrBotAPIKeyInvalid):
			response.Error(w, http.StatusUnprocessableEntity, "EXCHANGE_NOT_CONFIGURED", "API Key is not in Connected status. Please verify configuration.")
		case errors.Is(err, logic.ErrBotInvalidLogicJSON):
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_LOGIC_JSON", "Strategy logic is invalid or missing event trigger block.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"message": "Bot has been created and is running.",
		"data":    created,
	})
}

// Get handles GET /api/v1/bots/{id}.
//
// Flow (WBS 2.7.5, api.yaml §GET /bots/{id}):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter via chi.URLParam.
//  3. Delegate to BotLogic.GetBotDetail (JOINs strategy name, fetches position/orders
//     from Binance API — deferred to Task 2.8.4, returns nil/empty for now).
//  4. Return 200 { data: BotDetail }.
//
// Success       → 200  { data: BotDetail }
// Bot not found → 404  BOT_NOT_FOUND
// Auth ✗        → 401  UNAUTHORIZED
// Server ✗      → 500  INTERNAL_ERROR
func (h *BotHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	botID := chi.URLParam(r, "id")
	if botID == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Bot ID is required.")
		return
	}

	detail, err := h.botLogic.GetBotDetail(r.Context(), botID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrBotNotFound):
			response.Error(w, http.StatusNotFound, "BOT_NOT_FOUND", "Bot not found.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": detail,
	})
}

// Delete handles DELETE /api/v1/bots/{id}.
//
// Flow (WBS 2.7.5, api.yaml §DELETE /bots/{id}):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter via chi.URLParam.
//  3. Delegate to BotLogic.DeleteBot (enforces status != Running constraint).
//  4. Return 200 { message }.
//
// Success            → 200  { message: "Bot deleted successfully." }
// Bot not found      → 404  BOT_NOT_FOUND
// Bot still running  → 409  BOT_STILL_RUNNING
// Auth ✗             → 401  UNAUTHORIZED
// Server ✗           → 500  INTERNAL_ERROR
func (h *BotHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	botID := chi.URLParam(r, "id")
	if botID == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Bot ID is required.")
		return
	}

	err := h.botLogic.DeleteBot(r.Context(), botID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrBotNotFound):
			response.Error(w, http.StatusNotFound, "BOT_NOT_FOUND", "Bot not found.")
		case errors.Is(err, logic.ErrBotStillRunning):
			response.Error(w, http.StatusConflict, "BOT_STILL_RUNNING", "Cannot delete bot while running. Please stop bot first.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Bot deleted successfully.",
	})
}

// Start handles POST /api/v1/bots/{id}/start.
//
// Restarts a stopped bot by rebuilding its goroutine from the pinned
// strategy version stored at creation time (Data Integrity).
//
// Flow (WBS 2.7.6, api.yaml §POST /bots/{id}/start, SRS FR-RUN-06):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter.
//  3. Delegate to BotLogic.StartBot.
//  4. Return 200 { message, data: BotStatusUpdate }.
//
// Success              → 200  { message, data: { id, status, updated_at } }
// Bot not found        → 404  BOT_NOT_FOUND
// Bot already running  → 409  BOT_ALREADY_RUNNING
// Auth ✗               → 401  UNAUTHORIZED
// Server ✗             → 500  INTERNAL_ERROR
func (h *BotHandler) Start(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	botID := chi.URLParam(r, "id")
	if botID == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Bot ID is required.")
		return
	}

	result, err := h.botLogic.StartBot(r.Context(), botID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrBotNotFound):
			response.Error(w, http.StatusNotFound, "BOT_NOT_FOUND", "Bot not found.")
		case errors.Is(err, logic.ErrBotAlreadyRunning):
			response.Error(w, http.StatusConflict, "BOT_ALREADY_RUNNING", "Bot is already in Running state.")
		case errors.Is(err, logic.ErrBotAPIKeyNotConfigured), errors.Is(err, logic.ErrBotAPIKeyInvalid):
			response.Error(w, http.StatusUnprocessableEntity, "EXCHANGE_NOT_CONFIGURED", "Exchange API key is not configured or not connected.")
		case errors.Is(err, logic.ErrBotInvalidLogicJSON):
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_LOGIC_JSON", "Strategy logic is invalid or missing event trigger block.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message": "Bot started successfully.",
		"data":    result,
	})
}

// stopBotRequest is the optional JSON body for POST /bots/{id}/stop.
type stopBotRequest struct {
	// ClosePosition controls whether to close all Binance positions and cancel
	// open orders before stopping the bot goroutine (api.yaml §StopBotRequest).
	// Defaults to false when omitted — bot is stopped but positions are kept.
	ClosePosition bool `json:"close_position"`
}

// Stop handles POST /api/v1/bots/{id}/stop.
//
// Stops a running bot. Supports two modes via the close_position flag:
//   - false (default): stop goroutine only; Binance positions are untouched.
//   - true: cancel open orders + close position, then stop goroutine.
//
// Flow (WBS 2.7.6, api.yaml §POST /bots/{id}/stop, SRS FR-RUN-06):
//  1. Extract JWT claims from context.
//  2. Read {id} path parameter.
//  3. Decode optional JSON body; default close_position=false.
//  4. Delegate to BotLogic.StopBot.
//  5. Return 200 { message, data: BotStopResult }.
//
// Success          → 200  { message, data: { id, status, total_pnl, updated_at } }
// Bot not found    → 404  BOT_NOT_FOUND
// Bot not running  → 409  BOT_NOT_RUNNING
// Auth ✗           → 401  UNAUTHORIZED
// Server ✗         → 500  INTERNAL_ERROR
func (h *BotHandler) Stop(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	botID := chi.URLParam(r, "id")
	if botID == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Bot ID is required.")
		return
	}

	// Decode optional body — if body is absent or empty, close_position defaults to false.
	var req stopBotRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_JSON_STRUCTURE", "Invalid JSON structure. Please check the request body format.")
			return
		}
	}

	result, err := h.botLogic.StopBot(r.Context(), botID, claims.UserID, req.ClosePosition)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrBotNotFound):
			response.Error(w, http.StatusNotFound, "BOT_NOT_FOUND", "Bot not found.")
		case errors.Is(err, logic.ErrBotNotRunning):
			response.Error(w, http.StatusConflict, "BOT_NOT_RUNNING", "Bot is not in Running state.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		}
		return
	}

	msg := "Bot stopped successfully."
	if req.ClosePosition {
		msg = "Bot stopped and position closed successfully."
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message": msg,
		"data":    result,
	})
}
