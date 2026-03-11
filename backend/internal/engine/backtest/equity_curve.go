// Package backtest — Task 2.6.4: Equity Curve Data Generation.
//
// equity_curve.go is the fourth stage in the backtest pipeline:
//
//	OrderMatcher.Match() ──► MatchResult ──► GenerateEquityCurve() ──► []EquityPoint
//	      (task 2.6.2)                            (task 2.6.4)
//
// It transforms the BalanceHistory produced by the order matcher into a
// typed slice of (timestamp, equity) data points ready for serialisation into
// the BacktestResult.equity_curve field (api.yaml) and consumption by the
// TradingView Lightweight Charts library on the frontend (SRS FR-RUN-04).
//
// This is a pure, stateless transformation — no struct, no DB I/O, no
// new dependencies. All monetary values are kept as shopspring/decimal to
// maintain precision until the final JSON serialisation step in task 2.6.5.
//
// WBS: P2-Backend · 13/03/2026
// SRS: FR-RUN-04
package backtest

import (
	"time"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Output Type
// ═══════════════════════════════════════════════════════════════════════════

// EquityPoint is a single data point on the equity curve (growth chart).
//
// It maps 1:1 to the equity_curve[i] item schema in api.yaml
// (§BacktestResult.equity_curve) and is designed to be directly serialised
// into the JSON response by the backtest logic layer (task 2.6.5).
//
// The Lightweight Charts line-series API (TradingView) expects data in the
// form [{time, value}]; the serialiser in task 2.6.5 renames Timestamp→time
// and Equity→value when building the API response.
type EquityPoint struct {
	// Timestamp is the moment in time this equity sample was recorded.
	// For the origin point this is Config.StartTime; for subsequent points it
	// is the OpenTime of the corresponding candle from BalanceHistory.
	Timestamp time.Time

	// Equity is the simulated USDT account balance at Timestamp.
	// Uses decimal.Decimal to preserve precision through the pipeline.
	Equity decimal.Decimal
}

// ═══════════════════════════════════════════════════════════════════════════
//  GenerateEquityCurve — Main Transformer
// ═══════════════════════════════════════════════════════════════════════════

// GenerateEquityCurve converts the BalanceHistory from a completed order
// matching run into a slice of EquityPoint values suitable for chart rendering.
//
// Parameters:
//   - result:         the *MatchResult produced by OrderMatcher.Match() (2.6.2).
//   - initialCapital: the starting balance (Config.InitialCapital) used as the
//     equity value of the mandatory origin anchor point.
//   - startTime:      Config.StartTime — the timestamp of the origin anchor.
//
// The first element of the returned slice is always the origin anchor:
//
//	EquityPoint{Timestamp: startTime, Equity: initialCapital}
//
// This ensures the Lightweight Charts line begins at the baseline capital
// before any trading activity occurs, giving a visually accurate growth chart.
//
// Subsequent elements are a 1:1 projection of result.BalanceHistory:
//
//	EquityPoint{Timestamp: snap.Timestamp, Equity: snap.Balance}
//
// Returns ErrNilMatchResult when result is nil.
// Returns a slice containing only the origin anchor when BalanceHistory is empty.
func GenerateEquityCurve(
	result *MatchResult,
	initialCapital decimal.Decimal,
	startTime time.Time,
) ([]EquityPoint, error) {
	if result == nil {
		return nil, ErrNilMatchResult
	}

	// Pre-allocate with exact capacity: 1 origin anchor + N balance snapshots.
	points := make([]EquityPoint, 0, 1+len(result.BalanceHistory))

	// ── Origin anchor ─────────────────────────────────────────────────────
	// Always prepend the starting point so the chart line originates at the
	// initial capital value at t=startTime, before the first candle fires.
	points = append(points, EquityPoint{
		Timestamp: startTime,
		Equity:    initialCapital,
	})

	// ── Balance history projection ─────────────────────────────────────────
	// Each BalanceSnapshot was recorded by the order matcher after processing
	// all fills triggered by that candle, so it accurately reflects the
	// account balance at the close of each simulation step.
	for _, snap := range result.BalanceHistory {
		points = append(points, EquityPoint{
			Timestamp: snap.Timestamp,
			Equity:    snap.Balance,
		})
	}

	return points, nil
}
