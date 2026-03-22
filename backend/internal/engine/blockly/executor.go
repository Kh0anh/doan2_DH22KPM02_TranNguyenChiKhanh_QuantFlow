package blockly

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Sentinel Errors
// ═══════════════════════════════════════════════════════════════════════════

var (
	// ErrUnknownBlockType is returned when a block type found in logic_json is
	// not registered in the 26-type blockRegistry. Indicates a corrupted or
	// incompatible strategy payload.
	ErrUnknownBlockType = errors.New("blockly: unknown block type")

	// ErrUnitCostExceeded is returned when the session's unit budget is exhausted.
	// Execution must stop immediately and a "UNIT_COST_EXCEEDED" log must be
	// written (SRS FR-RUN-07, blockly.md §1.3).
	ErrUnitCostExceeded = errors.New("blockly: UNIT_COST_EXCEEDED")

	// ErrInvalidRootBlock is returned when the first top-level block in the
	// workspace is not event_on_candle. Every valid strategy must start with
	// the event trigger block (SRS FR-RUN-02, blockly.md §3.1).
	ErrInvalidRootBlock = errors.New("blockly: root block must be event_on_candle")

	// ErrEmptyWorkspace is returned when logic_json deserializes to a workspace
	// with no top-level blocks.
	ErrEmptyWorkspace = errors.New("blockly: workspace contains no blocks")

	// ErrNoHandlerRegistered is returned at runtime when ExecuteBlock finds no
	// handler for a type that exists in blockRegistry. This is a programming
	// error — every registered block type must have a corresponding handler
	// wired in via RegisterHandler (in block_logic.go, block_math.go, etc.).
	ErrNoHandlerRegistered = errors.New("blockly: no handler registered for block type")
)

// ═══════════════════════════════════════════════════════════════════════════
//  Unit Cost Tracker Interface
// ═══════════════════════════════════════════════════════════════════════════

// UnitCostTracker tracks unit consumption within a single Session.
// Each Session starts with a budget of DefaultUnitCostLimit units (Task 2.5.7).
// Using an interface here decouples the execution core from the concrete
// implementation in unit_cost.go (Task 2.5.7).
type UnitCostTracker interface {
	// Consume deducts the given number of units from the session budget.
	// Returns ErrUnitCostExceeded if the deduction would exceed the limit.
	// A return value of nil means the deduction succeeded.
	Consume(units int) error

	// Used returns the total units consumed so far in the current session.
	Used() int
}

// noOpUnitTracker is a no-op UnitCostTracker used as the default until
// unit_cost.go (Task 2.5.7) is implemented. It approves every Consume call,
// allowing block handlers (Tasks 2.5.2–2.5.6) to be developed in isolation.
type noOpUnitTracker struct{}

func (noOpUnitTracker) Consume(_ int) error { return nil }
func (noOpUnitTracker) Used() int           { return 0 }

