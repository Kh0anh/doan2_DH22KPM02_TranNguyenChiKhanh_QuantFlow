package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/engine/bot"
	"github.com/kh0anh/quantflow/internal/exchange"
	"github.com/kh0anh/quantflow/internal/repository"
)

// Sentinel errors for bot operations — mapped to specific HTTP codes by BotHandler.
var (
	// ErrBotNotFound is returned when the requested bot does not exist or
	// does not belong to the authenticated user. Handler maps to 404.
	ErrBotNotFound = errors.New("bot not found")

	// ErrBotStillRunning is returned when DELETE is attempted on a Running bot.
	// Handler maps to 409 BOT_STILL_RUNNING (api.yaml §DELETE /bots/{id}).
	ErrBotStillRunning = errors.New("bot still running, cannot delete")

	// ErrBotStrategyNotFound is returned when the strategy_id in CreateBotInput
	// does not exist or belongs to another user. Handler maps to 404.
	// Named with Bot prefix to avoid conflict with strategy_logic.ErrStrategyNotFound.
	ErrBotStrategyNotFound = errors.New("strategy not found for bot creation")

	// ErrBotStrategyInvalid is returned when the strategy status is not "Valid".
	// Handler maps to 422 STRATEGY_INVALID (api.yaml §POST /bots, precondition).
	ErrBotStrategyInvalid = errors.New("strategy status must be Valid to create bot")

	// ErrBotAPIKeyNotConfigured is returned when the user has no api_key record.
	// Handler maps to 422 EXCHANGE_NOT_CONFIGURED (api.yaml §POST /bots).
	// Named with Bot prefix to avoid conflict with api_key_logic.ErrAPIKeyNotConfigured.
	ErrBotAPIKeyNotConfigured = errors.New("no exchange api key configured for bot creation")

	// ErrBotAPIKeyInvalid is returned when the api_key status is not "Connected".
	// Handler maps to 422 EXCHANGE_NOT_CONFIGURED.
	ErrBotAPIKeyInvalid = errors.New("exchange api key status is not Connected")

	// ErrBotInvalidLogicJSON is returned when the strategy's logic_json cannot be
	// parsed or does not contain an event_on_candle block.
	// Handler maps to 422 INVALID_LOGIC_JSON.
	ErrBotInvalidLogicJSON = errors.New("strategy logic_json is invalid or missing event block")
)

// CreateBotInput is the internal DTO passed from BotHandler to BotLogic.
type CreateBotInput struct {
	BotName    string // Human-readable bot name (max 100 chars, validated by handler).
	StrategyID string // UUID of the strategy to snapshot.
	Symbol     string // Binance Futures trading pair (e.g., "BTCUSDT").
}

// BotCreated is the DTO returned by CreateBot, matching api.yaml §BotCreated.
type BotCreated struct {
	ID              string `json:"id"`
	BotName         string `json:"bot_name"`
	StrategyID      string `json:"strategy_id"`
	StrategyVersion int    `json:"strategy_version"`
	Symbol          string `json:"symbol"`
	Status          string `json:"status"`
	TotalPnL        string `json:"total_pnl"`
	CreatedAt       string `json:"created_at"` // ISO8601 timestamp
}

// BotLogic orchestrates bot lifecycle business rules (WBS 2.7.5).
type BotLogic struct {
	botRepo      repository.BotRepository
	strategyRepo repository.StrategyRepository
	apiKeyRepo   repository.ApiKeyRepository
	candleRepo   repository.CandleRepository
	botManager   *bot.BotManager
	aesKey       []byte                        // For decrypting API secret via ApiKeyLogic pattern
	limiter      *exchange.ExchangeRateLimiter // For building BinanceProxy
}

// NewBotLogic constructs a BotLogic with all required dependencies.
//   - aesKey: 32-byte AES-256 key from pkgcrypto.DeriveKey(cfg.AESKey).
//   - limiter: singleton ExchangeRateLimiter shared across all BinanceProxy instances.
func NewBotLogic(
	botRepo repository.BotRepository,
	strategyRepo repository.StrategyRepository,
	apiKeyRepo repository.ApiKeyRepository,
	candleRepo repository.CandleRepository,
	botManager *bot.BotManager,
	aesKey []byte,
	limiter *exchange.ExchangeRateLimiter,
) *BotLogic {
	return &BotLogic{
		botRepo:      botRepo,
		strategyRepo: strategyRepo,
		apiKeyRepo:   apiKeyRepo,
		candleRepo:   candleRepo,
		botManager:   botManager,
		aesKey:       aesKey,
		limiter:      limiter,
	}
}

