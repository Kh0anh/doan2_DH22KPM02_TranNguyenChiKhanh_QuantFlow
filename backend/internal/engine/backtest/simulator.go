// Package backtest implements the Backtest Simulation Engine for QuantFlow.
// It replays historical candle data through the Blockly execution engine,
// producing a per-candle execution log consumed by downstream tasks:
//
//   - Task 2.6.2 — order_matcher.go  (Order Matching Simulator)
//   - Task 2.6.3 — performance.go    (Performance Report)
//   - Task 2.6.4 — equity_curve.go   (Equity Curve Data Generation)
//   - Task 2.6.5 — backtest_logic.go (Async API orchestration)
//
// Task 2.6.1 — Simulation Engine: sequential On Candle Close simulation.
// WBS: P2-Backend · 13/03/2026
// SRS: FR-RUN-02, NFR-PERF-02 (35 K candles < 10 s)
package backtest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/engine/blockly"
	"github.com/kh0anh/quantflow/internal/repository"
	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Sentinel Errors
// ═══════════════════════════════════════════════════════════════════════════

var (
	// ErrNoCandleData is returned when the database contains no candles for
	// the requested (symbol, timeframe, start–end) range.
	// Callers should surface this as HTTP 422 — the user must trigger a
	// data sync before running the backtest.
	ErrNoCandleData = errors.New("backtest: no candle data found for the given range")

	// ErrInvalidTrigger is returned when the strategy's root block is not
	// configured with trigger type "ON_CANDLE_CLOSE". The simulator only
	// supports closed-candle simulation (SRS FR-RUN-02).
	ErrInvalidTrigger = errors.New("backtest: strategy must use ON_CANDLE_CLOSE trigger")
)

// ═══════════════════════════════════════════════════════════════════════════
//  Backtest Configuration
// ═══════════════════════════════════════════════════════════════════════════

// Config holds all parameters required to run a single backtest simulation.
// It is populated by BacktestLogic (task 2.6.5) from the CreateBacktestRequest
// (api.yaml §CreateBacktestRequest).
type Config struct {
	// StrategyVersionID identifies the pinned strategy_versions row whose
	// logic_json will be parsed and executed.
	StrategyVersionID string

	// Symbol is the Binance Futures trading pair (e.g., "BTCUSDT").
	Symbol string

	// Timeframe is the candle interval (e.g., "1m", "15m", "1h").
	// Must match one of domain.CandleInterval* constants.
	Timeframe string

	// StartTime is the inclusive start of the historical range to simulate.
	StartTime time.Time

	// EndTime is the inclusive end of the historical range to simulate.
	EndTime time.Time

	// InitialCapital is the simulated starting balance in USDT.
	InitialCapital decimal.Decimal

	// FeeRate is the simulated per-trade fee as a decimal fraction.
	// Example: 0.0004 represents 0.04% (Binance Futures taker fee).
	FeeRate decimal.Decimal

	// MaxUnit is the per-session unit budget injected into the UnitCostTracker.
	// Defaults to blockly.DefaultUnitCostLimit (1000) when the API caller
	// omits the field (api.yaml §CreateBacktestRequest.max_unit default).
	MaxUnit int
}

// ═══════════════════════════════════════════════════════════════════════════
//  Simulation State (mutable, shared across candle sessions in-memory)
// ═══════════════════════════════════════════════════════════════════════════

// SimulatedPosition represents the bot's open Futures position during
// simulation. Side is "" when no position is open.
type SimulatedPosition struct {
	// Side is "LONG" or "SHORT"; empty string when flat (no open position).
	Side string

	// Size is the absolute position quantity in base asset (e.g., BTC).
	// Always positive regardless of direction — Side carries the direction.
	Size decimal.Decimal

	// EntryPrice is the average entry fill price for the current position.
	EntryPrice decimal.Decimal
}

// IsFlat returns true when there is no open position.
func (p SimulatedPosition) IsFlat() bool { return p.Side == "" }

