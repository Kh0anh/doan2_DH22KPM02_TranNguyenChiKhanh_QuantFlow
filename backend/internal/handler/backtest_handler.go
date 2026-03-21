// Package handler — Task 2.6.5: Backtest API HTTP handlers.
//
// backtest_handler.go exposes three endpoints (api.yaml §/backtests):
//
//	POST   /api/v1/backtests              → Create  (201 BacktestCreated)
//	GET    /api/v1/backtests/{id}         → Get     (200 BacktestResult)
//	POST   /api/v1/backtests/{id}/cancel  → Cancel  (200 MessageResponse)
//
// All three endpoints require a valid JWT session cookie (JWTAuth middleware).
// The handler delegates business logic to BacktestLogic and renders responses
// using the shared response package.
//
// WBS: P2-Backend · 13/03/2026
// SRS: FR-RUN-01, FR-RUN-05
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/pkg/response"
	"github.com/shopspring/decimal"
)

// BacktestHandler serves the /backtests endpoint group (WBS 2.6.5).
type BacktestHandler struct {
	backtestLogic *logic.BacktestLogic
}

// NewBacktestHandler constructs a BacktestHandler with its dependencies.
func NewBacktestHandler(backtestLogic *logic.BacktestLogic) *BacktestHandler {
	return &BacktestHandler{backtestLogic: backtestLogic}
}

// createBacktestRequest is the JSON body expected by POST /backtests.
// Monetary fields (initial_capital, fee_rate) are decoded as float64 and
// immediately promoted to decimal.Decimal to preserve precision downstream
// (cursorrules §6.5 — no float64 in financial arithmetic).
//
// Note: Timeframe is NOT part of the request — it is extracted from the
// strategy's logic_json (the root event_on_candle block's TIMEFRAME field)
// by the logic layer. This avoids duplication and prevents mismatches between
// the strategy definition and the backtest configuration.
type createBacktestRequest struct {
	StrategyID     string  `json:"strategy_id"`
	Symbol         string  `json:"symbol"`
	StartTime      string  `json:"start_time"` // RFC 3339
	EndTime        string  `json:"end_time"`   // RFC 3339
	InitialCapital float64 `json:"initial_capital"`
	FeeRate        float64 `json:"fee_rate"`
	MaxUnit        int     `json:"max_unit"` // optional; 0 → engine default (1000)
}

// Create handles POST /api/v1/backtests.
//
// Flow:
//  1. Decode and validate the request body (all required fields present, times parseable).
//  2. Delegate to BacktestLogic.CreateBacktest — spawns the pipeline goroutine
//     and returns immediately with the new backtest_id (FR-RUN-01).
//  3. Respond 201 { backtest_id, status:"processing", created_at }.
//
// Success      → 201  { backtest_id, status, created_at }
// Invalid body → 400  MISSING_REQUIRED_FIELDS
// Not found    → 404  STRATEGY_NOT_FOUND
// Server error → 500  INTERNAL_ERROR
func (h *BacktestHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	var req createBacktestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS", "Invalid JSON body.")
		return
	}

	// Trim all string fields before validation to tolerate accidental whitespace.
	req.StrategyID = strings.TrimSpace(req.StrategyID)
	req.Symbol = strings.TrimSpace(req.Symbol)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)

	if req.StrategyID == "" || req.Symbol == "" ||
		req.StartTime == "" || req.EndTime == "" ||
		req.InitialCapital <= 0 || req.FeeRate < 0 {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS",
			"strategy_id, symbol, start_time, end_time, initial_capital, and fee_rate are required.")
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS",
			"start_time must be a valid RFC 3339 date-time string.")
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS",
			"end_time must be a valid RFC 3339 date-time string.")
		return
	}

	if !endTime.After(startTime) {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS",
			"end_time must be after start_time.")
		return
	}

	input := logic.CreateBacktestInput{
		StrategyID:     req.StrategyID,
		Symbol:         req.Symbol,
		StartTime:      startTime,
		EndTime:        endTime,
		InitialCapital: decimal.NewFromFloat(req.InitialCapital),
		FeeRate:        decimal.NewFromFloat(req.FeeRate),
		MaxUnit:        req.MaxUnit,
	}

	job, err := h.backtestLogic.CreateBacktest(r.Context(), claims.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, logic.ErrStrategyNotFound):
			response.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND",
				"Strategy not found or does not belong to the current user.")
		case errors.Is(err, logic.ErrBacktestStrategyInvalid):
			response.Error(w, http.StatusUnprocessableEntity, "STRATEGY_INVALID",
				"Strategy status is Draft. Please save the strategy with Valid status before backtesting.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
				"An internal error occurred. Please try again later.")
		}
		return
	}

	snap := job.Snapshot()
	response.JSON(w, http.StatusCreated, map[string]any{
		"backtest_id": snap.ID,
		"status":      snap.Status,
		"created_at":  snap.CreatedAt,
	})
}

