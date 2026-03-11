package blockly

// block_data.go implements execution handlers for the Trading → Market & Account
// Data block sub-group (4 blocks), as specified in blockly.md §3.6.1–§3.6.4
// and SRS FR-DESIGN-08.
//
// Task 2.5.6 — Execute Trading group (7 blocks - data + trade actions).
// WBS: P2-Backend · 12/03/2026
// SRS: FR-DESIGN-08, FR-RUN-05, FR-RUN-07
//
// Blocks implemented (all return decimal.Decimal wrapped in interface{}):
//
//	value: data_market_price, data_position_info, data_open_orders_count, data_balance
//
// Context-aware principle (blockly.md §1.2):
//
//	All blocks automatically use ExecutionContext.Symbol — users never specify
//	the trading pair on the block. The symbol is injected when the Bot Instance
//	or Backtest engine calls NewExecutionContext(ctx, symbol, logger).
//
// Exchange access pattern:
//
//	Handlers call ctx.TradingProxy methods which route to BinanceProxy (live)
//	or a Backtest stub. The exception is data_market_price(CLOSE_PRICE) which
//	avoids an extra API call by reusing ctx.CandleRepo — the candle cache is
//	always warm during live Bot operation (Task 2.4.1).
//
// Unit cost: 3 per execution (pre-charged by ExecuteBlock, blockly.md §1.4).
//
// Safe fallback: returns decimal.Zero + slog.Warn (not an error) when:
//   - ctx.TradingProxy is nil (unit-test contexts without live exchange).
//   - Exchange call returns an error for data_* blocks (degraded, not fatal).
//     Note: trade_* blocks in block_trading.go DO propagate errors.
//
// Dependencies (all already in go.mod — no new imports):
//   - github.com/shopspring/decimal v1.4.0
//   - log/slog (Go 1.21+ stdlib)

