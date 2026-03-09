package exchange

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
)

// Gap-filler constants (SRS FR-CORE-03, WBS 2.4.2).
const (
	// gapCheckInterval is how often the worker scans every watched (symbol, interval)
	// pair for temporal holes in candles_data. 5 minutes balances freshness against
	// unnecessary REST calls (each check costs 1 API weight unit per pair).
	gapCheckInterval = 5 * time.Minute

	// restFetchLimit is the maximum number of klines Binance returns per REST call
	// and also the GORM CreateInBatches page size (SRS FR-CORE-03).
	restFetchLimit = 1000
)

// intervalDurations maps Binance kline interval strings to their time.Duration
// equivalents. Used to compute gap boundaries and advance the page cursor.
var intervalDurations = map[string]time.Duration{
	domain.CandleInterval1m:  1 * time.Minute,
	domain.CandleInterval5m:  5 * time.Minute,
	domain.CandleInterval15m: 15 * time.Minute,
	domain.CandleInterval1h:  1 * time.Hour,
	domain.CandleInterval4h:  4 * time.Hour,
	domain.CandleInterval1d:  24 * time.Hour,
	// Extended set for bot engine / backtest use-cases.
	"3m":  3 * time.Minute,
	"30m": 30 * time.Minute,
	"2h":  2 * time.Hour,
	"6h":  6 * time.Hour,
	"8h":  8 * time.Hour,
	"12h": 12 * time.Hour,
	"3d":  72 * time.Hour,
	"1w":  7 * 24 * time.Hour,
}

// watchEntry holds the (symbol, interval) components for iteration.
type watchEntry struct {
	symbol   string
	interval string
}

// GapFillerWorker is a background goroutine that periodically detects and
// repairs temporal gaps in the candles_data table caused by WebSocket
// disconnections, server restarts, or transient network failures
// (SRS FR-CORE-03, WBS 2.4.2).
//
// # Algorithm (per watched pair on each tick)
//
//  1. FindLatest(symbol, interval) — db.First() to get latest stored open_time.
//     If no record exists, the WS seed (task 2.4.1) will handle it — skip.
//  2. Compute gap boundaries:
//     gapStart        = latestOpenTime + intervalDuration
//     expectedLatest  = now.UTC().Truncate(duration) - duration  (last closed candle)
//     If gapStart > expectedLatest → no gap, skip.
//  3. Page through [gapStart, expectedLatest] fetching restFetchLimit=1000 rows
//     per REST call, rate-gated through ExchangeRateLimiter (WBS 2.2.5).
//  4. InsertBatch(CreateInBatches 1000) with ON CONFLICT DO NOTHING.
//  5. Break when Binance returns fewer than restFetchLimit rows (end of gap).
//
// # Concurrency
//
// watchList is guarded by a RWMutex so AddWatch/RemoveWatch from concurrent
// bot goroutines never race with the background scan (NFR-PERF-03).
type GapFillerWorker struct {
	candleRepo    repository.CandleRepository
	limiter       *ExchangeRateLimiter
	checkInterval time.Duration

	mu        sync.RWMutex
	watchList map[string]watchEntry // subscriptionKey → (symbol, interval)
}

// NewGapFillerWorker constructs a GapFillerWorker with the default 5-minute
// check interval.
//
// The candleRepo and limiter are the same singletons used by KlineSyncService
// (WBS 2.4.1) — no additional connections are opened.
func NewGapFillerWorker(candleRepo repository.CandleRepository, limiter *ExchangeRateLimiter) *GapFillerWorker {
	return &GapFillerWorker{
		candleRepo:    candleRepo,
		limiter:       limiter,
		checkInterval: gapCheckInterval,
		watchList:     make(map[string]watchEntry),
	}
}

// AddWatch registers a (symbol, interval) pair for gap monitoring.
//
// Typically called by router.Setup at startup for all configured MARKET_SYMBOLS
// (WBS 2.4.3) and by KlineSyncService.Subscribe when a new WS stream is activated
// (WBS 2.7.2). Calling AddWatch for an already-watched pair is a no-op.
func (w *GapFillerWorker) AddWatch(symbol, interval string) {
	key := subscriptionKey(symbol, interval)
	w.mu.Lock()
	w.watchList[key] = watchEntry{symbol: symbol, interval: interval}
	w.mu.Unlock()
}

// RemoveWatch deregisters a (symbol, interval) pair.
//
// Called when the corresponding WS subscription is stopped (Unsubscribe /
// UnsubscribeAll) to avoid unnecessary REST calls for inactive pairs.
func (w *GapFillerWorker) RemoveWatch(symbol, interval string) {
	key := subscriptionKey(symbol, interval)
	w.mu.Lock()
	delete(w.watchList, key)
	w.mu.Unlock()
}

