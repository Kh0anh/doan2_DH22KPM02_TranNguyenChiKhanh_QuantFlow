// ============================================================
// QuantFlow — App-wide Constants
// Task: F-0.4 — Setup Constants & Utilities
// ============================================================

import type { BotStatus } from '@/types/bot';
import type { StrategyStatus } from '@/types/strategy';
import type { Timeframe } from '@/types/market';
import type { WsChannel } from '@/types/websocket';

// ─── Routes ────────────────────────────────────────────────

export const ROUTES = {
  LOGIN: '/login',
  DASHBOARD: '/',
  STRATEGIES: '/strategies',
  EDITOR_NEW: '/editor/new',
  EDITOR: (id: string) => `/editor/${id}`,
  SETTINGS: '/settings',
} as const;

// ─── WebSocket ─────────────────────────────────────────────

export const WS_ENDPOINT =
  process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080/ws/v1';

export const WS_CHANNELS: Record<WsChannel, WsChannel> = {
  market_ticker: 'market_ticker',
  bot_logs: 'bot_logs',
  position_update: 'position_update',
} as const;

/** Heartbeat interval (server sends ping every 30s) */
export const WS_PING_INTERVAL = 30_000;

/** Pong response timeout (must respond within 10s) */
export const WS_PONG_TIMEOUT = 10_000;

/** Reconnect backoff: delays in ms (cap at 30s) */
export const WS_RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000];

// ─── Timeframes ────────────────────────────────────────────

export const TIMEFRAMES: { label: string; value: Timeframe }[] = [
  { label: '1 phút', value: '1m' },
  { label: '5 phút', value: '5m' },
  { label: '15 phút', value: '15m' },
  { label: '30 phút', value: '30m' },
  { label: '1 giờ', value: '1h' },
  { label: '4 giờ', value: '4h' },
  { label: '1 ngày', value: '1D' },
];

// ─── Bot Statuses ──────────────────────────────────────────

export const BOT_STATUS_CONFIG: Record<
  BotStatus,
  { label: string; color: string; dotColor: string }
> = {
  Running: {
    label: 'Đang chạy',
    color: 'text-green-400',
    dotColor: 'bg-green-400',
  },
  Stopped: {
    label: 'Đã dừng',
    color: 'text-gray-400',
    dotColor: 'bg-gray-400',
  },
  Error: {
    label: 'Lỗi',
    color: 'text-red-400',
    dotColor: 'bg-red-400',
  },
} as const;

// ─── Strategy Statuses ─────────────────────────────────────

export const STRATEGY_STATUS_CONFIG: Record<
  StrategyStatus,
  { label: string; color: string; dotColor: string }
> = {
  Draft: {
    label: 'Nháp',
    color: 'text-gray-400',
    dotColor: 'bg-gray-400',
  },
  Valid: {
    label: 'Hợp lệ',
    color: 'text-green-400',
    dotColor: 'bg-green-400',
  },
  Archived: {
    label: 'Đã lưu trữ',
    color: 'text-yellow-400',
    dotColor: 'bg-yellow-400',
  },
} as const;

// ─── Trade Side ────────────────────────────────────────────

export const TRADE_SIDE_CONFIG = {
  Long: { label: 'Long', color: 'price-up' },
  Short: { label: 'Short', color: 'price-down' },
} as const;

// ─── Pagination ────────────────────────────────────────────

export const DEFAULT_PAGE_SIZE = 20;
export const DEFAULT_LOG_LIMIT = 50;
export const MAX_LOG_DOM_ITEMS = 1000;

// ─── Trading Defaults ──────────────────────────────────────

export const DEFAULT_FEE_RATE = 0.04; // 0.04%
export const DEFAULT_INITIAL_CAPITAL = 1000; // USDT
export const DEFAULT_MAX_UNIT = 1000;
export const MAX_LEVERAGE = 125;

// ─── Debounce ──────────────────────────────────────────────

export const SEARCH_DEBOUNCE_MS = 500;
