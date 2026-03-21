// ===================================================================
// QuantFlow — WebSocket Manager (Singleton)
// Task 3.4.4 — WebSocket Client Manager
// ===================================================================
//
// Core responsibilities:
//   1. Auth: connect via HttpOnly Cookie (browser sends automatically)
//   2. Heartbeat: auto-respond pong to server ping (websocket.md §1.3)
//   3. Exponential Backoff: 1s→2s→4s→8s→16s→30s cap (websocket.md §1.3)
//   4. Re-subscribe: restore all active subscriptions on reconnect
//   5. Event emitter: typed on/off/emit for downstream React hooks
//
// Spec: docs/api/websocket.md §1.1–1.3
// SRS: NFR-SEC-04, FR-MON
// ===================================================================

import type {
  WSConnectionState,
  WSChannelName,
  WSClientMessage,
  WSServerMessage,
  WSSubscriptionEntry,
  WSEventCallback,
  WSListenerMap,
} from "@/types/websocket";

// -----------------------------------------------------------------
// Constants (websocket.md §1.3)
// -----------------------------------------------------------------

/** Initial reconnection delay in milliseconds. */
const INITIAL_BACKOFF_MS = 1000;

/** Maximum reconnection delay in milliseconds (cap). */
const MAX_BACKOFF_MS = 30_000;

/** Close code 4001 = AUTH_FAILED (websocket.md §1.2). */
const CLOSE_CODE_AUTH_FAILED = 4001;

/** Close code for intentional disconnect (RFC 6455 Normal Closure). */
const CLOSE_CODE_NORMAL = 1000;

// -----------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------

/**
 * Build a stable string key for a subscription entry so we can deduplicate
 * subscriptions in a Map. Example: "market_ticker|symbol=BTCUSDT"
 */
function subscriptionKey(channel: WSChannelName, params?: Record<string, string>): string {
  if (!params || Object.keys(params).length === 0) return channel;
  const sorted = Object.entries(params)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
    .join("&");
  return `${channel}|${sorted}`;
}

// -----------------------------------------------------------------
// WSManager — Singleton Class
// -----------------------------------------------------------------

/**
 * WSManager manages a single WebSocket connection to the QuantFlow
 * backend. It handles authentication, heartbeat, automatic reconnection
 * with exponential backoff, and re-subscription.
 *
 * Usage:
 * ```ts
 * const ws = WSManager.getInstance();
 * ws.connect("ws://localhost:8080/ws/v1");
 * ws.on("market_ticker", (data) => { ... });
 * ws.subscribe("market_ticker", { symbol: "BTCUSDT" });
 * ```
 */
export class WSManager {
  // ── Singleton ──────────────────────────────────────────────────────
  private static instance: WSManager | null = null;

  static getInstance(): WSManager {
    if (!WSManager.instance) {
      WSManager.instance = new WSManager();
    }
    return WSManager.instance;
  }

  // ── State ──────────────────────────────────────────────────────────
  private ws: WebSocket | null = null;
  private url: string = "";
  private state: WSConnectionState = "disconnected";
  private reconnectAttempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private intentionalClose = false;

  /** Active subscriptions that will be re-sent on reconnect. */
  private activeSubscriptions = new Map<string, WSSubscriptionEntry>();

  /** Event listeners registered by consumers (hooks, components). */
  private listeners: WSListenerMap = new Map();

  /** Listeners for connection state changes. */
  private stateListeners = new Set<() => void>();

  private constructor() {
    // Private — use getInstance()
  }

  // ═══════════════════════════════════════════════════════════════════
  //  Public API
  // ═══════════════════════════════════════════════════════════════════

  /**
   * Open WebSocket connection to the given URL.
   * Auth is handled via HttpOnly Cookie sent automatically by the browser
   * during the WebSocket handshake (websocket.md §1.2 Method 2).
   */
  connect(url: string): void {
    // Avoid duplicate connections
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    this.url = url;
    this.intentionalClose = false;
    this.setState("connecting");

    try {
      this.ws = new WebSocket(url);
      this.ws.onopen = this.handleOpen;
      this.ws.onmessage = this.handleMessage;
      this.ws.onclose = this.handleClose;
      this.ws.onerror = this.handleError;
    } catch {
      // WebSocket constructor can throw on invalid URL
      this.scheduleReconnect();
    }
  }

