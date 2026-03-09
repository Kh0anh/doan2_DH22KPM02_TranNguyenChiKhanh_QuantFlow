package repository

import (
	"context"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// StrategyRepository defines the data-access contract for the strategies table.
type StrategyRepository interface {
	// ListWithPagination returns a paginated, optionally filtered slice of
	// StrategySummary rows for the given user.
	//
	//   - search is matched case-insensitively (ILIKE) against strategies.name.
	//     Pass an empty string to skip the name filter.
	//   - page is 1-based; offset = (page-1) * limit.
	//   - total is the unfilitered count for the current search term, used by
	//     the caller to compute total_pages.
	ListWithPagination(ctx context.Context, userID, search string, page, limit int) ([]domain.StrategySummary, int64, error)

	// Create atomically inserts a new strategy record and its initial version
	// (version_number = 1) inside a single database transaction.
	// strategy.ID and version.ID are back-filled by PostgreSQL gen_random_uuid().
	Create(ctx context.Context, strategy *domain.Strategy, version *domain.StrategyVersion) error

	// FindByID returns the full detail of a strategy owned by the given user,
	// including its latest logic_json and the IDs of any Running bot_instances.
	// Returns (nil, nil) when the strategy does not exist or belongs to another user.
	FindByID(ctx context.Context, strategyID, userID string) (*domain.StrategyDetail, error)
}

type strategyRepository struct {
	db *gorm.DB
}

// NewStrategyRepository constructs a GORM-backed StrategyRepository.
func NewStrategyRepository(db *gorm.DB) StrategyRepository {
	return &strategyRepository{db: db}
}

// ListWithPagination executes two queries against PostgreSQL:
//
//  1. A COUNT query to determine total matching rows (for PagePagination).
//  2. A data query using a LEFT JOIN LATERAL subquery to resolve the latest
//     version_number per strategy, ordered by updated_at DESC.
//
// The LATERAL subquery leverages the existing index
// idx_strategy_versions_lookup (strategy_id, version_number DESC).
//
// ILIKE search is applied via a GORM parameterized placeholder — safe against
// SQL injection.
func (r *strategyRepository) ListWithPagination(
	ctx context.Context,
	userID, search string,
	page, limit int,
) ([]domain.StrategySummary, int64, error) {

	// --- 1. COUNT query (no ORDER BY / LIMIT for efficiency) ---------------------
	countQ := r.db.WithContext(ctx).
		Table("strategies").
		Where("user_id = ?", userID)

	if search != "" {
		countQ = countQ.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("strategy_repo: ListWithPagination: count: %w", err)
	}

	// --- 2. Data query with LATERAL join -----------------------------------------
	offset := (page - 1) * limit

	dataQ := r.db.WithContext(ctx).
		Table("strategies s").
		Select(`s.id,
			s.name,
			s.status,
			s.created_at,
			s.updated_at,
			COALESCE(sv.version_number, 0) AS version`).
		Joins(`LEFT JOIN LATERAL (
			SELECT version_number
			FROM strategy_versions
			WHERE strategy_id = s.id
			ORDER BY version_number DESC
			LIMIT 1
		) sv ON true`).
		Where("s.user_id = ?", userID)

	if search != "" {
		dataQ = dataQ.Where("s.name ILIKE ?", "%"+search+"%")
	}

	var summaries []domain.StrategySummary
	if err := dataQ.
		Order("s.updated_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&summaries).Error; err != nil {
		return nil, 0, fmt.Errorf("strategy_repo: ListWithPagination: scan: %w", err)
	}

	return summaries, total, nil
}

// Create atomically inserts a strategy and its initial version_number=1 record
// inside a single GORM transaction.
//
// Sequence:
//  1. INSERT into strategies → PostgreSQL fills strategy.ID via gen_random_uuid().
//  2. Set version.StrategyID = strategy.ID.
//  3. INSERT into strategy_versions.
//
// Both inserts are rolled back automatically if either step fails.
func (r *strategyRepository) Create(ctx context.Context, strategy *domain.Strategy, version *domain.StrategyVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(strategy).Error; err != nil {
			return fmt.Errorf("strategy_repo: Create: insert strategy: %w", err)
		}
		version.StrategyID = strategy.ID
		if err := tx.Create(version).Error; err != nil {
			return fmt.Errorf("strategy_repo: Create: insert version: %w", err)
		}
		return nil
	})
}

// FindByID fetches full strategy detail for the given (strategyID, userID) pair.
//
// Two queries are executed:
//  1. LATERAL JOIN to retrieve strategy metadata + latest version_number + logic_json.
//  2. SELECT id FROM bot_instances WHERE strategy_id=? AND user_id=? AND status='Running'
//     to collect active bot IDs for the warning banner.
//
// Returns (nil, nil) when the row does not exist or belongs to another user.
func (r *strategyRepository) FindByID(ctx context.Context, strategyID, userID string) (*domain.StrategyDetail, error) {
	// Scan directly into StrategyDetail — GORM derives column names from
	// the snake_case of each exported field name (id, name, status, version,
	// logic_json, created_at, updated_at). warning and active_bot_ids are
	// not present in the DB row; they are filled afterwards.
	var detail domain.StrategyDetail

	res := r.db.WithContext(ctx).
		Table("strategies s").
		Select(`s.id,
			s.name,
			s.status,
			s.created_at,
			s.updated_at,
			COALESCE(sv.version_number, 0) AS version,
			sv.logic_json`).
		Joins(`LEFT JOIN LATERAL (
			SELECT version_number, logic_json
			FROM strategy_versions
			WHERE strategy_id = s.id
			ORDER BY version_number DESC
			LIMIT 1
		) sv ON true`).
		Where("s.id = ? AND s.user_id = ?", strategyID, userID).
		Scan(&detail)

	if res.Error != nil {
		return nil, fmt.Errorf("strategy_repo: FindByID: scan: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}

	// --- Query 2: collect Running bot IDs linked to this strategy ---------------
	var botIDs []string
	if err := r.db.WithContext(ctx).
		Table("bot_instances").
		Select("id").
		Where("strategy_id = ? AND user_id = ? AND status = 'Running'", strategyID, userID).
		Pluck("id", &botIDs).Error; err != nil {
		return nil, fmt.Errorf("strategy_repo: FindByID: bot check: %w", err)
	}

	if len(botIDs) > 0 {
		detail.ActiveBotIDs = botIDs
	}

	return &detail, nil
}
