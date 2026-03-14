package websocket

// manager.go — WebSocket Connection Manager.
//
// WSManager maintains the registry of active Client connections and routes
// push events to the correct subscribers.
//
// Task 2.8.1 — WebSocket Connection Manager (JWT Auth + Heartbeat 30s).
// Referenced by: Task 2.7.4 (bot_logs push), Task 2.8.2-2.8.4 (channel routing).

import (
	"log/slog"
	"sync"
)

// WSManager maintains the set of active Client connections and provides
// channel-specific push methods used by background workers and bot goroutines.
//
// ─── Concurrency Model ───────────────────────────────────────────────────
//
//   - clients map: guarded by mu.
//     RLock for fan-out reads — concurrent pushes from multiple goroutines do
//     not block each other.
//     Lock for RegisterClient / UnregisterClient — exclusive mutation.
//
//   - UnregisterClient removes the client from the map before closing its
//     send channel, guaranteeing that no concurrent push call will enqueue
//     to a closed channel.
//
// WSManager is safe for concurrent use from multiple goroutines.
type WSManager struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	logger  *slog.Logger
}

// NewWSManager constructs a WSManager.
//
//   - logger: slog.Logger decorated with service-level fields. If nil,
//     slog.Default() is used as a safe fallback.
func NewWSManager(logger *slog.Logger) *WSManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &WSManager{
		clients: make(map[*Client]struct{}),
		logger:  logger,
	}
}

// RegisterClient adds c to the active client registry.
// Called by WSHandler immediately after a successful WebSocket upgrade and
// JWT authentication.
func (m *WSManager) RegisterClient(c *Client) {
	m.mu.Lock()
	m.clients[c] = struct{}{}
	m.mu.Unlock()
	m.logger.Debug("ws: client registered", slog.String("user_id", c.UserID()))
}

// UnregisterClient removes c from the active client registry and closes its
// send channel so the associated writePump goroutine terminates.
//
// The client is removed from the map BEFORE the send channel is closed.
// This sequencing guarantees that no concurrent push call can enqueue to
// the closed channel: push methods hold m.mu.RLock() during fan-out, so they
// cannot observe c after the exclusive Lock has already deleted it.
func (m *WSManager) UnregisterClient(c *Client) {
	m.mu.Lock()
	if _, exists := m.clients[c]; exists {
		delete(m.clients, c)
		m.mu.Unlock()
		close(c.send)
	} else {
		m.mu.Unlock()
	}
	m.logger.Debug("ws: client unregistered", slog.String("user_id", c.UserID()))
}

// ─── Push Methods ─────────────────────────────────────────────────────────────

// PushBotLog fans out payload to every Client that has subscribed to the
// bot_logs channel for botID.
//
// Delivery is non-blocking per client (see Client.enqueue). A slow client
// whose send buffer is full has its frame silently dropped; it can catch up
// via the REST pagination endpoint (GET /bots/{botId}/logs, Task 2.7.7).
//
// PushBotLog is called from bot goroutines (engine/bot/logger.go) and
// satisfies the BotLogPusher interface defined in engine/bot/logger.go.
func (m *WSManager) PushBotLog(botID string, payload []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for c := range m.clients {
		if c.IsSubscribedToBot(botID) {
			c.enqueue(payload)
		}
	}
}

// PushToSubscribersForSymbol fans out payload to every Client that has
// subscribed to the market_ticker channel for symbol.
// Called by the market_ticker channel handler (Task 2.8.2) when a new
// ticker or candle event arrives from the Binance WS stream.
func (m *WSManager) PushToSubscribersForSymbol(symbol string, payload []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for c := range m.clients {
		if c.IsSubscribedToSymbol(symbol) {
			c.enqueue(payload)
		}
	}
}

// PushPositionUpdate fans out payload to every Client that has subscribed to
// the position_update channel.
// Called by the position_update channel handler (Task 2.8.4) on each position
// or PnL change for running bots.
func (m *WSManager) PushPositionUpdate(payload []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for c := range m.clients {
		if c.IsSubscribedToPositions() {
			c.enqueue(payload)
		}
	}
}
