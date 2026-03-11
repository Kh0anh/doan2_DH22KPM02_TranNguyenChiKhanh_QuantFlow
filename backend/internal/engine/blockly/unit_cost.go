package blockly

// unit_cost.go implements the concrete per-Session unit budget enforcer
// (SRS FR-RUN-07, blockly.md §1.3 — "Cơ chế giới hạn Unit Cost").
//
// Task 2.5.7 — Unit Cost mechanism (1000 Unit/Session) and Session Management.
// WBS: P2-Backend · 12/03/2026
// SRS: FR-RUN-07
//
// Design:
//   - DefaultUnitCostLimit = 1000 is the authoritative constant used in
//     comments and logic across executor.go, block_logic.go, and blockly.md.
//   - SessionUnitTracker is a lightweight struct (no goroutine, no mutex) that
//     tracks how many units the current Session has consumed and enforces the
//     ceiling. ExecutionContext is NOT thread-safe by design (each Bot goroutine
//     owns its own context), so no synchronisation is needed here.
//   - Enforcement path (already wired in Tasks 2.5.1–2.5.2):
//       ExecuteBlock → Consume(meta.UnitCost) → ErrUnitCostExceeded
//       loop handlers  → Consume(1) per iteration → ErrUnitCostExceeded
//   - A fresh SessionUnitTracker (used=0) is created for every new Session via
//     NewExecutionContext. The Session ends when ExecuteChain returns — the
//     tracker is discarded along with the ExecutionContext.
//
// No imports required — pure Go stdlib, no external dependencies.

// DefaultUnitCostLimit is the maximum number of units a single Session may
// consume before ErrUnitCostExceeded is returned and execution is halted.
//
// Value: 1000 — matches the example in blockly.md §1.3:
//
//	"Vòng 91: 1001 → VƯỢT NGƯỠNG → Dừng Session → Ghi log lỗi"
//
// This constant is also referenced in comments throughout executor.go and
// block_logic.go. Changing this value adjusts the limit globally.
const DefaultUnitCostLimit = 1000

// SessionUnitTracker is the concrete implementation of UnitCostTracker.
// It maintains a running total of units consumed in the current Session and
// refuses any deduction that would push the total above the configured limit.
//
// Lifecycle: created by NewExecutionContext at Session start; discarded when
// the Session ends (ExecutionContext is not reused across Sessions).
type SessionUnitTracker struct {
	used  int // total units consumed so far in this Session
	limit int // maximum units allowed; set at construction time
}

// NewSessionUnitTracker creates a SessionUnitTracker with the given limit.
// Use NewDefaultSessionUnitTracker for the standard 1000-unit budget.
//
// Panics if limit ≤ 0 — a non-positive budget would reject every block
// immediately, which indicates a programming error (mis-wired configuration).
func NewSessionUnitTracker(limit int) UnitCostTracker {
	if limit <= 0 {
		panic("blockly: NewSessionUnitTracker: limit must be > 0")
	}
	return &SessionUnitTracker{used: 0, limit: limit}
}

// NewDefaultSessionUnitTracker creates a SessionUnitTracker with the standard
// DefaultUnitCostLimit (1000 units). This is the function wired into
// NewExecutionContext so every live Bot session and Backtest simulation gets
// the correct budget out of the box.
func NewDefaultSessionUnitTracker() UnitCostTracker {
	return NewSessionUnitTracker(DefaultUnitCostLimit)
}

// Consume deducts units from the session budget.
//
// Pre-flight check: if used + units would exceed the limit, the budget is NOT
// modified and ErrUnitCostExceeded is returned immediately. The caller
// (ExecuteBlock, controls_repeat, controls_while) must treat this error as
// terminal — it propagates all the way up to the bot goroutine which logs
// "UNIT_COST_EXCEEDED" and stops the Session.
//
// A zero-or-negative units argument is a no-op (returns nil). This handles
// blocks whose UnitCost is intentionally 0 (e.g., event_on_candle).
func (t *SessionUnitTracker) Consume(units int) error {
	if units <= 0 {
		return nil
	}
	if t.used+units > t.limit {
		return ErrUnitCostExceeded
	}
	t.used += units
	return nil
}

// Used returns the total units consumed so far in the current Session.
// Used by ExecuteBlock and loop handlers for structured log fields such as
// slog.Int("units_used", ctx.UnitTracker.Used()).
func (t *SessionUnitTracker) Used() int {
	return t.used
}
