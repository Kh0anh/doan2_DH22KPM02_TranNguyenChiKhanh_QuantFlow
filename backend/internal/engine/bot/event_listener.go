// event_listener.go — Task 2.7.2: Bot WebSocket Event Listener.
//
// BotEventListener subscribes to Binance Futures kline WebSocket streams and
// fans out closed-candle events to all registered bot goroutines via
// BotManager.DispatchEvent().
//
// ─── Fan-out Model ──────────────────────────────────────────────────────────
//
// ONE WS stream is maintained per unique (symbol, interval) pair, regardless of
// how many bots trade that pair simultaneously. When a candle closes:
//
//	Binance WS → BotEventListener.fanOut(symbol, interval, candle)
//	          → BotManager.DispatchEvent(botID_A, payload)
//	          → BotManager.DispatchEvent(botID_B, payload)
//	          → ...
//
// This keeps Binance connection count O(unique symbols × timeframes) rather
// than O(bots), satisfying NFR-PERF-03 (5 bots in parallel without separate
// connections for each).
//
// ─── Reconnect Strategy (NFR-REL-02) ────────────────────────────────────────
//
// runStream() uses jpillora/backoff with:
//
//	Min=500ms · Max=5min · Factor=2 · Jitter=true
//
// If the connection stays alive ≥ 30 seconds, the counter resets so the next
// disconnect starts fresh from 500ms. This prevents permanent max-delay lock-in
// after a brief network hiccup following a long stable run.
//
// ─── Lifecycle Integration ──────────────────────────────────────────────────
//
//	BotManager.StartBot()              → BotEventListener.Subscribe(botID, symbol, interval)
//	BotManager.removeBotFromRegistry() → BotEventListener.Unsubscribe(botID)
//
// ─── Initialization Order ───────────────────────────────────────────────────
//
// Because BotManager and BotEventListener reference each other, use two-phase init:
//
//	manager  := bot.NewBotManager(db, logger)
//	listener := bot.NewBotEventListener(serverCtx, manager, logger)
//	manager.SetListener(listener)
//
// Task 2.7.2 — Bot WebSocket Event Listener.
// WBS: P2-Backend · 14/03/2026
// SRS: FR-RUN-06, FR-DESIGN-03, NFR-REL-02
package bot

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/jpillora/backoff"
	"github.com/kh0anh/quantflow/internal/domain"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Internal: stream registry types
// ═══════════════════════════════════════════════════════════════════════════

// streamKey returns the map key for a (symbol, interval) pair used as the
// lookup key in the BotEventListener streams registry.
func streamKey(symbol, interval string) string {
	return symbol + "_" + interval
}

// streamEntry holds the subscription state for one active (symbol, interval)
// kline WebSocket stream.
type streamEntry struct {
	// botIDs is the set of bot IDs that should receive candle events
	// from this stream. Modified under BotEventListener.mu write lock.
	botIDs map[string]struct{}

	// stopCh is closed when the last subscriber calls Unsubscribe,
	// signalling the runStream goroutine to exit cleanly.
	stopCh chan struct{}
}

// ═══════════════════════════════════════════════════════════════════════════
//  BotEventListener
// ═══════════════════════════════════════════════════════════════════════════

// BotEventListener manages Binance Futures kline WebSocket streams for the
// Live Trade Bot engine. It dispatches closed-candle events to running bots
// via BotManager.DispatchEvent().
//
// BotEventListener is safe for concurrent use from multiple goroutines.
type BotEventListener struct {
	serverCtx context.Context
	manager   *BotManager
	logger    *slog.Logger

	mu sync.RWMutex
	// streams maps streamKey(symbol, interval) → active streamEntry.
	streams map[string]*streamEntry
	// botToKeys is a reverse index: botID → set of stream keys the bot has joined.
	// Used by Unsubscribe(botID) to locate all streams to remove the bot from
	// without scanning the entire streams map.
	botToKeys map[string]map[string]struct{}
}

