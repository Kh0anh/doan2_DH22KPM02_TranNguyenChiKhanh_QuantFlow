package repository

import (
	"context"
	"errors"
	"fmt"

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
