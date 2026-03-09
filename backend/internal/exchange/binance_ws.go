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

// subscriptionKey returns the map key for a (symbol, interval) pair.
func subscriptionKey(symbol, interval string) string {
	return symbol + "_" + interval
}

// KlineSyncService manages per-(symbol, interval) Binance Futures kline
// WebSocket streams and provides a REST fallback to seed the initial candle
// when the local database has no record yet (SRS FR-CORE-02, WBS 2.4.1).
//
// # Hybrid Sync Flow (per subscription)
//
//  1. db.First() guard  — FindLatest(symbol, interval)
//     - Record found   → skip REST call (conserves Binance API weight).
//     - No record      → REST fallback: NewKlinesService(limit=1) → InsertOne.
//
//  2. WsKlineServe goroutine starts in background.
//     - On every WsKlineEvent where Kline.IsFinal == true:
//     build domain.Candle → InsertOne (ON CONFLICT DO NOTHING).
//
// # Concurrency
//
// Each (symbol, interval) subscription owns one goroutine and one stopCh.
// The mu Mutex guards the subs map against concurrent Subscribe/Unsubscribe
// calls from multiple bot goroutines (NFR-PERF-03: 5 bots in parallel).
type KlineSyncService struct {
	candleRepo repository.CandleRepository
	limiter    *ExchangeRateLimiter

	mu   sync.Mutex
	subs map[string]chan struct{} // key: subscriptionKey → stopCh
}

// NewKlineSyncService constructs a KlineSyncService.
//
// The provided limiter is the singleton ExchangeRateLimiter shared with all
// BinanceProxy instances to enforce the global Binance Futures IP weight cap
// (WBS 2.2.5).
func NewKlineSyncService(candleRepo repository.CandleRepository, limiter *ExchangeRateLimiter) *KlineSyncService {
	return &KlineSyncService{
		candleRepo: candleRepo,
		limiter:    limiter,
		subs:       make(map[string]chan struct{}),
	}
}

// Subscribe starts a kline WebSocket stream for the given (symbol, interval)
// pair, seeding the DB with the current candle via REST if no record exists.
//
// Calling Subscribe for an already-active (symbol, interval) is a no-op —
// the existing goroutine is not restarted (idempotent).
//
// The stream runs until Unsubscribe is called or ctx is cancelled.
//
// Parameters:
//   - ctx       — parent context; cancellation propagates to the goroutine.
//   - symbol    — Binance Futures pair, e.g. "BTCUSDT".
//   - interval  — kline timeframe, e.g. "1m", "1h" (domain.CandleInterval*).
func (s *KlineSyncService) Subscribe(ctx context.Context, symbol, interval string) error {
	key := subscriptionKey(symbol, interval)

	s.mu.Lock()
	if _, exists := s.subs[key]; exists {
		s.mu.Unlock()
		return nil // already subscribed — no-op
	}
	stopCh := make(chan struct{})
	s.subs[key] = stopCh
	s.mu.Unlock()

	// --- Step 1: db.First() guard — REST fallback if no candle in DB yet -------
	if err := s.seedLatestCandle(ctx, symbol, interval); err != nil {
		// Non-fatal: log and continue. The WS stream will still provide data.
		// A missing seed only means the very first candle may appear with a gap.
		slog.Warn("kline_sync: seedLatestCandle failed", "symbol", symbol, "interval", interval, "error", err)
	}

	// --- Step 2: Start the WS stream goroutine ----------------------------------
	go s.runKlineStream(ctx, symbol, interval, stopCh)

	return nil
}

// Unsubscribe stops the kline WebSocket stream for the given (symbol, interval).
// It is safe to call Unsubscribe for a pair that is not currently subscribed.
func (s *KlineSyncService) Unsubscribe(symbol, interval string) {
	key := subscriptionKey(symbol, interval)

	s.mu.Lock()
	stopCh, exists := s.subs[key]
	if exists {
		delete(s.subs, key)
	}
	s.mu.Unlock()

	if exists {
		close(stopCh)
	}
}

