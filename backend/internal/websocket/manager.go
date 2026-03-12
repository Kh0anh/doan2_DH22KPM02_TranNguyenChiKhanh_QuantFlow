package websocket

// manager.go — WebSocket Connection Manager.
//
// WSManager maintains the registry of active Client connections and routes
// push events to the correct subscribers. In Task 2.7.4 the manager exposes
// only PushBotLog; the full connection lifecycle (HTTP upgrade, JWT auth,
// Ping/Pong heartbeat, Subscribe/Unsubscribe message routing) is completed
// in Task 2.8.1.
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
//     RLock for PushBotLog fan-out — concurrent pushes from multiple bot
//     goroutines do not block each other.
//     Lock for RegisterClient / UnregisterClient — exclusive mutation.
//
//   - UnregisterClient removes the client from the map before closing its
//     send channel, guaranteeing that no concurrent PushBotLog will enqueue
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
// Called by the HTTP upgrade handler (Task 2.8.1) immediately after a
// successful WebSocket handshake and JWT authentication.
func (m *WSManager) RegisterClient(c *Client) {
	m.mu.Lock()
	m.clients[c] = struct{}{}
	m.mu.Unlock()
	m.logger.Debug("ws: client registered", slog.String("user_id", c.UserID()))
}

// UnregisterClient removes c from the active client registry and closes its
// send channel so the associated write-pump goroutine (Task 2.8.1) terminates.
//
// The client is removed from the map BEFORE the send channel is closed.
// This sequencing guarantees that no concurrent PushBotLog call can enqueue
// to the closed channel: PushBotLog holds m.mu.RLock() during fan-out, so it
// cannot observe c after the exclusive Lock has deleted it.
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
