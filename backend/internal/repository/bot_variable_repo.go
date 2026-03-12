package repository

// bot_variable_repo.go implements BotLifecycleVarRepository — the data-access
// layer for the bot_lifecycle_variables table (Database Schema §6).
//
// Task 2.7.3 — Lifecycle Variables Persistence (read/write bot_lifecycle_variables).
// WBS: P2-Backend · 14/03/2026
// SRS: FR-DESIGN-04, NFR-REL-04
//
// Design: the interface exposes two operations only — LoadAll and FlushAll —
// modelling the full-replace semantics demanded by the "data integrity on
// restart" requirement. A partial update (UPSERT per variable) would require
// a UNIQUE constraint on (bot_id, variable_name), which the current schema
// omits. The atomic DELETE + batch INSERT transaction is therefore the
// conformant approach that matches the existing schema.sql (Task 1.1.2).

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// BotLifecycleVarRepository defines the data-access contract for the
// bot_lifecycle_variables table (DB Schema §6, WBS 2.7.3).
//
// The interface works with map[string]interface{} so the bot engine and
// blockly executor remain decoupled from the GORM entity model.
type BotLifecycleVarRepository interface {
	// LoadAll fetches all lifecycle variable rows for the given bot and returns
	// them as a name → decoded-value map. JSONB values are unmarshalled back
	// into Go interface{} so the blockly executor can use them directly.
	//
	// Returns an empty (non-nil) map when the bot has no persisted variables yet.
	// This is the normal case on the first Session of a brand-new bot.
	LoadAll(ctx context.Context, botID string) (map[string]interface{}, error)

	// FlushAll atomically replaces all lifecycle variables for the given bot.
	// The operation runs inside a single DB transaction:
	//   1. DELETE all existing rows for botID.
	//   2. INSERT a new row for each key–value pair in vars.
	//
	// When vars is empty, only the DELETE executes — ensuring stale rows from
	// a previous run are cleaned up. The atomic transaction guarantees that the
	// persisted state always exactly mirrors the in-RAM state at the end of a
	// successful Session (NFR-REL-04, DB Schema §6 ACID).
	FlushAll(ctx context.Context, botID string, vars map[string]interface{}) error
}

type botLifecycleVarRepository struct {
	db *gorm.DB
}

// NewBotLifecycleVarRepository constructs a GORM-backed BotLifecycleVarRepository.
func NewBotLifecycleVarRepository(db *gorm.DB) BotLifecycleVarRepository {
	return &botLifecycleVarRepository{db: db}
}

// LoadAll fetches all rows for the specified bot from bot_lifecycle_variables
// and returns them as a name → decoded-value map.
//
// The composite index idx_bot_variables_lookup(bot_id, variable_name) makes
// this query O(log n) even when a bot accumulates many variables (tech_stack.md §3.3).
//
// Each row's VariableValue (json.RawMessage / JSONB) is json.Unmarshal-ed into
// a generic interface{} value. Numbers arrive as float64 per the Go JSON spec.
// The blockly executor (block_variable.go) normalises them to shopspring/decimal
// when retrieved via variables_lifecycle_get.
func (r *botLifecycleVarRepository) LoadAll(
	ctx context.Context,
	botID string,
) (map[string]interface{}, error) {
	var rows []domain.BotLifecycleVariable

	if err := r.db.WithContext(ctx).
		Where("bot_id = ?", botID).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("bot_variable_repo: LoadAll (bot_id=%s): %w", botID, err)
	}

	result := make(map[string]interface{}, len(rows))
	for _, row := range rows {
		var val interface{}
		if err := json.Unmarshal(row.VariableValue, &val); err != nil {
			return nil, fmt.Errorf(
				"bot_variable_repo: LoadAll: unmarshal variable %q (bot_id=%s): %w",
				row.VariableName, botID, err,
			)
		}
		result[row.VariableName] = val
	}

	return result, nil
}

// FlushAll atomically replaces all bot_lifecycle_variables rows for the given bot.
//
// DELETE + batch INSERT run inside a single GORM transaction to guarantee that
// no partial or inconsistent state is visible if the server crashes between
// the two operations (DB Schema §6 ACID, NFR-REL-04).
//
// Batch size 100 is consistent with the CreateInBatches pattern used for candle
// data in this codebase (SRS FR-CORE-03, WBS 2.4.2).
func (r *botLifecycleVarRepository) FlushAll(
	ctx context.Context,
	botID string,
	vars map[string]interface{},
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 1: Remove the entire current persisted state for this bot.
		if err := tx.
			Where("bot_id = ?", botID).
			Delete(&domain.BotLifecycleVariable{}).Error; err != nil {
			return fmt.Errorf("bot_variable_repo: FlushAll delete (bot_id=%s): %w", botID, err)
		}

		if len(vars) == 0 {
			// Nothing to insert — clean slate achieved.
			return nil
		}

		// Step 2: Build new entity rows from the provided variable map.
		rows := make([]domain.BotLifecycleVariable, 0, len(vars))
		for name, val := range vars {
			encoded, err := json.Marshal(val)
			if err != nil {
				return fmt.Errorf(
					"bot_variable_repo: FlushAll marshal %q (bot_id=%s): %w",
					name, botID, err,
				)
			}
			rows = append(rows, domain.BotLifecycleVariable{
				BotID:         botID,
				VariableName:  name,
				VariableValue: json.RawMessage(encoded),
			})
		}

		// Step 3: Batch insert — consistent with SRS FR-CORE-03 pattern.
		if err := tx.CreateInBatches(rows, 100).Error; err != nil {
			return fmt.Errorf("bot_variable_repo: FlushAll insert (bot_id=%s): %w", botID, err)
		}

		return nil
	})
}
