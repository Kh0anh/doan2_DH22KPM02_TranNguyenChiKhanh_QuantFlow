package domain

import "time"

// Bot instance status constants — matches the allowed values for
// bot_instances.status (VARCHAR 20, Database Schema §5).
// Used by BotManager, BotLogic, and the bot CRUD handlers.
const (
	// BotStatusRunning means the bot goroutine is active and processing candle events.
	BotStatusRunning = "Running"

	// BotStatusStopped means the bot was gracefully stopped (by the user or by
	// a clean shutdown). The goroutine has exited cleanly.
	BotStatusStopped = "Stopped"

	// BotStatusError means the bot goroutine exited abnormally — either due to
	// an unrecovered panic (caught by the recover() handler in manager.go) or a
	// fatal error that the bot logic could not handle. The user must investigate
	// the bot_logs table for details.
	BotStatusError = "Error"
)

// BotInstance maps to the `bot_instances` table (Database Schema §5).
//
// Each row represents one live-trade bot process. The strategy logic is pinned
// to a specific StrategyVersionID at creation time — editing the parent strategy
// after the bot starts does NOT affect the running bot (SRS FR-RUN-05).
//
// PK is UUID; GORM auto-fills it via PostgreSQL gen_random_uuid() on INSERT.
//
// Indexes:
//   - idx_bot_status on (status) — fast query of all Running bots at startup
//     restore (WBS 5.1.3, tech_stack.md §3.3).
type BotInstance struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID            string    `gorm:"type:uuid;not null"                             json:"-"`
	StrategyID        string    `gorm:"type:uuid;not null"                             json:"strategy_id"`
	StrategyVersionID string    `gorm:"type:uuid;not null"                             json:"strategy_version_id"`
	APIKeyID          string    `gorm:"type:uuid;not null"                             json:"-"`
	BotName           string    `gorm:"type:varchar(100);not null"                     json:"bot_name"`
	Symbol            string    `gorm:"type:varchar(20);not null"                      json:"symbol"`
	Status            string    `gorm:"type:varchar(20);not null;default:'Running'"    json:"status"`
	TotalPnL          string    `gorm:"type:decimal(18,8);not null;default:0"          json:"total_pnl"`
	CreatedAt         time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"                                 json:"updated_at"`
}

// TableName overrides the default GORM table name to match the DB schema.
func (BotInstance) TableName() string { return "bot_instances" }

// BotSummary is the lightweight DTO returned by GET /bots.
// Exposes only the fields needed for the bot list panel (api.yaml §BotSummary).
type BotSummary struct {
	ID        string    `json:"id"`
	BotName   string    `json:"bot_name"`
	Symbol    string    `json:"symbol"`
	Status    string    `json:"status"`
	TotalPnL  string    `json:"total_pnl"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BotDetail is the full DTO returned by GET /bots/{id}.
// Includes the strategy name resolved by a JOIN (api.yaml §BotDetail).
type BotDetail struct {
	ID                string    `json:"id"`
	BotName           string    `json:"bot_name"`
	Symbol            string    `json:"symbol"`
	Status            string    `json:"status"`
	TotalPnL          string    `json:"total_pnl"`
	StrategyID        string    `json:"strategy_id"`
	StrategyVersionID string    `json:"strategy_version_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
