package websocket

// client.go — WebSocket Client abstraction.
//
// Client represents a single authenticated WebSocket connection, its outbound
// message queue, and its channel-subscription state.
//
// Each Client runs two goroutines:
//   - writePump: drains the send channel and forwards JSON frames to the
//     underlying gorilla/websocket connection. Also manages the application-
//     level heartbeat: a 30-second ticker triggers a JSON ping message; if the
//     client does not respond with a JSON pong within 10 seconds the connection
//     is closed (websocket.md §1.3).
//   - readPump: reads incoming JSON frames from the connection and dispatches
//     subscribe / unsubscribe / pong actions.  When the read loop exits
//     (network error, client close, pong timeout), it calls
//     manager.UnregisterClient() to clean up and close the send channel,
//     which signals writePump to exit as well.
//
// Task 2.8.1 — WebSocket Connection Manager (JWT Auth + Heartbeat 30s).
// Referenced by: Task 2.7.4 (PushBotLog), Task 2.8.2-2.8.4 (channel routing).

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ─── Timing constants ────────────────────────────────────────────────────────

const (
	// sendBufferSize is the capacity of each Client's outbound message channel.
	// Sized to absorb short bursts without blocking fan-out callers.
	sendBufferSize = 64

	// pingInterval is how often writePump sends an application-level JSON ping.
	pingInterval = 30 * time.Second

	// pongDeadline is the maximum time the server waits for a JSON pong after
	// sending a JSON ping before closing the connection.
	pongDeadline = 10 * time.Second

	// writeDeadline is the per-write timeout for the gorilla connection.
	writeDeadline = 10 * time.Second

	// maxMessageSize limits incoming JSON messages from the client.
	maxMessageSize = 4096
)

// ─── Incoming message schema ─────────────────────────────────────────────────

// clientMessage is the top-level JSON structure for every message the client
// sends to the server (subscribe, unsubscribe, pong).
type clientMessage struct {
	Action  string          `json:"action"`
	Event   string          `json:"event"`
	Channel string          `json:"channel"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// subscribeParams is used to decode the "params" field for subscribe and
// unsubscribe requests (websocket.md §2.1).
type subscribeParams struct {
	Symbol string `json:"symbol,omitempty"`
	BotID  string `json:"bot_id,omitempty"`
}

// ─── Outgoing message helpers ────────────────────────────────────────────────

// serverEvent is the generic outgoing envelope used for ping, subscribed,
// unsubscribed, and error events.
type serverEvent struct {
	Event   string `json:"event"`
	Channel string `json:"channel,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// pingPayload is the payload of the application-level ping event
// (websocket.md §1.3).
type pingPayload struct {
	Timestamp string `json:"timestamp"`
}

// errorPayload carries a machine-readable code and a human message for error
// events (websocket.md §2.2).
type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ─── Client ──────────────────────────────────────────────────────────────────

// Client represents a single authenticated WebSocket connection.
//
// Subscription state is guarded by mu so that concurrent Subscribe /
// Unsubscribe calls (from readPump) and concurrent read checks (from push
// fan-out methods) are safe.
//
// The send channel is drained by writePump.  WSManager.UnregisterClient
// closes it to signal writePump to exit cleanly.
type Client struct {
	// userID is the UUID of the authenticated user who owns this connection.
	userID string

	// conn is the underlying gorilla/websocket connection.
	conn *websocket.Conn

	// send is the outbound message queue.  All push fan-out methods enqueue
	// pre-serialised JSON frames here.  writePump drains the channel.
	send chan []byte

	// pongReceived guards the application-level pong acknowledgement.
	// writePump sets a read deadline of (now + pongDeadline) after sending a
	// ping; readPump resets it to zero when it processes a pong event.
	pongReceived chan struct{}

	// subscribedBots is the set of bot UUIDs the client has subscribed to
	// for bot_logs events.  Key = bot UUID.
	subscribedBots map[string]struct{}

	// subscribedSymbols is the set of symbols the client has subscribed to
	// for market_ticker events.  Key = symbol string (e.g. "BTCUSDT").
	subscribedSymbols map[string]struct{}

	// subscribedPositions tracks whether the client has subscribed to the
	// position_update channel.
	subscribedPositions bool

	mu     sync.RWMutex
	logger *slog.Logger
}

// NewClient constructs a Client for the given authenticated user and
// gorilla/websocket connection.
func NewClient(userID string, conn *websocket.Conn, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		userID:            userID,
		conn:              conn,
		send:              make(chan []byte, sendBufferSize),
		pongReceived:      make(chan struct{}, 1),
		subscribedBots:    make(map[string]struct{}),
		subscribedSymbols: make(map[string]struct{}),
		logger:            logger.With(slog.String("user_id", userID)),
	}
}

