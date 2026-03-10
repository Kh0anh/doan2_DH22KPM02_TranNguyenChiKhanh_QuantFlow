package logic

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/exchange"
	"github.com/kh0anh/quantflow/internal/repository"
)

// MarketLogic implements the business rules for the Market data endpoints
// (WBS 2.4.3-2.4.4). It fetches live ticker data from Binance Futures public
// REST API and filters results to the platform's watched symbol set.
type MarketLogic struct {
	limiter    *exchange.ExchangeRateLimiter
	candleRepo repository.CandleRepository
	markerRepo repository.TradeMarkerRepository
}

// NewMarketLogic constructs a MarketLogic.
//
// Parameters:
//   - limiter     — singleton ExchangeRateLimiter shared across all Binance REST callers (WBS 2.2.5).
//   - candleRepo  — candle data access; used for DB-first lookup and on-demand sync (WBS 2.4.4).
//   - markerRepo  — trade marker access; used to attach bot executions on the chart (WBS 2.4.4).
func NewMarketLogic(
	limiter *exchange.ExchangeRateLimiter,
	candleRepo repository.CandleRepository,
	markerRepo repository.TradeMarkerRepository,
) *MarketLogic {
	return &MarketLogic{
		limiter:    limiter,
		candleRepo: candleRepo,
		markerRepo: markerRepo,
	}
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

// onDemandSyncLimit is the maximum number of candles fetched from Binance REST
// during a single on-demand sync triggered by GET /market/candles (WBS 2.4.4).
// Capped at the Binance Futures API maximum per request.
const onDemandSyncLimit = 1500

// GetCandleChart is the core business logic for GET /market/candles (WBS 2.4.4).
//
// It follows a DB-first strategy:
//  1. Query candles_data for the requested (symbol, interval, range, limit).
//  2. If the DB returns 0 rows → perform an On-demand Sync: call Binance REST
//     NewKlinesService, persist the result, then re-query the DB.
//  3. Query trade markers from trade_history JOIN bot_instances for the same
//     time window and attach them to the response.
//
// Parameters:
//   - symbol    — Binance Futures pair, e.g. "BTCUSDT".
//   - interval  — candle timeframe, one of the domain.CandleInterval* constants.
//   - start,end — optional time range (both nil = use limit-based default).
//   - limit     — max candles to return (1–1500).
func (l *MarketLogic) GetCandleChart(
	ctx context.Context,
	symbol, interval string,
	start, end *time.Time,
	limit int,
) (*domain.CandleChartData, error) {

	// ── Step 1: DB-first lookup ──────────────────────────────────────────────
	candles, err := l.candleRepo.QueryCandles(ctx, symbol, interval, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("market_logic: GetCandleChart: QueryCandles: %w", err)
	}

	// ── Step 2: On-demand Sync when DB has no data ───────────────────────────
	if len(candles) == 0 {
		slog.Info("market_logic: on-demand sync triggered",
			"symbol", symbol,
			"interval", interval,
		)

		if err := l.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("market_logic: GetCandleChart: rate limiter: %w", err)
		}

		syncLimit := limit
		if syncLimit < onDemandSyncLimit {
			syncLimit = onDemandSyncLimit
		}

		svc := futures.NewClient("", "").NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			Limit(syncLimit)

		if start != nil {
			svc = svc.StartTime(start.UnixMilli())
		}
		if end != nil {
			svc = svc.EndTime(end.UnixMilli())
		}

		klines, err := svc.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("market_logic: GetCandleChart: Binance NewKlinesService: %w", err)
		}

		if len(klines) > 0 {
			batch := make([]domain.Candle, 0, len(klines))
			for _, k := range klines {
				batch = append(batch, domain.Candle{
					Symbol:     symbol,
					Interval:   interval,
					OpenTime:   time.UnixMilli(k.OpenTime).UTC(),
					OpenPrice:  k.Open,
					HighPrice:  k.High,
					LowPrice:   k.Low,
					ClosePrice: k.Close,
					Volume:     k.Volume,
					IsClosed:   true,
				})
			}
			if err := l.candleRepo.InsertBatch(ctx, batch); err != nil {
				// Non-fatal: log the error but continue — the candles may already
				// exist (race with GapFiller or WS stream) due to ON CONFLICT DO NOTHING.
				slog.Warn("market_logic: GetCandleChart: InsertBatch failed",
					"symbol", symbol, "interval", interval, "error", err)
			}

			// Re-query from DB so the response reflects what was actually persisted.
			candles, err = l.candleRepo.QueryCandles(ctx, symbol, interval, start, end, limit)
			if err != nil {
				return nil, fmt.Errorf("market_logic: GetCandleChart: re-query after sync: %w", err)
			}
		}
	}

	// ── Step 3: Determine marker time window from candle range ───────────────
	var markerStart, markerEnd time.Time
	if len(candles) > 0 {
		markerStart = candles[0].OpenTime
		markerEnd = candles[len(candles)-1].OpenTime
	} else {
		// No candles available at all (symbol/interval has no data on Binance).
		markerStart = time.Now().UTC().Add(-24 * time.Hour)
		markerEnd = time.Now().UTC()
	}

	// ── Step 4: Fetch trade markers ──────────────────────────────────────────
	markers, err := l.markerRepo.FindMarkersBySymbolAndTimeRange(ctx, symbol, markerStart, markerEnd)
	if err != nil {
		// Non-fatal: render chart without markers rather than failing the
		// entire request, since trade history tables may not yet be seeded.
		slog.Warn("market_logic: GetCandleChart: FindMarkersBySymbolAndTimeRange failed",
			"symbol", symbol, "error", err)
		markers = []domain.TradeMarker{}
	}

	// ── Step 5: Build response payload ───────────────────────────────────────
	candleOHLCV := make([]domain.CandleOHLCV, 0, len(candles))
	for _, c := range candles {
		candleOHLCV = append(candleOHLCV, domain.CandleOHLCV{
			OpenTime: c.OpenTime,
			Open:     parsePrice(c.OpenPrice),
			High:     parsePrice(c.HighPrice),
			Low:      parsePrice(c.LowPrice),
			Close:    parsePrice(c.ClosePrice),
			Volume:   parsePrice(c.Volume),
			IsClosed: c.IsClosed,
		})
	}

	if markers == nil {
		markers = []domain.TradeMarker{}
	}

	return &domain.CandleChartData{
		Symbol:    symbol,
		Timeframe: interval,
		Candles:   candleOHLCV,
		Markers:   markers,
	}, nil
}

// parsePrice converts a PostgreSQL DECIMAL string (e.g. "64500.12345678") to
// float64 for JSON serialisation. Returns 0 on parse error — a malformed price
// is non-fatal and will render at y=0 on the chart rather than crashing.
func parsePrice(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
