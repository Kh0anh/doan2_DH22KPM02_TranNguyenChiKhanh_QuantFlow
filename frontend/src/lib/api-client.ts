// ===================================================================
// QuantFlow — Centralized REST API Client
// Task 3.2.4 — Save/Load Strategy via API
// ===================================================================
//
// Responsibilities:
//   - Base URL /api/v1 (proxied by Next.js rewrites in dev, Nginx in prod)
//   - credentials: "include" for HttpOnly Cookie auth (NFR-SEC-04)
//   - JSON request/response parsing
//   - ApiError class for structured error handling
//
// Usage:
//   import { strategyApi } from "@/lib/api-client";
//   const detail = await strategyApi.get("uuid-here");
// ===================================================================

// -----------------------------------------------------------------
// ApiError — structured error thrown for non-2xx responses
// -----------------------------------------------------------------

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    /** Optional extra data from the error response (e.g. active_bot_ids) */
    public details?: Record<string, unknown>,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// -----------------------------------------------------------------
// Core fetch wrapper
// -----------------------------------------------------------------

const API_BASE = "/api/v1";

/**
 * Centralized fetch helper.
 * - Prepends `/api/v1` to the path.
 * - Sets `credentials: "include"` so HttpOnly cookies are sent.
 * - Parses JSON responses and throws `ApiError` for non-2xx.
 */
async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const url = `${API_BASE}${path}`;

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  const res = await fetch(url, {
    ...options,
    headers,
    credentials: "include",
  });

  // Handle empty responses (204 No Content, etc.)
  const text = await res.text();
  const body = text ? JSON.parse(text) : null;

  if (!res.ok) {
    const err = body?.error ?? body;
    throw new ApiError(
      res.status,
      err?.code ?? "UNKNOWN_ERROR",
      err?.message ?? `Request failed with status ${res.status}`,
      err,
    );
  }

  return body as T;
}

// -----------------------------------------------------------------
// Strategy API functions
// -----------------------------------------------------------------

/** Response from GET /strategies/{id} */
interface GetStrategyResponse {
  data: {
    id: string;
    name: string;
    description?: string;
    status: string;
    logic_json: Record<string, unknown>;
    version_number: number;
    active_bot_ids?: string[];
    warning?: string;
    bots_using: number;
    created_at: string;
    updated_at: string;
  };
}

/** Response from POST /strategies */
interface CreateStrategyResponse {
  message: string;
  data: {
    id: string;
    name: string;
    status: string;
    version_number: number;
    created_at: string;
    updated_at: string;
  };
}

/** Response from PUT /strategies/{id} */
interface UpdateStrategyResponse {
  message: string;
  data: {
    id: string;
    name: string;
    status: string;
    version_number: number;
    warning?: string;
    updated_at: string;
  };
}

// -----------------------------------------------------------------
// Strategy List API types (Task 3.2.6)
// -----------------------------------------------------------------

/** Response from GET /strategies (paginated list) */
interface ListStrategiesResponse {
  data: {
    id: string;
    name: string;
    version: number;
    status: string;
    created_at: string;
    updated_at: string;
  }[];
  pagination: {
    page: number;
    limit: number;
    total: number;
    total_pages: number;
  };
}

/** Response from POST /strategies/import */
interface ImportStrategyResponse {
  message: string;
  data: {
    id: string;
    name: string;
    version: number;
    status: string;
    created_at: string;
  };
}

/** Response from GET /strategies/{id}/export */
interface ExportStrategyResponse {
  name: string;
  logic_json: Record<string, unknown>;
  version: number;
  exported_at: string;
}

