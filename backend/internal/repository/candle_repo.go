package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CandleRepository defines the data-access contract for the candles_data table.
// Used by KlineSyncService (WBS 2.4.1), GapFillerWorker (WBS 2.4.2), and the
// Backtest simulator (WBS 2.6.1).
type CandleRepository interface {
	// FindLatest returns the most recently open_time candle for the given
	// (symbol, interval) pair, or (nil, nil) when no record exists.
	//
	// This is the db.First() guard described in SRS FR-CORE-02 and WBS 2.4.1:
	//   - Result found  → skip REST API call (save Binance API weight).
	//   - ErrRecordNotFound → caller must trigger REST fallback.
	FindLatest(ctx context.Context, symbol, interval string) (*domain.Candle, error)

	// InsertOne persists a single candle with ON CONFLICT DO NOTHING semantics.
	//
	// The UNIQUE constraint on (symbol, interval, open_time) guarantees
	// idempotency — duplicate inserts from concurrent WS events or retry paths
	// are silently dropped without error (DB Schema §9, SRS FR-CORE-02).
	InsertOne(ctx context.Context, candle *domain.Candle) error

	// InsertBatch persists multiple candles in a single transaction using GORM
	// CreateInBatches (1000 rows per batch) with ON CONFLICT DO NOTHING.
	//
	// This is the primary write path for the GapFillerWorker (WBS 2.4.2) which
	// fetches missing candle ranges from Binance REST and bulk-inserts them.
	// The batch size of 1000 balances PostgreSQL round-trip overhead against
	// per-statement parameter limits. Duplicate candles that already exist
	// (e.g., inserted by the WS stream between gap detection and fill) are
	// silently skipped via the UNIQUE constraint on (symbol, interval, open_time).
	InsertBatch(ctx context.Context, candles []domain.Candle) error

	// QueryCandles fetches candles for a (symbol, interval) pair within an
	// optional time range, ordered by open_time ASC, capped at limit rows.
	//
	// This is the primary read path for GET /market/candles (WBS 2.4.4).
	// The query leverages the composite index idx_candles_symbol_interval_time
	// on (symbol, interval, open_time DESC) for efficient range scans.
	//
	// Parameters:
	//   - start  — inclusive lower bound on open_time; nil = no lower bound.
	//   - end    — inclusive upper bound on open_time; nil = no upper bound.
	//   - limit  — maximum number of rows to return (1–1500).
	QueryCandles(ctx context.Context, symbol, interval string, start, end *time.Time, limit int) ([]domain.Candle, error)
}

type candleRepository struct {
	db *gorm.DB
}

// NewCandleRepository constructs a GORM-backed CandleRepository.
func NewCandleRepository(db *gorm.DB) CandleRepository {
	return &candleRepository{db: db}
}

// FindLatest executes a db.First() query leveraging the composite index
// idx_candles_symbol_interval_time on (symbol, interval, open_time DESC).
//
// Returns:
//   - (*Candle, nil)  — record found.
//   - (nil, nil)      — no matching record (gorm.ErrRecordNotFound translated).
//   - (nil, error)    — unexpected database error.
func (r *candleRepository) FindLatest(ctx context.Context, symbol, interval string) (*domain.Candle, error) {
	var candle domain.Candle
	err := r.db.WithContext(ctx).
		Where("symbol = ? AND interval = ?", symbol, interval).
		Order("open_time DESC").
		First(&candle).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Translate to (nil, nil) — callers use nil-check, not error-type check,
			// to decide whether to trigger the REST fallback (WBS 2.4.1 notes).
			return nil, nil
		}
		return nil, fmt.Errorf("candle_repo: FindLatest(%s, %s): %w", symbol, interval, err)
	}

	return &candle, nil
}

// InsertOne persists a single closed candle to the candles_data table using
// PostgreSQL's ON CONFLICT DO NOTHING clause.
//
// This guarantees idempotency across concurrent goroutines (e.g., two WS
// events or a WS event racing with a REST fallback batch for the same candle).
// Duplicate inserts are silently dropped — no error is returned.
func (r *candleRepository) InsertOne(ctx context.Context, candle *domain.Candle) error {
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(candle).Error
	if err != nil {
		return fmt.Errorf("candle_repo: InsertOne(%s, %s, %v): %w",
			candle.Symbol, candle.Interval, candle.OpenTime, err)
	}
	return nil
}

// InsertBatch performs a bulk insert of candles using GORM CreateInBatches
// with a batch size of 1000 rows and PostgreSQL ON CONFLICT DO NOTHING.
//
// The 1000-row batch size is chosen to stay well within PostgreSQL's default
// max_parameters limit (65535) while minimising round-trip overhead for the
// GapFillerWorker's large backfill payloads (WBS 2.4.2). Each batch is
// committed in a single INSERT statement — partial failures within a batch
// are not possible because ON CONFLICT DO NOTHING handles duplicates at the
// constraint level, not at the application level.
const insertBatchSize = 1000

func (r *candleRepository) InsertBatch(ctx context.Context, candles []domain.Candle) error {
	if len(candles) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&candles, insertBatchSize).Error
	if err != nil {
		return fmt.Errorf("candle_repo: InsertBatch(%d candles): %w", len(candles), err)
	}
	return nil
}

// QueryCandles retrieves up to limit closed candles for the given (symbol,
// interval) pair, optionally filtered by a time range, ordered by open_time ASC.
//
// The query hits the idx_candles_symbol_interval_time composite index
// (symbol, interval, open_time DESC). PostgreSQL will use an index scan for the
// WHERE clause then sort the result set; for typical UI limits (≤1500 rows) this
// is well within the < 500 ms NFR-PERF target (SRS §3.1).
func (r *candleRepository) QueryCandles(
	ctx context.Context,
	symbol, interval string,
	start, end *time.Time,
	limit int,
) ([]domain.Candle, error) {
	q := r.db.WithContext(ctx).
		Where("symbol = ? AND interval = ?", symbol, interval)

	if start != nil {
		q = q.Where("open_time >= ?", *start)
	}
	if end != nil {
		q = q.Where("open_time <= ?", *end)
	}

	var candles []domain.Candle
	err := q.
		Order("open_time ASC").
		Limit(limit).
		Find(&candles).Error
	if err != nil {
		return nil, fmt.Errorf("candle_repo: QueryCandles(%s, %s): %w", symbol, interval, err)
	}
	return candles, nil
}