// UnsubscribeAll stops every active stream. Called on graceful application
// shutdown (SIGTERM from Docker stop — SRS FR-CORE-05, docker-compose restart policy).
func (s *KlineSyncService) UnsubscribeAll() {
	s.mu.Lock()
	keys := make([]string, 0, len(s.subs))
	for k := range s.subs {
		keys = append(keys, k)
	}
	s.mu.Unlock()

	for _, key := range keys {
		// Re-check under lock — another goroutine could have removed the entry.
		s.mu.Lock()
		stopCh, exists := s.subs[key]
		if exists {
			delete(s.subs, key)
		}
		s.mu.Unlock()
		if exists {
			close(stopCh)
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// seedLatestCandle performs the db.First() guard and REST fallback described in
// SRS FR-CORE-02 and WBS 2.4.1 notes.
//
// Logic:
//  1. Call FindLatest — if a candle exists, return nil immediately (no REST).
//  2. If no record:  rate-gate → call REST NewKlinesService(limit=1) → InsertOne.
func (s *KlineSyncService) seedLatestCandle(ctx context.Context, symbol, interval string) error {
	// db.First() check — avoids burning Binance API weight unnecessarily.
	existing, err := s.candleRepo.FindLatest(ctx, symbol, interval)
	if err != nil {
		return fmt.Errorf("kline_sync: seedLatestCandle: FindLatest: %w", err)
	}
	if existing != nil {
		// Record already in DB — skip REST call.
		return nil
	}

	// No record found → REST fallback: fetch the current (latest) closed kline.
	//
	// Public endpoint — no API key required for kline history queries on Binance.
	// Rate-gate before the call to respect the shared IP weight cap (WBS 2.2.5).
	if err := s.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("kline_sync: seedLatestCandle: rate limiter: %w", err)
	}

	publicClient := futures.NewClient("", "")
	klines, err := publicClient.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(1).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("kline_sync: seedLatestCandle: REST NewKlinesService(%s, %s): %w", symbol, interval, err)
	}
	if len(klines) == 0 {
		return nil // Binance returned empty — nothing to seed.
	}

	candle := buildCandleFromREST(klines[0], symbol, interval)
	if err := s.candleRepo.InsertOne(ctx, candle); err != nil {
		return fmt.Errorf("kline_sync: seedLatestCandle: InsertOne: %w", err)
	}
	return nil
}

// runKlineStream is the background goroutine that connects to the Binance
// Futures kline WebSocket stream and persists every fully closed candle.
//
// The goroutine exits cleanly when:
//   - stopCh is closed (Unsubscribe / UnsubscribeAll called).
//   - ctx is cancelled (application shutdown).
//
// WsKlineServe from go-binance returns (doneC, stopC chan struct{}).
// Sending on stopC signals the SDK to close the WS connection; doneC is
// closed by the SDK once the connection is fully torn down.
func (s *KlineSyncService) runKlineStream(ctx context.Context, symbol, interval string, stopCh <-chan struct{}) {
	wsHandler := func(event *futures.WsKlineEvent) {
		// Only persist fully closed candles (IsFinal == true).
		// In-progress klines (is_final=false) are not stored — they are
		// ephemeral tick data served via the market_ticker WS channel (WBS 2.8.2).
		if !event.Kline.IsFinal {
			return
		}

		candle := buildCandleFromWS(event)
		if err := s.candleRepo.InsertOne(ctx, candle); err != nil {
			// Log and continue — a failed insert does not stop the stream.
			// The GapFillerWorker (WBS 2.4.2) will recover any missing rows.
			slog.Warn("kline_sync: InsertOne failed", "symbol", symbol, "interval", interval, "error", err)
		}
	}

	errHandler := func(err error) {
		// Log WS protocol errors. The go-binance SDK internally reconnects on
		// transient failures; persistent errors will surface here and are logged
		// to preserve observability without crashing the goroutine.
		slog.Warn("kline_sync: WS error", "symbol", symbol, "interval", interval, "error", err)
	}

	doneC, sdkStopC, err := futures.WsKlineServe(symbol, interval, wsHandler, errHandler)
	if err != nil {
		slog.Error("kline_sync: WsKlineServe failed", "symbol", symbol, "interval", interval, "error", err)
		return
	}

	select {
	case <-stopCh:
		// Unsubscribe called — signal the SDK to stop the WS connection.
		sdkStopC <- struct{}{}
		<-doneC // wait for the SDK to fully close the connection.
	case <-ctx.Done():
		// Parent context cancelled (application shutdown).
		sdkStopC <- struct{}{}
		<-doneC
	case <-doneC:
		// SDK closed the connection on its own (e.g. Binance server-side close).
		// Log for observability; the stream is no longer active.
		slog.Info("kline_sync: WS stream closed by remote", "symbol", symbol, "interval", interval)
	}
}

// ---------------------------------------------------------------------------
// Candle builders
// ---------------------------------------------------------------------------

// StartWatchedSymbols subscribes to the 1-minute kline stream for every
// symbol in the provided list. It is called once on server startup, driven
// by the WATCHED_SYMBOLS environment variable (WBS 2.4.1).
//
// Each subscription is non-blocking — the underlying goroutine runs until
// ctx is cancelled (SIGTERM graceful shutdown) or Unsubscribe is called.
// Errors from individual subscriptions are logged but do not abort the loop
// so that one bad symbol never prevents the rest from being monitored.
func (s *KlineSyncService) StartWatchedSymbols(ctx context.Context, symbols []string) {
	for _, sym := range symbols {
		if err := s.Subscribe(ctx, sym, "1m"); err != nil {
			slog.Error("kline_sync: subscribe failed", "symbol", sym, "error", err)
		}
	}
}

// buildCandleFromWS constructs a domain.Candle from a fully-closed
// WsKlineEvent (IsFinal == true). All price/volume fields are strings —
// stored as-is to preserve Binance's decimal precision (SRS FR-DESIGN-06).
func buildCandleFromWS(event *futures.WsKlineEvent) *domain.Candle {
	return &domain.Candle{
		Symbol:     event.Symbol,
		Interval:   event.Kline.Interval,
		OpenTime:   time.UnixMilli(event.Kline.StartTime).UTC(),
		OpenPrice:  event.Kline.Open,
		HighPrice:  event.Kline.High,
		LowPrice:   event.Kline.Low,
		ClosePrice: event.Kline.Close,
		Volume:     event.Kline.Volume,
		IsClosed:   true,
	}
}

// buildCandleFromREST constructs a domain.Candle from a REST KlinesService
// response row. Symbol and interval are taken from the call parameters since
// the REST response does not echo them back (unlike the WS event).
func buildCandleFromREST(k *futures.Kline, symbol, interval string) *domain.Candle {
	return &domain.Candle{
		Symbol:     symbol,
		Interval:   interval,
		OpenTime:   time.UnixMilli(k.OpenTime).UTC(),
		OpenPrice:  k.Open,
		HighPrice:  k.High,
		LowPrice:   k.Low,
		ClosePrice: k.Close,
		Volume:     k.Volume,
		IsClosed:   true,
	}
}
