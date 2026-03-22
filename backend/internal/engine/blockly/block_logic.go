package blockly

// block_logic.go implements execution handlers for the Logic & Control block
// group (8 blocks), as specified in blockly.md §3.2 and SRS FR-DESIGN-05.
//
// Task 2.5.2 — Execute Logic and Control group (8 blocks).
// WBS: P2-Backend · 11/03/2026
// SRS: FR-DESIGN-05, FR-RUN-07
//
// Blocks implemented:
//   statement (side-effect): controls_if, controls_if_else,
//                            controls_repeat, controls_while
//   value     (return bool): logic_compare, logic_operation,
//                            logic_negate, logic_boolean

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Constants
// ═══════════════════════════════════════════════════════════════════════════

// MaxRepeatTimes is the maximum number of iterations allowed for a single
// controls_repeat block. This is a safety clamp: the TIMES input may be a
// calculated runtime value (e.g., from math_arithmetic) that could be
// unreasonably large even before the unit cost limit catches it.
//
// With DefaultUnitCostLimit=1000 and 1 unit/iteration, the UnitCostTracker
// already bounds real execution to ≤1000 iterations. This constant provides
// an additional fast-exit for extreme edge cases (e.g., TIMES = 1e12).
const MaxRepeatTimes = 10_000

// ═══════════════════════════════════════════════════════════════════════════
//  Handler Registration (init)
// ═══════════════════════════════════════════════════════════════════════════

