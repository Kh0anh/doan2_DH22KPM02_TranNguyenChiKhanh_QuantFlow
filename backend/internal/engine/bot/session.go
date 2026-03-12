// Package bot implements the Live Trade Bot execution engine for QuantFlow.
//
// This package is the runtime core for Task 2.7.x (Bot Live Trade). It provides:
//
//   - session.go (this file, Task 2.7.1): One-shot execution of a single candle
//     event trigger. Wires ExecutionContext with all required dependencies and
//     delegates to the blockly executor (Tasks 2.5.x).
//
//   - manager.go (Task 2.7.1): Goroutine-per-bot lifecycle manager with
//     recover() fault isolation — Bot A crash never affects Bot B.
//
//   - event_listener.go (Task 2.7.2): Binance kline WebSocket subscription that
//     feeds candle events into each RunningBot's event channel.
//
//   - state.go (Task 2.7.3): Lifecycle variable persistence — reads/writes
//     bot_lifecycle_variables (JSONB) in PostgreSQL.
//
// Task 2.7.1 — Bot Instance Lifecycle: Goroutine + recover() Fault Isolation.
// WBS: P2-Backend · 14/03/2026
// SRS: FR-RUN-05, FR-RUN-06, FR-RUN-07, NFR-PERF-03
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kh0anh/quantflow/internal/engine/blockly"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Session Dependencies — injected by BotManager per candle event
// ═══════════════════════════════════════════════════════════════════════════

// SessionConfig bundles all dependencies required to execute one Session.
//
// A Session is defined in SRS §1.3 as "one invocation of the event_on_candle
// trigger". A fresh SessionConfig is prepared by the bot goroutine for each
// incoming candle event (fed by event_listener.go, Task 2.7.2).
//
// CandleRepo and TradingProxy are interfaces defined in the blockly package
// to avoid import cycles (engine/bot → engine/blockly, not the reverse).
// The concrete implementations (repository.CandleRepository and
// exchange.BinanceProxy) are injected by BotManager.StartBot() when the bot
// goroutine is launched.
type SessionConfig struct {
	// BotID is the UUID of the bot_instances row this session belongs to.
	// Used to scope log lines and lifecycle variable reads/writes.
	BotID string

	// Symbol is the Binance Futures trading pair (e.g., "BTCUSDT").
	// Injected into ExecutionContext so context-aware blocks (indicators,
	// data, trade) automatically use the correct pair (blockly.md §1.2).
	Symbol string

	// LogicJSON is the raw Blockly workspace JSON loaded from
	// strategy_versions.logic_json at bot creation time. Identical bytes are
	// reused across every Session within the same bot lifetime — the JSON is
	// never re-fetched from the database during execution.
	LogicJSON []byte

	// LifecycleVars holds the current persistent variable state loaded from
	// bot_lifecycle_variables before each Session starts (Task 2.7.3).
	// The map is mutated by variables_lifecycle_set blocks during execution
	// and the updated values are persisted back to DB by the caller after
	// Session.Run() returns.
	LifecycleVars map[string]interface{}

	// CandleRepo provides read access to the candles_data table used by
	// context-aware indicator blocks (RSI, EMA — Task 2.5.5).
	CandleRepo blockly.CandleRepositoryReader

	// TradingProxy provides live Binance exchange access for data and trade
	// action blocks (Task 2.5.6). In Task 2.7.1 this field carries a nil
	// placeholder; Task 2.7.2 wires in the concrete exchange.BinanceProxy.
	TradingProxy blockly.TradingProxy
}

// ═══════════════════════════════════════════════════════════════════════════
//  Session Result
// ═══════════════════════════════════════════════════════════════════════════