// UserID returns the authenticated user ID for this connection.
func (c *Client) UserID() string { return c.userID }

// Send returns a read-only view of the outbound channel (used in tests).
func (c *Client) Send() <-chan []byte { return c.send }

// ─── Subscription management ─────────────────────────────────────────────────

// SubscribeBot registers the client to receive bot_logs events for botID.
func (c *Client) SubscribeBot(botID string) {
	c.mu.Lock()
	c.subscribedBots[botID] = struct{}{}
	c.mu.Unlock()
}

// UnsubscribeBot removes the bot_logs subscription for botID.
func (c *Client) UnsubscribeBot(botID string) {
	c.mu.Lock()
	delete(c.subscribedBots, botID)
	c.mu.Unlock()
}

// IsSubscribedToBot reports whether this client is subscribed to bot_logs for botID.
func (c *Client) IsSubscribedToBot(botID string) bool {
	c.mu.RLock()
	_, ok := c.subscribedBots[botID]
	c.mu.RUnlock()
	return ok
}

// SubscribeSymbol registers the client to receive market_ticker events for symbol.
func (c *Client) SubscribeSymbol(symbol string) {
	c.mu.Lock()
	c.subscribedSymbols[symbol] = struct{}{}
	c.mu.Unlock()
}

// UnsubscribeSymbol removes the market_ticker subscription for symbol.
func (c *Client) UnsubscribeSymbol(symbol string) {
	c.mu.Lock()
	delete(c.subscribedSymbols, symbol)
	c.mu.Unlock()
}

// IsSubscribedToSymbol reports whether this client is subscribed to market_ticker for symbol.
func (c *Client) IsSubscribedToSymbol(symbol string) bool {
	c.mu.RLock()
	_, ok := c.subscribedSymbols[symbol]
	c.mu.RUnlock()
	return ok
}

// SubscribePositions registers the client to receive position_update events.
func (c *Client) SubscribePositions() {
	c.mu.Lock()
	c.subscribedPositions = true
	c.mu.Unlock()
}

// UnsubscribePositions removes the position_update subscription.
func (c *Client) UnsubscribePositions() {
	c.mu.Lock()
	c.subscribedPositions = false
	c.mu.Unlock()
}

// IsSubscribedToPositions reports whether this client is subscribed to position_update.
func (c *Client) IsSubscribedToPositions() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscribedPositions
}

// ─── Outbound queue ───────────────────────────────────────────────────────────

// enqueue attempts a non-blocking write to the send channel.
// If the buffer is full (slow consumer) the frame is silently dropped.
func (c *Client) enqueue(msg []byte) {
	select {
	case c.send <- msg:
	default:
		c.logger.Debug("ws: send buffer full — frame dropped")
	}
}

// marshalAndEnqueue serialises v to JSON and enqueues the result.
// Serialisation errors are logged and the frame is discarded.
func (c *Client) marshalAndEnqueue(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		c.logger.Error("ws: marshal outgoing event", slog.Any("error", err))
		return
	}
	c.enqueue(b)
}

// ─── Pump goroutines ──────────────────────────────────────────────────────────

// WritePump drains the send channel and writes frames to the WS connection.
// It also manages the application-level heartbeat:
//   - A ticker fires every pingInterval (30s).
//   - WritePump serialises and writes a JSON ping event.
//   - It then waits up to pongDeadline (10s) for ReadPump to signal a pong.
//   - If the deadline expires without a pong the connection is closed.
//
// WritePump exits when the send channel is closed (by UnregisterClient) or
// when a write error occurs.  On exit it closes the underlying connection.
func (c *Client) WritePump(manager *WSManager) {
	defer func() {
		c.conn.Close()
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if !ok {
				// send channel closed — graceful shutdown.
				c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.logger.Debug("ws: write error", slog.Any("error", err))
				return
			}

		case <-ticker.C:
			// Send application-level JSON ping.
			ping := serverEvent{
				Event: "ping",
				Data:  pingPayload{Timestamp: time.Now().UTC().Format(time.RFC3339)},
			}
			b, err := json.Marshal(ping)
			if err != nil {
				c.logger.Error("ws: marshal ping", slog.Any("error", err))
				continue
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				c.logger.Debug("ws: ping write error", slog.Any("error", err))
				return
			}

			// Wait for pong within pongDeadline.
			pongTimer := time.NewTimer(pongDeadline)
			select {
			case <-c.pongReceived:
				pongTimer.Stop()
			case <-pongTimer.C:
				c.logger.Debug("ws: pong timeout — closing connection")
				manager.UnregisterClient(c)
				return
			}
		}
	}
}

