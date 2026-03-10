package handler

import (
	"net/http"
	"strings"

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