// UnrealizedPNL computes the unrealized profit/loss against the given mark
// price using the standard linear futures formula.
//
//	Long:  (markPrice - entryPrice) * size
//	Short: (entryPrice - markPrice) * size
func (p SimulatedPosition) UnrealizedPNL(markPrice decimal.Decimal) decimal.Decimal {
	if p.IsFlat() {
		return decimal.Zero
	}
	if p.Side == "LONG" {
		return markPrice.Sub(p.EntryPrice).Mul(p.Size)
	}
	// SHORT
	return p.EntryPrice.Sub(markPrice).Mul(p.Size)
}

// SimulationState is the mutable account state that persists across all
// candle sessions within a single backtest run.
// It is owned exclusively by the Simulator.Run() goroutine — no locking needed.
type SimulationState struct {
	// Balance is the current simulated USDT balance.
	// Starts at Config.InitialCapital; updated when orders are filled (2.6.2).
	Balance decimal.Decimal

	// Position is the current open Futures position (flat when Side == "").
	Position SimulatedPosition

	// PendingOrders accumulates orders submitted by trade action blocks
	// during the current candle session. Drained and processed by the order
	// matcher (task 2.6.2) after each session completes.
	PendingOrders []PendingOrder
}

// ═══════════════════════════════════════════════════════════════════════════
//  Pending Order (submitted during a session, filled by order_matcher.go)
// ═══════════════════════════════════════════════════════════════════════════

// PendingOrder represents an order submitted by a trade action block during
// a single candle session. It is NOT yet filled — order matching (task 2.6.2)
// processes these against the NEXT candle's OHLCV data.
//
// Order matching rules (SRS FR-RUN-02, api.yaml §createBacktest description):
//   - MARKET orders → fill at the Open price of the NEXT candle.
//   - LIMIT orders  → fill when the NEXT candle's range (High/Low) crosses
//     the limit price.
type PendingOrder struct {
	// Side is "LONG" or "SHORT" (matches blockly SIDE field values).
	Side string

	// OrderType is "MARKET" or "LIMIT".
	OrderType string

	// LimitPrice is the target fill price for LIMIT orders.
	// Zero for MARKET orders.
	LimitPrice decimal.Decimal

	// Quantity is the order size in base asset (e.g., BTC).
	Quantity decimal.Decimal

	// Leverage is the desired multiplier (1–125).
	Leverage int

	// MarginType is "ISOLATED" or "CROSS".
	MarginType string

	// IsReduceOnly marks orders generated by ClosePosition (reduce-only
	// MARKET orders that close the existing position rather than open a new one).
	IsReduceOnly bool

	// SubmittedAtCandle is the open_time of the candle during which the order
	// was submitted. Used by the order matcher to select the next candle.
	SubmittedAtCandle time.Time
}

// ═══════════════════════════════════════════════════════════════════════════
//  Candle Execution Result
// ═══════════════════════════════════════════════════════════════════════════

// CandleExecution records the outcome of running the strategy logic against
// a single historical candle.
type CandleExecution struct {
	// Candle is the closed candle that triggered this session.
	Candle domain.Candle

	// OrdersSubmitted contains all orders that trade action blocks submitted
	// during this session. May be empty if no trade actions were executed.
	OrdersSubmitted []PendingOrder

	// SessionError is non-nil if the session was terminated abnormally
	// (e.g., ErrUnitCostExceeded, context cancelled). A non-nil error means
	// OrdersSubmitted may be incomplete for this candle.
	SessionError error
}

// ═══════════════════════════════════════════════════════════════════════════
//  Run Output
// ═══════════════════════════════════════════════════════════════════════════

// RunOutput is the complete result of a Simulator.Run() call. It is the
// primary input to the downstream tasks (order matching, performance report,
// equity curve generation).
type RunOutput struct {
	// Config is a copy of the Config passed to Run(), enabling downstream
	// consumers to reference simulation parameters without extra state.
	Config Config

	// Candles is the full slice of historical candles that was loaded and
	// iterated. Ordered by open_time ASC (same order as simulation).
	Candles []domain.Candle

	// Executions has one entry per element in Candles, at the same index.
	// len(Executions) == len(Candles) is always guaranteed.
	Executions []CandleExecution

	// FinalLifecycleVars is the state of lifecycle variables at the end of
	// the simulation. Useful for debugging and future partial-resume scenarios.
	FinalLifecycleVars map[string]interface{}
}

