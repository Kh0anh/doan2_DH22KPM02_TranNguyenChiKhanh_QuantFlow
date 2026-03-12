package bot

// manager.go implements BotManager — the in-process registry that launches,
// supervises, and stops Bot goroutines for Live Trade execution.
//
// ─── Fault Isolation Guarantee (Task 2.7.1 core requirement) ───────────────
//
// Each bot runs in its own goroutine. A defer/recover() wrapper inside
// runBotLoop() catches any panic originating from the strategy execution
// (blockly engine, exchange proxy, or indicator logic). The panic terminates
// ONLY the panicking bot's goroutine; all other bot goroutines continue
// running uninterrupted.
//
// Without recover(): a panic in Bot A propagates up the goroutine stack and
// crashes the entire Go process — killing Bot B, Bot C, and the HTTP server.
// With recover(): Bot A is silently quarantined, its status marked as Error,
// a full stack trace is written to slog, and the rest of the system is stable.
//
// ─── Concurrency Model ──────────────────────────────────────────────────────
//
//   - bots map: guarded by sync.RWMutex.
//     RLock for read-only paths (IsRunning, DispatchEvent, GetRunningBotIDs).
//     Lock for mutating paths (StartBot inserts, StopBot deletes after done).
//
//   - RunningBot.eventCh: buffered channel (capacity 1).
//     DispatchEvent uses a non-blocking send (select + default). If the bot is
//     still processing the previous candle when a new one arrives, the event is
//     dropped and a warning is logged. This prevents backpressure from a slow
//     bot from blocking the Binance WS dispatcher (Task 2.7.2).
//
//   - RunningBot.doneCh: unbuffered channel, closed by runBotLoop() on exit.
//     StopBot() waits on doneCh with a configurable timeout so callers can
//     cancel/close positions before returning (Task 2.7.6).
//
// Task 2.7.1 — Bot Instance Lifecycle: Goroutine + recover() Fault Isolation.
// WBS: P2-Backend · 14/03/2026
// SRS: FR-RUN-05, FR-RUN-06, FR-RUN-07, NFR-PERF-03, NFR-REL

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/engine/blockly"
	"github.com/kh0anh/quantflow/internal/repository"
	"gorm.io/gorm"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Sentinel Errors
// ═══════════════════════════════════════════════════════════════════════════

var (
	// ErrBotAlreadyRunning is returned by StartBot when a bot with the given
	// BotID is already registered in the bots map.
	ErrBotAlreadyRunning = errors.New("bot: already running")

	// ErrBotNotFound is returned by StopBot and DispatchEvent when the given
	// BotID is not present in the bots map (not running).
	ErrBotNotFound = errors.New("bot: not found")

	// ErrStopTimeout is returned by StopBot when the bot goroutine does not
	// acknowledge the stop signal within the requested timeout.
	ErrStopTimeout = errors.New("bot: stop timed out waiting for goroutine to exit")
)

// ═══════════════════════════════════════════════════════════════════════════
//  BotConfig — immutable snapshot used to launch a bot goroutine
// ═══════════════════════════════════════════════════════════════════════════

// BotConfig is the complete set of parameters required to launch and run a
// single bot goroutine. All fields are populated by BotLogic (Task 2.7.5) from
// the POST /bots request body and the resolved strategy_version row.
//
// All fields are read-only after StartBot() stores the RunningBot — they are
// never mutated while the goroutine is alive.
type BotConfig struct {
	// BotID is the UUID of the bot_instances row (the primary key).
	BotID string

	// UserID is the UUID of the owning user — used to scope log lines and
	// any future per-user resource limits.
	UserID string

	// StrategyVersionID identifies the pinned strategy_versions row whose
	// logic_json is used for every Session of this bot.
	StrategyVersionID string

	// LogicJSON is the raw Blockly workspace JSON loaded from
	// strategy_versions.logic_json at bot creation time.
	LogicJSON []byte

	// Symbol is the Binance Futures trading pair (e.g., "BTCUSDT").
	Symbol string

	// APIKeyID is the UUID of the api_keys row; used by the TradingProxy to
	// look up and decrypt the exchange credentials (Task 2.7.2 / 2.2.4).
	// Stored here for reference — the CandleRepo and TradingProxy are injected
	// directly into SessionConfig by the bot goroutine.
	APIKeyID string

	// Interval is the kline timeframe extracted from the strategy's logic_json
	// event_on_candle block at bot creation time (e.g., "1m", "1h").
	// BotEventListener uses this to subscribe to the correct Binance WS stream
	// for this bot (Task 2.7.2). Set by BotLogic.StartBotInstance() (Task 2.7.5)
	// before calling BotManager.StartBot().
	Interval string

	// CandleRepo provides read access to candles_data for indicator blocks.
	// Injected by BotLogic.StartBotInstance() before calling StartBot().
	CandleRepo blockly.CandleRepositoryReader

	// TradingProxy provides live Binance exchange access for data and trade
	// action blocks. Injected by BotLogic.StartBotInstance() before StartBot().
	// Task 2.7.2 wires in the concrete exchange.BinanceProxy instance.
	TradingProxy blockly.TradingProxy
}

