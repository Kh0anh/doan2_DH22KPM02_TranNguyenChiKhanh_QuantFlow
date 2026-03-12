// Package logic — Task 2.6.5: Async Backtest API orchestration.
//
// backtest_logic.go wires the three /backtests endpoints behind an
// in-memory job store backed by a sync.Map:
//
//   - CreateBacktest  — validates the request, spawns a goroutine, returns
//     immediately with a backtest_id (FR-RUN-01).
//   - GetBacktest     — polls the job store; returns progress (0–100 %) while
//     processing, or the full result when completed (FR-RUN-05).
//   - CancelBacktest  — cancels the goroutine's context, halting the pipeline
//     at the next blocking call.
//
// Pipeline executed inside the goroutine (one stage per task):
//
//	Simulator.Run()        (task 2.6.1) → RunOutput
//	OrderMatcher.Match()   (task 2.6.2) → MatchResult
//	CalculatePerformance() (task 2.6.3) → PerformanceSummary
//	GenerateEquityCurve()  (task 2.6.4) → []EquityPoint
//
// All results live in process memory only — no database persistence
// within WBS 2.6.5 scope.
//
// WBS: P2-Backend · 13/03/2026
// SRS: FR-RUN-01, FR-RUN-02, FR-RUN-03, FR-RUN-04, FR-RUN-05
package logic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kh0anh/quantflow/internal/engine/backtest"
	"github.com/kh0anh/quantflow/internal/repository"
	"github.com/shopspring/decimal"
)

// ═══════════════════════════════════════════════════════════════════════════
//  Backtest status constants
// ═══════════════════════════════════════════════════════════════════════════

// Backtest job status values — match api.yaml §BacktestResult.status enum.
const (
	BacktestStatusProcessing = "processing"
	BacktestStatusCompleted  = "completed"
	BacktestStatusCanceled   = "canceled"
)

// defaultBacktestMaxUnit is the per-session unit budget injected into the
// UnitCostTracker when max_unit is omitted from CreateBacktestRequest
// (api.yaml §CreateBacktestRequest.max_unit default).
const defaultBacktestMaxUnit = 1000

// ═══════════════════════════════════════════════════════════════════════════
//  Sentinel errors
// ═══════════════════════════════════════════════════════════════════════════

var (
	// ErrBacktestNotFound is returned by GetBacktest / CancelBacktest when the
	// backtest_id is absent from the job store or belongs to another user.
	// The same error is returned in both cases to prevent session-ID enumeration.
	// Handler maps this to HTTP 404 BACKTEST_NOT_FOUND.
	ErrBacktestNotFound = errors.New("backtest: session not found")

	// ErrBacktestAlreadyDone is returned by CancelBacktest when the session has
	// already reached a terminal state (completed or canceled).
	// Handler maps this to HTTP 409 BACKTEST_ALREADY_DONE.
	ErrBacktestAlreadyDone = errors.New("backtest: session is already completed or canceled")
)

// ═══════════════════════════════════════════════════════════════════════════
//  Input DTO
// ═══════════════════════════════════════════════════════════════════════════

// CreateBacktestInput carries the validated fields parsed from the
// CreateBacktestRequest body (api.yaml §CreateBacktestRequest).
// Populated by BacktestHandler.Create after JSON decode and field validation.
type CreateBacktestInput struct {
	StrategyID     string
	Symbol         string
	Timeframe      string
	StartTime      time.Time
	EndTime        time.Time
	InitialCapital decimal.Decimal
	FeeRate        decimal.Decimal
	// MaxUnit is the per-session Blockly unit budget; 0 triggers defaultBacktestMaxUnit.
	MaxUnit int
}

// ═══════════════════════════════════════════════════════════════════════════
//  Response DTOs (consumed by BacktestHandler to build JSON responses)
// ═══════════════════════════════════════════════════════════════════════════

