package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

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

	// Update atomically updates strategy metadata and snapshots a new version row.
	// Inside a single transaction:
	//   1. SELECT FOR UPDATE verifies ownership and locks the row.
	//   2. Computes nextVersion = MAX(version_number) + 1.
	//   3. Carries forward the latest logic_json when logicJSON is nil/empty.
	//   4. UPDATEs strategies.name / strategies.status.
	//   5. INSERTs a new strategy_versions row.
	// Returns (nil, nil) when strategyID does not exist or belongs to another user.
	Update(ctx context.Context, strategyID, userID, name, status string, logicJSON []byte) (*domain.StrategyUpdated, error)

	// DeleteByID removes a strategy owned by the given user.
	//
	// Returns:
	//   - ([]string, nil) — Running bots found; deletion blocked. Slice contains bot IDs.
	//   - (nil, ErrNotFound) — strategy not found or not owned by user.
	//   - (nil, nil) — deletion successful.
	DeleteByID(ctx context.Context, strategyID, userID string) (activeBotIDs []string, err error)

	// FindVersionByID retrieves a specific strategy_versions row by its UUID.
	// Returns (nil, nil) when the version does not exist.
	// Used by BotLogic.StartBot to reload the pinned logic_json for bot restart
	// (Task 2.7.6 — Data Integrity: use same snapshot version pinned at creation).
	FindVersionByID(ctx context.Context, versionID string) (*domain.StrategyVersion, error)
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
			sv.id AS version_id,
			sv.logic_json`).
		Joins(`LEFT JOIN LATERAL (
			SELECT id, version_number, logic_json
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

// Update atomically updates strategy metadata and appends a new snapshot version.
//
// Transaction sequence:
//  1. SELECT id, name, status FROM strategies WHERE id=? AND user_id=? FOR UPDATE
//     — verifies ownership and takes a row-level lock.
//  2. SELECT COALESCE(MAX(version_number),0)+1 FROM strategy_versions — next ver.
//  3. If logicJSON is empty, carry forward the latest logic_json from DB.
//  4. UPDATE strategies SET name=?, status=?, updated_at=NOW().
//  5. INSERT INTO strategy_versions (version_number=nextVer, logic_json=?, status=?).
//
// Returns (nil, nil) when the strategy does not exist or belongs to another user.
func (r *strategyRepository) Update(
	ctx context.Context,
	strategyID, userID, name, status string,
	logicJSON []byte,
) (*domain.StrategyUpdated, error) {
	var result *domain.StrategyUpdated

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 1: Verify ownership with row lock.
		var current struct {
			ID     string `gorm:"column:id"`
			Name   string `gorm:"column:name"`
			Status string `gorm:"column:status"`
		}
		res := tx.Raw(
			"SELECT id, name, status FROM strategies WHERE id = ? AND user_id = ? FOR UPDATE",
			strategyID, userID,
		).Scan(&current)
		if res.Error != nil {
			return fmt.Errorf("strategy_repo: Update: lock: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			// Not found or wrong owner — signal to caller via nil result.
			return nil
		}

		// Apply field defaults: keep existing values when caller omits them.
		if name == "" {
			name = current.Name
		}
		if status == "" {
			status = current.Status
		}

		// Step 2: Compute next version number.
		var nextVersion int
		if err := tx.Raw(
			"SELECT COALESCE(MAX(version_number), 0) + 1 FROM strategy_versions WHERE strategy_id = ?",
			strategyID,
		).Scan(&nextVersion).Error; err != nil {
			return fmt.Errorf("strategy_repo: Update: next version: %w", err)
		}

		// Step 3: Carry forward existing logic_json when none was provided.
		if len(logicJSON) == 0 {
			var existing []byte
			if err := tx.Raw(
				"SELECT logic_json FROM strategy_versions WHERE strategy_id = ? ORDER BY version_number DESC LIMIT 1",
				strategyID,
			).Scan(&existing).Error; err != nil {
				return fmt.Errorf("strategy_repo: Update: fetch logic_json: %w", err)
			}
			logicJSON = existing
		}

		// Step 4: Update strategy metadata (updated_at auto-set by GORM autoUpdateTime).
		if err := tx.Exec(
			"UPDATE strategies SET name = ?, status = ?, updated_at = NOW() WHERE id = ?",
			name, status, strategyID,
		).Error; err != nil {
			return fmt.Errorf("strategy_repo: Update: update strategy: %w", err)
		}

		// Step 5: Insert new version snapshot.
		newVersion := &domain.StrategyVersion{
			StrategyID:    strategyID,
			VersionNumber: nextVersion,
			LogicJSON:     logicJSON,
			Status:        status,
		}
		if err := tx.Create(newVersion).Error; err != nil {
			return fmt.Errorf("strategy_repo: Update: insert version: %w", err)
		}

		// Fetch updated_at from DB to return the server-assigned timestamp.
		var updatedAt struct {
			UpdatedAt string `gorm:"column:updated_at"`
		}
		_ = tx.Raw("SELECT updated_at FROM strategies WHERE id = ?", strategyID).Scan(&updatedAt)

		result = &domain.StrategyUpdated{
			ID:      strategyID,
			Name:    name,
			Version: nextVersion,
			Status:  status,
		}
		// Parse updated_at — fall back to zero value on parse error (non-fatal).
		if t, err := parseUpdatedAt(updatedAt.UpdatedAt); err == nil {
			result.UpdatedAt = t
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// parseUpdatedAt parses the PostgreSQL timestamptz string returned by a raw scan.
// Tries RFC3339Nano first, then the common Postgres layout.
func parseUpdatedAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05Z07:00",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as timestamp", s)
}

// DeleteByID removes a strategy owned by the given user.
//
//  1. Check for Running bot_instances linked to this strategy — block if found.
//  2. DELETE FROM strategies WHERE id=? AND user_id=? (CASCADE removes versions).
//  3. If RowsAffected == 0, strategy was not found or belonged to another user.
//
// Return semantics:
//   - ([]string, nil)  — Running bots exist; deletion blocked. Slice = their IDs.
//   - (nil, errNotFound) — strategy not found / wrong owner (sentinel via fmt.Errorf).
//   - (nil, nil)       — deleted successfully.
func (r *strategyRepository) DeleteByID(ctx context.Context, strategyID, userID string) ([]string, error) {
	// Step 1: Check for Running bots.
	var botIDs []string
	if err := r.db.WithContext(ctx).
		Table("bot_instances").
		Select("id").
		Where("strategy_id = ? AND user_id = ? AND status = 'Running'", strategyID, userID).
		Pluck("id", &botIDs).Error; err != nil {
		return nil, fmt.Errorf("strategy_repo: DeleteByID: bot check: %w", err)
	}
	if len(botIDs) > 0 {
		return botIDs, nil
	}

	// Step 2: DELETE with ownership guard.
	res := r.db.WithContext(ctx).
		Exec("DELETE FROM strategies WHERE id = ? AND user_id = ?", strategyID, userID)
	if res.Error != nil {
		return nil, fmt.Errorf("strategy_repo: DeleteByID: delete: %w", res.Error)
	}

	// Step 3: RowsAffected == 0 means not found or wrong owner.
	if res.RowsAffected == 0 {
		return nil, errStrategyNotFoundRepo
	}

	return nil, nil
}

// errStrategyNotFoundRepo is a package-private sentinel used by DeleteByID
// to signal "not found" without depending on the logic layer.
var errStrategyNotFoundRepo = fmt.Errorf("strategy not found")

// FindVersionByID retrieves a specific strategy_versions row by its UUID.
// Returns (nil, nil) when the version does not exist.
//
// Used by BotLogic.StartBot to reload the pinned logic_json for the exact
// strategy version snapshotted at bot creation time, ensuring Data Integrity
// on bot restart (Task 2.7.6, api.yaml §POST /bots/{id}/start).
func (r *strategyRepository) FindVersionByID(ctx context.Context, versionID string) (*domain.StrategyVersion, error) {
	var sv domain.StrategyVersion
	err := r.db.WithContext(ctx).
		Where("id = ?", versionID).
		First(&sv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("strategy_repo: FindVersionByID: %w", err)
	}
	return &sv, nil
}
