package websocket

// client.go — WebSocket Client abstraction.
//
// Client represents a single authenticated WebSocket connection and its
// channel subscription state. In Task 2.7.4 the struct focuses on subscription
// tracking and outbound message queuing; the gorilla/websocket connection and
// the read/write pump goroutines are wired in Task 2.8.1 when the HTTP upgrade
// handler (ws_handler.go) is implemented.
//
// Task 2.8.1 — WebSocket Connection Manager (JWT Auth + Heartbeat).
// Referenced by: Task 2.7.4 (bot_logs push), Task 2.8.3 (channel routing).

import "sync"

// sendBufferSize is the capacity of each Client's outbound send channel.
// Sized to absorb short bursts (e.g. multiple bots firing simultaneously)
// without blocking PushBotLog for the entire subscriber set.
const sendBufferSize = 64

// Client represents a single authenticated WebSocket connection.
//
// Subscription state is guarded by mu so that concurrent Subscribe/Unsubscribe
// calls (from the message-routing goroutine, Task 2.8.1) and concurrent read
// checks (from PushBotLog, called by bot goroutines) are safe.
//
// The send channel is drained by the write-pump goroutine (Task 2.8.1).
// WSManager.UnregisterClient closes it to signal the pump to exit cleanly.
type Client struct {
	// userID is the UUID of the authenticated user who owns this connection.
	// Used by WSManager to scope bot_logs delivery to bots the user owns.
	userID string

	// send is the outbound message queue. WSManager.PushBotLog (and future
	// channel push methods) enqueue pre-serialised JSON frames here.
	// The write-pump goroutine (Task 2.8.1) drains the channel and writes
	// bytes to the underlying gorilla websocket connection.
	send chan []byte

	// subscribedBots is the set of bot UUIDs for which this client has
	// subscribed to the bot_logs channel. Key = bot UUID, value = empty struct.
	subscribedBots map[string]struct{}
	mu             sync.RWMutex
}

// NewClient constructs a Client for the given authenticated user.
func NewClient(userID string) *Client {
	return &Client{
		userID:         userID,
		send:           make(chan []byte, sendBufferSize),
		subscribedBots: make(map[string]struct{}),
	}
}

// UserID returns the authenticated user ID associated with this connection.
func (c *Client) UserID() string { return c.userID }

// Send returns a read-only view of the outbound channel. The write-pump
// goroutine (Task 2.8.1) listens here to forward frames to the WS connection.
func (c *Client) Send() <-chan []byte { return c.send }

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

// IsSubscribedToBot reports whether the client is currently subscribed to
// bot_logs events for botID.
func (c *Client) IsSubscribedToBot(botID string) bool {
	c.mu.RLock()
	_, ok := c.subscribedBots[botID]
	c.mu.RUnlock()
	return ok
}

// enqueue attempts a non-blocking write to the send channel.
// If the buffer is full (slow consumer), the frame is silently dropped.
// This prevents a single lagging client from stalling PushBotLog delivery
// to all other subscribers.
func (c *Client) enqueue(msg []byte) {
	select {
	case c.send <- msg:
	default:
		// Buffer full — frame dropped for this client.
	}
}
