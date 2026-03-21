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

// Gap-filler timing, pagination, and history constants (WBS 2.4.2).
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

	// historicalBackfillDays is the number of calendar days of historical
	// candle data the gap filler will attempt to seed on startup. 30 days
	// provides enough depth for backtests and chart rendering while keeping
	// the initial fetch size manageable (~43 200 candles per symbol for 1m).
	historicalBackfillDays = 30
)

// GapFillerWorker is a background goroutine that periodically detects and
// fills missing candle ranges in the candles_data table (WBS 2.4.2,
// SRS FR-CORE-02).
//
// Missing candles can occur when the WebSocket stream managed by
// MarketTickerChannel (WBS 2.8.2) is temporarily disconnected — e.g., during
// a server restart, Docker rebuild, or transient network failure. Without
// gap-filling, the Backtest engine (WBS 2.6.x) would produce inaccurate
// simulations and the Market Chart (WBS 2.4.3) would display visual holes.
//
// On startup, the worker first runs a historical backfill that seeds up to
// 30 calendar days of data for every (symbol, interval) pair, then enters the
// periodic gap-scan loop covering ALL six supported timeframes.
//
// Algorithm per (symbol, interval) pair:
//  1. FindLatest → determine the most recent candle already stored.
//  2. If no candle exists → backfill from (now − 30 days) to now.
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
//   - candleRepo — shared CandleRepository (same instance used by MarketTickerChannel).
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

