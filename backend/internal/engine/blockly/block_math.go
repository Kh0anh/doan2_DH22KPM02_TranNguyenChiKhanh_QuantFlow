package blockly

// block_math.go implements execution handlers for the Math block group
// (4 blocks), as specified in blockly.md §3.3 and SRS FR-DESIGN-06.
//
// Task 2.5.3 — Execute Math group (4 blocks) with BigInt/Decimal.
// WBS: P2-Backend · 11/03/2026
// SRS: FR-DESIGN-06, NFR-PERF-02
//
// Blocks implemented (all return decimal.Decimal wrapped in interface{}):
//   value: math_number, math_arithmetic, math_round, math_random_int
//
// All numeric operations use shopspring/decimal to avoid float64 precision
// errors on crypto prices and order quantities (SRS FR-DESIGN-06 "BigInt").
// The toDecimal() and toBool() helpers defined in block_logic.go are reused.

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Constants
// ═══════════════════════════════════════════════════════════════════════════

// MaxRoundDecimals is the upper bound for the DECIMALS field of math_round.
// Matches the field_number max=18 defined in blockly.md §3.3.3.
// shopspring/decimal supports up to 18 significant decimal places for the
// precision required by Binance Futures price/quantity filters.
const MaxRoundDecimals = 18

// ═══════════════════════════════════════════════════════════════════════════
//  Handler Registration (init)
// ═══════════════════════════════════════════════════════════════════════════

func init() {
	RegisterHandler("math_number", executeMathNumber)
	RegisterHandler("math_arithmetic", executeMathArithmetic)
	RegisterHandler("math_round", executeMathRound)
	RegisterHandler("math_random_int", executeMathRandomInt)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Value Block Handlers — Number outputs (decimal.Decimal)
// ═══════════════════════════════════════════════════════════════════════════

// executeMathNumber handles the `math_number` block (blockly.md §3.3.1).
//
// Returns the constant numeric value entered by the user in the NUM field
// as a decimal.Decimal. This is the primary way users supply literal numbers
// (prices, periods, quantities, leverage values) to other blocks.
//
// The field is stored as a JSON number (float64 after unmarshal); it is
// immediately promoted to decimal.Decimal to preserve precision throughout
// the execution pipeline.
// Unit cost: 1 (charged by ExecuteBlock).
func executeMathNumber(_ *ExecutionContext, block *Block) (interface{}, error) {
	val := GetFieldFloat(block, "NUM")
	return decimal.NewFromFloat(val), nil
}

// executeMathArithmetic handles the `math_arithmetic` block (blockly.md §3.3.2).
//
// Performs one of four arithmetic operations on two Number inputs A and B.
// All computation uses shopspring/decimal to avoid floating-point errors that
// accumulate across EMA/RSI/price calculations (SRS FR-DESIGN-06).
//
// OP field values: "ADD", "MINUS", "MULTIPLY", "DIVIDE".
//
// Divide-by-zero handling (WBS 2.5.3 note):
//   - When OP = "DIVIDE" and B evaluates to zero, the handler returns
//     decimal.Zero and logs a slog.Warn. The session is NOT terminated —
//     a division by zero is a user logic error, not a system fault, and the
//     strategy may still produce valid outcomes without crashing.
//
// Unit cost: 1.
func executeMathArithmetic(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawA, err := EvalValue(ctx, GetInputBlock(block, "A"))
	if err != nil {
		return nil, fmt.Errorf("math_arithmetic: evaluating A input: %w", err)
	}

	rawB, err := EvalValue(ctx, GetInputBlock(block, "B"))
	if err != nil {
		return nil, fmt.Errorf("math_arithmetic: evaluating B input: %w", err)
	}

	a := toDecimal(rawA)
	b := toDecimal(rawB)

	switch GetFieldString(block, "OP") {
	case "ADD":
		return a.Add(b), nil
	case "MINUS":
		return a.Sub(b), nil
	case "MULTIPLY":
		return a.Mul(b), nil
	case "DIVIDE":
		if b.IsZero() {
			ctx.Logger.Warn("math_arithmetic: division by zero; returning 0",
				slog.String("block_id", block.ID),
				slog.String("op", "DIVIDE"),
				slog.String("a", a.String()),
			)
			return decimal.Zero, nil
		}
		return a.Div(b), nil
	default:
		ctx.Logger.Warn("math_arithmetic: unknown OP field value; returning 0",
			slog.String("op", GetFieldString(block, "OP")),
			slog.String("block_id", block.ID),
		)
		return decimal.Zero, nil
	}
}

// executeMathRound handles the `math_round` block (blockly.md §3.3.3).
//
// Rounds the NUM input value to the number of decimal places specified by
// the DECIMALS field (range 0–18, default 2). Uses half-up rounding, which
// is the standard rounding mode for financial calculations and consistent
// with Binance's order quantity / price filter requirements.
//
// The DECIMALS value is read as a float64 field and rounded to the nearest
// integer. It is then clamped to [0, MaxRoundDecimals] to guard against
// out-of-range values that may arrive from runtime calculations.
//
// Critical use-case: computing order QUANTITY that must comply with the
// Binance stepSize filter (e.g., stepSize=0.001 → DECIMALS=3).
// Unit cost: 1.
func executeMathRound(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawNum, err := EvalValue(ctx, GetInputBlock(block, "NUM"))
	if err != nil {
		return nil, fmt.Errorf("math_round: evaluating NUM input: %w", err)
	}

	num := toDecimal(rawNum)

	decimals := int(math.Round(GetFieldFloat(block, "DECIMALS")))
	if decimals < 0 {
		decimals = 0
	}
	if decimals > MaxRoundDecimals {
		ctx.Logger.Warn("math_round: DECIMALS clamped to MaxRoundDecimals",
			slog.Int("requested", decimals),
			slog.Int("clamped_to", MaxRoundDecimals),
			slog.String("block_id", block.ID),
		)
		decimals = MaxRoundDecimals
	}

	return num.Round(int32(decimals)), nil
}

// executeMathRandomInt handles the `math_random_int` block (blockly.md §3.3.4).
//
// Returns a uniformly distributed random integer in the inclusive range
// [FROM, TO]. If FROM > TO the two bounds are swapped automatically — this
// handles the case where the user connects the inputs in reverse order without
// causing a panic or confusing error.
//
// The result is returned as decimal.Decimal (integer-valued) to remain
// consistent with the rest of the math group's return type. Downstream
// callers using toDecimal() will receive the correct integer Decimal.
//
// Implementation uses math/rand (stdlib) — cryptographic randomness is not
// required for trading strategy logic (SRS §2.6: stdlib, no external deps).
// Unit cost: 1.
func executeMathRandomInt(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawFrom, err := EvalValue(ctx, GetInputBlock(block, "FROM"))
	if err != nil {
		return nil, fmt.Errorf("math_random_int: evaluating FROM input: %w", err)
	}

	rawTo, err := EvalValue(ctx, GetInputBlock(block, "TO"))
	if err != nil {
		return nil, fmt.Errorf("math_random_int: evaluating TO input: %w", err)
	}

	from := int(math.Round(toDecimal(rawFrom).InexactFloat64()))
	to := int(math.Round(toDecimal(rawTo).InexactFloat64()))

	// Auto-swap if FROM > TO (defensive — user may connect inputs in reverse).
	if from > to {
		from, to = to, from
	}

	if from == to {
		return decimal.NewFromInt(int64(from)), nil
	}

	n := rand.Intn(to-from+1) + from // #nosec G404 — non-crypto use is intentional
	return decimal.NewFromInt(int64(n)), nil
}