import (
	"log/slog"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Handler Registration (init)
// ═══════════════════════════════════════════════════════════════════════════

func init() {
	RegisterHandler("data_market_price", executeDataMarketPrice)
	RegisterHandler("data_position_info", executeDataPositionInfo)
	RegisterHandler("data_open_orders_count", executeDataOpenOrdersCount)
	RegisterHandler("data_balance", executeDataBalance)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Value Block Handlers — Number outputs (decimal.Decimal)
// ═══════════════════════════════════════════════════════════════════════════

// executeDataMarketPrice handles the `data_market_price` block (blockly.md §3.6.1).
//
// Returns the current market price of the trading pair in the session context.
// The PRICE_TYPE field selects between two price sources:
//
//   - "LAST_PRICE"  → calls ctx.TradingProxy.GetLastPrice — the live ticker
//     price from the Binance REST API. The freshest available price.
//   - "CLOSE_PRICE" → reads the close price of the latest fully-closed candle
//     from ctx.CandleRepo (local DB cache). Avoids a live API call since the
//     candle cache is kept warm by the Binance WS stream (Task 2.4.1).
//
// Falls back to decimal.Zero + slog.Warn when ctx.TradingProxy is nil
// (LAST_PRICE) or ctx.CandleRepo is nil (CLOSE_PRICE).
//
// Unit cost: 3 (charged by ExecuteBlock).
func executeDataMarketPrice(ctx *ExecutionContext, block *Block) (interface{}, error) {
	priceType := GetFieldString(block, "PRICE_TYPE")
	if priceType == "" {
		priceType = "LAST_PRICE"
	}

	switch priceType {
	case "LAST_PRICE":
		if ctx.TradingProxy == nil {
			ctx.Logger.Warn("data_market_price(LAST_PRICE): TradingProxy is nil — returning 0",
				slog.String("block_id", block.ID),
				slog.String("symbol", ctx.Symbol),
			)
			return decimal.Zero, nil
		}
		price, err := ctx.TradingProxy.GetLastPrice(ctx.Ctx, ctx.Symbol)
		if err != nil {
			ctx.Logger.Warn("data_market_price(LAST_PRICE): exchange call failed — returning 0",
				slog.String("block_id", block.ID),
				slog.String("symbol", ctx.Symbol),
				slog.String("error", err.Error()),
			)
			return decimal.Zero, nil
		}
		return price, nil

	case "CLOSE_PRICE":
		if ctx.CandleRepo == nil {
			ctx.Logger.Warn("data_market_price(CLOSE_PRICE): CandleRepo is nil — returning 0",
				slog.String("block_id", block.ID),
				slog.String("symbol", ctx.Symbol),
				slog.String("timeframe", ctx.Timeframe),
			)
			return decimal.Zero, nil
		}
		candles, err := ctx.CandleRepo.QueryLatestClosedCandles(ctx.Ctx, ctx.Symbol, ctx.Timeframe, 1)
		if err != nil {
			ctx.Logger.Warn("data_market_price(CLOSE_PRICE): candle query failed — returning 0",
				slog.String("block_id", block.ID),
				slog.String("symbol", ctx.Symbol),
				slog.String("timeframe", ctx.Timeframe),
				slog.String("error", err.Error()),
			)
			return decimal.Zero, nil
		}
		if len(candles) == 0 {
			ctx.Logger.Warn("data_market_price(CLOSE_PRICE): no closed candles found — returning 0",
				slog.String("block_id", block.ID),
				slog.String("symbol", ctx.Symbol),
				slog.String("timeframe", ctx.Timeframe),
			)
			return decimal.Zero, nil
		}
		close, parseErr := decimal.NewFromString(candles[0].ClosePrice)
		if parseErr != nil {
			ctx.Logger.Warn("data_market_price(CLOSE_PRICE): invalid ClosePrice string — returning 0",
				slog.String("block_id", block.ID),
				slog.String("close_price", candles[0].ClosePrice),
			)
			return decimal.Zero, nil
		}
		return close, nil

	default:
		ctx.Logger.Warn("data_market_price: unknown PRICE_TYPE — returning 0",
			slog.String("block_id", block.ID),
			slog.String("price_type", priceType),
		)
		return decimal.Zero, nil
	}
}

// executeDataPositionInfo handles the `data_position_info` block (blockly.md §3.6.2).
//
// Returns a specific field of the currently open Futures position for
// ctx.Symbol. The FIELD dropdown selects the data point:
//
//   - "POSITION_SIZE"   → absolute position amount (positive = Long,
//     negative = Short, 0 = no position).
//   - "UNREALIZED_PNL"  → unrealized profit / loss in USDT.
//   - "ENTRY_PRICE"     → average entry price of the open position.
//
// Returns decimal.Zero for all fields when no position is open, or when
// ctx.TradingProxy is nil.
//
// Unit cost: 3 (charged by ExecuteBlock).
func executeDataPositionInfo(ctx *ExecutionContext, block *Block) (interface{}, error) {
	field := GetFieldString(block, "FIELD")
	if field == "" {
		field = "POSITION_SIZE"
	}

	if ctx.TradingProxy == nil {
		ctx.Logger.Warn("data_position_info: TradingProxy is nil — returning 0",
			slog.String("block_id", block.ID),
			slog.String("field", field),
			slog.String("symbol", ctx.Symbol),
		)
		return decimal.Zero, nil
	}

	var (
		val decimal.Decimal
		err error
	)
	switch field {
	case "POSITION_SIZE":
		val, err = ctx.TradingProxy.GetPositionSize(ctx.Ctx, ctx.Symbol)
	case "UNREALIZED_PNL":
		val, err = ctx.TradingProxy.GetPositionUnrealizedPNL(ctx.Ctx, ctx.Symbol)
	case "ENTRY_PRICE":
		val, err = ctx.TradingProxy.GetPositionEntryPrice(ctx.Ctx, ctx.Symbol)
	default:
		ctx.Logger.Warn("data_position_info: unknown FIELD — returning 0",
			slog.String("block_id", block.ID),
			slog.String("field", field),
		)
		return decimal.Zero, nil
	}

	if err != nil {
		ctx.Logger.Warn("data_position_info: exchange call failed — returning 0",
			slog.String("block_id", block.ID),
			slog.String("field", field),
			slog.String("symbol", ctx.Symbol),
			slog.String("error", err.Error()),
		)
		return decimal.Zero, nil
	}
	return val, nil
}

// executeDataOpenOrdersCount handles the `data_open_orders_count` block
// (blockly.md §3.6.3, SRS FR-DESIGN-08).
//
// Returns the count of pending (open) Futures orders for ctx.Symbol as a
// decimal.Decimal. Callers typically compare this with math_number(0) to
// implement "only open a new order when none is pending" guard logic.
//
// Returns decimal.Zero when ctx.TradingProxy is nil or the exchange call fails.
//
// Unit cost: 3 (charged by ExecuteBlock).
func executeDataOpenOrdersCount(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if ctx.TradingProxy == nil {
		ctx.Logger.Warn("data_open_orders_count: TradingProxy is nil — returning 0",
			slog.String("block_id", block.ID),
			slog.String("symbol", ctx.Symbol),
		)
		return decimal.Zero, nil
	}

	count, err := ctx.TradingProxy.GetOpenOrdersCount(ctx.Ctx, ctx.Symbol)
	if err != nil {
		ctx.Logger.Warn("data_open_orders_count: exchange call failed — returning 0",
			slog.String("block_id", block.ID),
			slog.String("symbol", ctx.Symbol),
			slog.String("error", err.Error()),
		)
		return decimal.Zero, nil
	}
	return decimal.NewFromInt(int64(count)), nil
}

// executeDataBalance handles the `data_balance` block (blockly.md §3.6.4,
// SRS FR-DESIGN-08).
//
// Returns the USDT available balance in the Futures wallet as decimal.Decimal.
// "Available" means the balance that can be used to open new positions —
// margin already committed to open positions has been deducted.
//
// Typical use: dynamic quantity calculation, e.g.
//
//	[Số dư khả dụng] × [0.1] ÷ [Giá hiện tại]
//
// Returns decimal.Zero when ctx.TradingProxy is nil or the exchange call fails.
//
// Unit cost: 3 (charged by ExecuteBlock).
func executeDataBalance(ctx *ExecutionContext, block *Block) (interface{}, error) {
	if ctx.TradingProxy == nil {
		ctx.Logger.Warn("data_balance: TradingProxy is nil — returning 0",
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	balance, err := ctx.TradingProxy.GetAvailableBalance(ctx.Ctx)
	if err != nil {
		ctx.Logger.Warn("data_balance: exchange call failed — returning 0",
			slog.String("block_id", block.ID),
			slog.String("error", err.Error()),
		)
		return decimal.Zero, nil
	}
	return balance, nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Compile-time interface check
// ═══════════════════════════════════════════════════════════════════════════