  /**
   * Gracefully disconnect. No automatic reconnection will occur.
   */
  disconnect(): void {
    this.intentionalClose = true;
    this.clearReconnectTimer();

    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close(CLOSE_CODE_NORMAL, "Client disconnect");
      }
      this.ws = null;
    }

    this.activeSubscriptions.clear();
    this.setState("disconnected");
    this.reconnectAttempt = 0;
  }

  /**
   * Subscribe to a channel. The subscription is tracked so that on
   * reconnect all active subscriptions are automatically restored.
   */
  subscribe(channel: WSChannelName, params?: Record<string, string>): void {
    const key = subscriptionKey(channel, params);

    // Track the subscription for re-subscribe on reconnect
    this.activeSubscriptions.set(key, { channel, params });

    // Send immediately if connected
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.send({ action: "subscribe", channel, params });
    }
  }

  /**
   * Unsubscribe from a channel and stop tracking it.
   */
  unsubscribe(channel: WSChannelName, params?: Record<string, string>): void {
    const key = subscriptionKey(channel, params);
    this.activeSubscriptions.delete(key);

    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.send({ action: "unsubscribe", channel, params });
    }
  }

  /**
   * Register an event listener. Events correspond to the `event` field
   * in server messages (e.g. "market_ticker", "bot_log", "ping",
   * "subscribed", "error"), plus the internal "auth_failed" event.
   */
  on(event: string, callback: WSEventCallback): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(callback);
  }

  /** Remove an event listener. */
  off(event: string, callback: WSEventCallback): void {
    this.listeners.get(event)?.delete(callback);
  }

  /** Get the current connection state. */
  getState(): WSConnectionState {
    return this.state;
  }

  /**
   * Subscribe to connection state changes (used by useSyncExternalStore).
   * Returns an unsubscribe function.
   */
  subscribeState(callback: () => void): () => void {
    this.stateListeners.add(callback);
    return () => {
      this.stateListeners.delete(callback);
    };
  }

  // ═══════════════════════════════════════════════════════════════════
  //  WebSocket Event Handlers (arrow functions for stable `this`)
  // ═══════════════════════════════════════════════════════════════════

  private handleOpen = (): void => {
    this.reconnectAttempt = 0;
    this.setState("connected");

    // Re-subscribe all tracked channels (websocket.md §1.3 rule 1)
    for (const entry of this.activeSubscriptions.values()) {
      this.send({ action: "subscribe", channel: entry.channel, params: entry.params });
    }
  };

  private handleMessage = (event: MessageEvent): void => {
    let msg: WSServerMessage;
    try {
      msg = JSON.parse(event.data as string) as WSServerMessage;
    } catch {
      // Non-JSON message — ignore
      return;
    }

    // ── Heartbeat: auto-respond pong (websocket.md §1.3) ────────────
    if (msg.event === "ping") {
      this.send({
        event: "pong",
        timestamp: new Date().toISOString(),
      });
      return;
    }

    // ── Auth error → server will close with 4001 ────────────────────
    if (msg.event === "error" && "data" in msg && msg.data.code === "AUTH_FAILED") {
      this.emit("auth_failed", msg.data);
      return;
    }

    // ── All other events → dispatch to listeners ────────────────────
    this.emit(msg.event, msg);
  };

  private handleClose = (event: CloseEvent): void => {
    this.ws = null;

    // Auth failure — do not reconnect (websocket.md §1.2)
    if (event.code === CLOSE_CODE_AUTH_FAILED) {
      this.setState("disconnected");
      this.emit("auth_failed", { code: "AUTH_FAILED", message: event.reason });
      return;
    }

    // Intentional disconnect — do not reconnect
    if (this.intentionalClose) {
      this.setState("disconnected");
      return;
    }

    // Unexpected disconnect — reconnect with exponential backoff
    this.setState("reconnecting");
    this.scheduleReconnect();
  };

  private handleError = (): void => {
    // onclose will fire after onerror — let handleClose deal with reconnect.
    // Nothing extra to do here.
  };

  // ═══════════════════════════════════════════════════════════════════
  //  Internals
  // ═══════════════════════════════════════════════════════════════════

  /** Send a JSON message to the server. */
  private send(msg: WSClientMessage | { event: string; timestamp: string }): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  /** Emit an event to all registered listeners. */
  private emit(event: string, data: unknown): void {
    const callbacks = this.listeners.get(event);
    if (callbacks) {
      for (const cb of callbacks) {
        try {
          cb(data);
        } catch (err) {
          console.error(`[WSManager] Listener error for "${event}":`, err);
        }
      }
    }
  }

  /** Update connection state and notify state subscribers. */
  private setState(newState: WSConnectionState): void {
    if (this.state === newState) return;
    this.state = newState;

    // Notify all useSyncExternalStore subscribers
    for (const cb of this.stateListeners) {
      cb();
    }
  }

  /**
   * Schedule a reconnection attempt with exponential backoff.
   * Delay = min(2^attempt * 1000, 30000). (websocket.md §1.3)
   */
  private scheduleReconnect(): void {
    this.clearReconnectTimer();

    const delay = Math.min(
      INITIAL_BACKOFF_MS * Math.pow(2, this.reconnectAttempt),
      MAX_BACKOFF_MS,
    );

    this.reconnectTimer = setTimeout(() => {
      this.reconnectAttempt++;
      this.connect(this.url);
    }, delay);
  }

  /** Clear any pending reconnection timer. */
  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}
