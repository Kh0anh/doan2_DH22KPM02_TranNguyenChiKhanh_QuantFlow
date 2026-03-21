// ===================================================================
// QuantFlow — WebSocket Message Types
// Task 3.4.4 — WebSocket Client Manager
// ===================================================================
//
// Type definitions for all WebSocket messages exchanged between
// the frontend client and backend server.
//
// Spec: docs/api/websocket.md
// SRS: NFR-SEC-04, FR-MON
// ===================================================================

// -----------------------------------------------------------------
// Connection State
// -----------------------------------------------------------------

/** Observable connection state exposed by WSManager. */
export type WSConnectionState =
  | "disconnected"
  | "connecting"
  | "connected"
  | "reconnecting";

// -----------------------------------------------------------------
// Channel names (websocket.md §3)
// -----------------------------------------------------------------

export type WSChannelName = "market_ticker" | "bot_logs" | "position_update";

// -----------------------------------------------------------------
// Client → Server Messages (websocket.md §2.1)
// -----------------------------------------------------------------

/** Subscribe request sent from client to server. */
export interface WSClientSubscribe {
  action: "subscribe";
  channel: WSChannelName;
  params?: Record<string, string>;
}

/** Unsubscribe request sent from client to server. */
export interface WSClientUnsubscribe {
  action: "unsubscribe";
  channel: WSChannelName;
  params?: Record<string, string>;
}

/** Pong heartbeat response (websocket.md §1.3). */
export interface WSClientPong {
  event: "pong";
  timestamp: string;
}

export type WSClientMessage =
  | WSClientSubscribe
  | WSClientUnsubscribe
  | WSClientPong;

// -----------------------------------------------------------------
// Server → Client Messages (websocket.md §2.2–2.3)
// -----------------------------------------------------------------

/** Server confirms subscription (websocket.md §2.2). */
export interface WSSubscribedEvent {
  event: "subscribed";
  channel: WSChannelName;
  params?: Record<string, string>;
}

/** Server confirms unsubscription (websocket.md §2.2). */
export interface WSUnsubscribedEvent {
  event: "unsubscribed";
  channel: WSChannelName;
  params?: Record<string, string>;
}

/** Server error message (websocket.md §2.2). */
export interface WSErrorEvent {
  event: "error";
  data: {
    code: string;
    message: string;
  };
}

/** Server heartbeat ping (websocket.md §1.3). */
export interface WSPingEvent {
  event: "ping";
  timestamp: string;
}

/** Generic server push event (websocket.md §2.3). */
export interface WSServerPushEvent {
  event: string;
  channel: WSChannelName;
  data: Record<string, unknown>;
}

/** Union of all possible server messages. */
export type WSServerMessage =
  | WSSubscribedEvent
  | WSUnsubscribedEvent
  | WSErrorEvent
  | WSPingEvent
  | WSServerPushEvent;

// -----------------------------------------------------------------
// Subscription tracking (internal to WSManager)
// -----------------------------------------------------------------

/** A tracked subscription entry for re-subscribe on reconnect. */
export interface WSSubscriptionEntry {
  channel: WSChannelName;
  params?: Record<string, string>;
}

// -----------------------------------------------------------------
// Event listener types
// -----------------------------------------------------------------

/** Callback for WSManager event listeners. */
export type WSEventCallback = (data: unknown) => void;

/** Map of event names to sets of callbacks. */
export type WSListenerMap = Map<string, Set<WSEventCallback>>;