// ═══════════════════════════════════════════════════════════════════════════
//  EventPayload — candle event delivered to a running bot
// ═══════════════════════════════════════════════════════════════════════════

// EventPayload carries the data for a single candle event dispatched to a
// running bot. The BotEventListener (Task 2.7.2) constructs one EventPayload
// per closed candle and calls BotManager.DispatchEvent().
type EventPayload struct {
	// Candle is the fully-closed candle that triggered this session.
	// Delivery is guaranteed by the event_listener to only occur when
	// domain.Candle.IsClosed == true.
	Candle domain.Candle
}

// ═══════════════════════════════════════════════════════════════════════════
//  RunningBot — in-memory handle for one active bot goroutine
// ═══════════════════════════════════════════════════════════════════════════

// RunningBot holds the control handles for a single running bot goroutine.
// It is created by StartBot() and removed from the bots map after the
// goroutine signals completion via doneCh.
//
// All fields are set once at construction time and never mutated — except
// for statusMu/status which protect the mutable status string.
type RunningBot struct {
	config BotConfig

	// cancel sends the stop signal to the bot goroutine's context.
	// Called by StopBot() to initiate graceful shutdown.
	cancel context.CancelFunc

	// doneCh is closed by runBotLoop() immediately before it returns.
	// StopBot() selects on doneCh with a timeout to confirm the goroutine exited.
	doneCh chan struct{}

	// eventCh is a buffered channel (capacity 1) used by DispatchEvent()
	// to deliver candle events to the bot goroutine's select loop.
	// Capacity 1 ensures that a newly arrived event is held if the goroutine
	// is still processing the previous one; a second simultaneous event is
	// dropped (non-blocking send) to prevent backpressure.
	eventCh chan EventPayload

	// statusMu guards the status field for concurrent reads from the HTTP
	// layer (GET /bots) and concurrent writes from the bot goroutine.
	statusMu sync.RWMutex
	status   string
}

// getStatus returns the concurrent-safe current status string.
func (rb *RunningBot) getStatus() string {
	rb.statusMu.RLock()
	defer rb.statusMu.RUnlock()
	return rb.status
}

// setStatus updates the status in a concurrent-safe way.
func (rb *RunningBot) setStatus(s string) {
	rb.statusMu.Lock()
	rb.status = s
	rb.statusMu.Unlock()
}

// ═══════════════════════════════════════════════════════════════════════════
//  BotManager
// ═══════════════════════════════════════════════════════════════════════════

// BotManager is the singleton registry responsible for the goroutine-based
// lifecycle of all Live Trade bots.
//
// It is constructed once at application startup (cmd/server/main.go) and
// injected into BotLogic (Task 2.7.5) via dependency injection.
//
// BotManager is safe for concurrent use from multiple goroutines.
type BotManager struct {
	mu             sync.RWMutex
	bots           map[string]*RunningBot
	db             *gorm.DB
	logger         *slog.Logger
	lifecycleState *LifecycleStateManager // Task 2.7.3: lifecycle variable persistence
	botLogger      *BotLogger             // Task 2.7.4: bot session log persistence and WS push

	// listener is the BotEventListener that manages Binance kline WS streams.
	// Wired after construction via SetListener() to break the mutual init cycle:
	//
	//   manager  := NewBotManager(db, logger)
	//   listener := NewBotEventListener(serverCtx, manager, logger)
	//   manager.SetListener(listener)  ← back-reference
	//
	// nil-checked before use in StartBot and removeBotFromRegistry.
	listener *BotEventListener
}

