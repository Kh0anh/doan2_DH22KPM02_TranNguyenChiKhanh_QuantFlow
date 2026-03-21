// ===================================================================
// QuantFlow — WebSocket Channel Helpers
// Task 3.4.5 — Real-time Subscriptions (3 channels)
// ===================================================================
//
// Type-safe payload parsers for each WebSocket server event.
// These convert raw `data: Record<string, unknown>` payloads from
// WSManager into typed objects for consumption by React hooks.
//
// Channels (websocket.md §3):
//   1. market_ticker → events: market_ticker, market_candle
//   2. bot_logs      → events: bot_log
//   3. position_update → events: position_update, bot_status_change, bot_error
//
// SRS: FR-MON-01, FR-MON-02, FR-MON-03, FR-RUN-05
// ===================================================================

// -----------------------------------------------------------------
// Channel 1: market_ticker (websocket.md §3.1)
// -----------------------------------------------------------------

/** Parsed market_ticker event payload. */
export interface MarketTickerPayload {
  symbol: string;
  lastPrice: number;
  priceChangePercent: number;
  high24h: number;
  low24h: number;
  volume24h: number;
  timestamp: string;
}

/** Parse raw market_ticker event data. */
export function parseMarketTicker(msg: unknown): MarketTickerPayload | null {
  const m = msg as Record<string, unknown>;
  const d = m?.data as Record<string, unknown> | undefined;
  if (!d || typeof d.symbol !== "string") return null;

  return {
    symbol: d.symbol as string,
    lastPrice: Number(d.last_price ?? 0),
    priceChangePercent: Number(d.price_change_percent ?? 0),
    high24h: Number(d.high_24h ?? 0),
    low24h: Number(d.low_24h ?? 0),
    volume24h: Number(d.volume_24h ?? 0),
    timestamp: (d.timestamp as string) ?? new Date().toISOString(),
  };
}

/** Parsed market_candle event payload. */
export interface MarketCandlePayload {
  symbol: string;
  timeframe: string;
  candle: {
    openTime: string;
    open: number;
    high: number;
    low: number;
    close: number;
    volume: number;
    isClosed: boolean;
  };
}

/** Parse raw market_candle event data. */
export function parseMarketCandle(msg: unknown): MarketCandlePayload | null {
  const m = msg as Record<string, unknown>;
  const d = m?.data as Record<string, unknown> | undefined;
  if (!d || typeof d.symbol !== "string") return null;

  const c = d.candle as Record<string, unknown> | undefined;
  if (!c) return null;

  return {
    symbol: d.symbol as string,
    timeframe: (d.timeframe as string) ?? "15m",
    candle: {
      openTime: (c.open_time as string) ?? "",
      open: Number(c.open ?? 0),
      high: Number(c.high ?? 0),
      low: Number(c.low ?? 0),
      close: Number(c.close ?? 0),
      volume: Number(c.volume ?? 0),
      isClosed: Boolean(c.is_closed),
    },
  };
}

// -----------------------------------------------------------------
// Channel 2: bot_logs (websocket.md §3.2)
// -----------------------------------------------------------------

/** Parsed bot_log event payload. */
export interface BotLogPayload {
  botId: string;
  log: {
    id: number;
    actionDecision: string;
    message: string;
    createdAt: string;
  };
}

/** Parse raw bot_log event data. */
export function parseBotLog(msg: unknown): BotLogPayload | null {
  const m = msg as Record<string, unknown>;
  const d = m?.data as Record<string, unknown> | undefined;
  if (!d || typeof d.bot_id !== "string") return null;

  const log = d.log as Record<string, unknown> | undefined;
  if (!log) return null;

  return {
    botId: d.bot_id as string,
    log: {
      id: Number(log.id ?? 0),
      actionDecision: (log.action_decision as string) ?? "",
      message: (log.message as string) ?? "",
      createdAt: (log.created_at as string) ?? new Date().toISOString(),
    },
  };
}

// -----------------------------------------------------------------
// Channel 3: position_update (websocket.md §3.3)
// -----------------------------------------------------------------

/** Parsed position_update event payload. */
export interface PositionUpdatePayload {
  botId: string;
  botName: string;
  symbol: string;
  status: string;
  totalPnl: number;
  position: {
    side: string;
    entryPrice: number;
    quantity: number;
    leverage: number;
    unrealizedPnl: number;
    marginType: string;
  } | null;
  timestamp: string;
}

/** Parse raw position_update event data. */
export function parsePositionUpdate(msg: unknown): PositionUpdatePayload | null {
  const m = msg as Record<string, unknown>;
  const d = m?.data as Record<string, unknown> | undefined;
  if (!d || typeof d.bot_id !== "string") return null;

  const pos = d.position as Record<string, unknown> | undefined;

  return {
    botId: d.bot_id as string,
    botName: (d.bot_name as string) ?? "",
    symbol: (d.symbol as string) ?? "",
    status: (d.status as string) ?? "",
    totalPnl: Number(d.total_pnl ?? 0),
    position: pos
      ? {
          side: (pos.side as string) ?? "",
          entryPrice: Number(pos.entry_price ?? 0),
          quantity: Number(pos.quantity ?? 0),
          leverage: Number(pos.leverage ?? 1),
          unrealizedPnl: Number(pos.unrealized_pnl ?? 0),
          marginType: (pos.margin_type as string) ?? "Isolated",
        }
      : null,
    timestamp: (d.timestamp as string) ?? new Date().toISOString(),
  };
}

/** Parsed bot_status_change event payload. */
export interface BotStatusChangePayload {
  botId: string;
  botName: string;
  previousStatus: string;
  newStatus: string;
  reason: string;
  timestamp: string;
}

/** Parse raw bot_status_change event data. */
export function parseBotStatusChange(msg: unknown): BotStatusChangePayload | null {
  const m = msg as Record<string, unknown>;
  const d = m?.data as Record<string, unknown> | undefined;
  if (!d || typeof d.bot_id !== "string") return null;

  return {
    botId: d.bot_id as string,
    botName: (d.bot_name as string) ?? "",
    previousStatus: (d.previous_status as string) ?? "",
    newStatus: (d.new_status as string) ?? "",
    reason: (d.reason as string) ?? "",
    timestamp: (d.timestamp as string) ?? new Date().toISOString(),
  };
}

/** Parsed bot_error event payload. */
export interface BotErrorPayload {
  botId: string;
  botName: string;
  errorType: string;
  message: string;
  timestamp: string;
}

/** Parse raw bot_error event data. */
export function parseBotError(msg: unknown): BotErrorPayload | null {
  const m = msg as Record<string, unknown>;
  const d = m?.data as Record<string, unknown> | undefined;
  if (!d || typeof d.bot_id !== "string") return null;

  return {
    botId: d.bot_id as string,
    botName: (d.bot_name as string) ?? "",
    errorType: (d.error_type as string) ?? "UNKNOWN",
    message: (d.message as string) ?? "",
    timestamp: (d.timestamp as string) ?? new Date().toISOString(),
  };
}
