package blockly

// block_trading.go implements execution handlers for the Trading → Trade Action
// block sub-group (3 blocks), as specified in blockly.md §3.6.5–§3.6.7 and
// SRS FR-DESIGN-09 (Smart Order) and FR-DESIGN-10 (Close / Cancel).
//
// Task 2.5.6 — Execute Trading group (7 blocks - data + trade actions).
// WBS: P2-Backend · 12/03/2026
// SRS: FR-DESIGN-09, FR-DESIGN-10
//
// Blocks implemented (all are statement blocks — no return value):
//
//	statement: trade_smart_order, trade_close_position, trade_cancel_all_orders
//
// Error-propagation policy:
//
//	Unlike data_* blocks (which degrade gracefully to decimal.Zero), trade_*
//	blocks propagate EVERY error up the execution chain. A failed order placement
//	is never silently ignored — the Bot Instance must log it and enter a
//	suspended / alert state (SRS FR-RUN-06).
//
//	When ctx.TradingProxy is nil the handler returns an immediate error.
//	This fails fast rather than silently skipping a real trade command.
//
// Context-aware principle (blockly.md §1.2):
//
//	ctx.Symbol is always used as the trading pair — users never specify the
//	pair on the block. Symbol is injected by the Bot engine when it creates
//	the ExecutionContext.
//
// Unit cost: 10 per execution (charged by ExecuteBlock, blockly.md §1.4).
//
// SmartOrder pre-flight (SRS FR-DESIGN-09):
//
//	Before placing the order the BinanceProxy.SmartOrder method automatically:
//	  1. Adjusts leverage if it differs from the current position leverage.
//	  2. Changes margin type if it differs AND no position is open.
//	     (ChangeMarginType is rejected by Binance when a position is open.)
//	The blockly handler passes the desired values; the proxy handles the
//	exchange-level conditional logic (exchange/binance_client.go).