// NewBotManager constructs a BotManager.
//
//   - db:      GORM DB instance used to update bot_instances.status on state
//     transitions (Running → Stopped / Error) for persistence across restarts.
//   - logger:  a slog.Logger decorated with service-level fields. Each bot
//     goroutine derives a child logger with its bot_id attached.
//   - varRepo: repository for bot_lifecycle_variables — used by
//     LifecycleStateManager to Load/Persist variable state per Session
//     (Task 2.7.3). Pass nil to skip persistence (e.g. in unit tests).
//   - logRepo:  repository for bot_logs — BotLogger uses it to persist one
//     row per Session outcome (Task 2.7.4). Pass nil to disable log persistence.
//   - wsPusher: BotLogPusher implementation for live WS fan-out (Task 2.7.4).
//     Pass nil to disable WS push (e.g. before HTTP server starts, unit tests).
//
// After construction, call SetListener() to wire in the BotEventListener
// (Task 2.7.2) before the first StartBot() call.
func NewBotManager(
	db *gorm.DB,
	logger *slog.Logger,
	varRepo repository.BotLifecycleVarRepository,
	logRepo repository.BotLogRepository,
	wsPusher BotLogPusher,
) *BotManager {
	if logger == nil {
		logger = slog.Default()
	}
	var lifecycleState *LifecycleStateManager
	if varRepo != nil {
		lifecycleState = NewLifecycleStateManager(varRepo, logger)
	}
	var botLogger *BotLogger
	if logRepo != nil {
		botLogger = NewBotLogger(logRepo, wsPusher, logger)
	}
	return &BotManager{
		bots:           make(map[string]*RunningBot),
		db:             db,
		logger:         logger,
		lifecycleState: lifecycleState,
		botLogger:      botLogger,
	}
}

// SetListener wires the BotEventListener into the manager after both have been
// constructed. This two-phase initialization breaks the mutual reference cycle:
//
//	manager  := bot.NewBotManager(db, logger)
//	listener := bot.NewBotEventListener(serverCtx, manager, logger)
//	manager.SetListener(listener)
//
// SetListener must be called before the first StartBot() invocation to ensure
// every started bot is automatically subscribed to its kline stream (Task 2.7.2).
// It is not safe to call SetListener after bots have been started.
func (m *BotManager) SetListener(listener *BotEventListener) {
	m.listener = listener
}

// ═══════════════════════════════════════════════════════════════════════════
//  Public API
// ═══════════════════════════════════════════════════════════════════════════

// StartBot registers and launches a new bot goroutine for the given config.
//
// Returns ErrBotAlreadyRunning if a bot with the same BotID is already active.
// The bot goroutine begins its select event-loop immediately and is ready to
// process events delivered via DispatchEvent().
//
// After the goroutine is launched, Subscribe is called on the BotEventListener
// (if wired) so the bot starts receiving closed-candle events for its
// (Symbol, Interval) pair (Task 2.7.2).
func (m *BotManager) StartBot(ctx context.Context, cfg BotConfig) error {
	m.mu.Lock()
	if _, exists := m.bots[cfg.BotID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("StartBot (bot_id=%s): %w", cfg.BotID, ErrBotAlreadyRunning)
	}

	botCtx, cancel := context.WithCancel(ctx)

	rb := &RunningBot{
		config:  cfg,
		cancel:  cancel,
		doneCh:  make(chan struct{}),
		eventCh: make(chan EventPayload, 1),
		status:  domain.BotStatusRunning,
	}

	m.bots[cfg.BotID] = rb
	m.mu.Unlock()

	botLogger := m.logger.With(
		slog.String("bot_id", cfg.BotID),
		slog.String("user_id", cfg.UserID),
		slog.String("symbol", cfg.Symbol),
	)

	go m.runBotLoop(botCtx, rb, botLogger)

	// Subscribe to the Binance kline stream AFTER launching the goroutine so the
	// bot's event channel is ready before the first candle event can arrive.
	// The call is made outside m.mu to avoid a lock-order dependency with
	// BotEventListener.mu (which can call DispatchEvent → m.mu.RLock in fanOut).
	if m.listener != nil {
		m.listener.Subscribe(cfg.BotID, cfg.Symbol, cfg.Interval)
	}

	botLogger.Info("bot: goroutine started",
		slog.String("strategy_version_id", cfg.StrategyVersionID),
		slog.String("interval", cfg.Interval),
	)

	return nil
}