// BacktestConfig is the configuration snapshot embedded in BacktestSnapshot.
// Maps to api.yaml §BacktestResult.config.
type BacktestConfig struct {
	StrategyID     string          `json:"strategy_id"`
	StrategyName   string          `json:"strategy_name"`
	Symbol         string          `json:"symbol"`
	Timeframe      string          `json:"timeframe"`
	StartTime      time.Time       `json:"start_time"`
	EndTime        time.Time       `json:"end_time"`
	InitialCapital decimal.Decimal `json:"initial_capital"`
	FeeRate        decimal.Decimal `json:"fee_rate"`
}

// BacktestSummaryDTO is the serialisable performance summary returned by
// GET /backtests/{id} when status=completed.
// Maps to api.yaml §BacktestResult.summary.
type BacktestSummaryDTO struct {
	TotalPnL           decimal.Decimal `json:"total_pnl"`
	TotalPnLPercent    decimal.Decimal `json:"total_pnl_percent"`
	WinRate            decimal.Decimal `json:"win_rate"`
	TotalTrades        int             `json:"total_trades"`
	WinningTrades      int             `json:"winning_trades"`
	LosingTrades       int             `json:"losing_trades"`
	MaxDrawdown        decimal.Decimal `json:"max_drawdown"`
	MaxDrawdownPercent decimal.Decimal `json:"max_drawdown_percent"`
	ProfitFactor       decimal.Decimal `json:"profit_factor"`
}

// BacktestEquityPointDTO is a (timestamp, equity) pair in the equity_curve
// array of GET /backtests/{id}.
// Maps to api.yaml §BacktestResult.equity_curve[i].
type BacktestEquityPointDTO struct {
	Timestamp time.Time       `json:"timestamp"`
	Equity    decimal.Decimal `json:"equity"`
}

// BacktestTradeDTO is a completed round-trip trade in the trades array
// of GET /backtests/{id}.
// Maps to api.yaml §BacktestTrade.
// Side is normalised to "Long"/"Short" (api.yaml enum) from the engine's
// internal "LONG"/"SHORT" representation.
type BacktestTradeDTO struct {
	OpenTime   time.Time       `json:"open_time"`
	CloseTime  time.Time       `json:"close_time"`
	Side       string          `json:"side"`
	EntryPrice decimal.Decimal `json:"entry_price"`
	ExitPrice  decimal.Decimal `json:"exit_price"`
	Quantity   decimal.Decimal `json:"quantity"`
	Fee        decimal.Decimal `json:"fee"`
	PnL        decimal.Decimal `json:"pnl"`
}

// BacktestSnapshot is a consistent, read-only copy of a BacktestJob produced
// under the job's read lock. Passed to BacktestHandler to build the HTTP
// response without exposing unexported job internals or requiring the handler
// to hold any synchronisation primitives.
type BacktestSnapshot struct {
	ID          string
	Status      string
	Progress    int32
	CreatedAt   time.Time
	CompletedAt *time.Time
	Config      *BacktestConfig
	// The following fields are non-nil only when Status == BacktestStatusCompleted.
	Summary     *BacktestSummaryDTO
	EquityCurve []BacktestEquityPointDTO
	Trades      []BacktestTradeDTO
}

// ═══════════════════════════════════════════════════════════════════════════
//  BacktestJob
// ═══════════════════════════════════════════════════════════════════════════

// BacktestJob is the in-memory record of a single async backtest session.
//
// Concurrency invariants:
//   - id, userID, cancel, createdAt, config: written once before the goroutine
//     starts; safe to read without any lock thereafter.
//   - status, completedAt, summary, equityCurve, trades: written by the
//     pipeline goroutine; guarded by mu.
//   - progress: written by Simulator.Run via atomic.StoreInt32; read via
//     atomic.LoadInt32. The mu lock is NOT held for progress access.
type BacktestJob struct {
	mu sync.RWMutex

	// Written once before goroutine starts — no lock needed for reads.
	id        string
	userID    string
	cancel    context.CancelFunc
	createdAt time.Time
	config    *BacktestConfig

	// Guarded by mu — written by the pipeline goroutine.
	status      string
	completedAt *time.Time
	summary     *backtest.PerformanceSummary
	equityCurve []backtest.EquityPoint
	trades      []backtest.FilledTrade

	// Accessed exclusively via sync/atomic by both the pipeline goroutine
	// (writer) and the HTTP handler goroutine (reader).
	progress int32
}

