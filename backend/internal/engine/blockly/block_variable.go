package blockly

// block_variable.go implements execution handlers for the Variable block group
// (4 blocks), as specified in blockly.md §3.4 and SRS FR-DESIGN-04.
//
// Task 2.5.4 — Execute Variable Session and Lifecycle group (4 blocks).
// WBS: P2-Backend · 11/03/2026
// SRS: FR-DESIGN-04, FR-RUN-05, NFR-REL-04
//
// Blocks implemented:
//   statement: variables_session_set, variables_lifecycle_set
//   value:     variables_session_get, variables_lifecycle_get
//
// Variable storage model (strict scope separation):
//
//   Session Variables  → ctx.SessionVars  (RAM map[string]interface{})
//     Scope: 1 Session only. Created empty at Session start; GC'd when Session
//     ends. Never persisted. Use case: cache RSI within one candle event so the
//     same value is not recomputed by multiple blocks (blockly.md §3.4).
//
//   Lifecycle Variables → ctx.LifecycleVars (RAM map[string]interface{})
//     Scope: full Bot lifetime. Survives between Sessions (e.g., "has_position").
//     Load from bot_lifecycle_variables (JSONB) before Session start and flush
//     back to DB after Session end — BOTH are the responsibility of Task 2.7.3
//     (Bot Lifecycle Variables Persistence). This file only reads and writes the
//     RAM map; it does not touch the database (clean separation of concerns).
//
// Unset variable reads (blockly.md §3.4.2, §3.4.4):
//   Both _get handlers return decimal.Zero when the key is absent or set to nil.
//   This matches the documented behaviour "trả về 0 nếu biến chưa được gán".
//
// Dependencies: shopspring/decimal (direct, promoted in Task 2.5.2).
// No new external imports.

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Handler Registration (init)
// ═══════════════════════════════════════════════════════════════════════════

func init() {
	RegisterHandler("variables_session_set", executeVariablesSessionSet)
	RegisterHandler("variables_session_get", executeVariablesSessionGet)
	RegisterHandler("variables_lifecycle_set", executeVariablesLifecycleSet)
	RegisterHandler("variables_lifecycle_get", executeVariablesLifecycleGet)
}

// ═══════════════════════════════════════════════════════════════════════════
//  Session Variable Handlers — RAM only, per-session scope
// ═══════════════════════════════════════════════════════════════════════════

// executeVariablesSessionSet handles the `variables_session_set` block
// (blockly.md §3.4.1).
//
// Evaluates the VALUE input and stores the result under VAR_NAME in
// ctx.SessionVars. Overwrites any previously stored value with the same name.
// The map is discarded at the end of the Session, so no cleanup is needed.
//
// VAR_NAME is trimmed of surrounding whitespace to prevent silent key mismatches
// when the user types a name with accidental leading/trailing spaces in the
// Blockly field_input widget.
//
// Statement block: returns (nil, nil) on success.
// Unit cost: 1 (charged by ExecuteBlock).
func executeVariablesSessionSet(ctx *ExecutionContext, block *Block) (interface{}, error) {
	name := strings.TrimSpace(GetFieldString(block, "VAR_NAME"))
	if name == "" {
		return nil, fmt.Errorf("variables_session_set: VAR_NAME field is empty (block_id=%s)", block.ID)
	}

	rawVal, err := EvalValue(ctx, GetInputBlock(block, "VALUE"))
	if err != nil {
		return nil, fmt.Errorf("variables_session_set: evaluating VALUE input for %q: %w", name, err)
	}

	ctx.SessionVars[name] = rawVal
	return nil, nil
}

// executeVariablesSessionGet handles the `variables_session_get` block
// (blockly.md §3.4.2).
//
// Returns the value stored under VAR_NAME in ctx.SessionVars.
// If the key is absent or was explicitly set to nil, returns decimal.Zero.
//
// Value block: always returns a non-error result to allow chaining into
// arithmetic or comparison blocks without defensive nil checks in callers.
// Unit cost: 1.
func executeVariablesSessionGet(ctx *ExecutionContext, block *Block) (interface{}, error) {
	name := strings.TrimSpace(GetFieldString(block, "VAR_NAME"))
	if name == "" {
		return decimal.Zero, nil
	}

	val, ok := ctx.SessionVars[name]
	if !ok || val == nil {
		return decimal.Zero, nil
	}

	return val, nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Lifecycle Variable Handlers — RAM map, DB sync delegated to Task 2.7.3
// ═══════════════════════════════════════════════════════════════════════════

// executeVariablesLifecycleSet handles the `variables_lifecycle_set` block
// (blockly.md §3.4.3).
//
// Evaluates the VALUE input and writes the result into ctx.LifecycleVars under
// VAR_NAME. The write is to the RAM map only — the Bot goroutine (Task 2.7.3)
// is responsible for flushing ctx.LifecycleVars back to bot_lifecycle_variables
// (JSONB) in PostgreSQL after the Session ends, ensuring durability across
// Session boundaries and Server restarts (NFR-REL-04).
//
// A slog.Debug record is emitted for each Lifecycle set to provide per-session
// variable traceability in bot logs without incurring the I/O cost of a DB write
// inside the hot execution path.
//
// Statement block: returns (nil, nil) on success.
// Unit cost: 1.
func executeVariablesLifecycleSet(ctx *ExecutionContext, block *Block) (interface{}, error) {
	name := strings.TrimSpace(GetFieldString(block, "VAR_NAME"))
	if name == "" {
		return nil, fmt.Errorf("variables_lifecycle_set: VAR_NAME field is empty (block_id=%s)", block.ID)
	}

	rawVal, err := EvalValue(ctx, GetInputBlock(block, "VALUE"))
	if err != nil {
		return nil, fmt.Errorf("variables_lifecycle_set: evaluating VALUE input for %q: %w", name, err)
	}

	ctx.LifecycleVars[name] = rawVal

	ctx.Logger.Debug("lifecycle variable set",
		slog.String("var_name", name),
		slog.Any("value", rawVal),
		slog.String("block_id", block.ID),
	)

	return nil, nil
}

// executeVariablesLifecycleGet handles the `variables_lifecycle_get` block
// (blockly.md §3.4.4).
//
// Returns the value stored under VAR_NAME in ctx.LifecycleVars.
// ctx.LifecycleVars is pre-populated from bot_lifecycle_variables (DB) by the
// Bot goroutine at Session start (Task 2.7.3). If the variable was never set
// in a previous Session, the key will be absent; this handler returns
// decimal.Zero in that case, matching the documented default.
//
// Value block: always returns a non-error result for unset keys.
// Unit cost: 1.
func executeVariablesLifecycleGet(ctx *ExecutionContext, block *Block) (interface{}, error) {
	name := strings.TrimSpace(GetFieldString(block, "VAR_NAME"))
	if name == "" {
		return decimal.Zero, nil
	}

	val, ok := ctx.LifecycleVars[name]
	if !ok || val == nil {
		return decimal.Zero, nil
	}

	return val, nil
}
