package repository

// bot_log_repo.go implements BotLogRepository — the data-access layer for the
// bot_logs table (Database Schema §7).
//
// Task 2.7.4 requires Insert only. ListByBotID is pre-declared in the interface
// here for Task 2.7.7 (GET /bots/{botId}/logs with cursor-based pagination) to
// avoid a second interface-breaking change at that point.
//
// Task 2.7.4 — Bot Logging and Error Handling.
// WBS: P2-Backend · 14/03/2026
// SRS: FR-RUN-08, FR-MONITOR-03

import (
	"context"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// BotLogRepository defines the data-access contract for the bot_logs table
// (DB Schema §7, WBS 2.7.4).
//
// The table is append-only (no UPDATE operations). Rows are removed by
// the PostgreSQL CASCADE constraint when the parent bot_instance is deleted.
type BotLogRepository interface {
	// Insert persists one BotLog row and writes back the generated BIGSERIAL
	// auto-increment ID into log.ID. The ID is required immediately after the
	// insert so BotLogger can include it in the WS push payload (websocket.md §3.2).
	Insert(ctx context.Context, log *domain.BotLog) error

	// ListByBotID returns at most limit rows for the given bot, ordered by
	// created_at DESC. When cursor > 0, only rows with id < cursor are returned
	// (exclusive upper-bound), enabling stable forward-only cursor pagination.
	// The caller (Task 2.7.7) maps results to the REST response DTO.
	ListByBotID(ctx context.Context, botID string, limit int, cursor int64) ([]*domain.BotLog, error)
}

type botLogRepository struct {
	db *gorm.DB
}

// NewBotLogRepository constructs a GORM-backed BotLogRepository.
func NewBotLogRepository(db *gorm.DB) BotLogRepository {
	return &botLogRepository{db: db}
}

// Insert persists log and populates log.ID with the generated BIGSERIAL value.
// GORM's Create() reads back the generated id via the RETURNING clause (pgx driver).
func (r *botLogRepository) Insert(ctx context.Context, log *domain.BotLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("botLogRepository.Insert (bot_id=%s): %w", log.BotID, err)
	}
	return nil
}

// ListByBotID returns cursor-paginated bot log rows in descending time order.
// Rows are sorted by (created_at DESC, id DESC) to produce a stable ordering
// when multiple rows share the same timestamp (high-frequency bots).
func (r *botLogRepository) ListByBotID(ctx context.Context, botID string, limit int, cursor int64) ([]*domain.BotLog, error) {
	var logs []*domain.BotLog

	q := r.db.WithContext(ctx).
		Where("bot_id = ?", botID).
		Order("created_at DESC, id DESC").
		Limit(limit)

	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}

	if err := q.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("botLogRepository.ListByBotID (bot_id=%s): %w", botID, err)
	}
	return logs, nil
}