// NewBotEventListener constructs a BotEventListener.
//
//   - serverCtx: the application-lifetime context (from signal.NotifyContext in
//     cmd/server/main.go). Cancellation propagates to all stream goroutines for
//     graceful shutdown on SIGTERM.
//   - manager:   the BotManager whose DispatchEvent() method receives the
//     closed-candle events from the fan-out.
//   - logger:    structured logger; decorated with component=event_listener.
func NewBotEventListener(serverCtx context.Context, manager *BotManager, logger *slog.Logger) *BotEventListener {
	if logger == nil {
		logger = slog.Default()
	}
	return &BotEventListener{
		serverCtx: serverCtx,
		manager:   manager,
		logger:    logger.With(slog.String("component", "event_listener")),
		streams:   make(map[string]*streamEntry),
		botToKeys: make(map[string]map[string]struct{}),
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Public API
// ═══════════════════════════════════════════════════════════════════════════

// Subscribe registers botID as a listener for closed-candle events on the
// given (symbol, interval) pair.
//
//   - If no stream for (symbol, interval) exists, a new reconnect-loop goroutine
//     is launched using the server-level context.
//   - If a stream is already active, botID is added to its fan-out set (no new
//     goroutine).
//   - Calling Subscribe for an already-registered (botID, symbol, interval) is a
//     no-op (idempotent).
//
// symbol and interval must both be non-empty; a zero-value for either is skipped
// with a warning log to avoid launching a stream for an unconfigured bot.
func (l *BotEventListener) Subscribe(botID, symbol, interval string) {
	if symbol == "" || interval == "" {
		l.logger.Warn("event_listener: Subscribe called with empty symbol or interval — skipped",
			slog.String("bot_id", botID),
			slog.String("symbol", symbol),
			slog.String("interval", interval),
		)
		return
	}

	key := streamKey(symbol, interval)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Register the reverse index for efficient Unsubscribe lookups.
	if l.botToKeys[botID] == nil {
		l.botToKeys[botID] = make(map[string]struct{})
	}
	l.botToKeys[botID][key] = struct{}{}

	// Add to an existing stream — no new goroutine needed.
	if entry, exists := l.streams[key]; exists {
		entry.botIDs[botID] = struct{}{}
		l.logger.Info("event_listener: bot joined existing kline stream",
			slog.String("bot_id", botID),
			slog.String("symbol", symbol),
			slog.String("interval", interval),
		)
		return
	}

	// First subscriber for this (symbol, interval) — create a new stream entry
	// and launch the reconnect-loop goroutine.
	stopCh := make(chan struct{})
	l.streams[key] = &streamEntry{
		botIDs: map[string]struct{}{botID: {}},
		stopCh: stopCh,
	}

	l.logger.Info("event_listener: starting kline stream for new (symbol, interval)",
		slog.String("symbol", symbol),
		slog.String("interval", interval),
		slog.String("first_bot_id", botID),
	)

	go l.runStream(symbol, interval, stopCh)
}

// Unsubscribe removes botID from all kline streams it has subscribed to.
//
// If removing botID leaves a stream with zero listeners, the stream goroutine is
// stopped by closing its stopCh. Safe to call for a bot that is not currently
// subscribed (no-op).
func (l *BotEventListener) Unsubscribe(botID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	keys, ok := l.botToKeys[botID]
	if !ok {
		return // bot was never subscribed — no-op
	}
	delete(l.botToKeys, botID)

	for key := range keys {
		entry, exists := l.streams[key]
		if !exists {
			continue
		}
		delete(entry.botIDs, botID)

		if len(entry.botIDs) == 0 {
			// Last subscriber removed — signal the stream goroutine to exit.
			close(entry.stopCh)
			delete(l.streams, key)

			l.logger.Info("event_listener: kline stream stopped — no remaining subscribers",
				slog.String("stream_key", key),
			)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Private: reconnect loop
// ═══════════════════════════════════════════════════════════════════════════

// runStream is the goroutine body for one (symbol, interval) kline stream.
//
// It runs a reconnect loop: on each iteration it calls connectOnce() which
// blocks until the WS connection ends. If connectOnce signals a reconnect is
// needed (unexpected close), it waits for a backoff-calculated duration before
// attempting again.
//
// The loop exits cleanly when:
//   - stopCh is closed (last subscriber called Unsubscribe).
//   - l.serverCtx is cancelled (application SIGTERM shutdown).
//
// Backoff parameters (SRS NFR-REL-02):
//
//	Min=500ms · Max=5min · Factor=2 · Jitter=true
//
// The backoff counter resets after a connection that stayed alive ≥ 30 seconds,
// preventing permanent max-delay lock-in after a brief transient failure
// following a long stable run.
func (l *BotEventListener) runStream(symbol, interval string, stopCh <-chan struct{}) {
	b := &backoff.Backoff{
		Min:    500 * time.Millisecond,
		Max:    5 * time.Minute,
		Factor: 2,
		Jitter: true,
	}

	streamLogger := l.logger.With(
		slog.String("symbol", symbol),
		slog.String("interval", interval),
	)

	for {
		// Check for stop or shutdown before each connect attempt so the goroutine
		// exits immediately if it was queued to stop during a previous iteration.
		select {
		case <-stopCh:
			streamLogger.Info("event_listener: stream goroutine exiting (unsubscribed)")
			return
		case <-l.serverCtx.Done():
			streamLogger.Info("event_listener: stream goroutine exiting (server shutdown)")
			return
		default:
		}

		connectStart := time.Now()
		shouldReconnect := l.connectOnce(symbol, interval, stopCh)

		if !shouldReconnect {
			// Clean stop requested (stopCh or serverCtx) — exit the loop.
			return
		}

		// Reset the backoff counter if the connection was stable for ≥30s.
		// This prevents accumulating delay for brief transient disconnects
		// after an otherwise healthy long-lived connection.
		elapsed := time.Since(connectStart)
		if elapsed >= 30*time.Second {
			b.Reset()
		}

		delay := b.Duration()
		streamLogger.Warn("event_listener: WS stream disconnected — scheduling reconnect",
			slog.Duration("backoff_delay", delay),
			slog.Duration("connection_uptime", elapsed),
			slog.Float64("attempt", b.Attempt()),
		)

		select {
		case <-time.After(delay):
			// Backoff elapsed — proceed to the next connect attempt.
		case <-stopCh:
			streamLogger.Info("event_listener: stream goroutine exiting during backoff (unsubscribed)")
			return
		case <-l.serverCtx.Done():
			streamLogger.Info("event_listener: stream goroutine exiting during backoff (server shutdown)")
			return
		}
	}
}

// connectOnce performs a single WS connection attempt for (symbol, interval)
// and blocks until the connection ends.
//
// Returns:
//
//	true  — the connection ended unexpectedly (WsKlineServe error or remote
//	        close); runStream should schedule a reconnect.
//	false — a clean stop was requested (stopCh closed or serverCtx cancelled);
//	        runStream should exit the reconnect loop.
func (l *BotEventListener) connectOnce(symbol, interval string, stopCh <-chan struct{}) (shouldReconnect bool) {
	connectLogger := l.logger.With(
		slog.String("symbol", symbol),
		slog.String("interval", interval),
	)

	wsHandler := func(event *futures.WsKlineEvent) {
		// Only fully closed candles trigger a Session (SRS FR-DESIGN-03).
		// In-progress klines (IsFinal=false) represent live tick data and
		// must NOT trigger strategy execution.
		if !event.Kline.IsFinal {
			return
		}
		candle := buildBotCandle(event)
		l.fanOut(symbol, interval, candle)
	}

	errHandler := func(err error) {
		// WS protocol-level errors (ping timeout, read errors) are logged at
		// Warn level. The SDK internally handles minor transient errors; only
		// persistent failures escalate to doneC being closed.
		connectLogger.Warn("event_listener: WS protocol error",
			slog.String("error", err.Error()),
		)
	}

	doneC, sdkStopC, err := futures.WsKlineServe(symbol, interval, wsHandler, errHandler)
	if err != nil {
		// WsKlineServe failed to establish the initial connection.
		connectLogger.Error("event_listener: WsKlineServe failed to connect",
			slog.String("error", err.Error()),
		)
		return true // signal reconnect
	}

	connectLogger.Info("event_listener: WS stream connected")

	select {
	case <-stopCh:
		// Unsubscribe called — stop the SDK stream cleanly and wait for teardown.
		sdkStopC <- struct{}{}
		<-doneC
		return false

	case <-l.serverCtx.Done():
		// Application SIGTERM shutdown — stop cleanly.
		sdkStopC <- struct{}{}
		<-doneC
		return false

	case <-doneC:
		// SDK closed the connection: Binance server-side close, network failure,
		// or a WS protocol error that the SDK could not recover internally.
		connectLogger.Warn("event_listener: WS stream closed by remote — will reconnect")
		return true
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Private: fan-out and candle builder
// ═══════════════════════════════════════════════════════════════════════════

// fanOut delivers the closed candle to all bots registered for (symbol, interval).
//
// It snapshots the botIDs set under a read lock, then calls DispatchEvent for
// each bot. DispatchEvent is non-blocking by design (Task 2.7.1: buffered
// channel capacity 1 with drop-on-full), so fanOut never stalls the WS handler
// goroutine.
func (l *BotEventListener) fanOut(symbol, interval string, candle domain.Candle) {
	key := streamKey(symbol, interval)

	l.mu.RLock()
	entry, exists := l.streams[key]
	if !exists {
		l.mu.RUnlock()
		return
	}
	// Snapshot the botIDs slice to avoid holding the read lock during
	// DispatchEvent calls, which acquire BotManager.mu (different mutex,
	// but releasing l.mu promptly keeps the critical section minimal).
	botIDs := make([]string, 0, len(entry.botIDs))
	for id := range entry.botIDs {
		botIDs = append(botIDs, id)
	}
	l.mu.RUnlock()

	for _, botID := range botIDs {
		if err := l.manager.DispatchEvent(botID, EventPayload{Candle: candle}); err != nil {
			// ErrBotNotFound is a benign race condition: the bot goroutine has
			// stopped but removeBotFromRegistry() has not yet called Unsubscribe.
			// Log at Debug level only — this resolves itself on the next fanOut call.
			l.logger.Debug("event_listener: DispatchEvent failed (bot likely stopping)",
				slog.String("bot_id", botID),
				slog.String("symbol", symbol),
				slog.String("interval", interval),
				slog.String("error", err.Error()),
			)
		}
	}
}

// buildBotCandle constructs a domain.Candle from a fully-closed WsKlineEvent.
//
// This is an engine/bot-local builder, intentionally separate from the equivalent
// in exchange/binance_ws.go. Keeping them separate respects Clean Architecture:
// inner layers (engine/bot) must not import outer layers (exchange). Both builders
// produce identical domain.Candle values from the same WS event data.
func buildBotCandle(event *futures.WsKlineEvent) domain.Candle {
	return domain.Candle{
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
