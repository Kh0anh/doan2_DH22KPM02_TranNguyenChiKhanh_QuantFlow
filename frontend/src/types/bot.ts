// ============================================================
// QuantFlow — Bot & Backtest Types
// Task: F-0.2 — Setup TypeScript Types
// Source: docs/api/api.yaml (Bot & Backtest schemas),
//         docs/database/schema.md (§5, §6, §7)
// ============================================================

// ─── Bot Enums ─────────────────────────────────────────────

export type BotStatus = 'Running' | 'Stopped' | 'Error';

export type PositionSide = 'Long' | 'Short';

export type MarginType = 'Isolated' | 'Cross';

export type OrderSide = 'Buy' | 'Sell';

export type OrderType = 'Limit' | 'Market' | 'Stop';

// ─── Bot Position & Orders ─────────────────────────────────

/** Current position of a bot on the exchange. null = no position */
export interface BotPosition {
  side: PositionSide;
  entry_price: number;
  quantity: number;
  leverage: number;
  unrealized_pnl: number;
  margin_type: MarginType;
}

/** Pending order on the exchange */
export interface OpenOrder {
  order_id: string;
  side: OrderSide;
  type: OrderType;
  price: number;
  quantity: number;
  status: string; // e.g., "Pending", "PartialFilled"
}

// ─── Bot Domain Types ──────────────────────────────────────

/** Bot list item (GET /bots response) */
export interface BotSummary {
  id: string;
  bot_name: string;
  strategy_id: string;
  strategy_name: string;
  strategy_version: number;
  symbol: string;
  status: BotStatus;
  total_pnl: number;
  created_at: string;
  updated_at: string;
}

/** Full bot detail (GET /bots/:id response) */
export interface BotDetail {
  id: string;
  bot_name: string;
  strategy_id: string;
  strategy_name: string;
  strategy_version: number;
  symbol: string;
  status: BotStatus;
  total_pnl: number;
  position: BotPosition | null;
  open_orders: OpenOrder[];
  created_at: string;
  updated_at: string;
}

/** Response after creating a bot */
export interface BotCreated {
  id: string;
  bot_name: string;
  strategy_id: string;
  strategy_version: number;
  symbol: string;
  status: BotStatus;
  total_pnl: number;
  created_at: string;
}

/** Response after starting/stopping a bot */
export interface BotStatusUpdate {
  id: string;
  status: BotStatus;
  updated_at: string;
}

/** Response after stopping a bot (includes final PnL) */
export interface BotStopResult {
  id: string;
  status: string;
  total_pnl: number;
  updated_at: string;
}

/** Single bot log entry (GET /bots/:id/logs, WS bot_log event) */
export interface BotLogEntry {
  id: number;
  action_decision: string | null;
  message: string;
  created_at: string;
}

// ─── Bot Request Types ─────────────────────────────────────

export interface CreateBotRequest {
  bot_name: string;
  strategy_id: string;
  symbol: string;
}

export interface StopBotRequest {
  close_position?: boolean; // default: false
}

// ─── Backtest Types ────────────────────────────────────────

export type BacktestStatus = 'processing' | 'completed' | 'canceled';

export interface CreateBacktestRequest {
  strategy_id: string;
  symbol: string;
  timeframe: string;
  start_time: string; // ISO 8601
  end_time: string;   // ISO 8601
  initial_capital: number;
  fee_rate: number;
  max_unit?: number; // default: 1000
}

/** Response after creating a backtest */
export interface BacktestCreated {
  backtest_id: string;
  status: BacktestStatus;
  created_at: string;
}

/** Backtest configuration (embedded in result) */
export interface BacktestConfig {
  strategy_id: string;
  strategy_name: string;
  symbol: string;
  timeframe: string;
  start_time: string;
  end_time: string;
  initial_capital: number;
  fee_rate: number;
}

/** Backtest performance summary */
export interface BacktestSummary {
  total_pnl: number;
  total_pnl_percent: number;
  win_rate: number;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  max_drawdown: number;
  max_drawdown_percent: number;
  profit_factor: number;
}

/** Single point on the equity curve chart */
export interface EquityPoint {
  timestamp: string;
  equity: number;
}

/** Single simulated trade in backtest results */
export interface BacktestTrade {
  open_time: string;
  close_time: string;
  side: PositionSide;
  entry_price: number;
  exit_price: number;
  quantity: number;
  fee: number;
  pnl: number;
}

/** Full backtest result (GET /backtests/:id response) */
export interface BacktestResult {
  backtest_id: string;
  status: BacktestStatus;
  /** Progress percentage (only when status = 'processing') */
  progress?: number;
  config?: BacktestConfig;
  summary?: BacktestSummary;
  equity_curve?: EquityPoint[];
  trades?: BacktestTrade[];
  created_at: string;
  completed_at?: string | null;
}