// Snapshot returns a consistent, copied view of the job state, safe to use
// from any goroutine. The read lock is released before returning.
func (j *BacktestJob) Snapshot() BacktestSnapshot {
	j.mu.RLock()
	defer j.mu.RUnlock()

	snap := BacktestSnapshot{
		ID:          j.id,
		Status:      j.status,
		Progress:    atomic.LoadInt32(&j.progress),
		CreatedAt:   j.createdAt,
		CompletedAt: j.completedAt,
		Config:      j.config,
	}

	if j.summary != nil {
		snap.Summary = &BacktestSummaryDTO{
			TotalPnL:           j.summary.TotalPnL,
			TotalPnLPercent:    j.summary.TotalPnLPercent,
			WinRate:            j.summary.WinRate,
			TotalTrades:        j.summary.TotalTrades,
			WinningTrades:      j.summary.WinningTrades,
			LosingTrades:       j.summary.LosingTrades,
			MaxDrawdown:        j.summary.MaxDrawdown,
			MaxDrawdownPercent: j.summary.MaxDrawdownPercent,
			ProfitFactor:       j.summary.ProfitFactor,
		}
	}

	if len(j.equityCurve) > 0 {
		snap.EquityCurve = make([]BacktestEquityPointDTO, len(j.equityCurve))
		for i, pt := range j.equityCurve {
			snap.EquityCurve[i] = BacktestEquityPointDTO{
				Timestamp: pt.Timestamp,
				Equity:    pt.Equity,
			}
		}
	}

	if len(j.trades) > 0 {
		snap.Trades = make([]BacktestTradeDTO, len(j.trades))
		for i, t := range j.trades {
			snap.Trades[i] = BacktestTradeDTO{
				OpenTime:   t.OpenTime,
				CloseTime:  t.CloseTime,
				Side:       normaliseTradeSide(t.Side),
				EntryPrice: t.EntryPrice,
				ExitPrice:  t.ExitPrice,
				Quantity:   t.Quantity,
				Fee:        t.Fee,
				PnL:        t.PnL,
			}
		}
	}

	return snap
}

// normaliseTradeSide converts the engine's internal "LONG"/"SHORT" values to
// the api.yaml BacktestTrade.side enum values "Long"/"Short".
func normaliseTradeSide(s string) string {
	switch s {
	case "LONG":
		return "Long"
	case "SHORT":
		return "Short"
	default:
		return s
	}
}

// ═══════════════════════════════════════════════════════════════════════════
//  BacktestLogic
// ═══════════════════════════════════════════════════════════════════════════

// BacktestLogic orchestrates async backtest sessions for the /backtests API.
// Results live exclusively in process memory — no database persistence within
// the WBS 2.6.5 scope.
type BacktestLogic struct {
	strategyRepo repository.StrategyRepository
	candleRepo   repository.CandleRepository
	jobs         sync.Map // string (backtest_id) → *BacktestJob
	logger       *slog.Logger
}

// NewBacktestLogic constructs a BacktestLogic with its required dependencies.
// Passing nil for logger falls back to slog.Default().
func NewBacktestLogic(
	strategyRepo repository.StrategyRepository,
	candleRepo repository.CandleRepository,
	logger *slog.Logger,
) *BacktestLogic {
	if logger == nil {
		logger = slog.Default()
	}
	return &BacktestLogic{
		strategyRepo: strategyRepo,
		candleRepo:   candleRepo,
		logger:       logger,
	}
}

