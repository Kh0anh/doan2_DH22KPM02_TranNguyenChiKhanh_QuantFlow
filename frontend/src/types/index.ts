// ===================================================================
// QuantFlow — Domain Type Definitions
// Task 1.1.4 — Infrastructure setup
// ===================================================================

// -----------------------------------------------------------------
// Auth & Account
// -----------------------------------------------------------------

export interface AuthState {
  isAuthenticated: boolean;
  username: string;
}

export interface UserProfile {
  id: string;
  username: string;
  createdAt: string;
  updatedAt: string;
}

// -----------------------------------------------------------------
// Market & Candle
// -----------------------------------------------------------------

export interface SymbolInfo {
  symbol: string;
  baseAsset: string;
  quoteAsset: string;
  lastPrice: number;
  priceChangePercent: number;
  volume24h?: number;
  hasRunningBot?: boolean;
}

export type Timeframe = "1m" | "5m" | "15m" | "30m" | "1h" | "4h" | "1d";

/** OHLCV candle — time in Unix seconds */
export interface CandleData {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

/** Trade marker overlaid on the candlestick chart */
export interface TradeMarker {
  time: number;
  position: "aboveBar" | "belowBar";
  color: string;
  shape: "arrowUp" | "arrowDown";
  text: string;
}

// -----------------------------------------------------------------
// API Key (Exchange connection)
// -----------------------------------------------------------------

export interface ApiKeyInfo {
  id: string;
  exchange: string;
  apiKeyMasked: string; // Only last 4 chars visible — NFR-SEC-05. Secret Key never returned.
  status: "Connected" | "Disconnected";
  createdAt?: string;
  updatedAt?: string;
}

// -----------------------------------------------------------------
// Strategy
// -----------------------------------------------------------------

export type StrategyStatus = "valid" | "draft" | "archived";

export interface Strategy {
  id: string;
  name: string;
  description?: string;
  status: StrategyStatus;
  botsUsing: number;
  createdAt: string;
  updatedAt: string;
}

export interface StrategyDetail extends Strategy {
  blocklyJson: string; // JSONB serialized Blockly workspace
  versionNumber: number;
  activeBotIds?: string[];
}

// -----------------------------------------------------------------
// Bot Instance
// -----------------------------------------------------------------

export type BotStatus = "running" | "stopped" | "error" | "reconnecting";

export interface BotPosition {
  side: "Long" | "Short" | "None";
  entryPrice: number;
  size: number;
  unrealizedPnl: number;
  liquidationPrice?: number;
}

export interface OpenOrder {
  orderId: string;
  type: string;
  side: string;
  price: number;
  quantity: number;
  status: string;
}

export interface BotInstance {
  id: string;
  name: string;
  symbol: string;
  timeframe: Timeframe;
  strategyId: string;
  strategyName: string;
  status: BotStatus;
  totalPnl: number;
  position?: BotPosition;
  openOrders?: OpenOrder[];
  createdAt: string;
  updatedAt: string;
}

export interface BotLog {
  id: string;
  botId: string;
  level: "info" | "warn" | "error" | "debug";
  message: string;
  createdAt: string;
}

// -----------------------------------------------------------------
// Trade History
// -----------------------------------------------------------------
// Editor (Multi-tab Strategy Editor — v2)
// -----------------------------------------------------------------

/**
 * Represents a single open tab in the Multi-tab Strategy Editor.
 * workspaceXml is NOT persisted to sessionStorage — reconstructed from API on re-open.
 */
export interface EditorTab {
  id: string;                   // unique tab ID (= strategyId for existing, uuid for new)
  strategyId: string | null;    // null = new unsaved strategy
  name: string;                 // display name (max 20 chars)
  isDirty: boolean;             // has unsaved changes?
}

// -----------------------------------------------------------------

export type TradeSide = "Buy" | "Sell";
export type TradeOrderType = "Market" | "Limit";
export type TradeStatus = "filled" | "cancelled" | "partial";

export interface TradeRecord {
  id: string;
  botId: string;
  botName: string;
  symbol: string;
  side: TradeSide;
  orderType: TradeOrderType;
  price: number;
  quantity: number;
  realizedPnl: number;
  fee: number;
  status: TradeStatus;
  executedAt: string;
}

// -----------------------------------------------------------------
// Backtest
// -----------------------------------------------------------------

export type BacktestStatus = "pending" | "running" | "completed" | "cancelled" | "error";

export interface BacktestConfig {
  strategyId: string;
  symbol: string;
  timeframe: Timeframe;
  startDate: string;
  endDate: string;
  initialCapital: number;
  feeRate: number;
}

export interface BacktestReport {
  totalPnl: number;
  totalPnlPercent: number;
  winRate: number;
  totalTrades: number;
  maxDrawdown: number;
  profitFactor: number;
  equityCurve: Array<{ time: number; equity: number }>;
}

export interface BacktestResult {
  id: string;
  status: BacktestStatus;
  progress: number;
  config: BacktestConfig;
  report?: BacktestReport;
  createdAt: string;
}

// (EditorTab is defined above in the Editor section)

// -----------------------------------------------------------------
// WebSocket message types (channels)
// -----------------------------------------------------------------

export interface WsAuthMessage {
  type: "auth";
  token: string;
}

export interface WsSubscribeMessage {
  type: "subscribe";
  channel: "market_ticker" | "bot_logs" | "position_update";
  params?: { bot_id?: string };
}

export interface WsTickerEvent {
  type: "ticker";
  symbol: string;
  lastPrice: number;
  priceChangePercent: number;
  volume24h: number;
}

export interface WsCandleEvent {
  type: "candle";
  symbol: string;
  timeframe: Timeframe;
  candle: CandleData;
  isClosed: boolean;
}

export interface WsBotLogEvent {
  type: "bot_log";
  botId: string;
  log: BotLog;
}

export interface WsPositionEvent {
  type: "position_update";
  botId: string;
  position: BotPosition;
  openOrders: OpenOrder[];
  totalPnl: number;
}

export type WsIncomingEvent =
  | WsTickerEvent
  | WsCandleEvent
  | WsBotLogEvent
  | WsPositionEvent;

// -----------------------------------------------------------------
// Generic API response wrappers
// -----------------------------------------------------------------

export interface ApiResponse<T> {
  data: T;
  message?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
}