// NoOpUnitTracker returns a UnitCostTracker that never rejects any consumption.
// Use this as a stand-in until Task 2.5.7 provides the real implementation.
func NoOpUnitTracker() UnitCostTracker {
	return noOpUnitTracker{}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Candle Repository Reader Interface (for indicator blocks)
// ═══════════════════════════════════════════════════════════════════════════

// CandleRepositoryReader is the minimal read interface consumed by
// context-aware indicator blocks (indicator_rsi, indicator_ema — Task 2.5.5).
//
// Defined here in the blockly package to avoid an import cycle:
// engine/blockly must not depend on internal/repository directly (that layer
// depends on domain and GORM, creating unnecessary coupling to infrastructure).
// The concrete implementation in repository.CandleRepository satisfies this
// interface via Go's implicit structural typing — no adapter needed.
type CandleRepositoryReader interface {
	// QueryLatestClosedCandles returns the most recent `limit` fully-closed
	// candles for the given (symbol, interval) pair, ordered by open_time ASC.
	// Only candles with is_closed = true are included.
	QueryLatestClosedCandles(ctx context.Context, symbol, interval string, limit int) ([]domain.Candle, error)
}

// TradingProxy is the minimal interface consumed by Trading block group
// handlers (data_market_price, data_position_info, data_open_orders_count,
// data_balance, trade_smart_order, trade_close_position, trade_cancel_all_orders
// — Task 2.5.6, SRS FR-DESIGN-08, FR-DESIGN-09, FR-DESIGN-10).
//
// Defined here in the blockly package to avoid an import cycle:
// engine/blockly must not depend on internal/exchange directly (that layer
// contains infrastructure / HTTP client concerns). The concrete implementation
// exchange.BinanceProxy satisfies this interface via Go's implicit structural
// typing — no adapter needed. The Bot engine (Task 2.7.1) and Backtest
// simulator (Task 2.6.1) inject the concrete proxy after NewExecutionContext:
//
//	execCtx := blockly.NewExecutionContext(ctx, symbol, logger)
//	execCtx.TradingProxy = binanceProxy
//
// Data blocks return decimal.Zero + log a warning when TradingProxy is nil
// (e.g., in unit-test contexts that do not exercise data/trade blocks).
// Trade action blocks return an error when TradingProxy is nil.
type TradingProxy interface {
	// ── Data block methods (FR-DESIGN-08) ────────────────────────────────

	// GetLastPrice returns the latest ticker price for the given symbol.
	// Mapped to data_market_price with PRICE_TYPE = "LAST_PRICE".
	GetLastPrice(ctx context.Context, symbol string) (decimal.Decimal, error)

	// GetAvailableBalance returns the USDT available balance in the Futures wallet.
	// Mapped to data_balance.
	GetAvailableBalance(ctx context.Context) (decimal.Decimal, error)

	// GetPositionSize returns the absolute position amount for the symbol.
	// Positive = Long, Negative = Short. 0 = no open position.
	// Mapped to data_position_info with FIELD = "POSITION_SIZE".
	GetPositionSize(ctx context.Context, symbol string) (decimal.Decimal, error)

	// GetPositionEntryPrice returns the average entry price of the open position.
	// Returns decimal.Zero when no position is open.
	// Mapped to data_position_info with FIELD = "ENTRY_PRICE".
	GetPositionEntryPrice(ctx context.Context, symbol string) (decimal.Decimal, error)

	// GetPositionUnrealizedPNL returns the unrealized PnL of the open position.
	// Returns decimal.Zero when no position is open.
	// Mapped to data_position_info with FIELD = "UNREALIZED_PNL".
	GetPositionUnrealizedPNL(ctx context.Context, symbol string) (decimal.Decimal, error)

	// GetOpenOrdersCount returns the count of pending (open) orders for the symbol.
	// Mapped to data_open_orders_count.
	GetOpenOrdersCount(ctx context.Context, symbol string) (int, error)

	// ── Trade action methods (FR-DESIGN-09, FR-DESIGN-10) ─────────────────

	// SmartOrder is the "All-in-one" order placement method (FR-DESIGN-09).
	// Pre-flight: auto-adjusts Leverage and MarginType on the exchange before
	// placing the order if they differ from the account's current settings.
	//
	// Returns an *OrderResult with fill details (price, quantity, fee) so that
	// the caller can persist the trade to trade_history (Task 2.8.5).
	//
	// Parameters:
	//   symbol     — trading pair, e.g. "BTCUSDT" (from ExecutionContext.Symbol).
	//   side       — "LONG" or "SHORT" (blockly.md §3.6.5 SIDE field).
	//   orderType  — "MARKET" or "LIMIT".
	//   price      — limit price; ignored (may be Zero) for MARKET orders.
	//   quantity   — order size in base asset (e.g. BTC).
	//   leverage   — desired leverage multiplier (1–125).
	//   marginType — "CROSS" or "ISOLATED".
	SmartOrder(ctx context.Context, symbol, side, orderType string,
		price, quantity decimal.Decimal, leverage int, marginType string) (*domain.OrderResult, error)

	// ClosePosition closes the entire open position for the symbol via a
	// reduce-only MARKET order. No-op (returns (nil, nil)) when no position is open.
	// Returns an *OrderResult with fill details when a position was closed.
	// Mapped to trade_close_position.
	ClosePosition(ctx context.Context, symbol string) (*domain.OrderResult, error)

	// CancelAllOrders cancels every open order for the symbol.
	// No-op (returns nil) when no open orders exist.
	// Mapped to trade_cancel_all_orders.
	CancelAllOrders(ctx context.Context, symbol string) error
}

// ═══════════════════════════════════════════════════════════════════════════
//  Execution Context
// ═══════════════════════════════════════════════════════════════════════════

// ExecutionContext holds all mutable runtime state for a single Session.
//
// Lifecycle: A Session is one invocation of the event_on_candle trigger
// (blockly.md §1.3). A fresh ExecutionContext is created at the start of each
// Session via NewExecutionContext and discarded when the Session ends.
//
// Thread safety: ExecutionContext is NOT safe for concurrent access. Each
// goroutine (bot) maintains its own isolated context.
type ExecutionContext struct {
	// Ctx is the Go context.Context propagated from the parent goroutine.
	// Used to detect bot stop signals or deadline expiry mid-session.
	Ctx context.Context

	// Symbol is the trading pair (e.g., "BTCUSDT") injected when the Bot
	// Instance is created. All context-aware blocks (indicators, data, trade)
	// use this automatically — users never specify the symbol per block
	// (blockly.md §1.2 — Context-aware principle).
	Symbol string

	// Timeframe is the candle interval extracted from the root event_on_candle
	// block (e.g., "1m", "15m", "1h"). Set by ExtractEventMeta.
	Timeframe string

	// TriggerType indicates whether the session fires on candle close or open.
	// Possible values: "ON_CANDLE_CLOSE" | "ON_CANDLE_OPEN" (blockly.md §3.1.1).
	TriggerType string

	// SessionVars stores temporary variables for the current Session only.
	// Created as an empty map at session start; discarded at session end.
	// Read and written by variables_session_get and variables_session_set blocks.
	// Returns the zero value (nil → callers treat as 0) for unset names.
	SessionVars map[string]interface{}

	// LifecycleVars stores persistent variables that survive across Sessions.
	// Loaded from bot_lifecycle_variables (DB JSONB) at the start of each
	// Session and flushed back to the DB at the end (Task 2.5.4 / Task 2.7.3).
	// Read and written by variables_lifecycle_get and variables_lifecycle_set.
	LifecycleVars map[string]interface{}

	// UnitTracker enforces the per-session unit cost budget (SRS FR-RUN-07).
	// Defaults to NoOpUnitTracker() until Task 2.5.7 wires in the real impl.
	// ExecuteBlock calls UnitTracker.Consume before every block dispatch.
	UnitTracker UnitCostTracker

	// Logger is a structured slog.Logger pre-decorated with request-scoped
	// fields (e.g., bot_id or backtest_id). All block-level log calls go
	// through this logger so log lines are traceable to their originating bot.
	Logger *slog.Logger

	// CandleRepo provides read access to the candles_data table for
	// context-aware indicator blocks (indicator_rsi, indicator_ema — Task 2.5.5,
	// SRS FR-DESIGN-07). The Bot engine (Task 2.7.1) and Backtest simulator
	// (Task 2.6.1) inject this after calling NewExecutionContext:
	//
	//   execCtx := blockly.NewExecutionContext(ctx, symbol, logger)
	//   execCtx.CandleRepo = candleRepo
	//
	// Indicator blocks return decimal.Zero + log a warning when CandleRepo is
	// nil (e.g., in unit-test contexts that do not exercise indicator blocks).
	CandleRepo CandleRepositoryReader

	// TradingProxy provides live exchange access for Trading block group handlers
	// (data_market_price, data_position_info, data_open_orders_count, data_balance,
	// trade_smart_order, trade_close_position, trade_cancel_all_orders — Task 2.5.6,
	// SRS FR-DESIGN-08, FR-DESIGN-09, FR-DESIGN-10).
	//
	// The Bot engine (Task 2.7.1) and Backtest simulator (Task 2.6.1) inject the
	// concrete exchange.BinanceProxy after calling NewExecutionContext:
	//
	//   execCtx := blockly.NewExecutionContext(ctx, symbol, logger)
	//   execCtx.TradingProxy = binanceProxy
	//
	// Data blocks (data_*) return decimal.Zero + log a warning when TradingProxy
	// is nil (safe degradation for unit tests that don't exercise trading blocks).
	// Trade action blocks (trade_*) return an error when TradingProxy is nil.
	TradingProxy TradingProxy

	// TradeResults accumulates OrderResult entries produced by trade action
	// blocks (trade_smart_order, trade_close_position) during the current session.
	// The bot manager reads this slice after Session.Run() completes and persists
	// each entry to trade_history (Task 2.8.5).
	TradeResults []*domain.OrderResult
}

// NewExecutionContext constructs a fresh ExecutionContext for a new Session.
//
//   - ctx:    Go context from the parent bot goroutine (for cancellation).
//   - symbol: Trading pair from the Bot Instance (e.g., "BTCUSDT").
//   - logger: slog.Logger already decorated with bot_id / backtest_id fields.
//
// If logger is nil, slog.Default() is used as a fallback.
func NewExecutionContext(ctx context.Context, symbol string, logger *slog.Logger) *ExecutionContext {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExecutionContext{
		Ctx:           ctx,
		Symbol:        symbol,
		SessionVars:   make(map[string]interface{}),
		LifecycleVars: make(map[string]interface{}),
		UnitTracker:   NewDefaultSessionUnitTracker(),
		Logger:        logger,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Block Handler Registry
// ═══════════════════════════════════════════════════════════════════════════

// BlockHandler is the function signature for all block execution implementations.
//
//   - Statement blocks (previousStatement + nextStatement) should return
//     (nil, nil) on success — their effect is a side-effect (e.g., place order).
//   - Value blocks (with output) must return their computed result as interface{}:
//     float64 for Number outputs, bool for Boolean outputs.
//
// Context cancellation and ErrUnitCostExceeded must be propagated as errors.
type BlockHandler func(ctx *ExecutionContext, block *Block) (interface{}, error)

// handlerRegistry maps each block type string to its BlockHandler.
// Populated at program startup via RegisterHandler calls in block_*.go init().
var handlerRegistry = map[string]BlockHandler{}

// RegisterHandler registers a BlockHandler for the given block type.
// Must be called from package-level init() in block_logic.go, block_math.go,
// block_variable.go, block_indicator.go, block_trading.go, and block_data.go.
//
// Panics if a handler for the same block type is registered more than once —
// this is a programming error that must be caught at startup, not at runtime.
func RegisterHandler(blockType string, h BlockHandler) {
	if _, exists := handlerRegistry[blockType]; exists {
		panic(fmt.Sprintf("blockly: duplicate handler registration for block type %q", blockType))
	}
	handlerRegistry[blockType] = h
}

// ═══════════════════════════════════════════════════════════════════════════
//  Execution Engine
// ═══════════════════════════════════════════════════════════════════════════

// ExecuteChain walks a linked statement chain starting at block, executing
// each block sequentially by following Next connections until the chain ends.
//
// Used for:
//   - The main body under event_on_candle (the strategy's root logic chain).
//   - The DO and ELSE branches of controls_if / controls_if_else.
//   - The body of controls_repeat and controls_while loops.
//
// Returns on the first error encountered (including ErrUnitCostExceeded and
// context cancellation). A nil block argument is a no-op (empty branch).
func ExecuteChain(ctx *ExecutionContext, block *Block) error {
	current := block
	for current != nil {
		// Propagate context cancellation (e.g., bot was stopped mid-session).
		if err := ctx.Ctx.Err(); err != nil {
			return err
		}

		if _, err := ExecuteBlock(ctx, current); err != nil {
			return err
		}

		// Advance to the next statement in the chain.
		if current.Next != nil {
			current = current.Next.Block
		} else {
			current = nil
		}
	}
	return nil
}

// EvalValue evaluates a value block and returns its computed result.
// Value blocks are those wired into input slots (e.g., CONDITION, A, B,
// QUANTITY, PERIOD, NUM). Callers receive the raw interface{} and perform
// their own type assertion (float64 for Number, bool for Boolean).
//
// Returns (nil, nil) when block is nil — an unconnected input slot.
// Callers must treat nil as a sensible zero value for their context.
func EvalValue(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if block == nil {
		return nil, nil
	}
	return ExecuteBlock(ctx, block)
}

// ExecuteBlock dispatches execution of a single block to its registered handler.
//
// Execution pipeline (in order):
//  1. Nil guard — returns (nil, nil) immediately for nil blocks.
//  2. Registry lookup — validates block.Type is one of the 26 known types.
//  3. Unit cost deduction — calls UnitTracker.Consume(meta.UnitCost) before
//     the handler runs. Returns ErrUnitCostExceeded if budget is exhausted.
//  4. Handler dispatch — calls the registered BlockHandler for block.Type.
//
// Note: controls_repeat and controls_while handlers (Task 2.5.2) call
// UnitTracker.Consume(1) themselves on each loop iteration because their
// registry UnitCost is charged per-iteration, not per-block-encounter.
func ExecuteBlock(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if block == nil {
		return nil, nil
	}

	// Step 1 — Validate block type against the 26-type registry.
	meta, err := GetBlockMeta(block.Type)
	if err != nil {
		ctx.Logger.Error("unknown block type encountered during execution",
			slog.String("block_type", block.Type),
			slog.String("block_id", block.ID),
		)
		return nil, fmt.Errorf("%w: %q", ErrUnknownBlockType, block.Type)
	}

	// Step 2 — Deduct unit cost before running the block.
	// For loop blocks (controls_repeat, controls_while), UnitCost in registry
	// is 1 (charged per iteration by the handler), so Consume(1) here charges
	// the initial loop setup — subsequent per-iteration charges are in the handler.
	if consumeErr := ctx.UnitTracker.Consume(meta.UnitCost); consumeErr != nil {
		ctx.Logger.Warn("UNIT_COST_EXCEEDED — stopping session",
			slog.Int("units_used", ctx.UnitTracker.Used()),
			slog.String("block_type", block.Type),
			slog.String("block_id", block.ID),
		)
		return nil, ErrUnitCostExceeded
	}

	// Step 3 — Dispatch to the registered handler.
	handler, ok := handlerRegistry[block.Type]
	if !ok {
		// All types in blockRegistry must have a handler wired in block_*.go.
		// Reaching here means a handler was not registered — development bug.
		return nil, fmt.Errorf("%w: %q", ErrNoHandlerRegistered, block.Type)
	}

	return handler(ctx, block)
}

// ExtractEventMeta reads the TRIGGER and TIMEFRAME field values from the root
// event_on_candle block and returns them as plain strings (blockly.md §3.1.1).
//
// These values are used by the Bot event listener (Task 2.7.2) to subscribe
// to the correct Binance kline WebSocket stream, and by the Backtest simulator
// (Task 2.6.1) to select the correct candle series.
//
// Defaults: trigger = "ON_CANDLE_CLOSE", timeframe = "1m" when fields absent.
func ExtractEventMeta(root *Block) (trigger, timeframe string) {
	trigger = GetFieldString(root, "TRIGGER")
	if trigger == "" {
		trigger = "ON_CANDLE_CLOSE"
	}

	timeframe = GetFieldString(root, "TIMEFRAME")
	if timeframe == "" {
		timeframe = "1m"
	}

	return trigger, timeframe
}