// CreateBacktest validates the request, creates a BacktestJob, launches the
// simulation pipeline in a goroutine, and returns *immediately* — before the
// simulation completes — satisfying the async processing requirement (FR-RUN-01).
//
// Error mapping:
//   - ErrStrategyNotFound → handler 404 STRATEGY_NOT_FOUND
//   - other errors         → handler 500 INTERNAL_ERROR
func (l *BacktestLogic) CreateBacktest(
	ctx context.Context,
	userID string,
	req CreateBacktestInput,
) (*BacktestJob, error) {
	// Fetch strategy: verify ownership and obtain the latest logic_json + name.
	detail, err := l.strategyRepo.FindByID(ctx, req.StrategyID, userID)
	if err != nil {
		return nil, fmt.Errorf("backtest_logic: CreateBacktest: %w", err)
	}
	if detail == nil {
		return nil, ErrStrategyNotFound
	}

	// Copy logic_json bytes so the goroutine holds its own reference after the
	// HTTP request context is cancelled and the calling stack is released.
	logicJSON := make([]byte, len(detail.LogicJSON))
	copy(logicJSON, detail.LogicJSON)

	maxUnit := req.MaxUnit
	if maxUnit <= 0 {
		maxUnit = defaultBacktestMaxUnit
	}

	engineCfg := backtest.Config{
		// api.yaml §BacktestResult.config exposes strategy_id, not the version UUID,
		// so the strategy ID is used here for informational tracing only.
		StrategyVersionID: req.StrategyID,
		Symbol:            req.Symbol,
		Timeframe:         req.Timeframe,
		StartTime:         req.StartTime,
		EndTime:           req.EndTime,
		InitialCapital:    req.InitialCapital,
		FeeRate:           req.FeeRate,
		MaxUnit:           maxUnit,
	}

	// Use a detached background context for the goroutine — the pipeline must
	// not be cancelled when the originating HTTP request context expires.
	jobCtx, cancel := context.WithCancel(context.Background())

	job := &BacktestJob{
		id:        uuid.New().String(),
		userID:    userID,
		status:    BacktestStatusProcessing,
		cancel:    cancel,
		createdAt: time.Now().UTC(),
		config: &BacktestConfig{
			StrategyID:     req.StrategyID,
			StrategyName:   detail.Name,
			Symbol:         req.Symbol,
			Timeframe:      req.Timeframe,
			StartTime:      req.StartTime,
			EndTime:        req.EndTime,
			InitialCapital: req.InitialCapital,
			FeeRate:        req.FeeRate,
		},
	}

	l.jobs.Store(job.id, job)

	go l.runPipeline(jobCtx, job, logicJSON, engineCfg)

	l.logger.Info("backtest job created",
		slog.String("backtest_id", job.id),
		slog.String("user_id", userID),
		slog.String("symbol", req.Symbol),
		slog.String("timeframe", req.Timeframe),
	)

	return job, nil
}

// GetBacktest retrieves the current state of a backtest session by ID.
// Returns ErrBacktestNotFound for unknown IDs or sessions owned by another
// user — the same error prevents session-ID enumeration (NFR-SEC-01).
func (l *BacktestLogic) GetBacktest(
	_ context.Context,
	backtestID, userID string,
) (*BacktestJob, error) {
	raw, ok := l.jobs.Load(backtestID)
	if !ok {
		return nil, ErrBacktestNotFound
	}
	job := raw.(*BacktestJob)
	if job.userID != userID {
		// Return the same 404 as "not found" to prevent session-ID enumeration.
		return nil, ErrBacktestNotFound
	}
	return job, nil
}