func init() {
	// Value blocks — return a bool result used as CONDITION inputs.
	RegisterHandler("logic_boolean", executeLogicBoolean)
	RegisterHandler("logic_negate", executeLogicNegate)
	RegisterHandler("logic_operation", executeLogicOperation)
	RegisterHandler("logic_compare", executeLogicCompare)

	// Statement blocks — control flow, no return value.
	RegisterHandler("controls_if", executeControlsIf)
	RegisterHandler("controls_if_else", executeControlsIfElse)
	RegisterHandler("controls_repeat", executeControlsRepeat)
	RegisterHandler("controls_while", executeControlsWhile)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Type-coercion Helpers (package-internal, reused by block_math.go etc.)
// ═══════════════════════════════════════════════════════════════════════════

// toDecimal converts an interface{} value returned by EvalValue into a
// shopspring/decimal.Decimal for safe, precision-preserving numeric operations.
//
// Mapping:
//   - float64  → Decimal (encoding/json decodes all JSON numbers to float64)
//   - decimal.Decimal → returned as-is (already a Decimal from another block)
//   - nil      → Decimal zero (unconnected input slot → safe default)
//   - other    → Decimal zero (unexpected type — treated as 0, logged by caller)
func toDecimal(v interface{}) decimal.Decimal {
	switch val := v.(type) {
	case float64:
		return decimal.NewFromFloat(val)
	case decimal.Decimal:
		return val
	case string:
		// Lifecycle variables loaded from DB arrive as JSON strings because
		// shopspring/decimal.Decimal MarshalJSON produces a quoted number.
		// json.Unmarshal into interface{} keeps it as a string.
		d, err := decimal.NewFromString(val)
		if err != nil {
			return decimal.Zero
		}
		return d
	case int:
		return decimal.NewFromInt(int64(val))
	case int64:
		return decimal.NewFromInt(val)
	default:
		return decimal.Zero
	}
}

// toBool converts an interface{} value returned by EvalValue into a bool.
//
// Mapping:
//   - bool     → returned as-is
//   - nil      → false (unconnected CONDITION input → condition not met)
//   - other    → false (unexpected type)
func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════
//  Value Block Handlers — Boolean outputs
// ═══════════════════════════════════════════════════════════════════════════

// executeLogicBoolean handles the `logic_boolean` block (blockly.md §3.2.6).
//
// Returns the constant Boolean value selected by the user via the BOOL
// dropdown field. Field values: "TRUE" → true, any other string → false.
// Unit cost: 1 (charged by ExecuteBlock before this handler is called).
func executeLogicBoolean(_ *ExecutionContext, block *Block) (interface{}, error) {
	return GetFieldString(block, "BOOL") == "TRUE", nil
}

// executeLogicNegate handles the `logic_negate` block (blockly.md §3.2.5).
//
// Evaluates the connected BOOL input and inverts its boolean value.
// Unconnected input (nil) is treated as false → returns true.
// Unit cost: 1.
func executeLogicNegate(ctx *ExecutionContext, block *Block) (interface{}, error) {
	raw, err := EvalValue(ctx, GetInputBlock(block, "BOOL"))
	if err != nil {
		return nil, fmt.Errorf("logic_negate: evaluating BOOL input: %w", err)
	}
	return !toBool(raw), nil
}

// executeLogicOperation handles the `logic_operation` block (blockly.md §3.2.4).
//
// Combines two Boolean inputs using AND or OR as selected via the OP field.
// Short-circuit evaluation is NOT applied — both A and B are always evaluated.
// This is intentional: Blockly does not model short-circuit semantics and both
// sub-trees may contain side-effect blocks that users expect to run.
// Unit cost: 1.
func executeLogicOperation(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawA, err := EvalValue(ctx, GetInputBlock(block, "A"))
	if err != nil {
		return nil, fmt.Errorf("logic_operation: evaluating A input: %w", err)
	}

	rawB, err := EvalValue(ctx, GetInputBlock(block, "B"))
	if err != nil {
		return nil, fmt.Errorf("logic_operation: evaluating B input: %w", err)
	}

	a := toBool(rawA)
	b := toBool(rawB)

	switch GetFieldString(block, "OP") {
	case "AND":
		return a && b, nil
	case "OR":
		return a || b, nil
	default:
		// Defensive: unknown OP defaults to AND (strict).
		ctx.Logger.Warn("logic_operation: unknown OP field value; defaulting to AND",
			slog.String("op", GetFieldString(block, "OP")),
			slog.String("block_id", block.ID),
		)
		return a && b, nil
	}
}

// executeLogicCompare handles the `logic_compare` block (blockly.md §3.2.3).
//
// Compares two numeric inputs using shopspring/decimal for precision — avoids
// float64 equality pitfalls on crypto prices (e.g., RSI(14) == 70.0).
// Input slots A and B are evaluated; nil (unconnected) maps to Decimal zero.
//
// OP field values: "EQ", "NEQ", "GT", "GTE", "LT", "LTE".
// Unit cost: 1.
func executeLogicCompare(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawA, err := EvalValue(ctx, GetInputBlock(block, "A"))
	if err != nil {
		return nil, fmt.Errorf("logic_compare: evaluating A input: %w", err)
	}

	rawB, err := EvalValue(ctx, GetInputBlock(block, "B"))
	if err != nil {
		return nil, fmt.Errorf("logic_compare: evaluating B input: %w", err)
	}

	a := toDecimal(rawA)
	b := toDecimal(rawB)
	cmp := a.Cmp(b) // -1: a < b, 0: a == b, 1: a > b

	switch GetFieldString(block, "OP") {
	case "EQ":
		return cmp == 0, nil
	case "NEQ":
		return cmp != 0, nil
	case "GT":
		return cmp > 0, nil
	case "GTE":
		return cmp >= 0, nil
	case "LT":
		return cmp < 0, nil
	case "LTE":
		return cmp <= 0, nil
	default:
		ctx.Logger.Warn("logic_compare: unknown OP field value; defaulting to EQ",
			slog.String("op", GetFieldString(block, "OP")),
			slog.String("block_id", block.ID),
		)
		return cmp == 0, nil
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  Statement Block Handlers — Branching
// ═══════════════════════════════════════════════════════════════════════════

// executeControlsIf handles the `controls_if` block (blockly.md §3.2.1).
//
// Evaluates the CONDITION input value block. If true, executes the statement
// chain in the DO input. If false, does nothing and allows execution to
// continue at the next block in the parent chain.
// Unit cost: 1 (charged by ExecuteBlock).
func executeControlsIf(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawCond, err := EvalValue(ctx, GetInputBlock(block, "CONDITION"))
	if err != nil {
		return nil, fmt.Errorf("controls_if: evaluating CONDITION: %w", err)
	}

	if toBool(rawCond) {
		if err := ExecuteChain(ctx, GetInputBlock(block, "DO")); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// executeControlsIfElse handles the `controls_if_else` block (blockly.md §3.2.2).
//
// Evaluates CONDITION, then executes DO if true or ELSE if false.
// Either branch may be empty (nil block) — ExecuteChain handles nil no-op.
// Unit cost: 1 (charged by ExecuteBlock).
func executeControlsIfElse(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawCond, err := EvalValue(ctx, GetInputBlock(block, "CONDITION"))
	if err != nil {
		return nil, fmt.Errorf("controls_if_else: evaluating CONDITION: %w", err)
	}

	if toBool(rawCond) {
		if err := ExecuteChain(ctx, GetInputBlock(block, "DO")); err != nil {
			return nil, err
		}
	} else {
		if err := ExecuteChain(ctx, GetInputBlock(block, "ELSE")); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Statement Block Handlers — Loops
// ═══════════════════════════════════════════════════════════════════════════

// executeControlsRepeat handles the `controls_repeat` block (blockly.md §3.2.7).
//
// Executes the DO chain a fixed number of times determined by the TIMES input.
// TIMES is evaluated once before the loop starts (not re-evaluated per iteration).
//
// Unit cost model (blockly.md §1.4 — "1 Unit per iteration"):
//   - ExecuteBlock charges the block registry UnitCost (1) for block setup.
//   - This handler charges an additional Consume(1) per iteration.
//
// Safety guards:
//   - Negative TIMES is clamped to 0 (no iterations).
//   - TIMES > MaxRepeatTimes is clamped to MaxRepeatTimes (prevents runaway).
//   - ctx.Ctx.Err() is checked each iteration to honour bot stop signals.
//   - UnitTracker.Consume(1) per iteration enforces the session unit budget
//     (SRS FR-RUN-07): the UnitCostTracker (Task 2.5.7) will return
//     ErrUnitCostExceeded once the 1000-unit budget is exhausted.
func executeControlsRepeat(ctx *ExecutionContext, block *Block) (interface{}, error) {
	rawTimes, err := EvalValue(ctx, GetInputBlock(block, "TIMES"))
	if err != nil {
		return nil, fmt.Errorf("controls_repeat: evaluating TIMES: %w", err)
	}

	timesDecimal := toDecimal(rawTimes)
	times := int(math.Round(timesDecimal.InexactFloat64()))
	if times < 0 {
		times = 0
	}
	if times > MaxRepeatTimes {
		ctx.Logger.Warn("controls_repeat: TIMES clamped to MaxRepeatTimes",
			slog.Int("requested", times),
			slog.Int("clamped_to", MaxRepeatTimes),
			slog.String("block_id", block.ID),
		)
		times = MaxRepeatTimes
	}

	doBlock := GetInputBlock(block, "DO")

	for i := 0; i < times; i++ {
		// Check for bot stop signal before each iteration.
		if ctxErr := ctx.Ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		// Charge 1 unit per iteration (blockly.md §1.4).
		if unitErr := ctx.UnitTracker.Consume(1); unitErr != nil {
			ctx.Logger.Warn("controls_repeat: UNIT_COST_EXCEEDED mid-loop",
				slog.Int("iteration", i),
				slog.Int("total_iterations", times),
				slog.Int("units_used", ctx.UnitTracker.Used()),
				slog.String("block_id", block.ID),
			)
			return nil, ErrUnitCostExceeded
		}

		if err := ExecuteChain(ctx, doBlock); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// executeControlsWhile handles the `controls_while` block (blockly.md §3.2.8).
//
// Re-evaluates CONDITION at the start of each iteration. Exits when CONDITION
// returns false or when a stop/error condition occurs.
//
// Unit cost model (blockly.md §1.4 — "1 Unit per iteration"):
//   - ExecuteBlock charges the block-level UnitCost (1) for block setup.
//   - This handler charges Consume(1) per iteration after CONDITION is true.
//
// Safety guards (identical rationale to controls_repeat above):
//   - ctx.Ctx.Err() checked before CONDITION evaluation each iteration.
//   - UnitTracker.Consume(1) charged after condition passes, before body runs —
//     ensures the unit is only consumed when work actually happens.
//   - If CONDITION always evaluates to true, the loop will run until the
//     session unit budget is exhausted and ErrUnitCostExceeded is returned.
//     This is the primary infinite-loop prevention mechanism (SRS FR-RUN-07).
func executeControlsWhile(ctx *ExecutionContext, block *Block) (interface{}, error) {
	condInputBlock := GetInputBlock(block, "CONDITION")
	doBlock := GetInputBlock(block, "DO")

	for {
		// Check bot stop signal before every condition evaluation.
		if ctxErr := ctx.Ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		// Re-evaluate CONDITION each iteration (while semantics, not do-while).
		rawCond, err := EvalValue(ctx, condInputBlock)
		if err != nil {
			return nil, fmt.Errorf("controls_while: evaluating CONDITION: %w", err)
		}
		if !toBool(rawCond) {
			break
		}

		// Charge 1 unit per iteration after confirming condition is true.
		if unitErr := ctx.UnitTracker.Consume(1); unitErr != nil {
			ctx.Logger.Warn("controls_while: UNIT_COST_EXCEEDED mid-loop",
				slog.Int("units_used", ctx.UnitTracker.Used()),
				slog.String("block_id", block.ID),
			)
			return nil, ErrUnitCostExceeded
		}

		if err := ExecuteChain(ctx, doBlock); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