// ═══════════════════════════════════════════════════════════════════════════
//  BacktestCandleReader — implements blockly.CandleRepositoryReader
// ═══════════════════════════════════════════════════════════════════════════

// backtestCandleReader is a read-only view of a pre-loaded candle slice that
// satisfies the blockly.CandleRepositoryReader interface consumed by indicator
// blocks (indicator_rsi, indicator_ema — task 2.5.5).
//
// Anti-look-ahead principle: at candle index i, indicator blocks may only
// see candles at indices [0, i). The currentIdx field enforces this window.
// This prevents the simulation from using future candle data to compute
// indicators, which would produce unrealistically good results.
type backtestCandleReader struct {
	candles    []domain.Candle // full pre-loaded slice (ASC order)
	currentIdx int             // exclusive upper bound — candles[:currentIdx] are visible
}

// newBacktestCandleReader constructs a backtestCandleReader for the given
// candle slice, restricting indicator visibility to candles[:currentIdx].
func newBacktestCandleReader(candles []domain.Candle, currentIdx int) *backtestCandleReader {
	return &backtestCandleReader{candles: candles, currentIdx: currentIdx}
}

// QueryLatestClosedCandles satisfies blockly.CandleRepositoryReader.
// It returns the most recent `limit` fully-closed candles visible at the
// current simulation point (indices [0, currentIdx)), ordered ASC.
//
// The symbol and interval parameters are accepted for interface compliance but
// are not validated — the pre-loaded slice already contains the correct data
// for the (symbol, timeframe) pair configured at backtest creation time.
func (r *backtestCandleReader) QueryLatestClosedCandles(
	_ context.Context, _, _ string, limit int,
) ([]domain.Candle, error) {
	if r.currentIdx == 0 || limit <= 0 {
		return []domain.Candle{}, nil
	}

	// visible window: candles[0 : currentIdx]
	visible := r.candles[:r.currentIdx]

	// Take the last `limit` elements from visible (newest first from the end).
	start := len(visible) - limit
	if start < 0 {
		start = 0
	}
	return visible[start:], nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  SimulatedTradingProxy — implements blockly.TradingProxy
// ═══════════════════════════════════════════════════════════════════════════

// simulatedTradingProxy satisfies the blockly.TradingProxy interface using
// in-memory SimulationState rather than real Binance API calls.
//
// Data blocks return values derived from the current candle and account state.
// Trade action blocks append PendingOrders to the newOrders slice; actual
// order matching is deferred to task 2.6.2 (order_matcher.go).
type simulatedTradingProxy struct {
	state         *SimulationState
	currentCandle domain.Candle
	newOrders     *[]PendingOrder // pointer so appends are visible to the caller
}

// newSimulatedTradingProxy constructs a simulatedTradingProxy for one candle session.
func newSimulatedTradingProxy(
	state *SimulationState,
	currentCandle domain.Candle,
	newOrders *[]PendingOrder,
) *simulatedTradingProxy {
	return &simulatedTradingProxy{
		state:         state,
		currentCandle: currentCandle,
		newOrders:     newOrders,
	}
}

// ── Data block implementations (blockly.TradingProxy — FR-DESIGN-08) ─────

// GetLastPrice returns the close price of the current simulation candle.
// This mirrors live-trade behaviour where the strategy reads the price of the
// just-closed candle (On Candle Close trigger).
func (p *simulatedTradingProxy) GetLastPrice(_ context.Context, _ string) (decimal.Decimal, error) {
	price, err := decimal.NewFromString(p.currentCandle.ClosePrice)
	if err != nil {
		return decimal.Zero, fmt.Errorf("simulatedTradingProxy: GetLastPrice: parse close_price %q: %w",
			p.currentCandle.ClosePrice, err)
	}
	return price, nil
}

// GetAvailableBalance returns the current simulated USDT balance.
func (p *simulatedTradingProxy) GetAvailableBalance(_ context.Context) (decimal.Decimal, error) {
	return p.state.Balance, nil
}

// GetPositionSize returns the signed position size.
// Positive = Long, Negative = Short, Zero = flat.
func (p *simulatedTradingProxy) GetPositionSize(_ context.Context, _ string) (decimal.Decimal, error) {
	pos := p.state.Position
	if pos.IsFlat() {
		return decimal.Zero, nil
	}
	if pos.Side == "LONG" {
		return pos.Size, nil
	}
	// SHORT — return negative size so callers can determine direction
	return pos.Size.Neg(), nil
}

// GetPositionEntryPrice returns the average entry price of the open position.
// Returns zero when the position is flat.
func (p *simulatedTradingProxy) GetPositionEntryPrice(_ context.Context, _ string) (decimal.Decimal, error) {
	if p.state.Position.IsFlat() {
		return decimal.Zero, nil
	}
	return p.state.Position.EntryPrice, nil
}

// GetPositionUnrealizedPNL returns the unrealized PnL using the current
// candle's close price as the mark price.
func (p *simulatedTradingProxy) GetPositionUnrealizedPNL(_ context.Context, _ string) (decimal.Decimal, error) {
	closePrice, err := decimal.NewFromString(p.currentCandle.ClosePrice)
	if err != nil {
		return decimal.Zero, fmt.Errorf("simulatedTradingProxy: GetPositionUnrealizedPNL: parse close_price: %w", err)
	}
	return p.state.Position.UnrealizedPNL(closePrice), nil
}

// GetOpenOrdersCount returns the number of pending orders queued so far in
// the current candle session (i.e., submitted but not yet matched).
func (p *simulatedTradingProxy) GetOpenOrdersCount(_ context.Context, _ string) (int, error) {
	return len(*p.newOrders), nil
}

// ── Trade action implementations (blockly.TradingProxy — FR-DESIGN-09/10) ─

// SmartOrder records a new PendingOrder submitted by the trade_smart_order
// block. Actual order matching against the NEXT candle is performed by
// task 2.6.2 (order_matcher.go). No position state is mutated here.
func (p *simulatedTradingProxy) SmartOrder(
	_ context.Context,
	_ string, // symbol — always cfg.Symbol for a backtest session
	side, orderType string,
	price, quantity decimal.Decimal,
	leverage int,
	marginType string,
) error {
	if quantity.IsZero() || quantity.IsNegative() {
		return fmt.Errorf("simulatedTradingProxy: SmartOrder: quantity must be positive, got %s", quantity)
	}

	*p.newOrders = append(*p.newOrders, PendingOrder{
		Side:              side,
		OrderType:         orderType,
		LimitPrice:        price,
		Quantity:          quantity,
		Leverage:          leverage,
		MarginType:        marginType,
		IsReduceOnly:      false,
		SubmittedAtCandle: p.currentCandle.OpenTime,
	})
	return nil
}

// ClosePosition submits a reduce-only MARKET order that closes the entire
// open position. No-op when the current position is flat.
func (p *simulatedTradingProxy) ClosePosition(_ context.Context, _ string) error {
	pos := p.state.Position
	if pos.IsFlat() {
		// No open position — no-op, matches live-trade behaviour.
		return nil
	}

	// The closing order is on the OPPOSITE side to the open position.
	closeSide := "SHORT"
	if pos.Side == "SHORT" {
		closeSide = "LONG"
	}

	*p.newOrders = append(*p.newOrders, PendingOrder{
		Side:              closeSide,
		OrderType:         "MARKET",
		LimitPrice:        decimal.Zero,
		Quantity:          pos.Size,
		Leverage:          0, // inherited from existing position
		MarginType:        "",
		IsReduceOnly:      true,
		SubmittedAtCandle: p.currentCandle.OpenTime,
	})
	return nil
}

// CancelAllOrders clears all pending orders that were submitted during the
// current candle session but have not been matched yet.
func (p *simulatedTradingProxy) CancelAllOrders(_ context.Context, _ string) error {
	*p.newOrders = (*p.newOrders)[:0]
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Simulator
// ═══════════════════════════════════════════════════════════════════════════

// Simulator is the backtest engine. It iterates historical candles through the
// Blockly execution engine, simulating On Candle Close events sequentially.
//
// Construct one Simulator and call Run() for each backtest request — the
// struct itself is stateless; all mutable state lives inside Run()'s stack.
type Simulator struct {
	candleRepo repository.CandleRepository
	logger     *slog.Logger
}

// NewBacktestSimulator constructs a Simulator with the provided dependencies.
//
//   - candleRepo: used to load historical candles from candles_data in a
//     single batched query before the simulation loop starts.
//   - logger: base slog.Logger; Run() decorates it with backtest_id and symbol
//     fields for per-run traceability.
func NewBacktestSimulator(
	candleRepo repository.CandleRepository,
	logger *slog.Logger,
) *Simulator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Simulator{
		candleRepo: candleRepo,
		logger:     logger,
	}
}

// Run executes a complete backtest simulation for the given Config.
//
// Execution model (SRS FR-RUN-02):
//  1. Load ALL historical candles for (symbol, timeframe, start–end) from DB
//     in a SINGLE query — no per-candle I/O inside the loop (NFR-PERF-02).
//  2. Parse logic_json into a *blockly.Block AST and validate the root trigger.
//  3. Iterate each closed candle sequentially. For every candle i:
//     a. Wrap candles[:i] in a BacktestCandleReader (anti-look-ahead guard).
//     b. Inject a SimulatedTradingProxy backed by in-memory SimulationState.
//     c. Execute the strategy body via blockly.ExecuteChain.
//     d. Record submitted orders in CandleExecution.
//  4. Lifecycle variables persist in-memory across all sessions (no DB writes).
//  5. Return a RunOutput consumed by downstream tasks (2.6.2–2.6.5).
//
// progress is an atomic int32 counter updated from 0→100 as the loop advances.
// Pass a pointer to an int32 managed by the async task goroutine (task 2.6.5)
// so the GET /backtests/{id} endpoint can report live progress.
//
// The function respects ctx cancellation at every loop iteration. When the
// caller cancels ctx, Run returns context.Canceled (or context.DeadlineExceeded).
func (s *Simulator) Run(
	ctx context.Context,
	cfg Config,
	logicJSON []byte,
	progress *int32,
) (*RunOutput, error) {
	log := s.logger.With(
		slog.String("symbol", cfg.Symbol),
		slog.String("timeframe", cfg.Timeframe),
		slog.Time("start", cfg.StartTime),
		slog.Time("end", cfg.EndTime),
	)

	// ── Step 1: Load historical candles (single DB round-trip) ───────────────

	// Use a generous limit ceiling (100 000) so we capture even dense
	// minute-level ranges spanning months. The real upper bound is enforced
	// by the start/end time filter; this limit acts as a safety cap.
	const maxCandleLoad = 100_000

	candles, err := s.candleRepo.QueryCandles(
		ctx,
		cfg.Symbol,
		cfg.Timeframe,
		&cfg.StartTime,
		&cfg.EndTime,
		maxCandleLoad,
	)
	if err != nil {
		return nil, fmt.Errorf("backtest: Run: QueryCandles: %w", err)
	}
	if len(candles) == 0 {
		return nil, ErrNoCandleData
	}

	log.Info("backtest: candles loaded",
		slog.Int("count", len(candles)),
	)

	// ── Step 2: Parse and validate strategy logic ─────────────────────────────

	root, err := blockly.ParseLogicJSON(logicJSON)
	if err != nil {
		return nil, fmt.Errorf("backtest: Run: ParseLogicJSON: %w", err)
	}

	trigger, timeframe := blockly.ExtractEventMeta(root)
	if trigger != "ON_CANDLE_CLOSE" {
		return nil, ErrInvalidTrigger
	}

	// Resolve the strategy body — the statement chain attached to the DO input
	// of the event_on_candle root block.
	bodyBlock := blockly.GetInputBlock(root, "DO")

	log.Info("backtest: strategy parsed",
		slog.String("trigger", trigger),
		slog.String("strategy_timeframe", timeframe),
	)

	// ── Step 3: Initialise shared state ──────────────────────────────────────

	maxUnit := cfg.MaxUnit
	if maxUnit <= 0 {
		maxUnit = blockly.DefaultUnitCostLimit
	}

	state := &SimulationState{
		Balance:       cfg.InitialCapital,
		Position:      SimulatedPosition{},
		PendingOrders: nil,
	}

	// lifecycleVars persists across all candle sessions (in-memory only for
	// backtest — no DB read/write, unlike live Bot engine in task 2.7.3).
	lifecycleVars := make(map[string]interface{})

	total := len(candles)
	executions := make([]CandleExecution, 0, total)

	// ── Step 4: Main simulation loop ─────────────────────────────────────────

	for i, candle := range candles {
		// Honour context cancellation at each candle boundary.
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Update atomic progress counter (0–99 during loop; 100 after loop).
		if progress != nil {
			atomic.StoreInt32(progress, int32(i*100/total))
		}

		// ── 4a: Anti-look-ahead: indicator blocks only see candles[:i] ────
		// currentIdx = i means candles[0..i-1] are visible; candle i (current)
		// is NOT included so indicators compute on truly historical data.
		candleReader := newBacktestCandleReader(candles, i)

		// ── 4b: Build order collection slice for this session ─────────────
		newOrders := make([]PendingOrder, 0)

		// ── 4c: Construct SimulatedTradingProxy ───────────────────────────
		proxy := newSimulatedTradingProxy(state, candle, &newOrders)

		// ── 4d: Build fresh ExecutionContext ──────────────────────────────
		sessionLogger := log.With(
			slog.Int("candle_index", i),
			slog.Time("candle_open_time", candle.OpenTime),
		)
		execCtx := blockly.NewExecutionContext(ctx, cfg.Symbol, sessionLogger)
		execCtx.Timeframe = timeframe
		execCtx.TriggerType = "ON_CANDLE_CLOSE"
		execCtx.CandleRepo = candleReader
		execCtx.TradingProxy = proxy
		// Override the LifecycleVars map with the shared cross-session map.
		execCtx.LifecycleVars = lifecycleVars
		// Override UnitTracker with the configured budget ceiling.
		execCtx.UnitTracker = blockly.NewSessionUnitTracker(maxUnit)

		// ── 4e: Execute strategy body ─────────────────────────────────────
		sessionErr := blockly.ExecuteChain(execCtx, bodyBlock)

		if sessionErr != nil {
			if errors.Is(sessionErr, blockly.ErrUnitCostExceeded) {
				// Unit cost exceeded in a single session: log and skip this
				// candle's execution, but continue the simulation. This mirrors
				// live Bot behaviour (SRS FR-RUN-07).
				sessionLogger.Warn("backtest: session unit cost exceeded",
					slog.Int("units_used", execCtx.UnitTracker.Used()),
					slog.Int("max_unit", maxUnit),
				)
			} else if errors.Is(sessionErr, context.Canceled) || errors.Is(sessionErr, context.DeadlineExceeded) {
				// Caller cancelled the backtest (e.g., DELETE /backtests/{id}/cancel).
				return nil, sessionErr
			} else {
				// Strategy logic error (unexpected): log and continue.
				sessionLogger.Error("backtest: session execution error",
					slog.String("error", sessionErr.Error()),
				)
			}
		}

		// ── 4f: Record candle execution result ────────────────────────────
		exec := CandleExecution{
			Candle:          candle,
			OrdersSubmitted: newOrders,
			SessionError:    sessionErr,
		}
		executions = append(executions, exec)
	}

	// Mark progress complete.
	if progress != nil {
		atomic.StoreInt32(progress, 100)
	}

	log.Info("backtest: simulation complete",
		slog.Int("candles_processed", total),
		slog.Int("executions_recorded", len(executions)),
	)

	return &RunOutput{
		Config:             cfg,
		Candles:            candles,
		Executions:         executions,
		FinalLifecycleVars: lifecycleVars,
	}, nil
}