// Run starts the gap-filling background loop. On startup it:
//  1. Runs backfillHistorical — seeds up to 30 days of candle data for every
//     (symbol, interval) pair that does not yet have sufficient history.
//  2. Runs scanAll — fills any remaining forward gaps (latest candle → now).
//  3. Enters the periodic ticker loop that calls scanAll every 5 minutes.
//
// The loop exits cleanly when ctx is cancelled (SIGTERM graceful shutdown —
// SRS FR-CORE-05). Run is designed to be launched as a goroutine:
//
//	go gapFiller.Run(ctx)
func (w *GapFillerWorker) Run(ctx context.Context) {
	// Phase 1: seed historical data (30 days) for all (symbol, interval) pairs.
	w.backfillHistorical(ctx)

	// Phase 2: fill forward gaps (latest candle → now) for all pairs.
	w.scanAll(ctx)

	ticker := time.NewTicker(gapFillTickInterval)
	defer ticker.Stop()

	slog.Info("gap_filler: background worker started",
		"tick_interval", gapFillTickInterval,
		"symbols", w.symbols,
		"intervals", domain.AllCandleIntervals,
		"historical_days", historicalBackfillDays,
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

// ---------------------------------------------------------------------------
// Phase 1 — Historical backfill (30 days)
// ---------------------------------------------------------------------------

// backfillHistorical seeds up to 30 calendar days of candle data for every
// watched (symbol, interval) pair.
//
// For each pair:
//   - threshold = now − 30 days (truncated to midnight UTC for clean alignment).
//   - FindOldest: if the oldest candle is already at or before threshold → skip.
//   - Otherwise (no data, or oldest candle is newer): fetch from threshold up to
//     the oldest existing candle (or now, if no data exists).
//
// All REST calls go through the shared ExchangeRateLimiter to respect the
// Binance IP weight cap (SRS FR-CORE-04, WBS 2.2.5).
func (w *GapFillerWorker) backfillHistorical(ctx context.Context) {
	now := time.Now().UTC()
	threshold := now.AddDate(0, 0, -historicalBackfillDays).Truncate(24 * time.Hour)

	slog.Info("gap_filler: backfill historical started",
		"threshold", threshold,
		"symbols", len(w.symbols),
		"intervals", len(domain.AllCandleIntervals),
	)

	for _, sym := range w.symbols {
		for _, interval := range domain.AllCandleIntervals {
			if ctx.Err() != nil {
				return
			}

			oldest, err := w.candleRepo.FindOldest(ctx, sym, interval)
			if err != nil {
				slog.Warn("gap_filler: FindOldest failed",
					"symbol", sym, "interval", interval, "error", err)
				continue
			}

			// Determine the fetch window [fetchStart, fetchEnd).
			var fetchStart, fetchEnd time.Time

			if oldest == nil {
				// No data at all — fetch full 30-day range.
				fetchStart = threshold
				fetchEnd = now
			} else if oldest.OpenTime.After(threshold) {
				// Data exists but does not reach back to threshold.
				fetchStart = threshold
				fetchEnd = oldest.OpenTime // up to existing data
			} else {
				// Oldest candle is already at or before threshold — skip.
				continue
			}

			slog.Info("gap_filler: backfill historical range",
				"symbol", sym,
				"interval", interval,
				"from", fetchStart,
				"to", fetchEnd,
			)

			inserted, err := w.fetchAndInsert(ctx, sym, interval, fetchStart, fetchEnd)
			if err != nil {
				slog.Warn("gap_filler: backfill historical failed",
					"symbol", sym, "interval", interval, "error", err)
				continue
			}

			if inserted > 0 {
				slog.Info("gap_filler: backfill historical completed",
					"symbol", sym,
					"interval", interval,
					"candles_inserted", inserted,
				)
			}
		}
	}

	slog.Info("gap_filler: backfill historical finished for all pairs")
}

// ---------------------------------------------------------------------------
// Phase 2 — Forward gap scan (periodic)
// ---------------------------------------------------------------------------

// scanAll iterates over every watched symbol and fills forward gaps for ALL
// six supported intervals. Errors from individual pairs are logged but do not
// abort the scan — one failing pair must never block the others.
func (w *GapFillerWorker) scanAll(ctx context.Context) {
	for _, sym := range w.symbols {
		for _, interval := range domain.AllCandleIntervals {
			if ctx.Err() != nil {
				return
			}
			if err := w.fillGaps(ctx, sym, interval); err != nil {
				slog.Warn("gap_filler: fillGaps failed",
					"symbol", sym,
					"interval", interval,
					"error", err,
				)
			}
		}
	}
}

// fillGaps detects and backfills missing candles for a single (symbol, interval)
// pair by paginating through the Binance Futures REST kline endpoint.
//
// If no candle exists in the DB for this pair, the method backfills from
// (now − 30 days) to now, ensuring data is always available for backtest and
// chart rendering even on a fresh deployment.
func (w *GapFillerWorker) fillGaps(ctx context.Context, symbol, interval string) error {
	latest, err := w.candleRepo.FindLatest(ctx, symbol, interval)
	if err != nil {
		return fmt.Errorf("FindLatest: %w", err)
	}

	now := time.Now().UTC()

	var gapStart time.Time
	if latest == nil {
		// No candle exists — backfill from 30 days ago.
		gapStart = now.AddDate(0, 0, -historicalBackfillDays).Truncate(24 * time.Hour)
	} else {
		dur := intervalDuration(interval)
		gapStart = latest.OpenTime.Add(dur)
	}

	if !gapStart.Before(now) {
		// No gap detected.
		return nil
	}

	slog.Info("gap_filler: gap detected",
		"symbol", symbol,
		"interval", interval,
		"from", gapStart,
		"to", now,
	)

	totalInserted, err := w.fetchAndInsert(ctx, symbol, interval, gapStart, now)
	if err != nil {
		return err
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

// ---------------------------------------------------------------------------
// Shared REST fetch helper
// ---------------------------------------------------------------------------

// fetchAndInsert paginates through the Binance Futures REST kline endpoint
// for the given (symbol, interval) pair from startTime to endTime, inserting
// all fetched candles in batches. Returns the total number of candles inserted.
//
// Each REST call is rate-gated through the shared ExchangeRateLimiter to
// respect the Binance IP weight cap (SRS FR-CORE-04, WBS 2.2.5).
func (w *GapFillerWorker) fetchAndInsert(
	ctx context.Context,
	symbol, interval string,
	startTime, endTime time.Time,
) (int, error) {
	dur := intervalDuration(interval)
	totalInserted := 0
	cursor := startTime

	for cursor.Before(endTime) {
		if ctx.Err() != nil {
			return totalInserted, ctx.Err()
		}

		// Rate-gate before every REST call to respect the shared Binance IP
		// weight cap (SRS FR-CORE-04, WBS 2.2.5). The GapFillerWorker shares
		// the same limiter as BinanceProxy and MarketTickerChannel.
		if err := w.limiter.Wait(ctx); err != nil {
			return totalInserted, fmt.Errorf("rate limiter: %w", err)
		}

		// Public endpoint — no API key required for kline history queries.
		publicClient := futures.NewClient("", "")
		klines, err := publicClient.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(cursor.UnixMilli()).
			EndTime(endTime.UnixMilli()).
			Limit(binanceKlinePageSize).
			Do(ctx)
		if err != nil {
			return totalInserted, fmt.Errorf("REST NewKlinesService(%s, %s): %w", symbol, interval, err)
		}

		if len(klines) == 0 {
			break
		}

		candles := make([]domain.Candle, 0, len(klines))
		for _, k := range klines {
			candles = append(candles, *buildCandleFromREST(k, symbol, interval))
		}

		if err := w.candleRepo.InsertBatch(ctx, candles); err != nil {
			return totalInserted, fmt.Errorf("InsertBatch: %w", err)
		}
		totalInserted += len(candles)

		// Advance cursor past the last fetched kline to avoid re-fetching the
		// same page on the next iteration. Adding intervalDuration guarantees
		// we start from the next expected candle open_time.
		lastOpenTime := time.UnixMilli(klines[len(klines)-1].OpenTime).UTC()
		cursor = lastOpenTime.Add(dur)
	}

	return totalInserted, nil
}

// intervalDuration maps a Binance kline interval string to its Go duration.
// Only intervals supported by the system (domain.CandleInterval* constants)
// are included. Unknown intervals default to 1 minute as a safe fallback.
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
