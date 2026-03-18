// ============================================================
// QuantFlow — Market Data Types
// Task: F-0.2 — Setup TypeScript Types
// Source: docs/api/api.yaml (Market schemas),
//         docs/database/schema.md (§9 — candles_data)
// ============================================================

// ─── Enums ─────────────────────────────────────────────────

export type Timeframe = '1m' | '5m' | '15m' | '30m' | '1h' | '4h' | '1D';

// ─── Market Symbol ─────────────────────────────────────────

/** Symbol list item (GET /market/symbols response) */
export interface MarketSymbol {
  symbol: string;
  last_price: number;
  price_change_percent: number;
  volume_24h: number;
}

// ─── Candle (OHLCV) ────────────────────────────────────────

/** Single candle data point */
export interface Candle {
  open_time: string; // ISO 8601
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  is_closed: boolean;
}

/** Candle response wrapper (GET /market/candles) */
export interface CandleData {
  symbol: string;
  timeframe: string;
  candles: Candle[];
  markers: TradeMarker[];
}

// ─── Trade Markers (Chart overlay) ─────────────────────────

/** Buy/Sell marker displayed on the candlestick chart */
export interface TradeMarker {
  time: string; // ISO 8601
  price: number;
  side: 'Long' | 'Short';
  bot_name: string;
  bot_id: string;
}
