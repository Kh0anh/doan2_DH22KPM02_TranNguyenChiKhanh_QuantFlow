package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// BotRepository defines the data-access contract for the bot_instances table.
// Task 2.7.5: Bot CRUD APIs — GET/POST/DELETE /bots, GET /bots/{id}.
type BotRepository interface {
	// ListByUserID returns all bot instances owned by the given user.
	// When statusFilter is non-empty, only bots with matching status are returned.
	// Results are ordered by created_at DESC (most recently created first).
	// The query JOINs strategies and strategy_versions tables to resolve
	// strategy_name and version_number for the BotSummary DTO.
	ListByUserID(ctx context.Context, userID, statusFilter string) ([]domain.BotSummary, error)

	// Create inserts a new bot_instances record.
	// bot.ID is auto-filled by PostgreSQL gen_random_uuid() on INSERT.
	// Initial status should be set by the caller (typically Running for POST /bots).
	Create(ctx context.Context, bot *domain.BotInstance) error

	// FindByID retrieves the full detail of a bot owned by the given user.
	// Returns (nil, nil) when the bot does not exist or belongs to another user.
	// The query JOINs strategies and strategy_versions tables to resolve
	// strategy_name and version_number for the BotDetail DTO.
	FindByID(ctx context.Context, botID, userID string) (*domain.BotDetail, error)

	// UpdateStatus atomically updates the status field of a bot.
	// Used by BotManager to transition Running → Stopped / Error after
	// the goroutine exits (Task 2.7.1).
	UpdateStatus(ctx context.Context, botID, newStatus string) error

	// DeleteByID removes a bot owned by the given user.
	// Returns ErrBotStillRunning if the bot status is Running (409 constraint).
	// Returns ErrNotFound when the bot does not exist or belongs to another user.
	DeleteByID(ctx context.Context, botID, userID string) error

	// FindRawByID retrieves the full BotInstance row (including StrategyVersionID
	// and APIKeyID) for the given botID and userID.
	// Returns (nil, nil) when the bot does not exist or belongs to another user.
	// Used by BotLogic.StartBot and StopBot to rebuild bot config from the
	// pinned strategy version (Task 2.7.6).
	FindRawByID(ctx context.Context, botID, userID string) (*domain.BotInstance, error)

	// FindRunningByIDs returns the BotInstance rows for the given botIDs that
	// currently have status=Running. Used by BotLogic.GetRunningBotsSnapshot()
	// (Task 2.8.4) to bulk-fetch metadata for the position_update polling loop.
	// Returns an empty slice (not an error) when none of the IDs are found.
	FindRunningByIDs(ctx context.Context, botIDs []string) ([]*domain.BotInstance, error)
}

// Sentinel errors returned by BotRepository methods.
var (
	// ErrBotStillRunning is returned by DeleteByID when attempting to delete
	// a bot with status=Running (api.yaml §DELETE /bots/{id} 409 constraint).
	ErrBotStillRunning = errors.New("bot_repo: bot still running")

	// ErrNotFound is returned by DeleteByID when the bot does not exist or
	// belongs to another user (403/404 are indistinguishable for security).
	ErrNotFound = errors.New("bot_repo: not found")
)

type botRepository struct {
	db *gorm.DB
}

// NewBotRepository constructs a GORM-backed BotRepository.
func NewBotRepository(db *gorm.DB) BotRepository {
	return &botRepository{db: db}
}

// ListByUserID executes a LEFT JOIN query to resolve strategy_name and
// version_number for each bot. The query leverages the existing index
// idx_bot_status on (status) when statusFilter is non-empty.
//
// SQL Pattern:
//
//	SELECT bi.id, bi.bot_name, bi.symbol, bi.status, bi.total_pnl,
//	       bi.created_at, bi.updated_at,
//	       s.id AS strategy_id, s.name AS strategy_name,
//	       sv.version_number
//	FROM bot_instances bi
//	LEFT JOIN strategies s ON s.id = bi.strategy_id
//	LEFT JOIN strategy_versions sv ON sv.id = bi.strategy_version_id
//	WHERE bi.user_id = ? [AND bi.status = ?]
//	ORDER BY bi.created_at DESC
func (r *botRepository) ListByUserID(ctx context.Context, userID, statusFilter string) ([]domain.BotSummary, error) {
	query := r.db.WithContext(ctx).
		Table("bot_instances bi").
		Select(`bi.id,
			bi.bot_name,
			bi.symbol,
			bi.status,
			bi.total_pnl,
			bi.created_at,
			bi.updated_at,
			s.id AS strategy_id,
			s.name AS strategy_name,
			sv.version_number AS strategy_version`).
		Joins("LEFT JOIN strategies s ON s.id = bi.strategy_id").
		Joins("LEFT JOIN strategy_versions sv ON sv.id = bi.strategy_version_id").
		Where("bi.user_id = ?", userID)

	if statusFilter != "" {
		query = query.Where("bi.status = ?", statusFilter)
	}

	var summaries []domain.BotSummary
	if err := query.Order("bi.created_at DESC").Scan(&summaries).Error; err != nil {
		return nil, fmt.Errorf("bot_repo: ListByUserID: %w", err)
	}

	// Ensure non-nil slice so JSON response encodes as [] not null.
	if summaries == nil {
		summaries = []domain.BotSummary{}
	}

	return summaries, nil
}

// Create inserts a new bot_instances record.
// bot.ID is back-filled by PostgreSQL gen_random_uuid() via the GORM default tag.
func (r *botRepository) Create(ctx context.Context, bot *domain.BotInstance) error {
	if err := r.db.WithContext(ctx).Create(bot).Error; err != nil {
		return fmt.Errorf("bot_repo: Create: %w", err)
	}
	return nil
}

