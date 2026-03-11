// Package backtest — Task 2.6.2: Order Matching Simulator.
//
// order_matcher.go is the second stage in the backtest pipeline:
//
//	Simulator.Run() ──► RunOutput ──► OrderMatcher.Match() ──► MatchResult
//	   (task 2.6.1)                        (task 2.6.2)
//
// It receives the *RunOutput produced by the Simulation Engine and processes
// every PendingOrder submitted across all candle sessions, applying realistic
// fill rules against the OHLCV data of subsequent candles (SRS FR-RUN-02):
//
//   - MARKET orders  → fill at the Open price of the NEXT candle
//   - LIMIT LONG     → fill when the next candle's Low  <= limitPrice
//   - LIMIT SHORT    → fill when the next candle's High >= limitPrice
//
// Orders that are not filled in one candle carry forward (GTC semantics) and
// are tried again against every future candle until a fill occurs or the
// candle series ends (in which case the order expires unfilled).
//
// The MatchResult produced here is the primary input to:
//   - Task 2.6.3 — performance.go  (PnL, Win Rate, Max Drawdown, Profit Factor)
//   - Task 2.6.4 — equity_curve.go (Equity Chart data points)
//   - Task 2.6.5 — backtest_logic.go (Backtest API response assembly)
//
// WBS: P2-Backend · 13/03/2026
// SRS: FR-RUN-02
package backtest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Output Types (consumed by tasks 2.6.3, 2.6.4, 2.6.5)
// ═══════════════════════════════════════════════════════════════════════════

// FilledTrade represents a completed round-trip Futures trade:
// one open fill followed by one close fill.
//
// It maps 1:1 to the BacktestTrade schema in api.yaml and contains all
// information needed for performance reporting (task 2.6.3).
type FilledTrade struct {
	// OpenTime is the open_time of the candle on which the position was entered.
	OpenTime time.Time

	// CloseTime is the open_time of the candle on which the position was closed.
	CloseTime time.Time

	// Side is "LONG" or "SHORT" — the direction of the opening leg.
	Side string

	// EntryPrice is the simulated fill price for the opening leg.
	// MARKET → candle.Open; LIMIT → order.LimitPrice.
	EntryPrice decimal.Decimal

	// ExitPrice is the simulated fill price for the closing leg.
	ExitPrice decimal.Decimal

	// Quantity is the position size in base asset (e.g., BTC).
	Quantity decimal.Decimal

	// Fee is the total fee charged for both the entry and exit fills.
	//   fee = (entryPrice × quantity + exitPrice × quantity) × feeRate
	Fee decimal.Decimal

	// PnL is the net realized profit/loss after fees.
	//   Long:  (exitPrice - entryPrice) × quantity - fee
	//   Short: (entryPrice - exitPrice) × quantity - fee
	PnL decimal.Decimal
}

// BalanceSnapshot records the simulated account balance at a single point in
// time. Used by the equity curve generator (task 2.6.4) to draw the growth
// chart (SRS FR-RUN-04).
type BalanceSnapshot struct {
	// Timestamp is the open_time of the candle at which this snapshot was taken.
	Timestamp time.Time

	// Balance is the simulated USDT account balance at that moment.
	Balance decimal.Decimal
}

// MatchResult is the complete output of OrderMatcher.Match().
// It is passed as input to the performance reporter (2.6.3) and equity curve
// generator (2.6.4), and ultimately serialised into the BacktestResult API
// response by the backtest logic layer (2.6.5).
type MatchResult struct {
	// Trades is the list of completed round-trip trades, ordered by OpenTime ASC.
	Trades []FilledTrade

	// BalanceHistory has one entry per candle in the simulation range.
	// The entry at index i reflects the balance AFTER processing all fills
	// triggered by candles[i]. Used to draw the equity curve (SRS FR-RUN-04).
	BalanceHistory []BalanceSnapshot

	// FinalBalance is the simulated USDT balance at the end of the simulation.
	FinalBalance decimal.Decimal

	// UnclosedPosition is non-nil when a position opened during simulation
	// was never closed because the candle series ended before a close fill.
	// Performance reporters should mark this trade as open/incomplete.
	UnclosedPosition *openPositionLeg
}

