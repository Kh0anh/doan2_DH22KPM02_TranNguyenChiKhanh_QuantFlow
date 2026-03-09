package domain

import "time"

// Candle OHLCV interval constants — matches the set of timeframes supported by
// Binance Futures and stored in candles_data.interval (DB Schema §9).
const (
	CandleInterval1m  = "1m"
	CandleInterval5m  = "5m"
	CandleInterval15m = "15m"
	CandleInterval1h  = "1h"
	CandleInterval4h  = "4h"
	CandleInterval1d  = "1d"
)

// Candle maps to the `candles_data` table (Database Schema §9).
//
// Primary key is a BIGSERIAL auto-increment — chosen for high-write INSERT
// throughput over UUID (WBS 2.4.1, SRS FR-CORE-03).
//
// The composite UNIQUE constraint on (symbol, interval, open_time) ensures
// idempotent upserts via ON CONFLICT DO NOTHING (DB Schema §9, SRS FR-CORE-02).
//
// Index: idx_candles_symbol_interval_time on (symbol, interval, open_time DESC)
// optimises two critical read paths:
//   - REST fallback guard: db.First() for latest candle per (symbol, interval)
//   - Backtest scan: sequential full-range read of 35K+ rows (NFR-PERF-02)
type Candle struct {
	// ID is the BIGSERIAL primary key — auto-assigned by PostgreSQL.
	ID int64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// Symbol is the Binance Futures trading pair, e.g. "BTCUSDT".
	Symbol string `gorm:"type:varchar(20);not null;uniqueIndex:idx_candles_symbol_interval_time,compositeindex:3" json:"symbol"`

	// Interval is the candle timeframe: "1m", "5m", "15m", "1h", "4h", "1d".
	Interval string `gorm:"type:varchar(10);not null;uniqueIndex:idx_candles_symbol_interval_time,compositeindex:2" json:"interval"`

	// OpenTime is the candle open timestamp (UTC). Part of the UNIQUE composite key.
	OpenTime time.Time `gorm:"not null;uniqueIndex:idx_candles_symbol_interval_time,compositeindex:1" json:"open_time"`

	// OpenPrice is the candle open price (DECIMAL 18,8).
	OpenPrice string `gorm:"type:decimal(18,8);not null" json:"open_price"`

	// HighPrice is the highest price reached during the candle period.
	HighPrice string `gorm:"type:decimal(18,8);not null" json:"high_price"`

	// LowPrice is the lowest price reached during the candle period.
	LowPrice string `gorm:"type:decimal(18,8);not null" json:"low_price"`

	// ClosePrice is the candle close price (DECIMAL 18,8).
	ClosePrice string `gorm:"type:decimal(18,8);not null" json:"close_price"`

	// Volume is the total traded volume during the candle period.
	Volume string `gorm:"type:decimal(18,8);not null" json:"volume"`

	// IsClosed indicates whether this candle has fully closed on Binance.
	// Real-time WS events with IsClosed=false are NOT persisted to DB —
	// only final closed candles are inserted (SRS FR-CORE-02, WBS 2.8.2).
	IsClosed bool `gorm:"not null;default:false" json:"is_closed"`
}

// TableName overrides the default GORM table name to match the DB schema.
func (Candle) TableName() string { return "candles_data" }