// ListBots retrieves all bots owned by the given user, optionally filtered by status.
// statusFilter can be empty (no filter), "Running", "Stopped", or "Error".
// Results are ordered by created_at DESC (most recent first).
//
// Business rules (WBS 2.7.5, api.yaml §GET /bots):
//   - JOIN strategies and strategy_versions to resolve strategy_name and version_number.
//   - Return empty slice [] when no bots exist (never nil).
func (l *BotLogic) ListBots(ctx context.Context, userID, statusFilter string) ([]domain.BotSummary, error) {
	summaries, err := l.botRepo.ListByUserID(ctx, userID, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("bot_logic: ListBots: %w", err)
	}
	return summaries, nil
}

// CreateBot implements the POST /bots business flow (WBS 2.7.5, api.yaml §POST /bots):
//
//  1. Validate preconditions (strategy exists & Valid, api_key exists & Connected).
//  2. Parse strategy logic_json to extract Interval from event_on_candle block.
//  3. Snapshot the latest strategy_version_id.
//  4. Insert bot_instances record with status=Running, total_pnl=0.
//  5. Build BinanceProxy from encrypted api_key.
//  6. Call BotManager.StartBot() to launch the bot goroutine (Task 2.7.1).
//  7. On StartBot() failure: update status=Error + return 500 to caller.
//
// Return patterns:
//   - (*BotCreated, nil)            — success; bot goroutine is running.
//   - (nil, ErrStrategyNotFound)    — 404.
//   - (nil, ErrStrategyInvalid)     — 422 STRATEGY_INVALID.
//   - (nil, ErrAPIKeyNotConfigured) — 422 EXCHANGE_NOT_CONFIGURED.
//   - (nil, ErrInvalidLogicJSON)    — 422 INVALID_LOGIC_JSON.
//   - (nil, other)                  — 500 internal error.
func (l *BotLogic) CreateBot(ctx context.Context, userID string, input CreateBotInput) (*BotCreated, error) {
	// ─── Step 1: Validate strategy precondition ──────────────────────────────
	strategyDetail, err := l.strategyRepo.FindByID(ctx, input.StrategyID, userID)
	if err != nil {
		return nil, fmt.Errorf("bot_logic: CreateBot: find strategy: %w", err)
	}
	if strategyDetail == nil {
		return nil, ErrBotStrategyNotFound
	}
	if strategyDetail.Status != domain.StrategyStatusValid {
		return nil, ErrBotStrategyInvalid
	}

	// ─── Step 2: Validate API key precondition ───────────────────────────────
	apiKey, err := l.apiKeyRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("bot_logic: CreateBot: find api_key: %w", err)
	}
	if apiKey == nil {
		return nil, ErrBotAPIKeyNotConfigured
	}
	if apiKey.Status != "Connected" && apiKey.Status != "Active" {
		// Accept both "Connected" (spec) and "Active" (alt naming) for flexibility.
		return nil, ErrBotAPIKeyInvalid
	}

	// ─── Step 3: Parse logic_json to extract Interval ────────────────────────
	interval, err := extractIntervalFromLogicJSON(strategyDetail.LogicJSON)
	if err != nil {
		return nil, ErrBotInvalidLogicJSON
	}

	// ─── Step 4: Insert bot_instances record ─────────────────────────────────
	botInstance := &domain.BotInstance{
		UserID:            userID,
		StrategyID:        input.StrategyID,
		StrategyVersionID: strategyDetail.VersionID,
		APIKeyID:          apiKey.ID,
		BotName:           input.BotName,
		Symbol:            input.Symbol,
		Status:            domain.BotStatusRunning,
		TotalPnL:          "0",
	}

	if err := l.botRepo.Create(ctx, botInstance); err != nil {
		return nil, fmt.Errorf("bot_logic: CreateBot: insert bot: %w", err)
	}

	// ─── Step 5: Build BinanceProxy ──────────────────────────────────────────
	binanceProxy, err := exchange.NewBinanceProxy(apiKey.ApiKey, apiKey.SecretKeyEncrypted, l.aesKey, l.limiter)
	if err != nil {
		// Failed to decrypt or initialize exchange client.
		// Update bot status to Error so user can debug via bot_logs.
		_ = l.botRepo.UpdateStatus(ctx, botInstance.ID, domain.BotStatusError)
		return nil, fmt.Errorf("bot_logic: CreateBot: build binance proxy: %w", err)
	}

	// ─── Step 6: Start bot goroutine via BotManager ──────────────────────────
	botConfig := bot.BotConfig{
		BotID:             botInstance.ID,
		UserID:            userID,
		StrategyVersionID: strategyDetail.VersionID,
		LogicJSON:         strategyDetail.LogicJSON,
		Symbol:            input.Symbol,
		APIKeyID:          apiKey.ID,
		Interval:          interval,
		CandleRepo:        l.candleRepo,
		TradingProxy:      binanceProxy,
	}

	if err := l.botManager.StartBot(ctx, botConfig); err != nil {
		// BotManager.StartBot() failed (e.g., bot already running — should not happen
		// since we just created a new ID, or other internal error).
		// Persist status=Error so the bot row is visible to the user for debugging.
		_ = l.botRepo.UpdateStatus(ctx, botInstance.ID, domain.BotStatusError)
		return nil, fmt.Errorf("bot_logic: CreateBot: start bot goroutine: %w", err)
	}

	// ─── Step 7: Return success response ─────────────────────────────────────
	return &BotCreated{
		ID:              botInstance.ID,
		BotName:         botInstance.BotName,
		StrategyID:      input.StrategyID,
		StrategyVersion: strategyDetail.Version,
		Symbol:          input.Symbol,
		Status:          domain.BotStatusRunning,
		TotalPnL:        "0",
		CreatedAt:       botInstance.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// GetBotDetail retrieves the full detail of a bot (WBS 2.7.5, api.yaml §GET /bots/{id}).
//
// Business rules:
//   - JOIN strategies and strategy_versions to resolve strategy_name and version_number.
//   - Fetch Position and OpenOrders real-time from Binance API (not from DB).
//   - Return nil when bot does not exist or belongs to another user (handler maps to 404).
//
// Note: Position and OpenOrders fetching from Binance is deferred to a future phase
// (Task 2.8.4 — position_update WebSocket channel). For now, return nil Position
// and empty OpenOrders slice to satisfy the API contract without blocking this task.
func (l *BotLogic) GetBotDetail(ctx context.Context, botID, userID string) (*domain.BotDetail, error) {
	detail, err := l.botRepo.FindByID(ctx, botID, userID)
	if err != nil {
		return nil, fmt.Errorf("bot_logic: GetBotDetail: %w", err)
	}
	if detail == nil {
		return nil, ErrBotNotFound
	}

	// TODO(Task 2.8.4): Fetch real-time Position and OpenOrders from Binance API.
	// For now, set Position=nil and OpenOrders=[] to satisfy the API spec without
	// blocking this task (api.yaml §BotDetail: position is nullable, open_orders
	// defaults to empty array).
	detail.Position = nil
	detail.OpenOrders = []domain.OpenOrder{}

	return detail, nil
}

// DeleteBot removes a bot owned by the given user (WBS 2.7.5, api.yaml §DELETE /bots/{id}).
//
// Business rules:
//   - Bot must have status != Running (409 BOT_STILL_RUNNING if violated).
//   - Bot must exist and belong to the authenticated user (404 if not found).
//   - Cascade DELETE on bot_logs and bot_lifecycle_variables is handled by DB schema.
//
// Return patterns:
//   - nil                       — success; bot deleted.
//   - ErrBotNotFound            — 404.
//   - ErrBotStillRunning        — 409.
//   - other error               — 500.
func (l *BotLogic) DeleteBot(ctx context.Context, botID, userID string) error {
	err := l.botRepo.DeleteByID(ctx, botID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrBotStillRunning) {
			return ErrBotStillRunning
		}
		if errors.Is(err, repository.ErrNotFound) {
			return ErrBotNotFound
		}
		return fmt.Errorf("bot_logic: DeleteBot: %w", err)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
//  Helper: Extract Interval from strategy logic_json
// ═══════════════════════════════════════════════════════════════════════════

// extractIntervalFromLogicJSON parses the strategy's Blockly JSON and extracts
// the Interval field from the first event_on_candle block found.
//
// Blockly JSON structure (blockly.md §3.1.1, SRS FR-DESIGN-03):
//
//	{
//	  "blocks": {
//	    "blocks": [
//	      {
//	        "type": "event_on_candle",
//	        "fields": {
//	          "INTERVAL": "1m"  ← we want this string
//	        }
//	      }
//	    ]
//	  }
//	}
//
// Returns the Interval string (e.g., "1m", "5m", "1h") or an error if:
//   - JSON is malformed.
//   - No event_on_candle block is found.
//   - The INTERVAL field is missing or empty.
func extractIntervalFromLogicJSON(logicJSON []byte) (string, error) {
	var root struct {
		Blocks struct {
			Blocks []struct {
				Type   string `json:"type"`
				Fields struct {
					Interval string `json:"INTERVAL"`
				} `json:"fields"`
			} `json:"blocks"`
		} `json:"blocks"`
	}

	if err := json.Unmarshal(logicJSON, &root); err != nil {
		return "", fmt.Errorf("extractInterval: unmarshal json: %w", err)
	}

	for _, block := range root.Blocks.Blocks {
		if block.Type == "event_on_candle" {
			interval := block.Fields.Interval
			if interval == "" {
				return "", fmt.Errorf("extractInterval: event_on_candle INTERVAL field is empty")
			}
			return interval, nil
		}
	}

	return "", fmt.Errorf("extractInterval: no event_on_candle block found")
}
