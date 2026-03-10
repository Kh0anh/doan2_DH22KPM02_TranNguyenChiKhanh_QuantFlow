package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kh0anh/quantflow/internal/logic"
	"github.com/kh0anh/quantflow/pkg/response"
)

// MarketHandler groups HTTP handlers for the /market route group
// (WBS 2.4.3-2.4.4, api.yaml §Market tag).
type MarketHandler struct {
	marketLogic    *logic.MarketLogic
	watchedSymbols []string
}

// NewMarketHandler constructs a MarketHandler.
//
// Parameters:
//   - marketLogic    — business logic layer for market data operations.
//   - watchedSymbols — the WATCHED_SYMBOLS list from config, passed through to
//     MarketLogic on every request so the handler itself stays stateless w.r.t.
//     Binance API calls.
func NewMarketHandler(marketLogic *logic.MarketLogic, watchedSymbols []string) *MarketHandler {
	return &MarketHandler{
		marketLogic:    marketLogic,
		watchedSymbols: watchedSymbols,
	}
}

// ListSymbols handles GET /api/v1/market/symbols (WBS 2.4.3).
//
// Query parameters:
//   - search  string  optional — case-insensitive substring filter on symbol name.
//
// The handler delegates to MarketLogic.ListMarketSymbols which fetches live
// 24-hour ticker data from Binance Futures (public endpoint) and filters to
// the platform's watched symbols.
//
// Success  → 200  { data: []MarketSymbol }
// Auth ✗   → 401  UNAUTHORIZED  (handled by JWTAuth middleware)
// Server ✗ → 500  INTERNAL_ERROR
func (h *MarketHandler) ListSymbols(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	symbols, err := h.marketLogic.ListMarketSymbols(r.Context(), h.watchedSymbols, search)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to fetch market data. Please try again later.")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": symbols,
	})
}

// validTimeframes is the set of timeframe strings accepted by GET /market/candles
// (api.yaml §getMarketCandles parameters). Values match domain.CandleInterval* constants.
var validTimeframes = map[string]struct{}{
	"1m": {}, "5m": {}, "15m": {}, "1h": {}, "4h": {}, "1d": {},
}

// maxCandleLimit is the hard cap on the limit query parameter (api.yaml max: 1500).
const maxCandleLimit = 1500

// defaultCandleLimit is the default number of candles when limit is not supplied.
const defaultCandleLimit = 500

// GetCandles handles GET /api/v1/market/candles (WBS 2.4.4).
//
// Query parameters:
//   - symbol    string  required — Binance Futures pair, e.g. "BTCUSDT".
//   - timeframe string  required — one of: 1m, 5m, 15m, 1h, 4h, 1d (also accepts "1D").
//   - start     string  optional — ISO 8601 lower bound on open_time.
//   - end       string  optional — ISO 8601 upper bound on open_time.
//   - limit     integer optional — max candles to return, default 500, max 1500.
//
// Success  → 200  { "data": CandleChartData }
// Bad req  → 400  MISSING_REQUIRED_FIELDS | INVALID_TIMEFRAME | INVALID_PARAM
// Auth ✗   → 401  (handled by JWTAuth middleware)
// Server ✗ → 500  INTERNAL_ERROR
func (h *MarketHandler) GetCandles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// ── Required: symbol ────────────────────────────────────────────────────
	symbol := strings.ToUpper(strings.TrimSpace(q.Get("symbol")))
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS",
			"Required parameters 'symbol' and 'timeframe' must be provided.")
		return
	}

	// ── Required: timeframe ─────────────────────────────────────────────────
	// Normalise "1D" → "1d" to match the domain constant (api.yaml allows "1D").
	timeframe := strings.ToLower(strings.TrimSpace(q.Get("timeframe")))
	if timeframe == "" {
		response.Error(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS",
			"Required parameters 'symbol' and 'timeframe' must be provided.")
		return
	}
	if _, ok := validTimeframes[timeframe]; !ok {
		response.Error(w, http.StatusBadRequest, "INVALID_TIMEFRAME",
			"Invalid timeframe. Allowed values: 1m, 5m, 15m, 1h, 4h, 1d.")
		return
	}

	// ── Optional: start / end (ISO 8601) ─────────────────────────────────────
	var start, end *time.Time
	if raw := strings.TrimSpace(q.Get("start")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_PARAM",
				"Invalid 'start' parameter. Use ISO 8601 format (RFC3339), e.g. 2026-01-01T00:00:00Z.")
			return
		}
		start = &t
	}
	if raw := strings.TrimSpace(q.Get("end")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_PARAM",
				"Invalid 'end' parameter. Use ISO 8601 format (RFC3339), e.g. 2026-01-01T00:00:00Z.")
			return
		}
		end = &t
	}

	// ── Optional: limit ──────────────────────────────────────────────────────
	limit := defaultCandleLimit
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			response.Error(w, http.StatusBadRequest, "INVALID_PARAM",
				"Invalid 'limit' parameter. Must be a positive integer (1–1500).")
			return
		}
		if n > maxCandleLimit {
			n = maxCandleLimit
		}
		limit = n
	}

	// ── Business logic ───────────────────────────────────────────────────────
	data, err := h.marketLogic.GetCandleChart(r.Context(), symbol, timeframe, start, end, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to load candle data. Please try again later.")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": data,
	})
}