// StopBot sends a cancellation signal to the bot goroutine and waits up to
// timeout for it to acknowledge the stop by closing doneCh.
//
// Returns ErrBotNotFound if the bot is not currently running.
// Returns ErrStopTimeout if the goroutine does not exit within the timeout.
// A zero or negative timeout value defaults to 30 seconds.
func (m *BotManager) StopBot(ctx context.Context, botID string, timeout time.Duration) error {
	m.mu.RLock()
	rb, exists := m.bots[botID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("StopBot (bot_id=%s): %w", botID, ErrBotNotFound)
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Signal the goroutine to stop.
	rb.cancel()

	// Wait for the goroutine to confirm exit.
	select {
	case <-rb.doneCh:
		return nil
	case <-time.After(timeout):
		m.logger.Warn("bot: stop timed out — goroutine did not exit in time",
			slog.String("bot_id", botID),
			slog.Duration("timeout", timeout),
		)
		return fmt.Errorf("StopBot (bot_id=%s, timeout=%s): %w", botID, timeout, ErrStopTimeout)
	case <-ctx.Done():
		// The HTTP request context was cancelled (e.g., client disconnected).
		// The bot goroutine is still running — it will exit when it next checks ctx.
		return ctx.Err()
	}
}

// IsRunning reports whether a bot goroutine with the given ID is currently
// registered and active.
func (m *BotManager) IsRunning(botID string) bool {
	m.mu.RLock()
	_, exists := m.bots[botID]
	m.mu.RUnlock()
	return exists
}

// GetStatus returns the in-memory status string of a running bot.
// Returns ("", ErrBotNotFound) if the bot is not currently registered.
func (m *BotManager) GetStatus(botID string) (string, error) {
	m.mu.RLock()
	rb, exists := m.bots[botID]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("GetStatus (bot_id=%s): %w", botID, ErrBotNotFound)
	}
	return rb.getStatus(), nil
}

// GetRunningBotIDs returns a snapshot of all currently running bot IDs.
// The slice is unordered. Useful for startup restore (WBS 5.1.3) and
// the position_update WebSocket channel (Task 2.8.4).
func (m *BotManager) GetRunningBotIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.bots))
	for id := range m.bots {
		ids = append(ids, id)
	}
	return ids
}

// DispatchEvent delivers a candle event to the running bot's event channel.
//
// The send is NON-BLOCKING: if the bot is busy processing the previous
// candle and its event channel is already full, the event is silently dropped
// and a warning is logged. This prevents a slow bot from blocking the Binance
// WebSocket dispatcher (Task 2.7.2).
//
// Returns ErrBotNotFound if the bot is not currently running.
// Returns nil on both successful delivery and a dropped (full channel) event.
func (m *BotManager) DispatchEvent(botID string, payload EventPayload) error {
	m.mu.RLock()
	rb, exists := m.bots[botID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("DispatchEvent (bot_id=%s): %w", botID, ErrBotNotFound)
	}

	select {
	case rb.eventCh <- payload:
		// Event delivered successfully.
	default:
		// Channel is full — bot is still processing the previous candle.
		// Drop this event and warn. This is acceptable: the next candle event
		// will arrive and the bot will catch up.
		m.logger.Warn("bot: event channel full — candle event dropped",
			slog.String("bot_id", botID),
			slog.String("symbol", rb.config.Symbol),
			slog.Time("candle_open_time", payload.Candle.OpenTime),
		)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Private: Goroutine Event Loop + Fault Isolation
// ═══════════════════════════════════════════════════════════════════════════

// runBotLoop is the goroutine body for a single bot.
//
// ── Fault Isolation (THE core of Task 2.7.1) ─────────────────────────────
//
// The deferred recover() at the top of this function ensures that any panic
// originating inside the event-loop body (Session.Run, exchange proxy calls,
// blockly executor, etc.) is caught HERE and does not propagate to the Go
// runtime scheduler. Result: only this goroutine terminates; all peer bot
// goroutines continue running.
//
// After recovery, the bot status is set to Error and persisted to DB so the
// UI can display the failure state. The full panic stack trace is emitted via
// slog.Error for post-mortem analysis.
//
// ── Event Loop ────────────────────────────────────────────────────────────
//
// The loop selects between two cases:
//
//  1. ctx.Done(): the user called StopBot() or the server is shutting down.
//     → Status → Stopped, doneCh closed, goroutine exits cleanly.
//
//  2. rb.eventCh receives an EventPayload: a new closed candle event arrived.
//     → One Session is executed synchronously (no inner goroutine).
//     → ErrUnitCostExceeded is logged as a warning; the bot stays Running.
//     → Any other non-context error is logged as an error; the bot stays
//     Running (resilient — a single bad session should not kill the bot).
func (m *BotManager) runBotLoop(ctx context.Context, rb *RunningBot, logger *slog.Logger) {
	// ── FAULT ISOLATION: recover() wrapper ────────────────────────────────
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			m.handlePanic(rb, r, stack, logger)
		}
		// Regardless of how the goroutine exits (panic or clean stop), always:
		// 1. Close doneCh so StopBot() unblocks.
		// 2. Remove the bot from the registry.
		close(rb.doneCh)
		m.removeBotFromRegistry(rb.config.BotID, logger)
	}()

	logger.Info("bot: event loop started")

	for {
		select {
		case <-ctx.Done():
			// ── Graceful stop ──────────────────────────────────────────────
			rb.setStatus(domain.BotStatusStopped)
			m.updateStatusInDB(rb.config.BotID, domain.BotStatusStopped, logger)
			logger.Info("bot: stopped gracefully",
				slog.String("reason", ctx.Err().Error()),
			)
			return

		case payload := <-rb.eventCh:
			// ── Process a single candle event (one Session) ────────────────
			m.runSession(ctx, rb, payload, logger)
		}
	}
}

