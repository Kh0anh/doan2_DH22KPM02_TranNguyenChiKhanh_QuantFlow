// Package backtest — Task 2.6.3: Performance Report Calculator.
//
// performance.go is the third stage in the backtest pipeline:
//
//	OrderMatcher.Match() ──► MatchResult ──► CalculatePerformance() ──► PerformanceSummary
//	      (task 2.6.2)                            (task 2.6.3)
//
// It is a pure, stateless calculator: no struct, no dependencies, no DB I/O.
// All arithmetic uses shopspring/decimal to preserve precision for financial
// reporting (SRS FR-RUN-03).
//
// The PerformanceSummary produced here maps 1:1 to the BacktestResult.summary
// schema in api.yaml and is consumed by:
//   - Task 2.6.5 — backtest_logic.go (Backtest API response assembly)
//
// WBS: P2-Backend · 13/03/2026
// SRS: FR-RUN-03
package backtest

import (
	"errors"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Output Type
// ═══════════════════════════════════════════════════════════════════════════

// PerformanceSummary holds all performance metrics computed from a completed
// backtest simulation. It maps 1:1 to the BacktestResult.summary object
// defined in api.yaml, enabling direct serialisation by the logic layer.
type PerformanceSummary struct {
	// TotalPnL is the sum of all FilledTrade.PnL values (net of fees).
	// Positive = overall profit; negative = overall loss.
	TotalPnL decimal.Decimal

	// TotalPnLPercent is TotalPnL expressed as a percentage of InitialCapital.
	//   TotalPnLPercent = TotalPnL / InitialCapital × 100
	// Zero when InitialCapital is zero.
	TotalPnLPercent decimal.Decimal

	// WinRate is the percentage of trades that were profitable (PnL > 0).
	//   WinRate = WinningTrades / TotalTrades × 100
	// Zero when TotalTrades is zero.
	WinRate decimal.Decimal

	// TotalTrades is the count of completed round-trip trades.
	TotalTrades int

	// WinningTrades is the count of trades with PnL > 0.
	WinningTrades int

	// LosingTrades is the count of trades with PnL <= 0.
	LosingTrades int

	// MaxDrawdown is the largest peak-to-trough decline in account balance
	// (in USDT) observed across the BalanceHistory series.
	//   MaxDrawdown = max(peakBalance − balance)
	MaxDrawdown decimal.Decimal

	// MaxDrawdownPercent is MaxDrawdown expressed as a percentage of the
	// balance peak at which the drawdown originated.
	//   MaxDrawdownPercent = MaxDrawdown / peakBalance × 100
	// Zero when BalanceHistory is empty or InitialCapital is zero.
	MaxDrawdownPercent decimal.Decimal

	// ProfitFactor is the ratio of total gross profit to total gross loss.
	//   ProfitFactor = Σ(PnL > 0) / |Σ(PnL < 0)|
	// Zero when there are no losing trades (the "infinite" edge case is
	// collapsed to zero for safe JSON serialisation via api.yaml).
	ProfitFactor decimal.Decimal
}

// ═══════════════════════════════════════════════════════════════════════════
//  CalculatePerformance — Main Calculator
// ═══════════════════════════════════════════════════════════════════════════

// CalculatePerformance computes all performance metrics for a completed
// backtest simulation from the provided MatchResult.
//
// Parameters:
//   - result:         the *MatchResult produced by OrderMatcher.Match() (task 2.6.2).
//   - initialCapital: the starting balance from Config.InitialCapital; used as
//     the denominator for TotalPnLPercent and MaxDrawdownPercent.
//
// Returns ErrNilMatchResult when result is nil.
//
// Edge cases handled:
//   - No trades          → all metrics are zero; no panic.
//   - No losing trades   → ProfitFactor = 0 (represents "∞", collapsed for API safety).
//   - Zero initialCapital → percent metrics are zero (no division-by-zero).
//   - Empty BalanceHistory → MaxDrawdown / MaxDrawdownPercent are zero.
func CalculatePerformance(result *MatchResult, initialCapital decimal.Decimal) (*PerformanceSummary, error) {
	if result == nil {
		return nil, ErrNilMatchResult
	}

	summary := &PerformanceSummary{}

	// ── Pass 1: iterate trades → PnL, Win/Loss counts, ProfitFactor ─────────
	var grossWin decimal.Decimal  // sum of all positive PnL values
	var grossLoss decimal.Decimal // sum of absolute values of all negative PnL values

	for _, trade := range result.Trades {
		summary.TotalPnL = summary.TotalPnL.Add(trade.PnL)
		summary.TotalTrades++

		if trade.PnL.IsPositive() {
			summary.WinningTrades++
			grossWin = grossWin.Add(trade.PnL)
		} else {
			// Breakeven (PnL == 0) counts as a losing trade — consistent with
			// standard backtesting convention where a fee-neutral trade is not a win.
			summary.LosingTrades++
			grossLoss = grossLoss.Add(trade.PnL.Abs())
		}
	}

	summary.TotalTrades = summary.WinningTrades + summary.LosingTrades

	// ── TotalPnLPercent ───────────────────────────────────────────────────────
	if initialCapital.IsPositive() {
		hundred := decimal.NewFromInt(100)
		summary.TotalPnLPercent = summary.TotalPnL.Div(initialCapital).Mul(hundred)
	}

	// ── WinRate ───────────────────────────────────────────────────────────────
	if summary.TotalTrades > 0 {
		hundred := decimal.NewFromInt(100)
		totalTradesDec := decimal.NewFromInt(int64(summary.TotalTrades))
		winningTradesDec := decimal.NewFromInt(int64(summary.WinningTrades))
		summary.WinRate = winningTradesDec.Div(totalTradesDec).Mul(hundred)
	}

	// ── ProfitFactor ──────────────────────────────────────────────────────────
	// Convention: ProfitFactor = 0 when grossLoss == 0 (no losing trades).
	// Representing "infinity" as a special float value would break JSON
	// serialisation and the api.yaml numeric type contract.
	if grossLoss.IsPositive() {
		summary.ProfitFactor = grossWin.Div(grossLoss)
	}

	// ── Pass 2: iterate BalanceHistory → MaxDrawdown, MaxDrawdownPercent ──────
	summary.MaxDrawdown, summary.MaxDrawdownPercent = calculateMaxDrawdown(
		result.BalanceHistory, initialCapital,
	)

	return summary, nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Max Drawdown Helper (unexported, pure)
// ═══════════════════════════════════════════════════════════════════════════

// calculateMaxDrawdown scans the BalanceHistory series and returns the
// maximum observed peak-to-trough drawdown in both absolute USDT and
// percentage terms.
//
// Algorithm: single pass with a rolling peak tracker.
//
//	For each snapshot:
//	  1. Update peak if snapshot.Balance > currentPeak.
//	  2. Compute drawdown = currentPeak − snapshot.Balance.
//	  3. Update maxDrawdown if drawdown > maxDrawdown.
//	  4. Compute drawdownPct = drawdown / peakAtThat Point × 100.
//	  5. Update maxDrawdownPct if drawdownPct > maxDrawdownPct.
//
// The percent is computed relative to the peak that was in effect at the
// moment the trough occurred — this matches the standard financial definition.
//
// Returns (zero, zero) for an empty history or zero initialCapital.
func calculateMaxDrawdown(history []BalanceSnapshot, initialCapital decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	if len(history) == 0 {
		return decimal.Zero, decimal.Zero
	}

	// Seed the peak with the initial capital so that a loss on the very first
	// candle is correctly captured as a drawdown from the starting point.
	peak := initialCapital
	if peak.IsZero() || peak.IsNegative() {
		// Fallback: seed from the first snapshot when initialCapital is unusable.
		peak = history[0].Balance
	}

	var maxDrawdown decimal.Decimal
	var maxDrawdownPct decimal.Decimal
	var peakAtMaxDD decimal.Decimal // the peak at the time maxDrawdown was recorded

	hundred := decimal.NewFromInt(100)

	for _, snapshot := range history {
		// Update rolling peak.
		if snapshot.Balance.GreaterThan(peak) {
			peak = snapshot.Balance
		}

		// Current drawdown from peak.
		drawdown := peak.Sub(snapshot.Balance)

		if drawdown.GreaterThan(maxDrawdown) {
			maxDrawdown = drawdown
			peakAtMaxDD = peak
		}
	}

	// Compute percent using the peak at which the maximum drawdown originated.
	if peakAtMaxDD.IsPositive() {
		maxDrawdownPct = maxDrawdown.Div(peakAtMaxDD).Mul(hundred)
	}

	return maxDrawdown, maxDrawdownPct
}

// ═══════════════════════════════════════════════════════════════════════════
//  Sentinel Error
// ═══════════════════════════════════════════════════════════════════════════

// ErrNilMatchResult is returned by CalculatePerformance when the provided
// MatchResult pointer is nil. Callers in the logic layer should surface this
// as an internal server error (HTTP 500).
var ErrNilMatchResult = errors.New("performance: MatchResult must not be nil")
