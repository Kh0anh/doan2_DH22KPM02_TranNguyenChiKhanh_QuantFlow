package domain

import (
	"encoding/json"
	"time"
)

// Strategy status constants — matches strategies.status (VARCHAR 20).
const (
	StrategyStatusDraft    = "Draft"
	StrategyStatusValid    = "Valid"
	StrategyStatusArchived = "Archived"
)

// Strategy maps to the `strategies` table (Database Schema §3).
// It holds only the metadata of a strategy; the actual Blockly logic JSON
// is versioned and stored in `strategy_versions` (Schema §4).
type Strategy struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    string    `gorm:"type:uuid;not null"                             json:"-"`
	Name      string    `gorm:"type:varchar(100);not null"                     json:"name"`
	Status    string    `gorm:"type:varchar(20);not null;default:'Draft'"      json:"status"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"                                 json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (Strategy) TableName() string { return "strategies" }

// StrategyVersion maps to the `strategy_versions` table (Database Schema §4).
// Each save of a strategy creates a new version row (immutable snapshot).
// Bots pin to a specific version at creation time — editing the parent
// strategy never affects running bots.
type StrategyVersion struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	StrategyID    string    `gorm:"type:uuid;not null"                             json:"strategy_id"`
	VersionNumber int       `gorm:"not null"                                       json:"version_number"`
	LogicJSON     []byte    `gorm:"type:jsonb;not null"                            json:"logic_json"`
	Status        string    `gorm:"type:varchar(20);not null;default:'Draft'"      json:"status"`
	CreatedAt     time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

// TableName overrides the default GORM table name.
func (StrategyVersion) TableName() string { return "strategy_versions" }

// StrategySummary is the read-only DTO returned by GET /strategies.
// It combines columns from `strategies` and the latest `version_number`
// resolved via a lateral subquery in the repository layer (api.yaml §StrategySummary).
type StrategySummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StrategyCreated is the response DTO returned by POST /strategies on 201 Created.
// Matches api.yaml §StrategyCreated — version is always 1 for a freshly created strategy.
type StrategyCreated struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// StrategyDetail is the response DTO returned by GET /strategies/{id}.
// LogicJSON carries the raw Blockly JSON bytes (api.yaml §BlocklyLogicJson).
// Warning and ActiveBotIDs are omitted from the JSON output when nil/empty —
// they are populated only when at least one bot_instance with status=Running
// references this strategy (WBS 2.3.3, api.yaml §StrategyDetail).
// VersionID is the UUID of the strategy_versions record; not exposed to client
// but used by BotLogic to snapshot strategy_version_id (Task 2.7.5).
type StrategyDetail struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      int             `json:"version"`
	VersionID    string          `json:"-"` // UUID, not exposed in API response
	Status       string          `json:"status"`
	LogicJSON    json.RawMessage `json:"logic_json"`
	Warning      *string         `json:"warning,omitempty"`
	ActiveBotIDs []string        `json:"active_bot_ids,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// StrategyUpdated is the response DTO returned by PUT /strategies/{id} on 200 OK.
// Warning is omitted when no Running bots reference this strategy
// (WBS 2.3.4, api.yaml §StrategyUpdated).
type StrategyUpdated struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	Warning   *string   `json:"warning,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StrategyExport is the response DTO returned by GET /strategies/{id}/export.
// Carried with a Content-Disposition: attachment header so the browser triggers
// a file download (api.yaml §StrategyExport, SRS FR-DESIGN-12, WBS 2.3.7).
type StrategyExport struct {
	Name       string          `json:"name"`
	LogicJSON  json.RawMessage `json:"logic_json"`
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
}
