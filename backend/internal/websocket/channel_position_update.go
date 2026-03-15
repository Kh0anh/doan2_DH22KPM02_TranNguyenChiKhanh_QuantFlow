package websocket

// channel_position_update.go — position_update WebSocket channel handler.
//
// PositionUpdateChannel polls Binance Futures REST API every 5 seconds for all
// Running Bots and fans out position/PnL/open-orders data to every subscribed
// WebSocket client.
//
// ─── Architecture ─────────────────────────────────────────────────────────────
//
// This is an ACTIVE channel — unlike the passive bot_logs channel, it owns a
// background goroutine that runs for the entire server lifetime:
//
//	Server startup → PositionUpdateChannel.Start(ctx)
//	                         │
//	                         └─ go runTicker(ctx, 5s)
//	                                  │ every 5 seconds
//	                                  ├── BotPositionFetcher.GetRunningBotsSnapshot(ctx)
//	                                  │       → []BotSnapshot{botID, botName, symbol,
//	                                  │                        status, totalPnL, proxy}
//	                                  │
//	                                  ├── for each BotSnapshot:
//	                                  │       proxy.GetPositionRisk(ctx, symbol)
//	                                  │       proxy.GetOpenOrders(ctx, symbol)
//	                                  │       marshal → botPositionUpdatePush{}
//	                                  │
//	                                  └── WSManager.PushPositionUpdate(payload)
//	                                              │  RLock fan-out
//	                                              ▼
//	                              for each Client: IsSubscribedToPositions() == true
//	                                  Client.enqueue(payload)
//	                                              │
//	                                              ▼
//	                              Client.WritePump → gorilla/websocket → Browser
//
// ─── Subscription lifecycle ───────────────────────────────────────────────────
//
// Subscribe / Unsubscribe messages are dispatched inside Client.handleSubscription()
// (client.go). The relevant case:
//
//	case "position_update":
//	    No params required (websocket.md §3.3).
//	    subscribe   → c.SubscribePositions()
//	    unsubscribe → c.UnsubscribePositions()
//	    ACK sent    → {"event":"subscribed","channel":"position_update","data":null}
//
// A single poll goroutine fans out to ALL subscribed clients simultaneously.
//
// ─── Dependency Inversion ─────────────────────────────────────────────────────
//
// To avoid a circular import cycle (websocket → logic → repository → domain),
// this file defines two interfaces:
//
//   - BotPositionFetcher: implemented by *logic.BotLogic (GetRunningBotsSnapshot).
//   - PositionProxy:      implemented by *exchange.BinanceProxy (GetPositionRisk,
//     GetOpenOrders).
//
// Go structural typing ensures no explicit declaration is needed in those packages.
//
// ─── Error handling ───────────────────────────────────────────────────────────
//
// Per-bot fetch errors are logged at Warn level and skipped — a single bot's
// exchange error must not block the push for other bots (resilient fan-out).
// If GetRunningBotsSnapshot itself fails, the tick is skipped entirely.
//
// ─── Payload format (websocket.md §3.3 — Event: position_update) ─────────────
//
//	{
//	  "event":   "position_update",
//	  "channel": "position_update",
//	  "data": {
//	    "bot_id":    "<UUID>",
//	    "bot_name":  "<string>",
//	    "symbol":    "<string>",
//	    "status":    "Running",
//	    "total_pnl": "<decimal string>",
//	    "position": {                // null when no open position
//	      "side":           "Long" | "Short",
//	      "entry_price":    <float64>,
//	      "quantity":       <float64>,
//	      "leverage":       <int>,
//	      "unrealized_pnl": <float64>,
//	      "margin_type":    "Isolated" | "Cross"
//	    },
//	    "open_orders": [
//	      {
//	        "order_id": "<string>",
//	        "side":     "Buy" | "Sell",
//	        "type":     "Limit" | "Market" | "Stop",
//	        "price":    <float64>,
//	        "quantity": <float64>,
//	        "status":   "New" | "PartiallyFilled" | ...
//	      }
//	    ],
//	    "timestamp": "<RFC3339 UTC>"
//	  }
//	}
//
// Task 2.8.4 — Channel position_update (position + PnL + open_orders; auto subscribe all Running Bots).
// WBS: P2-Backend · Category: WebSocket · 15/03/2026.
// SRS: FR-MONITOR-01 (Live Trade Dashboard), FR-MONITOR-02 (Position Panel), UC-09 Step 4.
// Referenced by: websocket.md §3.3, WBS 2.7.5 (BotLogic), WBS 2.8.1 (WSManager).

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// ─── Dependency-inversion interfaces ─────────────────────────────────────────

