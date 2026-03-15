package handler

// trade_handler.go — HTTP handlers for GET /trades and GET /trades/export.
//
// ─── Authentication ───────────────────────────────────────────────────────────
//
// Both handlers are mounted under the JWTAuth middleware group (router.go).
// User identity is extracted from ClaimsFromContext — same pattern as BotHandler.
//
// Task 2.8.5 — Trade History APIs.
// WBS: P2-Backend · 15/03/2026.
// api.yaml: §GET /trades, §GET /trades/export.

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kh0anh/quantflow/internal/logic"
	appMiddleware "github.com/kh0anh/quantflow/internal/middleware"
	"github.com/kh0anh/quantflow/pkg/response"
)

// TradeHandler handles HTTP requests for the Trades API (/trades, /trades/export).
type TradeHandler struct {
	tradeLogic *logic.TradeLogic
}

// NewTradeHandler constructs a TradeHandler with its logic dependency.
func NewTradeHandler(tradeLogic *logic.TradeLogic) *TradeHandler {
	return &TradeHandler{tradeLogic: tradeLogic}
}

// ─── GET /trades ──────────────────────────────────────────────────────────────

// List handles GET /api/v1/trades — returns cursor-paginated, filtered trade history
// (api.yaml §GET /trades, WBS 2.8.5).
//
// Query parameters (all optional):
//   - bot_id:     UUID string
//   - symbol:     string (e.g. "BTCUSDT")
//   - side:       "Long" | "Short"
//   - status:     "Filled" | "Canceled"
//   - start_date: ISO 8601 datetime (RFC3339)
//   - end_date:   ISO 8601 datetime (RFC3339)
//   - cursor:     base64url compound cursor from previous response
//   - limit:      integer [1,200], default 50
func (h *TradeHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	q := r.URL.Query()

	input := logic.TradeListInput{
		BotID:  q.Get("bot_id"),
		Symbol: q.Get("symbol"),
		Side:   q.Get("side"),
		Status: q.Get("status"),
		Cursor: q.Get("cursor"),
		Limit:  50, // default — overridden below if provided
	}

	// Parse limit, clamp handled inside TradeLogic.
	if limitStr := q.Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			input.Limit = v
		}
	}

	// Parse start_date (RFC3339).
	if sd := q.Get("start_date"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			utc := t.UTC()
			input.StartDate = &utc
		}
	}

	// Parse end_date (RFC3339).
	if ed := q.Get("end_date"); ed != "" {
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			utc := t.UTC()
			input.EndDate = &utc
		}
	}

	result, err := h.tradeLogic.ListTrades(r.Context(), claims.UserID, input)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred. Please try again later.")
		return
	}

	// Build CursorPagination envelope matching api.yaml §CursorPagination.
	type cursorPagination struct {
		NextCursor *string `json:"next_cursor"`
		HasMore    bool    `json:"has_more"`
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": result.Data,
		"pagination": cursorPagination{
			NextCursor: result.NextCursor,
			HasMore:    result.HasMore,
		},
	})
}

// ─── GET /trades/export ───────────────────────────────────────────────────────

// Export handles GET /api/v1/trades/export — streams trade history as a CSV file
// (api.yaml §GET /trades/export, WBS 2.8.5).
//
// Query parameters (all optional):
//   - bot_id:     UUID string
//   - symbol:     string
//   - start_date: ISO 8601 datetime (RFC3339)
//   - end_date:   ISO 8601 datetime (RFC3339)
func (h *TradeHandler) Export(w http.ResponseWriter, r *http.Request) {
	claims, ok := appMiddleware.ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "No active session.")
		return
	}

	q := r.URL.Query()

	filter := logic.TradeExportFilter{
		BotID:  q.Get("bot_id"),
		Symbol: q.Get("symbol"),
	}

	// Parse start_date (RFC3339).
	if sd := q.Get("start_date"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			utc := t.UTC()
			filter.StartDate = &utc
		}
	}

	// Parse end_date (RFC3339).
	if ed := q.Get("end_date"); ed != "" {
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			utc := t.UTC()
			filter.EndDate = &utc
		}
	}

	// Set CSV response headers before streaming begins.
	// Filename includes today's UTC date for easy identification.
	today := time.Now().UTC().Format("20060102")
	filename := fmt.Sprintf("trade-history-%s.csv", today)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := h.tradeLogic.ExportTradesCSV(r.Context(), claims.UserID, filter, w); err != nil {
		// CSV headers already sent — cannot emit a JSON error.
		// Append a comment so the partial file is still recognisable.
		_, _ = fmt.Fprintf(w, "\n# export error: internal server error\n")
	}
}
