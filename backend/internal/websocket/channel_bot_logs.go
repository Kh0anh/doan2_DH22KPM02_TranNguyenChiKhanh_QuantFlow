package websocket

// channel_bot_logs.go — bot_logs WebSocket channel handler.
//
// BotLogsChannel is the canonical façade for the bot_logs real-time channel.
//
// ─── Architecture ─────────────────────────────────────────────────────────────
//
// Unlike MarketTickerChannel (which self-starts Binance Futures WebSocket
// streams), BotLogsChannel is a PASSIVE channel — it does NOT own any
// background goroutines or external connections. Instead, it relies on the
// push pipeline that was already wired during Task 2.7.4:
//
//	BotLogger.Log()              (engine/bot/logger.go)
//	    │
//	    ├─ Step 1 (sync):  BotLogRepository.Insert() → PostgreSQL
//	    │                  → log.ID populated (BIGSERIAL)
//	    │
//	    └─ Step 2 (async): go wsPusher.PushBotLog(botID, payload)
//	                            │
//	                            ▼
//	                WSManager.PushBotLog(botID, payload)  (websocket/manager.go)
//	                            │  RLock fan-out
//	                            ▼
//	                for each Client: IsSubscribedToBot(botID) == true
//	                    Client.enqueue(payload)            (websocket/client.go)
//	                            │
//	                            ▼
//	                Client.WritePump → gorilla/websocket.WriteMessage → Browser
//
// ─── Subscription lifecycle ───────────────────────────────────────────────────
//
// Subscribe / Unsubscribe messages from the browser are parsed and dispatched
// entirely inside Client.handleSubscription() (client.go §handleSubscription).
// The relevant case:
//
//	case "bot_logs":
//	    params.bot_id (UUID) required → MISSING_PARAMS error if absent.
//	    subscribe   → c.SubscribeBot(botID)
//	    unsubscribe → c.UnsubscribeBot(botID)
//	    ACK sent    → {"event":"subscribed","channel":"bot_logs","data":{"bot_id":"..."}}
//
// A client may subscribe to multiple bot_ids simultaneously; each subscription
// is stored independently in Client.subscribedBots (map[string]struct{}).
//
// ─── Wire-up (cmd/server/main.go or router.go) ───────────────────────────────
//
// Instantiate once and expose the singleton to BotManager via NewBotManager():
//
//	botLogsChannel := websocket.NewBotLogsChannel(wsManager, logger)
//	// botLogsChannel is currently used as documentation/DI anchor;
//	// the concrete push is handled by WSManager.PushBotLog().
//
// ─── Payload format (websocket.md §3.2 — Event: bot_log) ─────────────────────
//
//	{
//	  "event":   "bot_log",
//	  "channel": "bot_logs",
//	  "data": {
//	    "bot_id": "<UUID>",
//	    "log": {
//	      "id":              <int64>,    // BIGSERIAL PK from bot_logs table
//	      "action_decision": "<string>", // e.g. "EXECUTED", "ERROR"
//	      "unit_used":       <int>,      // Unit Cost consumed (not stored in DB)
//	      "message":         "<string>", // human-readable session outcome
//	      "created_at":      "<RFC3339>" // UTC timestamp
//	    }
//	  }
//	}
//
// Note on payload types: engine/bot/logger.go defines its own private structs
// (botLogWS, botLogWSData, botLogWSItem) to avoid a circular import cycle
// (engine/bot → internal/websocket is forbidden by Clean Architecture).
// The types declared below (botLogPush, botLogPushData, botLogPushItem) are the
// websocket-package counterparts — canonical for documentation, tests, and any
// future WS-layer helpers that consume or verify the channel's frames.
//
// Task 2.8.3 — Channel bot_logs (live log push per bot_id).
// WBS: P2-Backend · Category: WebSocket · 15/03/2026.
// SRS: FR-RUN-08 (Live Logs), FR-MONITOR-03 (Bottom Panel).
// UC-09 (Monitor Live Bot) Step 5: subscribe bot_logs after REST history load.
// Referenced by: websocket.md §3.2, WBS 2.7.4 (BotLogger), WBS 2.8.1 (Manager).

import "log/slog"

// ─── Payload types ────────────────────────────────────────────────────────────

// botLogPush is the top-level JSON envelope pushed to WebSocket clients
// subscribed to the bot_logs channel (websocket.md §2.3, §3.2).
//
// Mirrors the private botLogWS type in engine/bot/logger.go.
// Kept private to this package; exposed only through BotLogsChannel methods
// or to websocket-layer unit tests via package-internal access.
type botLogPush struct {
	Event   string          `json:"event"`   // always "bot_log"
	Channel string          `json:"channel"` // always "bot_logs"
	Data    botLogPushData  `json:"data"`
}

// botLogPushData is the outer data envelope carrying bot_id and the log entry
// (websocket.md §3.2 — Event: bot_log).
type botLogPushData struct {
	BotID string           `json:"bot_id"`
	Log   botLogPushItem   `json:"log"`
}

// botLogPushItem contains the per-session log fields sent to the browser.
//
// Fields:
//   - ID:             BIGSERIAL PK from bot_logs table. Allows the frontend to
//                     deduplicate live WS events against REST-loaded history
//                     (GET /bots/{botId}/logs cursor pagination, Task 2.7.7).
//   - ActionDecision: one of domain.BotLogAction* constants ("EXECUTED",
//                     "UNIT_COST_EXCEEDED", "ERROR", "PANIC").
//   - UnitUsed:       Blockly Unit Cost consumed in the Session (SRS FR-RUN-07).
//                     Not stored in the bot_logs DB table — WS-only field.
//   - Message:        Human-readable session outcome shown in the Console UI.
//   - CreatedAt:      UTC timestamp (RFC3339) of when the log row was created.
type botLogPushItem struct {
	ID             int64  `json:"id"`
	ActionDecision string `json:"action_decision"`
	UnitUsed       int    `json:"unit_used"`
	Message        string `json:"message"`
	CreatedAt      string `json:"created_at"` // RFC3339 UTC
}

// ─── BotLogsChannel ──────────────────────────────────────────────────────────

// BotLogsChannel is the DI façade for the bot_logs WebSocket channel.
//
// It provides a consistent, named entry point in the websocket package,
// mirroring the pattern of MarketTickerChannel (channel_market_ticker.go).
//
// The actual push delivery is performed by WSManager.PushBotLog() which is
// called by BotLogger (engine/bot/logger.go) after every strategy Session.
// BotLogsChannel does not start any goroutines of its own.
type BotLogsChannel struct {
	manager *WSManager
	logger  *slog.Logger
}

// NewBotLogsChannel constructs a BotLogsChannel.
//
//   - manager: the singleton WSManager that routes push messages to subscribed
//     clients. Must be non-nil.
//   - logger:  slog.Logger decorated with service-level fields. If nil,
//     slog.Default() is used as a safe fallback.
func NewBotLogsChannel(manager *WSManager, logger *slog.Logger) *BotLogsChannel {
	if logger == nil {
		logger = slog.Default()
	}
	return &BotLogsChannel{
		manager: manager,
		logger:  logger.With(slog.String("component", "bot_logs_channel")),
	}
}
