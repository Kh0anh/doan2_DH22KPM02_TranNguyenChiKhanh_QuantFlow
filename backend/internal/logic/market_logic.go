package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/exchange"
)

// MarketLogic implements the business rules for the Market data endpoints
// (WBS 2.4.3-2.4.4). It fetches live ticker data from Binance Futures public
// REST API and filters results to the platform's watched symbol set.
type MarketLogic struct {
	limiter *exchange.ExchangeRateLimiter
}

// NewMarketLogic constructs a MarketLogic.
//
// The provided limiter is the singleton ExchangeRateLimiter shared across all
// Binance REST callers to enforce the global IP weight cap (WBS 2.2.5).
func NewMarketLogic(limiter *exchange.ExchangeRateLimiter) *MarketLogic {
	return &MarketLogic{limiter: limiter}
}

// ListMarketSymbols returns 24-hour ticker summaries for every symbol in
// watchedSymbols, optionally filtered by a case-insensitive search substring.
//
// Data source: Binance Futures public endpoint GET /fapi/v1/ticker/24hr
// (go-binance NewListPriceChangeStatsService). This is a public call — no
// API key is required. The call is rate-gated through the shared limiter
// because it still consumes Binance IP weight (weight=40 for all symbols).
//
// Parameters:
//   - ctx            — request context; cancellation aborts the Binance call.
//   - watchedSymbols — the WATCHED_SYMBOLS list from config (e.g. ["BTCUSDT","ETHUSDT"]).
//   - search         — optional substring filter (e.g. "BTC"). Empty = no filter.
func (l *MarketLogic) ListMarketSymbols(
	ctx context.Context,
	watchedSymbols []string,
	search string,
) ([]domain.MarketSymbol, error) {

	// Build a lookup set so filtering is O(1) per ticker result.
	watchedSet := make(map[string]struct{}, len(watchedSymbols))
	for _, s := range watchedSymbols {
		watchedSet[strings.ToUpper(s)] = struct{}{}
	}

	// Rate-gate before the outbound REST call to respect the shared Binance
	// Futures IP weight cap (SRS FR-CORE-04, WBS 2.2.5).
	if err := l.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("market_logic: ListMarketSymbols: rate limiter: %w", err)
	}

	// Public endpoint — empty API key / secret is intentional.
	tickers, err := futures.NewClient("", "").
		NewListPriceChangeStatsService().
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("market_logic: ListMarketSymbols: Binance 24hr ticker: %w", err)
	}

	searchUpper := strings.ToUpper(strings.TrimSpace(search))

	result := make([]domain.MarketSymbol, 0, len(watchedSymbols))
	for _, t := range tickers {
		sym := strings.ToUpper(t.Symbol)

		// Only include symbols the platform actively watches.
		if _, ok := watchedSet[sym]; !ok {
			continue
		}

		// Apply optional search filter (case-insensitive substring match).
		if searchUpper != "" && !strings.Contains(sym, searchUpper) {
			continue
		}

		result = append(result, domain.MarketSymbol{
			Symbol:             t.Symbol,
			LastPrice:          t.LastPrice,
			PriceChangePercent: t.PriceChangePercent,
			// QuoteVolume is the 24h volume denominated in the quote asset (USDT),
			// which is the financially meaningful figure for Futures pairs.
			Volume24h: t.QuoteVolume,
		})
	}

	return result, nil
}