// BotPositionFetcher is the interface PositionUpdateChannel uses to obtain a
// snapshot of all currently Running Bots together with a live Binance proxy
// for each one.
//
// Passing nil to NewPositionUpdateChannel disables fetching (useful in tests).
// Use BotPositionFetcherFunc to construct an implementation from a closure.
type BotPositionFetcher interface {
	// GetRunningBotsSnapshot returns a snapshot of all Running Bots for the
	// current server process, ready for position data fetching.
	// Each BotSnapshot includes bot metadata and an authenticated Binance proxy.
	// Returns an empty slice (not an error) when no bots are running.
	GetRunningBotsSnapshot(ctx context.Context) ([]BotSnapshot, error)
}

// BotPositionFetcherFunc is a functional adapter that lets a plain function
// satisfy the BotPositionFetcher interface.
//
// Usage in router.go (avoids import cycle between websocket ↔ logic):
//
//	appws.BotPositionFetcherFunc(func(ctx context.Context) ([]appws.BotSnapshot, error) {
//	    snaps, err := botLogic.GetRunningBotsSnapshot(ctx)
//	    // ... convert logic.RunningBotSnapshot ─→ appws.BotSnapshot
//	    return result, err
//	})
type BotPositionFetcherFunc func(ctx context.Context) ([]BotSnapshot, error)

// GetRunningBotsSnapshot implements BotPositionFetcher by calling the underlying
// function.
func (f BotPositionFetcherFunc) GetRunningBotsSnapshot(ctx context.Context) ([]BotSnapshot, error) {
	return f(ctx)
}

// PositionProxy is the interface for fetching live position and order data from
// the Binance Futures REST API.
//
// Satisfied by *exchange.BinanceProxy (GetPositionRisk, GetOpenOrders).
type PositionProxy interface {
	// GetPositionRisk returns the current position risk for the given symbol.
	GetPositionRisk(ctx context.Context, symbol string) ([]*futures.PositionRisk, error)

	// GetOpenOrders returns all open orders for the given symbol.
	GetOpenOrders(ctx context.Context, symbol string) ([]*futures.Order, error)
}

// BotSnapshot is the per-bot data snapshot used by PositionUpdateChannel
// for a single polling cycle. It bundles static bot metadata with a
// live-authenticated exchange proxy, avoiding repeated DB queries inside
// the ticker loop.
type BotSnapshot struct {
	BotID    string
	UserID   string
	BotName  string
	Symbol   string
	Status   string
	TotalPnL string       // decimal string from bot_instances.total_pnl
	Proxy    PositionProxy // live BinanceProxy; nil disables Binance calls for this bot
}

// ─── Push payload types ───────────────────────────────────────────────────────

// botPositionUpdatePush is the top-level JSON envelope for the "position_update"
// push event (websocket.md §2.3, §3.3).
type botPositionUpdatePush struct {
	Event   string                `json:"event"`   // always "position_update"
	Channel string                `json:"channel"` // always "position_update"
	Data    positionUpdateData    `json:"data"`
}

// positionUpdateData is the data payload for the "position_update" push event
// (websocket.md §3.3).
type positionUpdateData struct {
	BotID      string              `json:"bot_id"`
	BotName    string              `json:"bot_name"`
	Symbol     string              `json:"symbol"`
	Status     string              `json:"status"`
	TotalPnL   string              `json:"total_pnl"`
	Position   *positionPayload    `json:"position"`    // null when no open position
	OpenOrders []openOrderPayload  `json:"open_orders"` // empty slice, never null
	Timestamp  string              `json:"timestamp"`   // RFC3339 UTC
}