// CancelBacktest signals the pipeline goroutine to stop by cancelling its
// context. Returns ErrBacktestAlreadyDone when the session is no longer in
// the processing state (completed or already canceled).
func (l *BacktestLogic) CancelBacktest(
	_ context.Context,
	backtestID, userID string,
) error {
	raw, ok := l.jobs.Load(backtestID)
	if !ok {
		return ErrBacktestNotFound
	}
	job := raw.(*BacktestJob)
	if job.userID != userID {
		return ErrBacktestNotFound
	}

	job.mu.RLock()
	status := job.status
	job.mu.RUnlock()

	if status != BacktestStatusProcessing {
		return ErrBacktestAlreadyDone
	}

	// Trigger cancellation; runPipeline detects ctx.Done() at the next
	// blocking engine call and transitions status to BacktestStatusCanceled.
	job.cancel()
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Pipeline goroutine
// ═══════════════════════════════════════════════════════════════════════════

// runPipeline executes the full backtest pipeline:
//
//  1. Simulator.Run()        — replay candle history through the Blockly executor.
//  2. OrderMatcher.Match()   — simulate order fills against OHLCV price ranges.
//  3. CalculatePerformance() — compute PnL, Win Rate, Max Drawdown, Profit Factor.
//  4. GenerateEquityCurve()  — build the equity growth data series.
//
// Progress [0, 100] is updated atomically by Simulator.Run via &job.progress.
// Any error or context cancellation transitions the job to BacktestStatusCanceled.
//
// SRS: FR-RUN-02, FR-RUN-03, FR-RUN-04
func (l *BacktestLogic) runPipeline(
	ctx context.Context,
	job *BacktestJob,
	logicJSON []byte,
	cfg backtest.Config,
) {
	log := l.logger.With(
		slog.String("backtest_id", job.id),
		slog.String("symbol", cfg.Symbol),
		slog.String("timeframe", cfg.Timeframe),
	)

	// markTerminal transitions the job to a terminal status under the write lock
	// and releases the goroutine's context resources.
	markTerminal := func(status string, cause error) {
		now := time.Now().UTC()
		job.mu.Lock()
		job.status = status
		job.completedAt = &now
		job.mu.Unlock()
		job.cancel() // idempotent; releases context even if already done
		if cause != nil {
			log.Warn("backtest pipeline terminated",
				slog.String("status", status),
				slog.String("error", cause.Error()),
			)
		}
	}

	// Stage 1: Simulation (FR-RUN-02).
	// &job.progress is the atomic int32 counter updated per candle by the simulator.
	sim := backtest.NewBacktestSimulator(l.candleRepo, log)
	runOutput, err := sim.Run(ctx, cfg, logicJSON, &job.progress)
	if err != nil {
		markTerminal(BacktestStatusCanceled, err)
		return
	}

	// Stage 2: Order Matching (FR-RUN-02).
	matcher := backtest.NewOrderMatcher(cfg.FeeRate, log)
	matchResult, err := matcher.Match(ctx, runOutput)
	if err != nil {
		markTerminal(BacktestStatusCanceled, err)
		return
	}

	// Stage 3: Performance Report (FR-RUN-03).
	summary, err := backtest.CalculatePerformance(matchResult, cfg.InitialCapital)
	if err != nil {
		markTerminal(BacktestStatusCanceled, err)
		return
	}

	// Stage 4: Equity Curve generation (FR-RUN-04).
	equityCurve, err := backtest.GenerateEquityCurve(matchResult, cfg.InitialCapital, cfg.StartTime)
	if err != nil {
		markTerminal(BacktestStatusCanceled, err)
		return
	}

	// All stages succeeded — publish the completed result under the write lock,
	// then set progress=100 atomically so a concurrent Snapshot() never reads
	// status=completed with a stale progress < 100.
	now := time.Now().UTC()
	job.mu.Lock()
	job.status = BacktestStatusCompleted
	job.completedAt = &now
	job.summary = summary
	job.equityCurve = equityCurve
	job.trades = matchResult.Trades
	atomic.StoreInt32(&job.progress, 100)
	job.mu.Unlock()

	job.cancel() // release the goroutine's context resources

	log.Info("backtest completed",
		slog.Int("total_trades", len(matchResult.Trades)),
		slog.String("total_pnl", summary.TotalPnL.String()),
	)
}
