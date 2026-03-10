package exchange

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
)

// Gap-filler timing and pagination constants (WBS 2.4.2).
const (
	// gapFillTickInterval controls how often the background worker scans for
	// missing candle ranges. 5 minutes is frequent enough to keep gaps small
	// while avoiding excessive Binance REST calls during normal operation.
	gapFillTickInterval = 5 * time.Minute

	// binanceKlinePageSize is the maximum number of klines returned per REST
	// request by the Binance Futures NewKlinesService endpoint. Using the
	// API maximum (1500) minimises the number of paginated calls needed to
	// backfill large gaps (e.g., after a multi-hour outage).
	binanceKlinePageSize = 1500
)

// GapFillerWorker is a background goroutine that periodically detects and
// fills missing candle ranges in the candles_data table (WBS 2.4.2,
// SRS FR-CORE-02).
//
// Missing candles can occur when the WebSocket stream managed by
// KlineSyncService (WBS 2.4.1) is temporarily disconnected — e.g., during
// a server restart, Docker rebuild, or transient network failure. Without
// gap-filling, the Backtest engine (WBS 2.6.x) would produce inaccurate
// simulations and the Market Chart (WBS 2.4.3) would display visual holes.
//
// Algorithm per (symbol, interval) pair:
//  1. FindLatest → determine the most recent candle already stored.
//  2. If no candle exists → skip (the pair has never been seeded by WBS 2.4.1).
//  3. Compute expectedNextOpenTime = latest.OpenTime + intervalDuration.
//  4. If expectedNextOpenTime >= now → no gap detected, skip.
//  5. Paginate Binance REST NewKlinesService(startTime, endTime, limit=1500).
//  6. Collect klines → InsertBatch (CreateInBatches 1000/batch, ON CONFLICT DO NOTHING).
//  7. Advance startTime past the last fetched kline and repeat until the gap is closed.
//
// Concurrency: the worker runs as a single goroutine — no internal
// parallelism is needed because the Binance IP weight budget is the
// bottleneck, not CPU or DB throughput.
type GapFillerWorker struct {
	candleRepo repository.CandleRepository
	limiter    *ExchangeRateLimiter
	symbols    []string
}

// NewGapFillerWorker constructs a GapFillerWorker.
//
// Parameters:
//   - candleRepo — shared CandleRepository (same instance used by KlineSyncService).
//   - limiter    — singleton ExchangeRateLimiter to respect the Binance IP weight cap.
//   - symbols    — watched symbol list from WATCHED_SYMBOLS env (e.g., ["BTCUSDT","ETHUSDT"]).
func NewGapFillerWorker(
	candleRepo repository.CandleRepository,
	limiter *ExchangeRateLimiter,
	symbols []string,
) *GapFillerWorker {
	return &GapFillerWorker{
		candleRepo: candleRepo,
		limiter:    limiter,
		symbols:    symbols,
	}
}

// Run starts the gap-filling background loop. It ticks every 5 minutes,
// scanning all watched symbols for the "1m" interval (matching the WS
// subscription in StartWatchedSymbols — WBS 2.4.1).
//
// The loop exits cleanly when ctx is cancelled (SIGTERM graceful shutdown —
// SRS FR-CORE-05). Run is designed to be launched as a goroutine:
//
//	go gapFiller.Run(ctx)
func (w *GapFillerWorker) Run(ctx context.Context) {
	// Perform an initial scan immediately on startup so that gaps accumulated
	// during the previous downtime are filled without waiting for the first tick.
	w.scanAll(ctx)

	ticker := time.NewTicker(gapFillTickInterval)
	defer ticker.Stop()

	slog.Info("gap_filler: background worker started",
		"tick_interval", gapFillTickInterval,
		"symbols", w.symbols,
		"interval", domain.CandleInterval1m,
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("gap_filler: shutting down", "reason", ctx.Err())
			return
		case <-ticker.C:
			w.scanAll(ctx)
		}
	}
}