// positionPayload carries per-position Binance Futures fields
// (websocket.md §3.3 — position object).
type positionPayload struct {
	Side          string  `json:"side"`           // "Long" or "Short"
	EntryPrice    float64 `json:"entry_price"`
	Quantity      float64 `json:"quantity"`
	Leverage      int     `json:"leverage"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	MarginType    string  `json:"margin_type"` // "Isolated" or "Cross"
}

// openOrderPayload carries per-order fields (websocket.md §3.3 — open_orders array).
type openOrderPayload struct {
	OrderID  string  `json:"order_id"`
	Side     string  `json:"side"`     // "Buy" or "Sell"
	Type     string  `json:"type"`     // "Limit", "Market", "Stop"
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Status   string  `json:"status"` // "New", "PartiallyFilled", etc.
}

// ─── PositionUpdateChannel ───────────────────────────────────────────────────

// PositionUpdateChannel manages the position_update WebSocket channel.
//
// It runs a single shared goroutine that polls Binance every tickInterval
// for all Running Bots and fans out the results via WSManager.PushPositionUpdate
// to all subscribed clients.
//
// PositionUpdateChannel is safe to start from a single goroutine (router.Setup).
type PositionUpdateChannel struct {
	manager      *WSManager
	fetcher      BotPositionFetcher
	tickInterval time.Duration
	logger       *slog.Logger
}

// NewPositionUpdateChannel constructs a PositionUpdateChannel.
//
//   - manager:      the singleton WSManager for fan-out push delivery. Must be non-nil.
//   - fetcher:      BotPositionFetcher implementation (e.g. *logic.BotLogic). If nil,
//     the ticker goroutine will skip Binance calls and push nothing.
//   - logger:       slog.Logger; slog.Default() is used when nil.
func NewPositionUpdateChannel(
	manager *WSManager,
	fetcher BotPositionFetcher,
	logger *slog.Logger,
) *PositionUpdateChannel {
	if logger == nil {
		logger = slog.Default()
	}
	return &PositionUpdateChannel{
		manager:      manager,
		fetcher:      fetcher,
		tickInterval: 5 * time.Second,
		logger:       logger.With(slog.String("component", "position_update_channel")),
	}
}

// Start launches the background polling goroutine.
// It should be called once on server startup (via `go ch.Start(ctx)` in router.go).
// The goroutine exits cleanly when ctx is cancelled (SIGTERM shutdown).
func (ch *PositionUpdateChannel) Start(ctx context.Context) {
	ch.logger.Info("position_update: channel started", slog.Duration("tick_interval", ch.tickInterval))
	ch.runTicker(ctx)
}

// ─── Internal polling loop ────────────────────────────────────────────────────

// runTicker is the main polling loop. It ticks every tickInterval and calls
// fetchAndPush to fetch Binance data and fan out push events.
// The loop exits when ctx is cancelled (application SIGTERM).
func (ch *PositionUpdateChannel) runTicker(ctx context.Context) {
	ticker := time.NewTicker(ch.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ch.logger.Info("position_update: ticker goroutine exiting (server shutdown)")
			return
		case <-ticker.C:
			ch.fetchAndPush(ctx)
		}
	}
}

// fetchAndPush executes one polling cycle:
//  1. Obtain a snapshot of all Running Bots via BotPositionFetcher.
//  2. For each bot: fetch position + open orders from Binance.
//  3. Marshal the payload and fan out via WSManager.PushPositionUpdate.
//
// Per-bot errors are logged at Warn level and skipped (resilient fan-out).
func (ch *PositionUpdateChannel) fetchAndPush(ctx context.Context) {
	if ch.fetcher == nil {
		return
	}

	snapshots, err := ch.fetcher.GetRunningBotsSnapshot(ctx)
	if err != nil {
		ch.logger.Warn("position_update: GetRunningBotsSnapshot failed — skipping tick",
			slog.Any("error", err),
		)
		return
	}

	if len(snapshots) == 0 {
		// No running bots — nothing to push.
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for _, snap := range snapshots {
		payload, buildErr := ch.buildPayloadForBot(ctx, snap, now)
		if buildErr != nil {
			ch.logger.Warn("position_update: skipping bot due to fetch error",
				slog.String("bot_id", snap.BotID),
				slog.String("symbol", snap.Symbol),
				slog.Any("error", buildErr),
			)
			continue
		}

		ch.manager.PushPositionUpdate(payload)
	}
}

// buildPayloadForBot fetches position + open orders for one bot snapshot,
// builds the JSON push frame, and returns the serialised bytes.
func (ch *PositionUpdateChannel) buildPayloadForBot(ctx context.Context, snap BotSnapshot, timestamp string) ([]byte, error) {
	var pos *positionPayload
	var orders []openOrderPayload

	if snap.Proxy != nil {
		// ── Fetch position risk ──────────────────────────────────────────────
		risks, posErr := snap.Proxy.GetPositionRisk(ctx, snap.Symbol)
		if posErr != nil {
			return nil, posErr
		}
		pos = buildPositionPayload(risks, snap.Symbol)

		// ── Fetch open orders ────────────────────────────────────────────────
		rawOrders, ordErr := snap.Proxy.GetOpenOrders(ctx, snap.Symbol)
		if ordErr != nil {
			return nil, ordErr
		}
		orders = buildOpenOrdersPayload(rawOrders)
	} else {
		orders = []openOrderPayload{}
	}

	push := botPositionUpdatePush{
		Event:   "position_update",
		Channel: "position_update",
		Data: positionUpdateData{
			BotID:      snap.BotID,
			BotName:    snap.BotName,
			Symbol:     snap.Symbol,
			Status:     snap.Status,
			TotalPnL:   snap.TotalPnL,
			Position:   pos,
			OpenOrders: orders,
			Timestamp:  timestamp,
		},
	}

	return json.Marshal(push)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// buildPositionPayload converts a slice of *futures.PositionRisk from the
// Binance API into a *positionPayload for the push frame.
//
// Returns nil when there is no open position for the symbol (positionAmt == 0).
func buildPositionPayload(risks []*futures.PositionRisk, symbol string) *positionPayload {
	for _, r := range risks {
		if r.Symbol != symbol {
			continue
		}

		posAmt, err := strconv.ParseFloat(r.PositionAmt, 64)
		if err != nil || posAmt == 0 {
			// Zero position — no open position for this symbol.
			return nil
		}

		entryPrice, _ := strconv.ParseFloat(r.EntryPrice, 64)
		unrealizedPnL, _ := strconv.ParseFloat(r.UnRealizedProfit, 64)
		leverage, _ := strconv.Atoi(r.Leverage)

		// Determine side from position amount sign.
		side := "Long"
		quantity := posAmt
		if posAmt < 0 {
			side = "Short"
			quantity = -posAmt // report absolute quantity
		}

		// Normalise Binance margin type: "isolated" → "Isolated", "crossed"→"Cross".
		marginType := normaliseMarginType(r.MarginType)

		return &positionPayload{
			Side:          side,
			EntryPrice:    entryPrice,
			Quantity:      quantity,
			Leverage:      leverage,
			UnrealizedPnL: unrealizedPnL,
			MarginType:    marginType,
		}
	}
	return nil
}

// buildOpenOrdersPayload converts a slice of *futures.Order from the Binance
// API into the []openOrderPayload slice for the push frame.
//
// Returns an empty (non-nil) slice when there are no open orders.
func buildOpenOrdersPayload(rawOrders []*futures.Order) []openOrderPayload {
	result := make([]openOrderPayload, 0, len(rawOrders))
	for _, o := range rawOrders {
		price, _ := strconv.ParseFloat(o.Price, 64)
		qty, _ := strconv.ParseFloat(o.OrigQuantity, 64)

		result = append(result, openOrderPayload{
			OrderID:  strconv.FormatInt(o.OrderID, 10),
			Side:     string(o.Side),
			Type:     string(o.Type),
			Price:    price,
			Quantity: qty,
			Status:   string(o.Status),
		})
	}
	return result
}

// normaliseMarginType converts Binance's lowercase margin type strings
// ("isolated", "crossed") to the display values expected by websocket.md §3.3
// ("Isolated", "Cross").
func normaliseMarginType(raw string) string {
	switch strings.ToLower(raw) {
	case "isolated":
		return "Isolated"
	case "crossed", "cross":
		return "Cross"
	default:
		return raw
	}
}