export const strategyApi = {
  /**
   * GET /strategies — List strategies with pagination and search.
   * Used by the Strategy List Page (Task 3.2.6).
   */
  async list(params: {
    page?: number;
    limit?: number;
    search?: string;
  }): Promise<ListStrategiesResponse> {
    const query = new URLSearchParams();
    if (params.page) query.set("page", String(params.page));
    if (params.limit) query.set("limit", String(params.limit));
    if (params.search) query.set("search", params.search);
    const qs = query.toString();
    return apiFetch<ListStrategiesResponse>(`/strategies${qs ? `?${qs}` : ""}`);
  },

  /**
   * GET /strategies/{id} — Load strategy detail including logic_json.
   * Used by use-editor-tab to load blocks into Blockly workspace.
   */
  async get(id: string): Promise<GetStrategyResponse["data"]> {
    const res = await apiFetch<GetStrategyResponse>(`/strategies/${id}`);
    return res.data;
  },

  /**
   * POST /strategies — Create a new strategy (version_number = 1).
   * Used when saving a tab with strategyId === null.
   */
  async create(data: {
    name: string;
    logic_json: Record<string, unknown>;
    status?: string;
  }): Promise<CreateStrategyResponse["data"]> {
    const res = await apiFetch<CreateStrategyResponse>("/strategies", {
      method: "POST",
      body: JSON.stringify(data),
    });
    return res.data;
  },

  /**
   * PUT /strategies/{id} — Update existing strategy (auto version_number++).
   * Used when saving a tab with strategyId !== null.
   */
  async update(
    id: string,
    data: {
      name: string;
      logic_json: Record<string, unknown>;
      status?: string;
    },
  ): Promise<UpdateStrategyResponse["data"]> {
    const res = await apiFetch<UpdateStrategyResponse>(`/strategies/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
    return res.data;
  },

  /**
   * DELETE /strategies/{id} — Delete a strategy.
   * Returns 409 if strategy is in use by a running Bot.
   * Used by the Strategy List Page delete action (Task 3.2.6).
   */
  async delete(id: string): Promise<void> {
    await apiFetch<{ message: string }>(`/strategies/${id}`, {
      method: "DELETE",
    });
  },

  /**
   * POST /strategies/import — Import strategy from JSON data.
   * Used by the "Nhập từ file" button on Strategy List Page (Task 3.2.6).
   */
  async importStrategy(data: {
    name: string;
    logic_json: Record<string, unknown>;
  }): Promise<ImportStrategyResponse["data"]> {
    const res = await apiFetch<ImportStrategyResponse>("/strategies/import", {
      method: "POST",
      body: JSON.stringify(data),
    });
    return res.data;
  },

  /**
   * GET /strategies/{id}/export — Export strategy as JSON download.
   * Triggers a .json file download via Blob API (SRS FR-DESIGN-12).
   * Used by the "Xuất JSON" action on Strategy List Page (Task 3.2.6).
   */
  async exportStrategy(id: string, fileName: string): Promise<void> {
    const data = await apiFetch<ExportStrategyResponse>(
      `/strategies/${id}/export`,
    );
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${fileName.replace(/\s+/g, "-").toLowerCase()}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  },
};

// -----------------------------------------------------------------
// Market API functions (Task 3.3.1)
// -----------------------------------------------------------------

/** Individual symbol info returned by GET /market/symbols */
interface MarketSymbolResponse {
  symbol: string;
  base_asset: string;
  quote_asset: string;
  last_price: number;
  price_change_percent: number;
  volume_24h: number;
  has_running_bot: boolean;
}

export const marketApi = {
  /**
   * GET /market/symbols — List watched symbols with latest price info.
   * Backend returns symbols configured via WATCHED_SYMBOLS env var.
   * Used by MarketWatch component (Task 3.3.1).
   */
  async getSymbols(): Promise<MarketSymbolResponse[]> {
    const res = await apiFetch<{ data: MarketSymbolResponse[] }>(
      "/market/symbols",
    );
    return res.data;
  },

  /**
   * GET /market/candles — Fetch historical OHLCV data + trade markers.
   * Returns candles sorted by time ASC plus trade markers for overlay.
   * Used by CandleChart component (Task 3.3.2).
   */
  async getCandles(params: {
    symbol: string;
    timeframe: string;
    start?: string;
    end?: string;
    limit?: number;
  }): Promise<CandleDataResponse> {
    const query = new URLSearchParams({
      symbol: params.symbol,
      timeframe: params.timeframe,
    });
    if (params.limit) query.set("limit", String(params.limit));
    if (params.start) query.set("start", params.start);
    if (params.end) query.set("end", params.end);
    const res = await apiFetch<{ data: CandleDataResponse }>(
      `/market/candles?${query}`,
    );
    return res.data;
  },
};

// -----------------------------------------------------------------
// Market Candle API response types (Task 3.3.2)
// -----------------------------------------------------------------

/** Single candle returned by GET /market/candles */
interface CandleResponse {
  open_time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  is_closed: boolean;
}

/** Trade marker returned by GET /market/candles */
interface TradeMarkerResponse {
  time: string;
  price: number;
  side: "Long" | "Short";
  bot_name: string;
  bot_id: string;
}

/** Full response from GET /market/candles */
export interface CandleDataResponse {
  symbol: string;
  timeframe: string;
  candles: CandleResponse[];
  markers: TradeMarkerResponse[];
}

// -----------------------------------------------------------------
// Bot API (Task 3.3.3)
// -----------------------------------------------------------------

/** Bot summary returned by GET /bots (snake_case from backend) */
export interface BotSummaryResponse {
  id: string;
  bot_name: string;
  strategy_id: string;
  strategy_name: string;
  strategy_version: number;
  symbol: string;
  status: "Running" | "Stopped" | "Error";
  total_pnl: number;
  created_at: string;
  updated_at: string;
}

/** Bot detail returned by GET /bots/{id} */
export interface BotDetailResponse extends BotSummaryResponse {
  position: {
    side: "Long" | "Short";
    entry_price: number;
    quantity: number;
    leverage: number;
    unrealized_pnl: number;
    margin_type: "Isolated" | "Cross";
  } | null;
  open_orders: {
    order_id: string;
    side: "Buy" | "Sell";
    type: "Limit" | "Market" | "Stop";
    price: number;
    quantity: number;
    status: string;
  }[];
}

export const botApi = {
  /** GET /bots — List all bots, optionally filtered by status */
  async list(status?: string): Promise<BotSummaryResponse[]> {
    const query = status ? `?status=${status}` : "";
    const res = await apiFetch<{ data: BotSummaryResponse[] }>(
      `/bots${query}`,
    );
    return res.data;
  },

  /** POST /bots — Create a new bot */
  async create(params: {
    bot_name: string;
    strategy_id: string;
    symbol: string;
  }): Promise<BotSummaryResponse> {
    const res = await apiFetch<{ data: BotSummaryResponse }>("/bots", {
      method: "POST",
      body: JSON.stringify(params),
    });
    return res.data;
  },

  /** GET /bots/{id} — Get bot detail (position + open orders) */
  async getDetail(id: string): Promise<BotDetailResponse> {
    const res = await apiFetch<{ data: BotDetailResponse }>(`/bots/${id}`);
    return res.data;
  },

  /** DELETE /bots/{id} — Delete a stopped bot */
  async delete(id: string): Promise<void> {
    await apiFetch(`/bots/${id}`, { method: "DELETE" });
  },

  /** POST /bots/{id}/start — Start a stopped bot */
  async start(id: string): Promise<{ status: string }> {
    const res = await apiFetch<{ data: { status: string } }>(
      `/bots/${id}/start`,
      { method: "POST" },
    );
    return res.data;
  },

  /** POST /bots/{id}/stop — Stop a running bot */
  async stop(
    id: string,
    closePosition: boolean = false,
  ): Promise<{ status: string; total_pnl: number }> {
    const res = await apiFetch<{
      data: { status: string; total_pnl: number };
    }>(`/bots/${id}/stop`, {
      method: "POST",
      body: JSON.stringify({ close_position: closePosition }),
    });
    return res.data;
  },

  /** GET /bots/{id}/logs — Fetch bot activity logs (cursor pagination) */
  async getLogs(
    id: string,
    params?: { cursor?: string; limit?: number },
  ): Promise<BotLogsResponse> {
    const query = new URLSearchParams();
    if (params?.cursor) query.set("cursor", params.cursor);
    if (params?.limit) query.set("limit", String(params.limit));
    const qs = query.toString();
    const res = await apiFetch<BotLogsResponse>(
      `/bots/${id}/logs${qs ? `?${qs}` : ""}`,
    );
    return res;
  },
};

/** Single log entry from GET /bots/{id}/logs */
export interface BotLogEntryResponse {
  id: number;
  action_decision: string | null;
  message: string;
  created_at: string;
}

/** Full response from GET /bots/{id}/logs */
export interface BotLogsResponse {
  data: BotLogEntryResponse[];
  pagination: {
    next_cursor: string | null;
    has_more: boolean;
  };
}

// -----------------------------------------------------------------
// Trade History API (Task 3.3.6)
// -----------------------------------------------------------------

/** Trade record returned by GET /trades */
export interface TradeRecordResponse {
  id: string;
  bot_id: string;
  bot_name: string;
  symbol: string;
  side: "Long" | "Short";
  quantity: number;
  fill_price: number;
  fee: number;
  realized_pnl: number;
  status: "Filled" | "Canceled";
  executed_at: string;
}

/** Full response from GET /trades */
export interface TradesListResponse {
  data: TradeRecordResponse[];
  pagination: {
    next_cursor: string | null;
    has_more: boolean;
  };
  message?: string;
}

/** Filter params for GET /trades */
export interface TradeFilterParams {
  bot_id?: string;
  symbol?: string;
  side?: string;
  status?: string;
  start_date?: string;
  end_date?: string;
  cursor?: string;
  limit?: number;
}

export const tradeApi = {
  /** GET /trades — List trade history with filters and cursor pagination */
  async list(params?: TradeFilterParams): Promise<TradesListResponse> {
    const query = new URLSearchParams();
    if (params?.bot_id) query.set("bot_id", params.bot_id);
    if (params?.symbol) query.set("symbol", params.symbol);
    if (params?.side) query.set("side", params.side);
    if (params?.status) query.set("status", params.status);
    if (params?.start_date) query.set("start_date", params.start_date);
    if (params?.end_date) query.set("end_date", params.end_date);
    if (params?.cursor) query.set("cursor", params.cursor);
    if (params?.limit) query.set("limit", String(params.limit));
    const qs = query.toString();
    const res = await apiFetch<TradesListResponse>(
      `/trades${qs ? `?${qs}` : ""}`,
    );
    return res;
  },

  /** GET /trades/export — Download CSV file */
  async exportCSV(params?: {
    bot_id?: string;
    symbol?: string;
    start_date?: string;
    end_date?: string;
  }): Promise<Blob> {
    const query = new URLSearchParams();
    if (params?.bot_id) query.set("bot_id", params.bot_id);
    if (params?.symbol) query.set("symbol", params.symbol);
    if (params?.start_date) query.set("start_date", params.start_date);
    if (params?.end_date) query.set("end_date", params.end_date);
    const qs = query.toString();
    const url = `${process.env.NEXT_PUBLIC_API_URL ?? "/api/v1"}/trades/export${qs ? `?${qs}` : ""}`;
    const res = await fetch(url, { credentials: "include" });
    if (!res.ok) throw new Error("CSV export failed");
    return res.blob();
  },
};

// -----------------------------------------------------------------
// Backtest API (Task 3.4.1)
// -----------------------------------------------------------------

/** Params for POST /backtests */
export interface CreateBacktestParams {
  strategy_id: string;
  symbol: string;
  start_time: string;
  end_time: string;
  initial_capital: number;
  fee_rate: number;
  max_unit?: number;
}

/** Response from POST /backtests — flat JSON (no data wrapper) */
export interface BacktestCreatedResponse {
  backtest_id: string;
  status: "processing";
  created_at: string;
}

/**
 * Response from GET /backtests/{id} — shape varies by status:
 *   processing → { backtest_id, status, progress }
 *   completed  → { backtest_id, status, config, summary, equity_curve, trades, created_at, completed_at }
 *   canceled   → { backtest_id, status, created_at, completed_at }
 */
export interface BacktestResultResponse {
  backtest_id: string;
  status: "processing" | "completed" | "canceled" | "failed";
  progress?: number;
  config?: {
    strategy_id: string;
    strategy_name: string;
    symbol: string;
    timeframe: string;
    start_time: string;
    end_time: string;
    initial_capital: string;
    fee_rate: string;
  };
  summary?: {
    total_pnl: string;
    total_pnl_percent: string;
    win_rate: string;
    total_trades: number;
    winning_trades: number;
    losing_trades: number;
    max_drawdown: string;
    max_drawdown_percent: string;
    profit_factor: string;
  };
  equity_curve?: { timestamp: string; equity: string }[];
  trades?: {
    open_time: string;
    close_time: string;
    side: string;
    entry_price: string;
    exit_price: string;
    quantity: string;
    fee: string;
    pnl: string;
  }[];
  created_at?: string;
  completed_at?: string;
  error_message?: string;
}

export const backtestApi = {
  /** POST /backtests — Create a new backtest (flat JSON response) */
  async create(
    params: CreateBacktestParams,
  ): Promise<BacktestCreatedResponse> {
    return apiFetch<BacktestCreatedResponse>("/backtests", {
      method: "POST",
      body: JSON.stringify(params),
    });
  },

  /** GET /backtests/{id} — Get backtest result (flat JSON response) */
  async getResult(id: string): Promise<BacktestResultResponse> {
    return apiFetch<BacktestResultResponse>(`/backtests/${id}`);
  },

  /** POST /backtests/{id}/cancel — Cancel running backtest */
  async cancel(id: string): Promise<void> {
    await apiFetch(`/backtests/${id}/cancel`, { method: "POST" });
  },
};

