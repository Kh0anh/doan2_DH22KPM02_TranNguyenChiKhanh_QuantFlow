package domain

import (
	"encoding/json"
	"time"
)

// BotLifecycleVariable maps to the `bot_lifecycle_variables` table (Database Schema §6).
//
// Each row stores one named variable belonging to a running Bot. The JSONB
// variable_value column allows storing any JSON-serialisable Go value
// (string, number, boolean, array, object) that was written by a
// variables_lifecycle_set block during strategy execution (blockly.md §3.4.3).
//
// A lifecycle variable survives between Sessions (candle events) and across
// server restarts: the Bot goroutine reads all rows into RAM before each
// Session starts and flushes the updated map back to these rows after each
// Session ends (Task 2.7.3, NFR-REL-04).
//
// Indexes:
//   - idx_bot_variables_lookup on (bot_id, variable_name) — composite index
//     defined in database/schema.sql. Enables sub-second lookup of all variables
//     for a given bot at Session start time (tech_stack.md §3.3, DB Schema §Performance #4).
type BotLifecycleVariable struct {
	// ID is the UUID primary key — auto-assigned by PostgreSQL gen_random_uuid().
	ID string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// BotID is the FK to bot_instances.id.
	// Not null; CASCADE delete removes all variables when the bot is deleted.
	BotID string `gorm:"type:uuid;not null;index:idx_bot_variables_lookup,priority:1" json:"bot_id"`

	// VariableName is the user-defined variable name as entered in the Blockly
	// field_input widget, max 100 characters (DB Schema §6).
	VariableName string `gorm:"type:varchar(100);not null;index:idx_bot_variables_lookup,priority:2" json:"variable_name"`

	// VariableValue stores the JSON-serialised value of the lifecycle variable.
	// Using json.RawMessage allows round-tripping arbitrary Go values (numbers,
	// strings, booleans, shopspring/decimal) through PostgreSQL JSONB without
	// any schema changes (SRS FR-DESIGN-04, DB Schema §6).
	VariableValue json.RawMessage `gorm:"type:jsonb;not null" json:"variable_value"`

	// UpdatedAt records when this variable row was last written.
	// GORM autoUpdateTime keeps this current without explicit assignments.
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime" json:"updated_at"`
}

// TableName overrides the default GORM table name to match the DB schema.
func (BotLifecycleVariable) TableName() string { return "bot_lifecycle_variables" }
