package blockly

// block_indicator.go implements execution handlers for the Technical Indicator
// block group (2 blocks), as specified in blockly.md §3.5 and SRS FR-DESIGN-07.
//
// Task 2.5.5 — Execute Indicator group - RSI and EMA (Context-aware).
// WBS: P2-Backend · 12/03/2026
// SRS: FR-DESIGN-07, FR-RUN-05, FR-RUN-07
//
// Blocks implemented (both return decimal.Decimal wrapped in interface{}):
//
//	value: indicator_rsi, indicator_ema
//
// Context-aware principle (blockly.md §1.2):
//
//	Both blocks accept only TIMEFRAME (field_dropdown) and PERIOD (value input).
//	The trading symbol is NEVER specified on the block — it is injected via
//	ExecutionContext.Symbol by the Bot Instance or Backtest engine (FR-RUN-05).
//	Backend automatically fetches ctx.Symbol candles at the requested timeframe.
//
// Auto-fetch behavior (WBS 2.5.5 note "Auto-fetch candles for Current_Symbol"):
//
//	Handlers call ctx.CandleRepo.QueryLatestClosedCandles to retrieve fully-
//	closed candles from the local PostgreSQL cache. The Binance WS stream
//	(Task 2.4.1) keeps this cache warm; the GapFillerWorker (Task 2.4.2)
//	backfills missing ranges. Indicator blocks do NOT call Binance API —
//	they only read local DB, keeping latency low and rate-limit usage minimal.
//
// Warmup strategy:
//
//	Both handlers fetch (period × minCandlesBuffer) candles rather than the
//	bare minimum. The extra warmup iterations improve Wilder's SMMA convergence
//	for RSI and EMA accuracy for mid-series seeds. With minCandlesBuffer = 2,
//	a period-14 RSI fetches 29 candles (14×2+1); EMA(14) fetches 28 candles.
//
// Unit cost: 5 per execution (pre-charged by ExecuteBlock, blockly.md §1.4).
//
// Safe fallback: returns decimal.Zero + slog.Warn (not an error) when:
//   - ctx.CandleRepo is nil (no DB injected)
//   - period ≤ 0 (invalid input)
//   - available closed candles < minimum required (insufficient history)
//
// Dependencies (all already in go.mod — no new imports):
//   - github.com/shopspring/decimal v1.4.0
//   - log/slog (Go 1.21+ stdlib)

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Constants
// ═══════════════════════════════════════════════════════════════════════════

// minCandlesBuffer is the multiplier used to determine how many extra candles
// to fetch beyond the bare period minimum. Fetching 2× the period gives
// additional warmup iterations that improve Wilder's SMMA convergence for RSI
// and EMA seed accuracy. Example: period=14 → fetch 28+ candles (EMA) / 29 (RSI).
const minCandlesBuffer = 2

// ═══════════════════════════════════════════════════════════════════════════
//  Handler Registration (init)
// ═══════════════════════════════════════════════════════════════════════════

