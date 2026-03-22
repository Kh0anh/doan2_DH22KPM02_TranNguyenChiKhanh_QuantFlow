package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// OrderResult carries the observable execution details of a completed order.
// Populated by exchange.BinanceProxy (live) or backtest.simulatedTradingProxy,
// and passed back through ExecutionContext.TradeResults so that the bot manager
// can persist them to the trade_history table (Task 2.8.5).
type OrderResult struct {
	OrderID     int64
	Symbol      string
	Side        string          // "LONG" or "SHORT"
	Quantity    decimal.Decimal // filled quantity (base asset)
	Price       decimal.Decimal // average fill price
	Fee         decimal.Decimal // commission
	RealizedPnL decimal.Decimal // realized PnL (for close / reduce-only orders)
	Status      string          // "Filled", "New", etc.
	Time        time.Time
}

// TradeHistory maps to the `trade_history` table (Database Schema §8).
//
// Lưu trữ lịch sử tất cả các lệnh giao dịch đã được thực thi trên sàn phục vụ đối soát.
//
// Index: idx_trade_history_lookup on (user_id, bot_id, symbol, executed_at DESC)
// — tối ưu cho cursor-based pagination với multi-filter (WBS 2.8.5).
type TradeHistory struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      string    `gorm:"type:uuid;not null"                             json:"-"`
	BotID       string    `gorm:"type:uuid;not null"                             json:"bot_id"`
	Symbol      string    `gorm:"type:varchar(20);not null"                      json:"symbol"`
	Side        string    `gorm:"type:varchar(10);not null"                      json:"side"`         // "Long" or "Short"
	Quantity    string    `gorm:"type:decimal(18,8);not null"                    json:"quantity"`
	FillPrice   string    `gorm:"type:decimal(18,8);not null"                    json:"fill_price"`
	Fee         string    `gorm:"type:decimal(18,8);not null"                    json:"fee"`
	RealizedPnL string    `gorm:"type:decimal(18,8);not null"                    json:"realized_pnl"`
	Status      string    `gorm:"type:varchar(20);not null"                      json:"status"`       // "Filled" or "Canceled"
	ExecutedAt  time.Time `gorm:"not null"                                       json:"executed_at"`
}

// TableName overrides the default GORM table name to match the DB schema.
func (TradeHistory) TableName() string { return "trade_history" }

// TradeRecord is the response DTO for GET /trades (api.yaml §TradeRecord).
// Includes BotName resolved via JOIN with bot_instances.
// Used by TradeRepository and returned by TradeLogic.ListTrades.
type TradeRecord struct {
	ID          string  `json:"id"`
	BotID       string  `json:"bot_id"`
	BotName     string  `json:"bot_name"`     // resolved via JOIN bot_instances
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`         // "Long" or "Short"
	Quantity    float64 `json:"quantity"`
	FillPrice   float64 `json:"fill_price"`
	Fee         float64 `json:"fee"`
	RealizedPnL float64 `json:"realized_pnl"`
	Status      string  `json:"status"`       // "Filled" or "Canceled"
	ExecutedAt  string  `json:"executed_at"`  // ISO8601 UTC

	// executedAtTime is populated internally for cursor encoding; not exported.
	executedAtTime interface{} `gorm:"-" json:"-"`
}

// ExecutedAtTime is a helper used by TradeRepository to pass the raw time.Time
// to TradeLogic for cursor encoding without exposing it in JSON.
// GORM populates this via a named column alias "executed_at_time".
type TradeRecordRaw struct {
	TradeRecord
	ExecutedAtRaw string `gorm:"column:executed_at_time" json:"-"`
}