// ═══════════════════════════════════════════════════════════════════════════
//  Internal State (unexported — private to Match())
// ═══════════════════════════════════════════════════════════════════════════

// openPositionLeg tracks the opening details of the currently active Futures
// position while it waits for a closing fill.
type openPositionLeg struct {
	// Side is "LONG" or "SHORT".
	Side string

	// EntryPrice is the simulated fill price on the opening candle.
	EntryPrice decimal.Decimal

	// Quantity is the position size in base asset.
	Quantity decimal.Decimal

	// Leverage is the multiplier applied (1–125).
	// Used to compute the initial margin locked.
	Leverage int

	// Margin is the USDT margin locked for this position at entry time.
	//   margin = entryPrice × quantity / leverage
	// Returned to balance when the position is closed.
	Margin decimal.Decimal

	// EntryFee is the fee paid to open the position.
	//   entryFee = entryPrice × quantity × feeRate
	EntryFee decimal.Decimal

	// OpenTime is the open_time of the candle on which the position was entered.
	OpenTime time.Time
}

// ═══════════════════════════════════════════════════════════════════════════
//  OrderMatcher
// ═══════════════════════════════════════════════════════════════════════════

// OrderMatcher is the post-processing stage of the backtest pipeline.
// It is stateless; all mutable simulation state lives on the stack inside
// Match(). Construct one instance per backtest run (or reuse across runs —
// the struct itself has no mutable fields).
type OrderMatcher struct {
	feeRate decimal.Decimal
	logger  *slog.Logger
}

