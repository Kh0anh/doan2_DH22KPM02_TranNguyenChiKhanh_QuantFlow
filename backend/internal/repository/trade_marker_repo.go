package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// TradeMarkerRepository provides read access to trade execution points that
// are overlaid on the candle chart for GET /market/candles (WBS 2.4.4,
// api.yaml §TradeMarker).
//
// It queries the trade_history table joined with bot_instances to resolve the
// bot_name field. The interface is intentionally minimal — a single read method —
// because full trade history CRUD is implemented in WBS 2.8.5 (trade_repo.go).
type TradeMarkerRepository interface {
	// FindMarkersBySymbolAndTimeRange returns all trade executions for the given
	// symbol whose executed_at falls within [start, end] (both inclusive).
	//
	// Returns an empty slice (not an error) when the trade_history table contains
	// no matching rows — this is the normal state before any Bot has traded.
	FindMarkersBySymbolAndTimeRange(
		ctx context.Context,
		symbol string,
		start, end time.Time,
	) ([]domain.TradeMarker, error)
}

// tradeMarkerRow is a flat scan target for the JOIN query below.
// It avoids dependency on a full TradeHistory domain entity (WBS 2.8.5).
type tradeMarkerRow struct {
	ExecutedAt time.Time
	FillPrice  string
	Side       string
	BotID      string
	BotName    string
}

type tradeMarkerRepository struct {
	db *gorm.DB
}

// NewTradeMarkerRepository constructs a GORM-backed TradeMarkerRepository.
func NewTradeMarkerRepository(db *gorm.DB) TradeMarkerRepository {
	return &tradeMarkerRepository{db: db}
}

// FindMarkersBySymbolAndTimeRange executes a JOIN between trade_history and
// bot_instances to produce the marker data required by the candle chart.
//
// SQL equivalent:
//
//	SELECT th.executed_at, th.fill_price, th.side, th.bot_id, bi.bot_name
//	FROM   trade_history th
//	JOIN   bot_instances bi ON bi.id = th.bot_id
//	WHERE  th.symbol = ?
//	  AND  th.executed_at >= ?
//	  AND  th.executed_at <= ?
//	ORDER  BY th.executed_at ASC
//
// Index hint: idx_trade_history_lookup (user_id, bot_id, symbol, executed_at DESC)
// will be used for the symbol + executed_at predicate (DB Schema §Performance Index 2).
func (r *tradeMarkerRepository) FindMarkersBySymbolAndTimeRange(
	ctx context.Context,
	symbol string,
	start, end time.Time,
) ([]domain.TradeMarker, error) {
	var rows []tradeMarkerRow

	err := r.db.WithContext(ctx).
		Table("trade_history th").
		Select("th.executed_at, th.fill_price, th.side, th.bot_id, bi.bot_name").
		Joins("JOIN bot_instances bi ON bi.id = th.bot_id").
		Where("th.symbol = ? AND th.executed_at >= ? AND th.executed_at <= ?", symbol, start, end).
		Order("th.executed_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("trade_marker_repo: FindMarkersBySymbolAndTimeRange(%s): %w", symbol, err)
	}

	markers := make([]domain.TradeMarker, 0, len(rows))
	for _, row := range rows {
		price := parseDecimalToFloat(row.FillPrice)
		markers = append(markers, domain.TradeMarker{
			Time:    row.ExecutedAt,
			Price:   price,
			Side:    row.Side,
			BotName: row.BotName,
			BotID:   row.BotID,
		})
	}
	return markers, nil
}

// parseDecimalToFloat converts a PostgreSQL DECIMAL string (e.g. "64500.12345678")
// to float64. Returns 0 on parse error — a malformed price is non-fatal for a
// chart marker and will simply render at y=0 rather than crashing the endpoint.
func parseDecimalToFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