// Start launches the background gap-filling loop. It blocks until ctx is
// cancelled (SIGTERM from Docker stop — SRS FR-CORE-05).
//
// Call as a goroutine from the application entry-point:
//
//	go gapFiller.Start(ctx)
func (w *GapFillerWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	slog.Info("gap_filler: started", "component", "gap_filler", "check_interval", w.checkInterval)

	for {
		select {
		case <-ticker.C:
			w.runAllGapFills(ctx)
		case <-ctx.Done():
			slog.Info("gap_filler: context cancelled, stopping", "component", "gap_filler")
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// runAllGapFills iterates the current watchList snapshot and calls fillGap
// for each pair. Errors are logged and do not abort the iteration — a failure
// on BTCUSDT_1m should not prevent ETHUSDT_1h from being checked.
func (w *GapFillerWorker) runAllGapFills(ctx context.Context) {
	// Take a snapshot under RLock so AddWatch/RemoveWatch don't block the scan.
	w.mu.RLock()
	entries := make([]watchEntry, 0, len(w.watchList))
	for _, e := range w.watchList {
		entries = append(entries, e)
	}
	w.mu.RUnlock()

	for _, e := range entries {
		if ctx.Err() != nil {
			return // application shutting down
		}
		if err := w.fillGap(ctx, e.symbol, e.interval); err != nil {
			slog.Warn("gap_filler: fillGap failed", "symbol", e.symbol, "interval", e.interval, "error", err)
		}
	}
}

// fillGap detects and repairs a single (symbol, interval) gap.
//
// It is idempotent: running it when there is no gap is a no-op (only one
// db.First() call is made before returning early).
func (w *GapFillerWorker) fillGap(ctx context.Context, symbol, interval string) error {
	duration, err := parseIntervalDuration(interval)
	if err != nil {
		return err
	}

	// --- 1. db.First() check: find the latest stored candle -------------------
	latest, err := w.candleRepo.FindLatest(ctx, symbol, interval)
	if err != nil {
		return fmt.Errorf("gap_filler: fillGap: FindLatest: %w", err)
	}
	if latest == nil {
		// No data in DB at all — the WS seed (task 2.4.1) handles initial seeding.
		return nil
	}

	// --- 2. Compute gap boundaries --------------------------------------------
	//
	// gapStart:       the open_time of the FIRST missing candle.
	// expectedLatest: the open_time of the last candle that should be closed by now.
	//   = now.Truncate(duration) - duration
	//   Example (1m, now=12:07:45):
	//     now.Truncate(1m)  = 12:07:00
	//     expectedLatest    = 12:06:00   (last fully-closed 1m candle)
	gapStart := latest.OpenTime.Add(duration)
	expectedLatest := time.Now().UTC().Truncate(duration).Add(-duration)

	if !gapStart.Before(expectedLatest) {
		return nil // no gap
	}

	slog.Info("gap_filler: filling gap",
		"symbol", symbol, "interval", interval,
		"from", gapStart.Format(time.RFC3339),
		"to", expectedLatest.Format(time.RFC3339))

	// --- 3. Page through the gap via REST, 1000 candles per page --------------
	currentStart := gapStart
	totalInserted := 0

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Rate-gate every REST call through the shared Token Bucket (WBS 2.2.5).
		if err := w.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("gap_filler: fillGap: rate limiter: %w", err)
		}

		// Public endpoint — kline history does not require API credentials.
		publicClient := futures.NewClient("", "")
		klines, err := publicClient.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(currentStart.UnixMilli()).
			EndTime(expectedLatest.Add(duration).UnixMilli()). // inclusive upper bound
			Limit(restFetchLimit).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("gap_filler: fillGap: NewKlinesService(%s, %s, start=%s): %w",
				symbol, interval, currentStart.Format(time.RFC3339), err)
		}
		if len(klines) == 0 {
			break // nothing returned — gap is filled or Binance has no further data
		}

		// --- 4. Batch insert (CreateInBatches 1000, ON CONFLICT DO NOTHING) ----
		candles := buildBatchFromREST(klines, symbol, interval)
		if err := w.candleRepo.InsertBatch(ctx, candles); err != nil {
			return fmt.Errorf("gap_filler: fillGap: InsertBatch: %w", err)
		}
		totalInserted += len(candles)

		// --- 5. Advance cursor or break when end of gap is reached ------------
		if len(klines) < restFetchLimit {
			break // last page — gap fully filled
		}
		// Advance to the open_time of the next expected candle after the last
		// returned row so the next page starts without overlap.
		lastOpenTime := time.UnixMilli(klines[len(klines)-1].OpenTime).UTC()
		currentStart = lastOpenTime.Add(duration)

		if !currentStart.Before(expectedLatest) {
			break // cursor has passed the expected boundary
		}
	}

	if totalInserted > 0 {
		slog.Info("gap_filler: filled candles", "symbol", symbol, "interval", interval, "count", totalInserted)
	}
	return nil
}

// parseIntervalDuration converts a Binance kline interval string (e.g. "1m",
// "4h") into the equivalent time.Duration.
//
// Returns an error for unrecognised intervals so callers can skip unknown
// pairs without panicking — defensive coding for future interval additions.
func parseIntervalDuration(interval string) (time.Duration, error) {
	d, ok := intervalDurations[interval]
	if !ok {
		return 0, fmt.Errorf("gap_filler: unknown interval %q", interval)
	}
	return d, nil
}

// buildBatchFromREST converts a slice of go-binance REST Kline rows into
// []*domain.Candle. Symbol and interval are injected from the call parameters
// since the REST response does not echo them back.
//
// All returned candles have IsClosed=true — the REST endpoint only returns
// historically closed klines.
func buildBatchFromREST(klines []*futures.Kline, symbol, interval string) []*domain.Candle {
	candles := make([]*domain.Candle, 0, len(klines))
	for _, k := range klines {
		candles = append(candles, buildCandleFromREST(k, symbol, interval))
	}
	return candles
}