// scanAll iterates over every watched symbol and fills gaps for the 1-minute
// interval. Errors from individual symbols are logged but do not abort the
// scan — one failing symbol must never block the others.
func (w *GapFillerWorker) scanAll(ctx context.Context) {
	for _, sym := range w.symbols {
		if ctx.Err() != nil {
			return
		}
		if err := w.fillGaps(ctx, sym, domain.CandleInterval1m); err != nil {
			slog.Warn("gap_filler: fillGaps failed",
				"symbol", sym,
				"interval", domain.CandleInterval1m,
				"error", err,
			)
		}
	}
}

// fillGaps detects and backfills missing candles for a single (symbol, interval)
// pair by paginating through the Binance Futures REST kline endpoint.
func (w *GapFillerWorker) fillGaps(ctx context.Context, symbol, interval string) error {
	latest, err := w.candleRepo.FindLatest(ctx, symbol, interval)
	if err != nil {
		return fmt.Errorf("FindLatest: %w", err)
	}
	if latest == nil {
		// No seed candle exists — WBS 2.4.1 has not run for this pair yet.
		// Nothing to gap-fill against; skip silently.
		return nil
	}

	dur := intervalDuration(interval)
	gapStart := latest.OpenTime.Add(dur)
	now := time.Now().UTC()

	if !gapStart.Before(now) {
		// The latest candle is current — no gap detected.
		return nil
	}

	slog.Info("gap_filler: gap detected",
		"symbol", symbol,
		"interval", interval,
		"from", gapStart,
		"to", now,
	)

	totalInserted := 0
	cursor := gapStart

	for cursor.Before(now) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Rate-gate before every REST call to respect the shared Binance IP
		// weight cap (SRS FR-CORE-04, WBS 2.2.5). The GapFillerWorker shares
		// the same limiter as BinanceProxy and KlineSyncService.
		if err := w.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter: %w", err)
		}

		// Public endpoint — no API key required for kline history queries.
		publicClient := futures.NewClient("", "")
		klines, err := publicClient.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(cursor.UnixMilli()).
			EndTime(now.UnixMilli()).
			Limit(binanceKlinePageSize).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("REST NewKlinesService(%s, %s): %w", symbol, interval, err)
		}

		if len(klines) == 0 {
			break
		}

		candles := make([]domain.Candle, 0, len(klines))
		for _, k := range klines {
			candles = append(candles, *buildCandleFromREST(k, symbol, interval))
		}

		if err := w.candleRepo.InsertBatch(ctx, candles); err != nil {
			return fmt.Errorf("InsertBatch: %w", err)
		}
		totalInserted += len(candles)

		// Advance cursor past the last fetched kline to avoid re-fetching the
		// same page on the next iteration. Adding intervalDuration guarantees
		// we start from the next expected candle open_time.
		lastOpenTime := time.UnixMilli(klines[len(klines)-1].OpenTime).UTC()
		cursor = lastOpenTime.Add(dur)
	}

	if totalInserted > 0 {
		slog.Info("gap_filler: gap filled",
			"symbol", symbol,
			"interval", interval,
			"candles_inserted", totalInserted,
		)
	}

	return nil
}

// intervalDuration maps a Binance kline interval string to its Go duration.
// Only intervals supported by the system (domain.CandleInterval* constants)
// are included. Unknown intervals default to 1 minute as a safe fallback
// since the gap-filler currently only operates on "1m".
func intervalDuration(interval string) time.Duration {
	switch interval {
	case domain.CandleInterval1m:
		return 1 * time.Minute
	case domain.CandleInterval5m:
		return 5 * time.Minute
	case domain.CandleInterval15m:
		return 15 * time.Minute
	case domain.CandleInterval1h:
		return 1 * time.Hour
	case domain.CandleInterval4h:
		return 4 * time.Hour
	case domain.CandleInterval1d:
		return 24 * time.Hour
	default:
		return 1 * time.Minute
	}
}