// runSession executes one Session for the given candle event payload.
// It is called from within runBotLoop's event-loop iteration.
//
// Session errors are handled locally:
//   - ErrUnitCostExceeded → warn + continue (bot stays Running, SRS FR-RUN-07).
//   - context.Canceled / DeadlineExceeded → info + let the outer loop handle stop.
//   - All other errors → error log + continue (resilient; single bad session ≠ bot death).
//
// Panics inside Session.Run() propagate naturally to the deferred recover() in
// runBotLoop() — they are NOT caught here.
func (m *BotManager) runSession(
	ctx context.Context,
	rb *RunningBot,
	payload EventPayload,
	logger *slog.Logger,
) {
	sessionLogger := logger.With(
		slog.Time("candle_open_time", payload.Candle.OpenTime),
		slog.String("candle_interval", payload.Candle.Interval),
	)

	cfg := rb.config

	// ── Task 2.7.3: Pre-load lifecycle variables from DB ──────────────────
	var lifecycleVars map[string]interface{}
	if m.lifecycleState != nil {
		loaded, loadErr := m.lifecycleState.Load(ctx, cfg.BotID)
		if loadErr != nil {
			sessionLogger.Warn("bot: failed to load lifecycle vars — starting session with empty state",
				slog.String("error", loadErr.Error()),
			)
			lifecycleVars = make(map[string]interface{})
		} else {
			lifecycleVars = loaded
		}
	} else {
		lifecycleVars = make(map[string]interface{})
	}

	sessionCfg := SessionConfig{
		BotID:         cfg.BotID,
		Symbol:        cfg.Symbol,
		LogicJSON:     cfg.LogicJSON,
		LifecycleVars: lifecycleVars,
		CandleRepo:    cfg.CandleRepo,
		TradingProxy:  cfg.TradingProxy,
	}

	session := NewSession(sessionCfg, sessionLogger)
	result, err := session.Run(ctx)

	if err == nil {
		sessionLogger.Info("bot: session completed",
			slog.Int("units_used", result.UnitsUsed),
		)
		// ── Task 2.7.3: Persist updated lifecycle vars to DB ─────────────
		if m.lifecycleState != nil {
			if persistErr := m.lifecycleState.Persist(ctx, cfg.BotID, result.UpdatedLifecycleVars); persistErr != nil {
				// Non-fatal: the bot continues Running. In-RAM state is still
				// correct for the current goroutine's lifetime; the next successful
				// session will overwrite the stale DB rows.
				sessionLogger.Warn("bot: failed to persist lifecycle vars — in-RAM state preserved",
					slog.String("error", persistErr.Error()),
					slog.Int("units_used", result.UnitsUsed),
				)
			}
		}
		// ── Task 2.7.4: Write bot_log to DB and push WS event ────────────
		if m.botLogger != nil {
			logMsg := fmt.Sprintf("Session completed: %d units consumed", result.UnitsUsed)
			if logErr := m.botLogger.Log(ctx, cfg.BotID, domain.BotLogActionExecuted, logMsg, result.UnitsUsed); logErr != nil {
				sessionLogger.Warn("bot: failed to write bot log",
					slog.String("error", logErr.Error()),
				)
			}
		}
		return
	}

	if errors.Is(err, blockly.ErrUnitCostExceeded) {
		sessionLogger.Warn("bot: session UNIT_COST_EXCEEDED — session terminated, bot continues",
			slog.Int("units_used", result.UnitsUsed),
		)
		if m.botLogger != nil {
			logMsg := fmt.Sprintf("Unit cost budget exhausted: %d units consumed", result.UnitsUsed)
			if logErr := m.botLogger.Log(ctx, cfg.BotID, domain.BotLogActionUnitCostExceeded, logMsg, result.UnitsUsed); logErr != nil {
				sessionLogger.Warn("bot: failed to write bot log",
					slog.String("error", logErr.Error()),
				)
			}
		}
		return
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Outer loop will catch ctx.Done() on next iteration.
		sessionLogger.Info("bot: session context cancelled — bot is stopping",
			slog.String("reason", err.Error()),
		)
		return
	}

	// Unexpected session error — log and continue running (resilient).
	sessionLogger.Error("bot: session execution error — bot remains Running",
		slog.String("error", err.Error()),
	)
	if m.botLogger != nil {
		if logErr := m.botLogger.Log(ctx, cfg.BotID, domain.BotLogActionError, err.Error(), result.UnitsUsed); logErr != nil {
			sessionLogger.Warn("bot: failed to write bot log",
				slog.String("error", logErr.Error()),
			)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Private: Helpers
// ═══════════════════════════════════════════════════════════════════════════

// handlePanic is called from the deferred recover() in runBotLoop when a panic
// is caught. It:
//  1. Marks the bot's in-memory status as Error.
//  2. Persists the Error status to bot_instances in PostgreSQL.
//  3. Emits a structured slog.Error with the panic value and full stack trace.
//
// After handlePanic returns, the deferred function in runBotLoop closes doneCh
// and removes the bot from the registry.
func (m *BotManager) handlePanic(rb *RunningBot, recovered any, stack []byte, logger *slog.Logger) {
	rb.setStatus(domain.BotStatusError)
	m.updateStatusInDB(rb.config.BotID, domain.BotStatusError, logger)

	logger.Error("bot: PANIC caught — bot terminated, other bots unaffected",
		slog.Any("panic_value", recovered),
		slog.String("stack_trace", string(stack)),
	)
	// Use context.Background() because the bot's context is already cancelled
	// by the time a panic is caught in the deferred recover() wrapper.
	if m.botLogger != nil {
		panicMsg := fmt.Sprintf("PANIC: %v", recovered)
		if logErr := m.botLogger.Log(context.Background(), rb.config.BotID, domain.BotLogActionPanic, panicMsg, 0); logErr != nil {
			logger.Warn("bot: failed to write panic bot log",
				slog.String("error", logErr.Error()),
			)
		}
	}
}

// updateStatusInDB persists the given status to bot_instances.status for the
// given bot ID. Errors are logged as warnings — a DB failure here must not
// re-panic the goroutine (we are inside a defer, recovery in progress).
func (m *BotManager) updateStatusInDB(botID, status string, logger *slog.Logger) {
	if m.db == nil {
		return
	}
	if err := m.db.Model(&domain.BotInstance{}).
		Where("id = ?", botID).
		Update("status", status).Error; err != nil {
		logger.Warn("bot: failed to persist status to DB",
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

// removeBotFromRegistry removes the bot from the manager's in-memory registry
// after the goroutine has exited. Called from the deferred cleanup in runBotLoop.
//
// Unsubscribe is called after releasing m.mu to preserve lock-order safety:
// BotEventListener.fanOut acquires l.mu then calls DispatchEvent (m.mu.RLock);
// calling Unsubscribe (l.mu) while holding m.mu would invert that order.
func (m *BotManager) removeBotFromRegistry(botID string, logger *slog.Logger) {
	m.mu.Lock()
	delete(m.bots, botID)
	m.mu.Unlock()

	if m.listener != nil {
		m.listener.Unsubscribe(botID)
	}

	logger.Info("bot: removed from registry")
}