// SessionResult carries the observable outcomes of a completed Session.
// The BotManager's goroutine loop uses this to decide whether to persist
// updated lifecycle variables (Task 2.7.3) and what to log (Task 2.7.4).
type SessionResult struct {
	// UnitsUsed is the number of unit-cost units consumed during this Session.
	// Reported in bot_logs for observability (SRS FR-RUN-08).
	UnitsUsed int

	// UpdatedLifecycleVars is the snapshot of LifecycleVars after execution.
	// The caller (bot goroutine) persists this to bot_lifecycle_variables
	// (Task 2.7.3) only when the slice is non-nil — nil means no lifecycle
	// variable block was touched during the session.
	UpdatedLifecycleVars map[string]interface{}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Session
// ═══════════════════════════════════════════════════════════════════════════

// Session encapsulates a single execution of the strategy logic triggered by
// one candle event. It is the foundational unit of the Live Trade engine.
//
// Lifecycle:
//  1. BotManager creates a Session for each incoming candle event.
//  2. Session.Run() parses the strategy JSON, wires the ExecutionContext,
//     and delegates to blockly.RunWorkspace().
//  3. Session.Run() returns a SessionResult plus an optional error.
//  4. The BotManager goroutine inspects the result and updates state.
//
// Thread safety: Session is NOT safe for concurrent use. Each call to Run()
// must complete before the next candle event spawns a new Session on the same
// bot goroutine. The bot goroutine processes one candle at a time (sequential
// On-Candle-Close model, SRS FR-RUN-02).
type Session struct {
	cfg    SessionConfig
	logger *slog.Logger
}

// NewSession constructs a Session for the given configuration.
//
//   - cfg:    all dependencies for this execution (symbol, logicJSON, proxies).
//   - logger: a slog.Logger already decorated with bot_id and any other
//     request-scoped attributes. If nil, slog.Default() is used as fallback.
func NewSession(cfg SessionConfig, logger *slog.Logger) *Session {
	if logger == nil {
		logger = slog.Default()
	}
	return &Session{cfg: cfg, logger: logger}
}

// Run executes the strategy logic against the current candle event.
//
// Execution steps:
//  1. Parse cfg.LogicJSON into the root event_on_candle Block via ParseLogicJSON.
//  2. Extract trigger and timeframe metadata from the root block.
//  3. Resolve the strategy body — the statement chain under the "DO" input slot.
//  4. Create a fresh blockly.ExecutionContext for this session.
//  5. Inject CandleRepo, TradingProxy, timeframe, and trigger type.
//  6. Seed LifecycleVars into the context (pre-loaded by Task 2.7.3).
//  7. Call blockly.ExecuteChain(execCtx, bodyBlock) — the main execution loop.
//  8. Return a SessionResult with units consumed and the (possibly mutated)
//     lifecycle variable map.
//
// Error semantics:
//   - blockly.ErrUnitCostExceeded — budget exhausted; caller logs
//     "UNIT_COST_EXCEEDED" and stops the session (SRS FR-RUN-07).
//   - blockly.ErrInvalidRootBlock / ErrEmptyWorkspace — strategy JSON is
//     malformed; caller should mark the bot as Error.
//   - context.Canceled / context.DeadlineExceeded — bot was stopped mid-session;
//     caller should treat as a graceful stop, not an error.
//   - Any other error — unexpected runtime fault; caller marks bot as Error.
//
// Run does NOT call recover() — panic isolation is the BotManager's
// responsibility (manager.go runBotLoop). This keeps Session focused on
// pure execution logic.
func (s *Session) Run(ctx context.Context) (SessionResult, error) {
	// ── Step 1: Parse strategy logic JSON into root Block ──────────────────
	root, err := blockly.ParseLogicJSON(s.cfg.LogicJSON)
	if err != nil {
		return SessionResult{}, fmt.Errorf("session.Run: parse logic_json (bot_id=%s): %w",
			s.cfg.BotID, err)
	}

	// ── Step 2: Extract event metadata ─────────────────────────────────────
	trigger, timeframe := blockly.ExtractEventMeta(root)

	// ── Step 3: Resolve the strategy body (DO chain) ───────────────────────
	// GetInputBlock returns nil for an empty body — ExecuteChain treats nil as a
	// no-op, so a strategy with no logic blocks exits cleanly without error.
	bodyBlock := blockly.GetInputBlock(root, "DO")

	// ── Step 4: Build a fresh ExecutionContext for this session ────────────
	execCtx := blockly.NewExecutionContext(ctx, s.cfg.Symbol, s.logger)
	execCtx.Timeframe = timeframe
	execCtx.TriggerType = trigger

	// ── Step 5: Inject infrastructure dependencies ─────────────────────────
	execCtx.CandleRepo = s.cfg.CandleRepo
	execCtx.TradingProxy = s.cfg.TradingProxy

	// ── Step 6: Seed lifecycle variables from the pre-loaded snapshot ──────
	// Clone the map so that mutations during execution do not affect the
	// caller's copy until we are ready to return the updated state.
	execCtx.LifecycleVars = cloneVarMap(s.cfg.LifecycleVars)

	// ── Step 7: Execute the strategy body chain ────────────────────────────
	runErr := blockly.ExecuteChain(execCtx, bodyBlock)

	// ── Step 8: Capture result regardless of error ─────────────────────────
	result := SessionResult{
		UnitsUsed:            execCtx.UnitTracker.Used(),
		UpdatedLifecycleVars: execCtx.LifecycleVars,
	}

	// Context cancellation means the bot was stopped — not a logic error.
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return result, runErr
	}

	if runErr != nil {
		return result, fmt.Errorf("session.Run: execution fault (bot_id=%s): %w",
			s.cfg.BotID, runErr)
	}

	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Internal Helpers
// ═══════════════════════════════════════════════════════════════════════════

// cloneVarMap returns a shallow copy of a lifecycle variable map.
// A nil src produces a non-nil empty map so callers can safely range over it.
func cloneVarMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