// NewOrderMatcher constructs an OrderMatcher with the given fee rate and logger.
//
//   - feeRate: per-trade fee as a decimal fraction (e.g., 0.0004 = 0.04%).
//     Passed through from Config.FeeRate set at backtest creation time.
//   - logger: base slog.Logger; Match() decorates with symbol + timeframe fields.
func NewOrderMatcher(feeRate decimal.Decimal, logger *slog.Logger) *OrderMatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &OrderMatcher{
		feeRate: feeRate,
		logger:  logger,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Core Fill Logic (pure, unexported helpers)
// ═══════════════════════════════════════════════════════════════════════════

// tryMatchOrder tests whether the given order can be filled against the
// provided candle and returns the fill price if so.
//
// Fill rules (SRS FR-RUN-02, task description "Market to Open – Limit to High/Low"):
//
//	MARKET → fills always; fillPrice = candle.OpenPrice
//	LIMIT LONG  → fills when candle.LowPrice  <= order.LimitPrice
//	LIMIT SHORT → fills when candle.HighPrice >= order.LimitPrice
//
// Returns (fillPrice, true) on a fill; (decimal.Zero, false) if no fill.
// This function has no side-effects and does no allocation beyond the
// decimal.NewFromString calls required to parse candle price strings.
func tryMatchOrder(order PendingOrder, candle domain.Candle) (decimal.Decimal, bool) {
	if order.OrderType == "MARKET" {
		openPrice, err := decimal.NewFromString(candle.OpenPrice)
		if err != nil {
			// Malformed candle data — treat as no-fill; caller will log.
			return decimal.Zero, false
		}
		return openPrice, true
	}

	// LIMIT order — price-crossing check.
	if order.OrderType == "LIMIT" {
		if order.Side == "LONG" {
			lowPrice, err := decimal.NewFromString(candle.LowPrice)
			if err != nil {
				return decimal.Zero, false
			}
			// Long limit fills when price drops to or below the limit.
			if lowPrice.LessThanOrEqual(order.LimitPrice) {
				return order.LimitPrice, true
			}
			return decimal.Zero, false
		}

		if order.Side == "SHORT" {
			highPrice, err := decimal.NewFromString(candle.HighPrice)
			if err != nil {
				return decimal.Zero, false
			}
			// Short limit fills when price rises to or above the limit.
			if highPrice.GreaterThanOrEqual(order.LimitPrice) {
				return order.LimitPrice, true
			}
			return decimal.Zero, false
		}
	}

	// Unknown order type or side — no fill.
	return decimal.Zero, false
}

// computeFee calculates the simulated exchange fee for a single fill.
//   fee = fillPrice × quantity × feeRate
func (m *OrderMatcher) computeFee(fillPrice, quantity decimal.Decimal) decimal.Decimal {
	return fillPrice.Mul(quantity).Mul(m.feeRate)
}

// computeMargin calculates the initial margin locked for a position at entry.
//   margin = entryPrice × quantity / leverage
// Uses leverage=1 when the provided value is zero or negative (safe fallback).
func computeMargin(entryPrice, quantity decimal.Decimal, leverage int) decimal.Decimal {
	lev := decimal.NewFromInt(int64(leverage))
	if !lev.IsPositive() {
		lev = decimal.NewFromInt(1)
	}
	return entryPrice.Mul(quantity).Div(lev)
}

// computeRealizedPnL calculates the net realized PnL for a closed round-trip.
//
//	Long:  (exitPrice - entryPrice) × quantity - totalFee
//	Short: (entryPrice - exitPrice) × quantity - totalFee
func computeRealizedPnL(side string, entryPrice, exitPrice, quantity, totalFee decimal.Decimal) decimal.Decimal {
	var grossPnL decimal.Decimal
	if side == "LONG" {
		grossPnL = exitPrice.Sub(entryPrice).Mul(quantity)
	} else {
		grossPnL = entryPrice.Sub(exitPrice).Mul(quantity)
	}
	return grossPnL.Sub(totalFee)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Match — Main Pipeline Stage
// ═══════════════════════════════════════════════════════════════════════════

// Match processes all pending orders from a completed simulation run and
// produces a MatchResult containing every filled round-trip trade, a per-candle
// balance history, and the final simulated account balance.
//
// Algorithm overview:
//
//  1. Initialise balance from runOutput.Config.InitialCapital.
//  2. Maintain a GTC (Good-Till-Cancelled) queue of pending orders.
//  3. For every candle j = 0 … N-1:
//     a. Try to fill each order in the queue against candle[j].
//        • Filled + IsReduceOnly  → close open position, record FilledTrade,
//          return margin + PnL to balance.
//        • Filled + !IsReduceOnly → open / flip position, deduct margin + fee.
//     b. Append new orders from executions[j].OrdersSubmitted to the queue
//        (they will be tried against candle[j+1] and beyond).
//     c. Record a BalanceSnapshot for candle[j].
//  4. Return MatchResult.
//
// Flip semantics: if the strategy submits a LONG order while SHORT (or vice
// versa) without first using ClosePosition, the matcher implicitly closes
// the opposing position before opening the new one.
//
// Context cancellation is checked at each candle boundary so that a cancelled
// backtest request (DELETE /backtests/{id}/cancel, task 2.6.5) propagates
// cleanly through the pipeline.
func (m *OrderMatcher) Match(ctx context.Context, runOutput *RunOutput) (*MatchResult, error) {
	if runOutput == nil {
		return nil, fmt.Errorf("order_matcher: Match: runOutput must not be nil")
	}

	cfg := runOutput.Config
	candles := runOutput.Candles
	executions := runOutput.Executions

	log := m.logger.With(
		slog.String("symbol", cfg.Symbol),
		slog.String("timeframe", cfg.Timeframe),
	)

	balance := cfg.InitialCapital
	var openLeg *openPositionLeg        // nil when no position is open
	pendingOrders := make([]PendingOrder, 0)
	trades := make([]FilledTrade, 0)
	balanceHistory := make([]BalanceSnapshot, 0, len(candles))

	for j, candle := range candles {
		// ── Context cancellation check ────────────────────────────────────
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// ── Step a: Try to fill all pending orders against this candle ────
		remaining := pendingOrders[:0] // re-slice in place to avoid allocation

		for _, order := range pendingOrders {
			fillPrice, filled := tryMatchOrder(order, candle)
			if !filled {
				remaining = append(remaining, order)
				continue
			}

			// ── Handle fill ───────────────────────────────────────────────
			if order.IsReduceOnly {
				// Close the open position.
				if openLeg == nil {
					// No open position — stale reduce-only order; discard it.
					log.Warn("order_matcher: reduce-only fill with no open position; discarding",
						slog.String("order_side", order.Side),
						slog.Time("submitted_at_candle", order.SubmittedAtCandle),
					)
					continue
				}

				exitFee := m.computeFee(fillPrice, openLeg.Quantity)
				totalFee := openLeg.EntryFee.Add(exitFee)
				realizedPnL := computeRealizedPnL(
					openLeg.Side, openLeg.EntryPrice, fillPrice, openLeg.Quantity, totalFee,
				)

				// Return margin to balance, then add net PnL.
				balance = balance.Add(openLeg.Margin).Add(realizedPnL)

				trade := FilledTrade{
					OpenTime:   openLeg.OpenTime,
					CloseTime:  candle.OpenTime,
					Side:       openLeg.Side,
					EntryPrice: openLeg.EntryPrice,
					ExitPrice:  fillPrice,
					Quantity:   openLeg.Quantity,
					Fee:        totalFee,
					PnL:        realizedPnL,
				}
				trades = append(trades, trade)

				log.Info("order_matcher: position closed",
					slog.String("side", openLeg.Side),
					slog.String("entry_price", openLeg.EntryPrice.String()),
					slog.String("exit_price", fillPrice.String()),
					slog.String("realized_pnl", realizedPnL.String()),
					slog.String("balance", balance.String()),
				)

				openLeg = nil

			} else {
				// Open (or flip) a position.

				// ── Flip detection: close opposing side first ─────────────
				if openLeg != nil && openLeg.Side != order.Side {
					// Opposite-direction entry → close existing position at
					// the same fill price before opening the new one.
					exitFee := m.computeFee(fillPrice, openLeg.Quantity)
					totalFee := openLeg.EntryFee.Add(exitFee)
					realizedPnL := computeRealizedPnL(
						openLeg.Side, openLeg.EntryPrice, fillPrice, openLeg.Quantity, totalFee,
					)
					balance = balance.Add(openLeg.Margin).Add(realizedPnL)

					trades = append(trades, FilledTrade{
						OpenTime:   openLeg.OpenTime,
						CloseTime:  candle.OpenTime,
						Side:       openLeg.Side,
						EntryPrice: openLeg.EntryPrice,
						ExitPrice:  fillPrice,
						Quantity:   openLeg.Quantity,
						Fee:        totalFee,
						PnL:        realizedPnL,
					})

					log.Info("order_matcher: flip — closed opposing position",
						slog.String("closed_side", openLeg.Side),
						slog.String("realized_pnl", realizedPnL.String()),
					)

					openLeg = nil
				}

				// ── Open new position ─────────────────────────────────────
				entryFee := m.computeFee(fillPrice, order.Quantity)
				margin := computeMargin(fillPrice, order.Quantity, order.Leverage)

				// Deduct margin and entry fee from available balance.
				balance = balance.Sub(margin).Sub(entryFee)

				openLeg = &openPositionLeg{
					Side:       order.Side,
					EntryPrice: fillPrice,
					Quantity:   order.Quantity,
					Leverage:   order.Leverage,
					Margin:     margin,
					EntryFee:   entryFee,
					OpenTime:   candle.OpenTime,
				}

				log.Info("order_matcher: position opened",
					slog.String("side", order.Side),
					slog.String("fill_price", fillPrice.String()),
					slog.String("quantity", order.Quantity.String()),
					slog.String("margin", margin.String()),
					slog.String("balance", balance.String()),
				)
			}
		}

		pendingOrders = remaining

		// ── Step b: Enqueue newly submitted orders from this candle session ─
		// These orders were submitted WHILE candle[j] was being simulated, so
		// they are eligible for filling from candle[j+1] onward.
		if j < len(executions) {
			pendingOrders = append(pendingOrders, executions[j].OrdersSubmitted...)
		}

		// ── Step c: Record balance snapshot for this candle ───────────────
		balanceHistory = append(balanceHistory, BalanceSnapshot{
			Timestamp: candle.OpenTime,
			Balance:   balance,
		})
	}

	log.Info("order_matcher: matching complete",
		slog.Int("trades_filled", len(trades)),
		slog.Int("orders_unfilled", len(pendingOrders)),
		slog.String("final_balance", balance.String()),
	)

	return &MatchResult{
		Trades:           trades,
		BalanceHistory:   balanceHistory,
		FinalBalance:     balance,
		UnclosedPosition: openLeg,
	}, nil
}