// FindByID retrieves the full BotDetail for the given botID and userID.
// Returns (nil, nil) when the bot does not exist or belongs to another user.
//
// SQL Pattern:
//
//	SELECT bi.id, bi.bot_name, bi.symbol, bi.status, bi.total_pnl,
//	       bi.created_at, bi.updated_at,
//	       s.id AS strategy_id, s.name AS strategy_name,
//	       sv.version_number
//	FROM bot_instances bi
//	LEFT JOIN strategies s ON s.id = bi.strategy_id
//	LEFT JOIN strategy_versions sv ON sv.id = bi.strategy_version_id
//	WHERE bi.id = ? AND bi.user_id = ?
//
// Note: Position and OpenOrders are NOT resolved here — they are fetched
// real-time from Binance API by BotLogic.GetBotDetail() (api.yaml spec).
func (r *botRepository) FindByID(ctx context.Context, botID, userID string) (*domain.BotDetail, error) {
	var detail domain.BotDetail
	err := r.db.WithContext(ctx).
		Table("bot_instances bi").
		Select(`bi.id,
			bi.bot_name,
			bi.symbol,
			bi.status,
			bi.total_pnl,
			bi.created_at,
			bi.updated_at,
			s.id AS strategy_id,
			s.name AS strategy_name,
			sv.version_number AS strategy_version`).
		Joins("LEFT JOIN strategies s ON s.id = bi.strategy_id").
		Joins("LEFT JOIN strategy_versions sv ON sv.id = bi.strategy_version_id").
		Where("bi.id = ? AND bi.user_id = ?", botID, userID).
		Scan(&detail).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("bot_repo: FindByID: %w", err)
	}

	// GORM Scan into a struct with zero values returns no error but leaves
	// ID empty when no rows match. Check ID to distinguish not-found from zero values.
	if detail.ID == "" {
		return nil, nil
	}

	return &detail, nil
}

// UpdateStatus atomically updates the status column for the given botID.
// Used by BotManager to persist state transitions (Running → Stopped / Error).
// Does not check user_id ownership — the caller (BotManager) already knows
// the bot is owned by the correct user when it was started.
func (r *botRepository) UpdateStatus(ctx context.Context, botID, newStatus string) error {
	result := r.db.WithContext(ctx).
		Model(&domain.BotInstance{}).
		Where("id = ?", botID).
		Update("status", newStatus)

	if result.Error != nil {
		return fmt.Errorf("bot_repo: UpdateStatus: %w", result.Error)
	}
	return nil
}

// DeleteByID removes a bot owned by the given user.
//
// Business rules enforced:
//   - The bot must have status != Running (api.yaml §DELETE /bots/{id} 409).
//   - The bot must belong to the authenticated user (ownership check).
//
// Returns:
//   - ErrBotStillRunning when status=Running (handler maps to 409).
//   - ErrNotFound when bot does not exist or belongs to another user (handler maps to 404).
//   - nil on successful deletion.
func (r *botRepository) DeleteByID(ctx context.Context, botID, userID string) error {
	// Step 1: Load the bot to check ownership and status.
	var bot domain.BotInstance
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", botID, userID).
		First(&bot).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("bot_repo: DeleteByID: query: %w", err)
	}

	// Step 2: Enforce status constraint (Running bots cannot be deleted).
	if bot.Status == domain.BotStatusRunning {
		return ErrBotStillRunning
	}

	// Step 3: DELETE the bot. Cascade DELETE on bot_logs and bot_lifecycle_variables
	// is handled by the database schema (ON DELETE CASCADE foreign key constraints).
	if err := r.db.WithContext(ctx).Delete(&bot).Error; err != nil {
		return fmt.Errorf("bot_repo: DeleteByID: delete: %w", err)
	}

	return nil
}

// FindRawByID retrieves the full BotInstance row (all columns, including
// StrategyVersionID and APIKeyID) for the given botID and userID.
// Returns (nil, nil) when the bot does not exist or belongs to another user.
//
// Used by BotLogic.StartBot and BotLogic.StopBot to rebuild the bot goroutine
// config from the pinned strategy version (Task 2.7.6, Data Integrity).
func (r *botRepository) FindRawByID(ctx context.Context, botID, userID string) (*domain.BotInstance, error) {
	var bot domain.BotInstance
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", botID, userID).
		First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("bot_repo: FindRawByID: %w", err)
	}
	return &bot, nil
}

// FindRunningByIDs returns the BotInstance rows for the given botIDs.
// Only rows with status=Running are returned — IDs that no longer match
// (race condition: bot stopped between GetRunningBotIDs and this query) are
// silently omitted.
//
// Returns an empty slice (not an error) when botIDs is empty or no rows match.
// The result order is unspecified (consistent with BotManager.GetRunningBotIDs).
//
// Task 2.8.4 — position_update channel (GetRunningBotsSnapshot bulk fetch).
func (r *botRepository) FindRunningByIDs(ctx context.Context, botIDs []string) ([]*domain.BotInstance, error) {
	if len(botIDs) == 0 {
		return []*domain.BotInstance{}, nil
	}

	var bots []*domain.BotInstance
	if err := r.db.WithContext(ctx).
		Where("id IN ? AND status = ?", botIDs, domain.BotStatusRunning).
		Find(&bots).Error; err != nil {
		return nil, fmt.Errorf("bot_repo: FindRunningByIDs: %w", err)
	}
	return bots, nil
}