// ReadPump reads incoming JSON frames from the WS connection and dispatches
// action events (subscribe, unsubscribe, pong).  It runs on the goroutine that
// called it (the HTTP handler goroutine) and blocks until the connection is
// closed or an error occurs, at which point it calls manager.UnregisterClient().
func (c *Client) ReadPump(manager *WSManager) {
	defer func() {
		manager.UnregisterClient(c)
	}()

	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Debug("ws: unexpected close", slog.Any("error", err))
			}
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.marshalAndEnqueue(serverEvent{
				Event: "error",
				Data:  errorPayload{Code: "INVALID_JSON", Message: "Message must be valid JSON."},
			})
			continue
		}

		// Handle application-level pong — signal writePump.
		if msg.Event == "pong" {
			select {
			case c.pongReceived <- struct{}{}:
			default:
			}
			continue
		}

		// Handle subscribe / unsubscribe actions.
		switch msg.Action {
		case "subscribe", "unsubscribe":
			c.handleSubscription(msg)
		default:
			if msg.Action != "" {
				c.marshalAndEnqueue(serverEvent{
					Event: "error",
					Data:  errorPayload{Code: "INVALID_ACTION", Message: "action must be 'subscribe' or 'unsubscribe'."},
				})
			}
		}
	}
}

// handleSubscription processes subscribe / unsubscribe requests from the
// client and sends a confirmation or error reply.
func (c *Client) handleSubscription(msg clientMessage) {
	isSubscribe := msg.Action == "subscribe"

	switch msg.Channel {
	case "market_ticker":
		var p subscribeParams
		if len(msg.Params) > 0 {
			if err := json.Unmarshal(msg.Params, &p); err != nil || p.Symbol == "" {
				c.marshalAndEnqueue(serverEvent{
					Event: "error",
					Data:  errorPayload{Code: "MISSING_PARAMS", Message: "market_ticker requires params.symbol."},
				})
				return
			}
		} else {
			c.marshalAndEnqueue(serverEvent{
				Event: "error",
				Data:  errorPayload{Code: "MISSING_PARAMS", Message: "market_ticker requires params.symbol."},
			})
			return
		}
		if isSubscribe {
			c.SubscribeSymbol(p.Symbol)
		} else {
			c.UnsubscribeSymbol(p.Symbol)
		}
		c.sendSubscriptionAck(msg.Action, msg.Channel, map[string]string{"symbol": p.Symbol})

	case "bot_logs":
		var p subscribeParams
		if len(msg.Params) > 0 {
			if err := json.Unmarshal(msg.Params, &p); err != nil || p.BotID == "" {
				c.marshalAndEnqueue(serverEvent{
					Event: "error",
					Data:  errorPayload{Code: "MISSING_PARAMS", Message: "bot_logs requires params.bot_id."},
				})
				return
			}
		} else {
			c.marshalAndEnqueue(serverEvent{
				Event: "error",
				Data:  errorPayload{Code: "MISSING_PARAMS", Message: "bot_logs requires params.bot_id."},
			})
			return
		}
		if isSubscribe {
			c.SubscribeBot(p.BotID)
		} else {
			c.UnsubscribeBot(p.BotID)
		}
		c.sendSubscriptionAck(msg.Action, msg.Channel, map[string]string{"bot_id": p.BotID})

	case "position_update":
		if isSubscribe {
			c.SubscribePositions()
		} else {
			c.UnsubscribePositions()
		}
		c.sendSubscriptionAck(msg.Action, msg.Channel, nil)

	default:
		if msg.Channel == "" {
			c.marshalAndEnqueue(serverEvent{
				Event: "error",
				Data:  errorPayload{Code: "INVALID_CHANNEL", Message: "channel is required."},
			})
		} else {
			c.marshalAndEnqueue(serverEvent{
				Event: "error",
				Data:  errorPayload{Code: "INVALID_CHANNEL", Message: "Unknown channel: " + msg.Channel},
			})
		}
	}
}

// sendSubscriptionAck sends a subscribed / unsubscribed confirmation to the client.
func (c *Client) sendSubscriptionAck(action, channel string, params any) {
	eventName := "subscribed"
	if action == "unsubscribe" {
		eventName = "unsubscribed"
	}
	c.marshalAndEnqueue(serverEvent{
		Event:   eventName,
		Channel: channel,
		Data:    params,
	})
}