import (
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Handler Registration (init)
// ═══════════════════════════════════════════════════════════════════════════

func init() {
	RegisterHandler("trade_smart_order", executeTradeSmartOrder)
	RegisterHandler("trade_close_position", executeTradeClosePosition)
	RegisterHandler("trade_cancel_all_orders", executeTradeCancelAllOrders)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Statement Block Handlers — no return value (returns nil, nil on success)
// ═══════════════════════════════════════════════════════════════════════════

// executeTradeSmartOrder handles the `trade_smart_order` block
// (blockly.md §3.6.5, SRS FR-DESIGN-09).
//
// Places a Binance Futures order for ctx.Symbol with automatic Leverage and
// MarginType pre-configuration.
//
// Block fields / inputs:
//
//	SIDE        field_dropdown  "LONG" | "SHORT"
//	ORDER_TYPE  field_dropdown  "MARKET" | "LIMIT"
//	LEVERAGE    field_number    1–125 (floored to int; default 1)
//	MARGIN_TYPE field_dropdown  "ISOLATED" | "CROSSED"
//	PRICE       input_value     Number — limit price; 0 for MARKET orders
//	QUANTITY    input_value     Number — order quantity in base asset (e.g. BTC)
//
// Unit cost: 10 (charged by ExecuteBlock).
func executeTradeSmartOrder(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if ctx.TradingProxy == nil {
		return nil, fmt.Errorf("trade_smart_order (block_id=%s): TradingProxy is nil — cannot place order", block.ID)
	}

	// ── Read dropdown fields ─────────────────────────────────────────────────
	side := GetFieldString(block, "SIDE")
	if side == "" {
		side = "LONG"
	}
	orderType := GetFieldString(block, "ORDER_TYPE")
	if orderType == "" {
		orderType = "MARKET"
	}
	marginType := GetFieldString(block, "MARGIN_TYPE")
	if marginType == "" {
		marginType = "ISOLATED"
	}

	// ── Leverage (field_number → int) ────────────────────────────────────────
	// GetFieldFloat returns float64; floor to int (Binance only accepts integers).
	leverage := 1
	if lf := GetFieldFloat(block, "LEVERAGE"); lf >= 1 {
		leverage = int(lf) // safe: blockly editor enforces valid numeric range
	}

	// ── Value inputs (connected value blocks → decimal) ──────────────────────
	priceVal, err := EvalValue(ctx, GetInputBlock(block, "PRICE"))
	if err != nil {
		return nil, fmt.Errorf("trade_smart_order (block_id=%s): eval PRICE: %w", block.ID, err)
	}
	price := toDecimal(priceVal)

	qtyVal, err := EvalValue(ctx, GetInputBlock(block, "QUANTITY"))
	if err != nil {
		return nil, fmt.Errorf("trade_smart_order (block_id=%s): eval QUANTITY: %w", block.ID, err)
	}
	quantity := toDecimal(qtyVal)

	// ── Guard: non-positive quantity is always a logic error ─────────────────
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf(
			"trade_smart_order (block_id=%s): quantity must be > 0, got %s",
			block.ID, quantity.String(),
		)
	}

	// ── Delegate to exchange proxy ────────────────────────────────────────────
	if err := ctx.TradingProxy.SmartOrder(
		ctx.Ctx, ctx.Symbol, side, orderType, price, quantity, leverage, marginType,
	); err != nil {
		return nil, fmt.Errorf("trade_smart_order (block_id=%s, symbol=%s): %w",
			block.ID, ctx.Symbol, err)
	}

	ctx.Logger.Info("trade_smart_order: order placed",
		slog.String("block_id", block.ID),
		slog.String("symbol", ctx.Symbol),
		slog.String("side", side),
		slog.String("order_type", orderType),
		slog.Int("leverage", leverage),
		slog.String("margin_type", marginType),
		slog.String("price", price.String()),
		slog.String("quantity", quantity.String()),
	)
	return nil, nil
}

// executeTradeClosePosition handles the `trade_close_position` block
// (blockly.md §3.6.6, SRS FR-DESIGN-10).
//
// Closes the entire open Futures position for ctx.Symbol by:
//  1. Querying the current position risk to determine side and size.
//  2. Placing a reduce-only MARKET order in the opposite direction.
//
// The operation is a no-op when no position is open — the exchange proxy
// checks this and logs "Không có vị thế đang mở" at Info level.
//
// Unit cost: 10 (charged by ExecuteBlock).
func executeTradeClosePosition(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if ctx.TradingProxy == nil {
		return nil, fmt.Errorf("trade_close_position (block_id=%s): TradingProxy is nil — cannot close position", block.ID)
	}

	if err := ctx.TradingProxy.ClosePosition(ctx.Ctx, ctx.Symbol); err != nil {
		return nil, fmt.Errorf("trade_close_position (block_id=%s, symbol=%s): %w",
			block.ID, ctx.Symbol, err)
	}

	ctx.Logger.Info("trade_close_position: close-position order dispatched",
		slog.String("block_id", block.ID),
		slog.String("symbol", ctx.Symbol),
	)
	return nil, nil
}

// executeTradeCancelAllOrders handles the `trade_cancel_all_orders` block
// (blockly.md §3.6.7, SRS FR-DESIGN-10).
//
// Cancels every open (pending) order for ctx.Symbol.
// The operation is idempotent — when no orders are open the block does nothing
// (the proxy pre-checks the order list to avoid Binance error -2011).
//
// Unit cost: 10 (charged by ExecuteBlock).
func executeTradeCancelAllOrders(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if ctx.TradingProxy == nil {
		return nil, fmt.Errorf("trade_cancel_all_orders (block_id=%s): TradingProxy is nil — cannot cancel orders", block.ID)
	}

	if err := ctx.TradingProxy.CancelAllOrders(ctx.Ctx, ctx.Symbol); err != nil {
		return nil, fmt.Errorf("trade_cancel_all_orders (block_id=%s, symbol=%s): %w",
			block.ID, ctx.Symbol, err)
	}

	ctx.Logger.Info("trade_cancel_all_orders: all open orders cancelled",
		slog.String("block_id", block.ID),
		slog.String("symbol", ctx.Symbol),
	)
	return nil, nil
}
