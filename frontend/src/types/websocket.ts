// ============================================================
// QuantFlow — WebSocket Types
// Task: F-0.2 — Setup TypeScript Types
// Source: docs/api/websocket.md (§1-3)
// ============================================================

import type { BotStatus } from './bot';

// ─── Channel & Action Enums ────────────────────────────────

export type WsChannel = 'market_ticker' | 'bot_logs' | 'position_update';

export type WsAction = 'subscribe' | 'unsubscribe';

// ─── Connection State (Frontend UI) ────────────────────────

export type WsConnectionState =
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'disconnected';

// ─── Client → Server Messages ──────────────────────────────

/** Message sent from Client to Server */
export interface WsClientMessage {
  action: WsAction;
  channel: WsChannel;
  params?: Record<string, string>;
}

// ─── Server → Client: Heartbeat ────────────────────────────

export interface WsPingMessage {
  event: 'ping';
  timestamp: string;
}

export interface WsPongMessage {
  event: 'pong';
  timestamp: string;
}

// ─── Server → Client: Subscription Confirmation ────────────

export interface WsSubscribedMessage {
  event: 'subscribed';
  channel: WsChannel;
  params?: Record<string, string>;
}

export interface WsUnsubscribedMessage {
  event: 'unsubscribed';
  channel: WsChannel;
  params?: Record<string, string>;
}

// ─── Server → Client: Error ────────────────────────────────

export type WsErrorCode =
  | 'AUTH_FAILED'
  | 'INVALID_ACTION'
  | 'INVALID_CHANNEL'
  | 'MISSING_PARAMS'
  | 'INVALID_PARAMS'
  | 'BOT_NOT_FOUND'
  | 'INTERNAL_ERROR';

export interface WsErrorMessage {
  event: 'error';
  data: {
    code: WsErrorCode;
    message: string;
  };
}

// ─── Server → Client: Event Payloads ───────────────────────

// --- Channel: market_ticker ---

/** market_ticker event — real-time price ticker */
export interface WsMarketTickerData {
  symbol: string;
  last_price: number;
  price_change_percent: number;
  high_24h: number;
  low_24h: number;
  volume_24h: number;
  timestamp: string;
}

export interface WsMarketTickerEvent {
  event: 'market_ticker';
  channel: 'market_ticker';
  data: WsMarketTickerData;
}

/** market_candle event — real-time OHLCV candle update */
export interface WsMarketCandleData {
  symbol: string;
  timeframe: string;
  candle: {
    open_time: string;
    open: number;
    high: number;
    low: number;
    close: number;
    volume: number;
    is_closed: boolean;
  };
}

export interface WsMarketCandleEvent {
  event: 'market_candle';
  channel: 'market_ticker';
  data: WsMarketCandleData;
}

// --- Channel: bot_logs ---

/** bot_log event — new log entry from a bot session */
export interface WsBotLogData {
  bot_id: string;
  log: {
    id: number;
    action_decision: string;
    unit_used?: number;
    message: string;
    created_at: string;
  };
}

export interface WsBotLogEvent {
  event: 'bot_log';
  channel: 'bot_logs';
  data: WsBotLogData;
}

// --- Channel: position_update ---

/** position_update event — real-time position & PnL sync */
export interface WsPositionUpdateData {
  bot_id: string;
  bot_name: string;
  symbol: string;
  status: BotStatus;
  total_pnl: number;
  position: {
    side: 'Long' | 'Short';
    entry_price: number;
    quantity: number;
    leverage: number;
    unrealized_pnl: number;
    margin_type: 'Isolated' | 'Cross';
  } | null;
  open_orders: Array<{
    order_id: string;
    side: 'Buy' | 'Sell';
    type: 'Limit' | 'Market' | 'Stop';
    price: number;
    quantity: number;
    status: string;
  }>;
  timestamp: string;
}

export interface WsPositionUpdateEvent {
  event: 'position_update';
  channel: 'position_update';
  data: WsPositionUpdateData;
}

/** bot_status_change event — bot state transition */
export interface WsBotStatusChangeData {
  bot_id: string;
  bot_name: string;
  previous_status: BotStatus;
  new_status: BotStatus;
  reason: string;
  timestamp: string;
}

export interface WsBotStatusChangeEvent {
  event: 'bot_status_change';
  channel: 'position_update';
  data: WsBotStatusChangeData;
}

/** bot_error event — runtime error from a bot */
export type BotErrorType =
  | 'ORDER_REJECTED'
  | 'UNIT_COST_EXCEEDED'
  | 'API_CONNECTION_LOST'
  | 'EXECUTION_ERROR'
  | 'LIQUIDATION_ALERT';

export interface WsBotErrorData {
  bot_id: string;
  bot_name: string;
  error_type: BotErrorType;
  message: string;
  timestamp: string;
}

export interface WsBotErrorEvent {
  event: 'bot_error';
  channel: 'position_update';
  data: WsBotErrorData;
}

// ─── Discriminated Union: All Server → Client Messages ─────

/** Union of all possible messages received from the WebSocket server */
export type WsServerMessage =
  | WsPingMessage
  | WsSubscribedMessage
  | WsUnsubscribedMessage
  | WsErrorMessage
  | WsMarketTickerEvent
  | WsMarketCandleEvent
  | WsBotLogEvent
  | WsPositionUpdateEvent
  | WsBotStatusChangeEvent
  | WsBotErrorEvent;