// Get handles GET /api/v1/backtests/{id}.
//
// Response shape varies by status (api.yaml §BacktestResult):
//   - processing → { backtest_id, status, progress }
//   - completed  → full BacktestResult (config, summary, equity_curve, trades, timestamps)
//   - canceled   → { backtest_id, status, created_at, completed_at }
//
// Success   → 200  BacktestResult
// Not found → 404  BACKTEST_NOT_FOUND
func (h *BacktestHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	backtestID := chi.URLParam(r, "id")
	if backtestID == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS", "backtest id is required.")
		return
	}

	job, err := h.backtestLogic.GetBacktest(r.Context(), backtestID, claims.UserID)
	if err != nil {
		if errors.Is(err, logic.ErrBacktestNotFound) {
			response.Error(w, http.StatusNotFound, "BACKTEST_NOT_FOUND",
				"Backtest session not found.")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"An internal error occurred. Please try again later.")
		return
	}

	response.JSON(w, http.StatusOK, buildBacktestResult(job.Snapshot()))
}

// Cancel handles POST /api/v1/backtests/{id}/cancel.
//
// Sends a cancellation signal to the running pipeline goroutine via context
// cancellation. The goroutine detects ctx.Done() at the next blocking engine
// call and transitions the job status to "canceled".
//
// Success      → 200  { message: "Backtest canceled." }
// Not found    → 404  BACKTEST_NOT_FOUND
// Already done → 409  BACKTEST_ALREADY_DONE
func (h *BacktestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	backtestID := chi.URLParam(r, "id")
	if backtestID == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS", "backtest id is required.")
		return
	}

	if err := h.backtestLogic.CancelBacktest(r.Context(), backtestID, claims.UserID); err != nil {
		switch {
		case errors.Is(err, logic.ErrBacktestNotFound):
			response.Error(w, http.StatusNotFound, "BACKTEST_NOT_FOUND",
				"Backtest session not found.")
		case errors.Is(err, logic.ErrBacktestAlreadyDone):
			response.Error(w, http.StatusConflict, "BACKTEST_ALREADY_DONE",
				"Backtest session has already completed or been canceled.")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
				"An internal error occurred. Please try again later.")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Backtest canceled.",
	})
}

// ─────────────────────────────────────────────────────────────────────────────
//  Response builder
// ─────────────────────────────────────────────────────────────────────────────

// buildBacktestResult assembles the GET /backtests/{id} response body from a
// BacktestSnapshot. The shape varies by status as defined in api.yaml §BacktestResult:
//
//   processing → { backtest_id, status, progress }
//   completed  → full result with config, summary, equity_curve, trades, timestamps
//   canceled   → { backtest_id, status, created_at, completed_at }
func buildBacktestResult(snap logic.BacktestSnapshot) map[string]any {
	switch snap.Status {
	case logic.BacktestStatusProcessing:
		return map[string]any{
			"backtest_id": snap.ID,
			"status":      snap.Status,
			"progress":    snap.Progress,
		}

	case logic.BacktestStatusCompleted:
		return map[string]any{
			"backtest_id":  snap.ID,
			"status":       snap.Status,
			"config":       snap.Config,
			"summary":      snap.Summary,
			"equity_curve": snap.EquityCurve,
			"trades":       snap.Trades,
			"created_at":   snap.CreatedAt,
			"completed_at": snap.CompletedAt,
		}

	case logic.BacktestStatusFailed:
		return map[string]any{
			"backtest_id":   snap.ID,
			"status":        snap.Status,
			"error_message": snap.ErrorMessage,
			"created_at":    snap.CreatedAt,
			"completed_at":  snap.CompletedAt,
		}

	default: // canceled
		return map[string]any{
			"backtest_id":  snap.ID,
			"status":       snap.Status,
			"created_at":   snap.CreatedAt,
			"completed_at": snap.CompletedAt,
		}
	}
}
