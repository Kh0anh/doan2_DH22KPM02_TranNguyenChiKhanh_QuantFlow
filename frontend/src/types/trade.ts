// ============================================================
// QuantFlow — Trade History Types
// Task: F-0.2 — Setup TypeScript Types
// Source: docs/api/api.yaml (Trade schemas),
//         docs/database/schema.md (§8 — trade_history)
// ============================================================

// ─── Enums ─────────────────────────────────────────────────

export type TradeSide = 'Long' | 'Short';

export type TradeStatus = 'Filled' | 'Canceled';

// ─── Trade Record ──────────────────────────────────────────

/** Single trade record (GET /trades response item) */
export interface TradeRecord {
  id: string;
  bot_id: string;
  bot_name: string;
  symbol: string;
  side: TradeSide;
  quantity: number;
  fill_price: number;
  fee: number;
  realized_pnl: number;
  status: TradeStatus;
  executed_at: string; // ISO 8601
}

// ─── Filter State (Frontend UI) ────────────────────────────

/** Trade history filter parameters (maps to query params of GET /trades) */
export interface TradeFilters {
  bot_id?: string;
  symbol?: string;
  side?: TradeSide;
  status?: TradeStatus;
  start_date?: string; // ISO 8601
  end_date?: string;   // ISO 8601
}
