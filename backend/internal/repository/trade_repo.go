package repository

// trade_repo.go — TradeRepository: cursor-based pagination + multi-filter for trade_history.
//
// ─── Query Strategy ───────────────────────────────────────────────────────────
//
// All reads are scoped to the authenticated user_id. Optional filters are
// applied as additional WHERE clauses only when the caller provides them.
//
// Cursor pagination uses a compound cursor (executed_at, id) to produce stable
// ordering even when multiple trades share the same timestamp:
//
//	"First page":  ORDER BY executed_at DESC, id DESC LIMIT limit+1
//	"Next pages":  AND (executed_at < :ctime OR (executed_at = :ctime AND id < :cid))
//	               ORDER BY executed_at DESC, id DESC LIMIT limit+1
//
// The cursor is encoded/decoded by TradeLogic (not this layer) — the repo
// accepts a *TradeCursor struct directly.
//
// ─── Index used ───────────────────────────────────────────────────────────────
//
// idx_trade_history_lookup on (user_id, bot_id, symbol, executed_at DESC)
// satisfies the leading columns of every multi-filter query variant.
//
// Task 2.8.5 — Trade History APIs (GET /trades + GET /trades/export CSV).
// WBS: P2-Backend · 15/03/2026.
// SRS: FR-MONITOR-05, api.yaml §GET /trades, §GET /trades/export.

import (
	"context"
	"fmt"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// ─── Input types ─────────────────────────────────────────────────────────────

// TradeFilter carries all optional query filters for trade_history queries.
// Zero-value fields are ignored (no WHERE clause added for that field).
type TradeFilter struct {
	BotID     string     // empty = all bots
	Symbol    string     // empty = all symbols
	Side      string     // empty = all sides ("Long" | "Short")
	Status    string     // empty = all statuses ("Filled" | "Canceled")
	StartDate *time.Time // nil = default (7 days ago — applied by TradeLogic)
	EndDate   *time.Time // nil = default (now — applied by TradeLogic)
}

// TradeCursor is the decoded cursor struct for compound cursor pagination.
// The cursor string is encoded/decoded by TradeLogic as "<uuid>|<RFC3339>".
//
// A nil *TradeCursor means "first page" (no cursor filter applied).
type TradeCursor struct {
	ID         string    // UUID of the last record on the previous page
	ExecutedAt time.Time // ExecutedAt of the last record on the previous page
}

// ─── Interface ───────────────────────────────────────────────────────────────

// TradeRepository defines the data-access contract for the trade_history table.
// Task 2.8.5: GET /trades + GET /trades/export CSV.
type TradeRepository interface {
	// ListByFilter returns a page of trade records for the given user, filtered
	// and ordered by (executed_at DESC, id DESC). Accepts an optional cursor for
	// compound cursor pagination. Returns limit+1 records so the caller can
	// detect has_more without a count query.
	//
	// cursor nil = first page.
	ListByFilter(ctx context.Context, userID string, filter TradeFilter, limit int, cursor *TradeCursor) ([]domain.TradeRecordRaw, error)

	// ListAllByFilter returns all trade records matching the filter without
	// pagination. Used by GET /trades/export to stream the full CSV file.
	// Applies the same filter AND ordering (executed_at DESC, id DESC) as ListByFilter.
	ListAllByFilter(ctx context.Context, userID string, filter TradeFilter) ([]domain.TradeRecordRaw, error)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type tradeRepository struct {
	db *gorm.DB
}

// NewTradeRepository constructs a GORM-backed TradeRepository.
func NewTradeRepository(db *gorm.DB) TradeRepository {
	return &tradeRepository{db: db}
}

// selectCols is the shared SELECT list for all trade_history queries.
// Includes a JOIN to bot_instances for bot_name resolution.
const tradeSelectCols = `
th.id,
th.bot_id,
COALESCE(bi.bot_name, '') AS bot_name,
th.symbol,
th.side,
CAST(th.quantity    AS FLOAT8) AS quantity,
CAST(th.fill_price  AS FLOAT8) AS fill_price,
CAST(th.fee         AS FLOAT8) AS fee,
CAST(th.realized_pnl AS FLOAT8) AS realized_pnl,
th.status,
TO_CHAR(th.executed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS executed_at,
TO_CHAR(th.executed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS executed_at_time
`

// buildBase returns a pre-configured GORM query with all stable clauses applied:
// table alias, JOIN, user_id ownership, and optional filters.
// Cursor filter and LIMIT are NOT applied here — left to caller.
func (r *tradeRepository) buildBase(ctx context.Context, userID string, filter TradeFilter) *gorm.DB {
	q := r.db.WithContext(ctx).
		Table("trade_history th").
		Select(tradeSelectCols).
		Joins("LEFT JOIN bot_instances bi ON bi.id = th.bot_id").
		Where("th.user_id = ?", userID)

	if filter.BotID != "" {
		q = q.Where("th.bot_id = ?", filter.BotID)
	}
	if filter.Symbol != "" {
		q = q.Where("th.symbol = ?", filter.Symbol)
	}
	if filter.Side != "" {
		q = q.Where("th.side = ?", filter.Side)
	}
	if filter.Status != "" {
		q = q.Where("th.status = ?", filter.Status)
	}
	if filter.StartDate != nil {
		q = q.Where("th.executed_at >= ?", filter.StartDate)
	}
	if filter.EndDate != nil {
		q = q.Where("th.executed_at <= ?", filter.EndDate)
	}

	return q
}

// ListByFilter implements TradeRepository.
// Returns limit+1 rows for has_more detection.
func (r *tradeRepository) ListByFilter(
	ctx context.Context,
	userID string,
	filter TradeFilter,
	limit int,
	cursor *TradeCursor,
) ([]domain.TradeRecordRaw, error) {
	q := r.buildBase(ctx, userID, filter)

	// Apply compound cursor filter for pages after the first.
	if cursor != nil {
		q = q.Where(
			"(th.executed_at < ? OR (th.executed_at = ? AND th.id < ?))",
			cursor.ExecutedAt, cursor.ExecutedAt, cursor.ID,
		)
	}

	var rows []domain.TradeRecordRaw
	if err := q.Order("th.executed_at DESC, th.id DESC").
		Limit(limit + 1). // +1 for has_more detection
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("trade_repo: ListByFilter: %w", err)
	}
	return rows, nil
}

// ListAllByFilter implements TradeRepository.
// Returns all rows matching the filter (no LIMIT) for CSV export.
func (r *tradeRepository) ListAllByFilter(
	ctx context.Context,
	userID string,
	filter TradeFilter,
) ([]domain.TradeRecordRaw, error) {
	q := r.buildBase(ctx, userID, filter)

	var rows []domain.TradeRecordRaw
	if err := q.Order("th.executed_at DESC, th.id DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("trade_repo: ListAllByFilter: %w", err)
	}
	return rows, nil
}
