package domain

import "time"

// Bot log action_decision constants — written to bot_logs.action_decision.
// Defined in the domain layer so every package that produces or consumes bot
// log entries can reference them without pulling in engine or repository
// dependencies.
const (
	// BotLogActionExecuted is logged when a Session completes successfully
	// and the strategy logic reaches a trade decision (SRS FR-RUN-08).
	BotLogActionExecuted = "EXECUTED"

	// BotLogActionUnitCostExceeded is logged when a Session is terminated
	// early because the blockly Unit Cost budget was exhausted (SRS FR-RUN-07).
	BotLogActionUnitCostExceeded = "UNIT_COST_EXCEEDED"

	// BotLogActionError is logged when an unexpected runtime error terminates
	// a Session. The bot remains Running after this event (resilient design).
	BotLogActionError = "ERROR"

	// BotLogActionPanic is logged when a panic is caught by the BotManager's
	// recover() wrapper. The bot is transitioned to Error status after this event.
	BotLogActionPanic = "PANIC"
)

// BotLog is the GORM entity for the bot_logs table (Database Schema §7).
// Each row records the outcome of one strategy Session triggered during live
// trade. Records are immutable after insert — they are only ever queried and
// deleted (via CASCADE when the parent bot_instance is removed).
//
// The table uses BIGSERIAL for the primary key to support high-frequency
// writes (one row per closed candle event per running bot) without the UUID
// generation overhead.
//
// Index: idx_bot_logs_created_at ON (bot_id, created_at DESC) — used by
// cursor-based pagination in GET /bots/{botId}/logs (Task 2.7.7).
//
// Referenced by: Task 2.7.4 (insert + WS push), Task 2.7.7 (REST query).
type BotLog struct {
	// ID is the BIGSERIAL primary key auto-populated by GORM after Create().
	// The value is included in the WS push payload (websocket.md §3.2) so that
	// the frontend can deduplicate live events against REST-loaded history.
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`

	// BotID is the UUID FK referencing bot_instances.id (ON DELETE CASCADE).
	BotID string `gorm:"column:bot_id;type:uuid;not null"`

	// ActionDecision records the bot's decision outcome for this Session.
	// One of the BotLogAction* constants above (max 50 chars, schema constraint).
	ActionDecision string `gorm:"column:action_decision;type:varchar(50)"`

	// Message is a human-readable description of the session outcome or error.
	// Displayed in the Frontend Console terminal (websocket.md §3.2).
	Message string `gorm:"column:message;type:text;not null"`

	// CreatedAt is auto-set to the current UTC timestamp by GORM and PostgreSQL.
	// Used as the secondary sort key for cursor-based pagination
	// (idx_bot_logs_created_at).
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName overrides the GORM default table name derivation.
func (BotLog) TableName() string { return "bot_logs" }
