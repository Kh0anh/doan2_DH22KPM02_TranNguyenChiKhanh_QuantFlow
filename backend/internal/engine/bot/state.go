// state.go — Task 2.7.3: Lifecycle Variables Persistence.
//
// LifecycleStateManager is the adapter between the Bot engine (manager.go)
// and the BotLifecycleVarRepository (repository/bot_variable_repo.go).
//
// It provides a concise Load / Persist API that is called by runSession() in
// manager.go. This thin adapter layer:
//
//   - Adds structured slog logging so variable persistence events are visible
//     in the application log stream alongside bot execution events.
//   - Translates repository errors into engine-friendly error messages.
//   - Shields manager.go from import-cycle risk: engine/bot imports
//     repository but NOT the reverse.
//
// ─── Data Integrity Contract ────────────────────────────────────────────────
//
// Every Session follows the read-modify-write pattern:
//
//  1. Load(ctx, botID)   → fetch persisted state from DB before Session start.
//  2. Session.Run(ctx)   → blockly executor mutates the in-RAM copy.
//  3. Persist(ctx, botID, updatedVars) → atomically flush the updated state.
//
// Because FlushAll wraps DELETE + INSERT in one transaction (bot_variable_repo.go),
// a server crash between steps 2 and 3 leaves the DB in its previous valid state.
// On restart the bot reloads the last committed snapshot — no partial state is
// ever observed (NFR-REL-04, DB Schema §6 ACID guarantee).
//
// Task 2.7.3 — Lifecycle Variables Persistence.
// WBS: P2-Backend · 14/03/2026
// SRS: FR-DESIGN-04, NFR-REL-04
package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kh0anh/quantflow/internal/repository"
)

// LifecycleStateManager coordinates reading and writing lifecycle variable state
// for a bot between the in-RAM execution context and the PostgreSQL
// bot_lifecycle_variables table.
//
// It wraps BotLifecycleVarRepository with slog-structured logging so that
// load and persist events appear in the bot's log stream for observability.
//
// LifecycleStateManager is safe for concurrent use: each Load and Persist call
// creates an independent DB transaction with no shared mutable state.
type LifecycleStateManager struct {
	repo   repository.BotLifecycleVarRepository
	logger *slog.Logger
}

// NewLifecycleStateManager constructs a LifecycleStateManager.
//
//   - repo:   GORM-backed BotLifecycleVarRepository (repository/bot_variable_repo.go).
//   - logger: slog.Logger, already decorated with any service-level fields.
//     If nil, slog.Default() is used as a safe fallback.
func NewLifecycleStateManager(
	repo repository.BotLifecycleVarRepository,
	logger *slog.Logger,
) *LifecycleStateManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &LifecycleStateManager{repo: repo, logger: logger}
}

// Load reads all persisted lifecycle variables for the given bot from the
// bot_lifecycle_variables table and returns them as a name → value map.
//
// Called by manager.go's runSession() BEFORE constructing the SessionConfig so
// that the blockly executor starts each Session with the last committed variable
// state (or an empty map for a brand-new bot on its first Session).
//
// On error, Load returns a non-nil error and a nil map. The caller in runSession()
// should fall back to an empty map and log a warning, allowing the bot to
// continue running in a degraded (stale-variable-free) state rather than halt.
func (m *LifecycleStateManager) Load(
	ctx context.Context,
	botID string,
) (map[string]interface{}, error) {
	vars, err := m.repo.LoadAll(ctx, botID)
	if err != nil {
		return nil, fmt.Errorf("lifecycle_state: Load (bot_id=%s): %w", botID, err)
	}

	m.logger.Debug("lifecycle state loaded",
		slog.String("bot_id", botID),
		slog.Int("variable_count", len(vars)),
	)

	return vars, nil
}

// Persist writes the provided lifecycle variable map back to the
// bot_lifecycle_variables table using the atomic FlushAll operation.
//
// Called by manager.go's runSession() AFTER a successful Session.Run() returns
// so that the final in-RAM variable state is durably committed to PostgreSQL
// before the next candle event arrives (Task 2.7.3, NFR-REL-04).
//
// When vars is nil or empty, Persist still calls FlushAll so that any stale rows
// left over from a previous bot run (e.g. after a strategy logic change that
// removes variables) are cleaned up atomically.
func (m *LifecycleStateManager) Persist(
	ctx context.Context,
	botID string,
	vars map[string]interface{},
) error {
	if err := m.repo.FlushAll(ctx, botID, vars); err != nil {
		return fmt.Errorf("lifecycle_state: Persist (bot_id=%s): %w", botID, err)
	}

	m.logger.Debug("lifecycle state persisted",
		slog.String("bot_id", botID),
		slog.Int("variable_count", len(vars)),
	)

	return nil
}
