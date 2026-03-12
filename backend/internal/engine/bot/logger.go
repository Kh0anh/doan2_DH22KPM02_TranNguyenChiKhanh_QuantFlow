package bot

// logger.go — Task 2.7.4: Bot Logging and Error Handling.
//
// BotLogger writes one bot_log row to PostgreSQL (synchronously) and pushes
// the same event to all subscribed WebSocket clients (asynchronously, via a
// fire-and-forget goroutine) for every strategy Session outcome.
//
// ─── Simultaneous Write Strategy (SRS FR-RUN-08) ─────────────────────────
//
//  1. DB INSERT (synchronous) — ensures the row is durable and the generated
//     BIGSERIAL id is available before the WS frame is constructed.
//
//  2. WS push (asynchronous goroutine) — launched after the DB write so that
//     (a) entry.ID is populated, (b) WS delivery failure never blocks or
//     errors the bot session loop.
//
// Non-fatal contract: Log() errors are returned to the caller (runSession),
// which emits a slog.Warn and continues running the bot — consistent with the
// Task 2.7.3 pattern for lifecycle persist failures.
//
// BotLogger is safe for concurrent use from multiple bot goroutines.
//
// Task 2.7.4 — Bot Logging and Error Handling.
// WBS: P2-Backend · 14/03/2026
// SRS: FR-RUN-08, FR-MONITOR-03

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
)

// ═══════════════════════════════════════════════════════════════════════════
//  BotLogPusher — dependency-inversion interface for WebSocket fan-out
// ═══════════════════════════════════════════════════════════════════════════

// BotLogPusher is the interface the bot engine uses to push log events to
// subscribed WebSocket clients. Defined here (at the point of consumption)
// so that engine/bot does not import internal/websocket — the concrete
// *websocket.WSManager satisfies this interface structurally without any
// explicit declaration.
//
// Passing nil to NewBotLogger or NewBotManager disables WS push without
// changing any other behaviour (useful for unit tests or CLI tools that do
// not start the HTTP server).
type BotLogPusher interface {
	// PushBotLog fans out a pre-serialised JSON frame to all Client connections
	// subscribed to the bot_logs channel for botID. Delivery is best-effort;
	// slow or disconnected clients are skipped without blocking the caller.
	PushBotLog(botID string, payload []byte)
}

// ═══════════════════════════════════════════════════════════════════════════
//  WS push payload structs (websocket.md §3.2)
// ═══════════════════════════════════════════════════════════════════════════

// botLogWS is the top-level JSON envelope pushed to WebSocket clients.
// Matches the event: "bot_log" / channel: "bot_logs" spec (websocket.md §3.2).
type botLogWS struct {
	Event   string       `json:"event"`
	Channel string       `json:"channel"`
	Data    botLogWSData `json:"data"`
}

type botLogWSData struct {
	BotID string       `json:"bot_id"`
	Log   botLogWSItem `json:"log"`
}

// botLogWSItem extends the DB row with unit_used, which has no corresponding
// column in the bot_logs table (schema.sql §7) but is required by the
// WebSocket spec (websocket.md §3.2).
type botLogWSItem struct {
	ID             int64     `json:"id"`
	ActionDecision string    `json:"action_decision"`
	UnitUsed       int       `json:"unit_used"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════════
//  BotLogger
// ═══════════════════════════════════════════════════════════════════════════

// BotLogger coordinates DB persistence and WebSocket push for every bot log
// event generated during live trade sessions.
//
// BotLogger is safe for concurrent use from multiple bot goroutines.
type BotLogger struct {
	logRepo  repository.BotLogRepository
	wsPusher BotLogPusher
	logger   *slog.Logger
}

// NewBotLogger constructs a BotLogger.
//
//   - logRepo:  GORM-backed BotLogRepository — must be non-nil.
//   - wsPusher: BotLogPusher implementation (e.g. *websocket.WSManager) for
//     live event fan-out. May be nil to disable WS delivery.
//   - logger:   slog.Logger. If nil, slog.Default() is used as a safe fallback.
func NewBotLogger(
	logRepo repository.BotLogRepository,
	wsPusher BotLogPusher,
	logger *slog.Logger,
) *BotLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &BotLogger{
		logRepo:  logRepo,
		wsPusher: wsPusher,
		logger:   logger,
	}
}

// Log inserts one bot_log row to the database and asynchronously pushes a
// bot_log WebSocket event to all subscribed clients for this bot.
//
//   - botID:          UUID of the bot_instance that generated this log.
//   - actionDecision: one of the BotLogAction* constants (domain/bot_log.go).
//   - message:        human-readable description of the session outcome.
//   - unitUsed:       units consumed in the Session (WS payload only; no DB column).
func (l *BotLogger) Log(
	ctx context.Context,
	botID string,
	actionDecision string,
	message string,
	unitUsed int,
) error {
	// ── Step 1: synchronous DB insert ──────────────────────────────────────
	entry := &domain.BotLog{
		BotID:          botID,
		ActionDecision: actionDecision,
		Message:        message,
	}
	if err := l.logRepo.Insert(ctx, entry); err != nil {
		return fmt.Errorf("BotLogger.Log: DB insert failed: %w", err)
	}

	// ── Step 2: asynchronous WS push (fire-and-forget) ─────────────────────
	// Launched after the synchronous DB write so:
	//   (a) entry.ID is populated with the generated BIGSERIAL value.
	//   (b) The log row is durable before the browser receives the WS event.
	if l.wsPusher != nil {
		payload, marshalErr := buildBotLogPayload(botID, entry, unitUsed)
		if marshalErr != nil {
			// Marshal failure is a programmer error — log and skip WS push.
			l.logger.Warn("BotLogger.Log: failed to marshal WS payload",
				slog.String("bot_id", botID),
				slog.String("error", marshalErr.Error()),
			)
			return nil
		}
		go l.wsPusher.PushBotLog(botID, payload)
	}

	return nil
}

// buildBotLogPayload serialises a bot_log WS event frame for the given entry.
func buildBotLogPayload(botID string, entry *domain.BotLog, unitUsed int) ([]byte, error) {
	frame := botLogWS{
		Event:   "bot_log",
		Channel: "bot_logs",
		Data: botLogWSData{
			BotID: botID,
			Log: botLogWSItem{
				ID:             entry.ID,
				ActionDecision: entry.ActionDecision,
				UnitUsed:       unitUsed,
				Message:        entry.Message,
				CreatedAt:      entry.CreatedAt,
			},
		},
	}
	return json.Marshal(frame)
}