func init() {
	RegisterHandler("indicator_rsi", executeIndicatorRSI)
	RegisterHandler("indicator_ema", executeIndicatorEMA)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Value Block Handlers — Number outputs (decimal.Decimal)
// ═══════════════════════════════════════════════════════════════════════════

// executeIndicatorRSI handles the `indicator_rsi` block (blockly.md §3.5.1).
//
// Reads TIMEFRAME (field_dropdown) and PERIOD (value input → Number) from the
// block, auto-fetches the required closed candles for ctx.Symbol at the given
// timeframe from ctx.CandleRepo, then computes RSI(period) using Wilder's
// Smoothed Moving Average (SMMA) method.
//
// The TIMEFRAME field may differ from the event_on_candle timeframe. For
// example, a bot that fires on 1m candles can still compute RSI over 15m candles
// (blockly.md §3.5.1 — "Khung thời gian của khối Chỉ báo có thể khác").
//
// Returns decimal.Zero (not an error) when ctx.CandleRepo is nil, period ≤ 0,
// or available candles < period + 1. Session continues normally (blockly.md §3.5).
//
// Unit cost: 5 (charged by ExecuteBlock before this handler is invoked).
func executeIndicatorRSI(ctx *ExecutionContext, block *Block) (interface{}, error) {
	timeframe := GetFieldString(block, "TIMEFRAME")
	if timeframe == "" {
		timeframe = "1m"
	}

	rawPeriod, err := EvalValue(ctx, GetInputBlock(block, "PERIOD"))
	if err != nil {
		return nil, fmt.Errorf("indicator_rsi: evaluating PERIOD input (block_id=%s): %w", block.ID, err)
	}
	period := int(toDecimal(rawPeriod).IntPart())

	if period <= 0 {
		ctx.Logger.Warn("indicator_rsi: period must be > 0, returning 0",
			slog.Int("period", period),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	if ctx.CandleRepo == nil {
		ctx.Logger.Warn("indicator_rsi: CandleRepo is nil — no DB injected, returning 0",
			slog.String("symbol", ctx.Symbol),
			slog.String("timeframe", timeframe),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	// Fetch (period × minCandlesBuffer) + 1 candles for RSI warmup accuracy.
	// RSI needs at minimum (period + 1) close prices to compute one value.
	// Extra warmup iterations tighten Wilder's SMMA convergence (avoids the
	// "cold start" bias present when using exactly period+1 candles).
	fetchLimit := period*minCandlesBuffer + 1
	closes, err := fetchClosePrices(ctx.Ctx, ctx.CandleRepo, ctx.Symbol, timeframe, fetchLimit)
	if err != nil {
		return nil, fmt.Errorf("indicator_rsi: fetching candles for %s/%s: %w", ctx.Symbol, timeframe, err)
	}

	if len(closes) < period+1 {
		ctx.Logger.Warn("indicator_rsi: insufficient closed candles, returning 0",
			slog.String("symbol", ctx.Symbol),
			slog.String("timeframe", timeframe),
			slog.Int("required", period+1),
			slog.Int("available", len(closes)),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	rsi := calcRSI(closes, period)
	ctx.Logger.Debug("indicator_rsi computed",
		slog.String("symbol", ctx.Symbol),
		slog.String("timeframe", timeframe),
		slog.Int("period", period),
		slog.String("rsi", rsi.StringFixed(4)),
	)
	return rsi, nil
}

// executeIndicatorEMA handles the `indicator_ema` block (blockly.md §3.5.2).
//
// Reads TIMEFRAME (field_dropdown) and PERIOD (value input → Number) from the
// block, fetches closed candles for ctx.Symbol at the given timeframe from
// ctx.CandleRepo, then computes the EMA of close prices using SMA seeding
// followed by exponential smoothing (k = 2 / (period + 1)).
//
// Returns decimal.Zero (not an error) when ctx.CandleRepo is nil, period ≤ 0,
// or available candles < period. Session continues normally (blockly.md §3.5).
//
// Unit cost: 5 (charged by ExecuteBlock before this handler is invoked).
func executeIndicatorEMA(ctx *ExecutionContext, block *Block) (interface{}, error) {
	timeframe := GetFieldString(block, "TIMEFRAME")
	if timeframe == "" {
		timeframe = "1m"
	}

	rawPeriod, err := EvalValue(ctx, GetInputBlock(block, "PERIOD"))
	if err != nil {
		return nil, fmt.Errorf("indicator_ema: evaluating PERIOD input (block_id=%s): %w", block.ID, err)
	}
	period := int(toDecimal(rawPeriod).IntPart())

	if period <= 0 {
		ctx.Logger.Warn("indicator_ema: period must be > 0, returning 0",
			slog.Int("period", period),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	if ctx.CandleRepo == nil {
		ctx.Logger.Warn("indicator_ema: CandleRepo is nil — no DB injected, returning 0",
			slog.String("symbol", ctx.Symbol),
			slog.String("timeframe", timeframe),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	// Fetch (period × minCandlesBuffer) candles for EMA warmup accuracy.
	// EMA requires at minimum `period` closes for the SMA seed.
	// Extra candles allow additional EMA rolling steps for better convergence.
	fetchLimit := period * minCandlesBuffer
	if fetchLimit < period {
		fetchLimit = period // safety clamp (minCandlesBuffer ≥ 1 always satisfies this)
	}
	closes, err := fetchClosePrices(ctx.Ctx, ctx.CandleRepo, ctx.Symbol, timeframe, fetchLimit)
	if err != nil {
		return nil, fmt.Errorf("indicator_ema: fetching candles for %s/%s: %w", ctx.Symbol, timeframe, err)
	}

	if len(closes) < period {
		ctx.Logger.Warn("indicator_ema: insufficient closed candles, returning 0",
			slog.String("symbol", ctx.Symbol),
			slog.String("timeframe", timeframe),
			slog.Int("required", period),
			slog.Int("available", len(closes)),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}

	ema := calcEMA(closes, period)
	ctx.Logger.Debug("indicator_ema computed",
		slog.String("symbol", ctx.Symbol),
		slog.String("timeframe", timeframe),
		slog.Int("period", period),
		slog.String("ema", ema.StringFixed(8)),
	)
	return ema, nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Private Computation Helpers
// ═══════════════════════════════════════════════════════════════════════════

// fetchClosePrices retrieves the latest `limit` closed candles for the given
// symbol/interval from the candle repository and converts each ClosePrice
// (stored as a decimal string in the DB) to decimal.Decimal.
//
// The returned slice is ordered oldest-first (open_time ASC) — this is the
// natural chronological order required by both the RSI and EMA rolling loops.
//
// Candles whose ClosePrice cannot be parsed (malformed DB data) are silently
// skipped with a warning. The caller's length check (`len(closes) < period+1`)
// will catch the resulting shortfall and return decimal.Zero safely.
func fetchClosePrices(
	ctx context.Context,
	repo CandleRepositoryReader,
	symbol, interval string,
	limit int,
) ([]decimal.Decimal, error) {
	candles, err := repo.QueryLatestClosedCandles(ctx, symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("fetchClosePrices: QueryLatestClosedCandles failed: %w", err)
	}

	closes := make([]decimal.Decimal, 0, len(candles))
	for _, c := range candles {
		d, parseErr := decimal.NewFromString(c.ClosePrice)
		if parseErr != nil {
			slog.Warn("fetchClosePrices: skipping candle with unparseable ClosePrice",
				slog.String("close_price", c.ClosePrice),
				slog.String("symbol", symbol),
				slog.String("interval", interval),
			)
			continue
		}
		closes = append(closes, d)
	}
	return closes, nil
}

// calcEMA computes the Exponential Moving Average of a close price series.
//
// Algorithm — SMA seed + exponential smoothing:
//
//  1. Seed: EMA₀ = simple average (SMA) of the first `period` closes.
//  2. Multiplier: k = 2 / (period + 1)   (standard EMA multiplier).
//  3. Rolling: EMAᵢ = closeᵢ × k + EMAᵢ₋₁ × (1 − k).
//
// Returns the EMA value at the last (most recent) candle in the slice.
//
// Precondition: len(closes) >= period (callers must validate before calling).
// All arithmetic uses shopspring/decimal for precision on crypto prices
// (avoids float64 rounding errors that accumulate across many iterations).
func calcEMA(closes []decimal.Decimal, period int) decimal.Decimal {
	periodDec := decimal.NewFromInt(int64(period))

	// Step 1: Seed EMA with the SMA of the first `period` candles.
	var sum decimal.Decimal
	for i := 0; i < period; i++ {
		sum = sum.Add(closes[i])
	}
	ema := sum.Div(periodDec)

	// k = 2 / (period + 1)
	k := decimal.NewFromInt(2).Div(periodDec.Add(decimal.NewFromInt(1)))
	oneMinusK := decimal.NewFromInt(1).Sub(k)

	// Step 2: Roll EMA over all candles after the seed window.
	for i := period; i < len(closes); i++ {
		ema = closes[i].Mul(k).Add(ema.Mul(oneMinusK))
	}
	return ema
}

// calcRSI computes the Relative Strength Index using Wilder's Smoothed Moving
// Average (SMMA) method — the standard formulation used by TradingView,
// MetaTrader, and most professional trading platforms.
//
// Algorithm:
//
//  1. Compute `period` price changes from the first `period+1` close prices.
//  2. Seed: avgGain and avgLoss = simple average of the initial gains / losses.
//  3. Wilder SMMA rolling for any subsequent candles past period+1:
//     avgGain = (prevAvgGain × (period−1) + currentGain) / period
//     avgLoss = (prevAvgLoss × (period−1) + currentLoss) / period
//  4. RS  = avgGain / avgLoss
//  5. RSI = 100 − (100 / (1 + RS))
//  6. Special case: avgLoss == 0 → RSI = 100 (pure uptrend, no losses).
//
// Precondition: len(closes) >= period + 1 (callers must validate before calling).
// All arithmetic uses shopspring/decimal (SRS FR-DESIGN-06 "BigInt").
func calcRSI(closes []decimal.Decimal, period int) decimal.Decimal {
	periodDec := decimal.NewFromInt(int64(period))
	periodMinusOne := decimal.NewFromInt(int64(period - 1))
	hundred := decimal.NewFromInt(100)
	zero := decimal.Zero

	// Step 1–2: Compute and average the initial `period` price changes.
	var initGainSum, initLossSum decimal.Decimal
	for i := 1; i <= period; i++ {
		change := closes[i].Sub(closes[i-1])
		if change.GreaterThan(zero) {
			initGainSum = initGainSum.Add(change)
		} else {
			// Store absolute value — losses are positive numbers in Wilder's formula.
			initLossSum = initLossSum.Add(change.Neg())
		}
	}
	avgGain := initGainSum.Div(periodDec)
	avgLoss := initLossSum.Div(periodDec)

	// Step 3: Apply Wilder's SMMA for candles beyond the seed window.
	for i := period + 1; i < len(closes); i++ {
		change := closes[i].Sub(closes[i-1])
		var currentGain, currentLoss decimal.Decimal
		if change.GreaterThan(zero) {
			currentGain = change
		} else {
			currentLoss = change.Neg()
		}
		avgGain = avgGain.Mul(periodMinusOne).Add(currentGain).Div(periodDec)
		avgLoss = avgLoss.Mul(periodMinusOne).Add(currentLoss).Div(periodDec)
	}

	// Step 4–6: RSI formula, guarded against division by zero.
	if avgLoss.IsZero() {
		// Pure uptrend: all candles closed higher — RSI = 100 by definition.
		return hundred
	}
	rs := avgGain.Div(avgLoss)
	// RSI = 100 − (100 / (1 + RS))
	return hundred.Sub(hundred.Div(decimal.NewFromInt(1).Add(rs)))
}
