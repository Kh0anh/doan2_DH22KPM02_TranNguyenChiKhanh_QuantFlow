// ============================================================
// QuantFlow — Centralized API Client
// Task: F-0.3 — Setup API Client
// Source: docs/api/api.yaml (all endpoints)
// ============================================================

import type {
  ApiResponse,
  ApiPaginatedResponse,
  ApiCursorResponse,
  ApiError,
  LoginRequest,
  User,
  UserProfile,
  UpdateProfileRequest,
  SaveApiKeysRequest,
  ApiKeyInfo,
} from '@/types/api';

import type {
  StrategySummary,
  StrategyDetail,
  StrategyCreated,
  StrategyUpdated,
  StrategyExport,
  CreateStrategyRequest,
  UpdateStrategyRequest,
  ImportStrategyRequest,
} from '@/types/strategy';

import type {
  BotSummary,
  BotDetail,
  BotCreated,
  BotStatusUpdate,
  BotStopResult,
  BotLogEntry,
  CreateBotRequest,
  StopBotRequest,
  BacktestCreated,
  BacktestResult,
  CreateBacktestRequest,
} from '@/types/bot';

import type { MarketSymbol, CandleData } from '@/types/market';

import type { TradeRecord } from '@/types/trade';

import type { CursorPagination } from '@/types/api';

// ─── Configuration ─────────────────────────────────────────

const BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

// ─── Custom Error Class ────────────────────────────────────

export class ApiClientError extends Error {
  public code: string;
  public status: number;
  public details: Record<string, unknown>;

  constructor(
    message: string,
    code: string,
    status: number,
    details: Record<string, unknown> = {}
  ) {
    super(message);
    this.name = 'ApiClientError';
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

// ─── Core Fetch Wrapper ────────────────────────────────────

async function apiFetch<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${BASE_URL}${endpoint}`;

  const config: RequestInit = {
    ...options,
    credentials: 'include', // Auto-attach HttpOnly cookie
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...options.headers,
    },
  };

  const response = await fetch(url, config);

  // 401 — Session expired → redirect to login
  if (response.status === 401) {
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new ApiClientError(
      'Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.',
      'SESSION_EXPIRED',
      401
    );
  }

  // Non-OK responses → parse error body and throw
  if (!response.ok) {
    let errorData: ApiError['error'] = {
      code: 'UNKNOWN_ERROR',
      message: 'Đã xảy ra lỗi không xác định.',
    };

    try {
      const body = await response.json();
      if (body?.error) {
        errorData = body.error;
      }
    } catch {
      // Response body is not JSON — use default error
    }

    throw new ApiClientError(
      errorData.message,
      errorData.code,
      response.status,
      errorData as Record<string, unknown>
    );
  }

  // 204 No Content — return empty
  if (response.status === 204) {
    return {} as T;
  }

  return response.json() as Promise<T>;
}

/** Fetch that returns raw Response (for file downloads) */
async function apiFetchRaw(
  endpoint: string,
  options: RequestInit = {}
): Promise<Response> {
  const url = `${BASE_URL}${endpoint}`;

  const config: RequestInit = {
    ...options,
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...options.headers,
    },
  };

  const response = await fetch(url, config);

  if (response.status === 401) {
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new ApiClientError(
      'Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.',
      'SESSION_EXPIRED',
      401
    );
  }

  if (!response.ok) {
    let errorData: ApiError['error'] = {
      code: 'UNKNOWN_ERROR',
      message: 'Đã xảy ra lỗi không xác định.',
    };

    try {
      const body = await response.json();
      if (body?.error) {
        errorData = body.error;
      }
    } catch {
      // Not JSON
    }

    throw new ApiClientError(
      errorData.message,
      errorData.code,
      response.status,
      errorData as Record<string, unknown>
    );
  }

  return response;
}

// ─── HTTP Method Helpers ───────────────────────────────────

function apiGet<T>(endpoint: string): Promise<T> {
  return apiFetch<T>(endpoint, { method: 'GET' });
}

function apiPost<T>(endpoint: string, body?: unknown): Promise<T> {
  return apiFetch<T>(endpoint, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  });
}

function apiPut<T>(endpoint: string, body?: unknown): Promise<T> {
  return apiFetch<T>(endpoint, {
    method: 'PUT',
    body: body ? JSON.stringify(body) : undefined,
  });
}

function apiDelete<T>(endpoint: string): Promise<T> {
  return apiFetch<T>(endpoint, { method: 'DELETE' });
}

// ============================================================
// API Functions — grouped by domain
// ============================================================

// ─── Auth (4 endpoints) ────────────────────────────────────

/** POST /auth/login — Đăng nhập hệ thống */
export async function login(data: LoginRequest) {
  return apiPost<ApiResponse<{ user: User }>>('/auth/login', data);
}

/** POST /auth/logout — Đăng xuất hệ thống */
export async function logout() {
  return apiPost<{ message: string }>('/auth/logout');
}

/** GET /auth/me — Lấy thông tin phiên đăng nhập hiện tại */
export async function getMe() {
  return apiGet<ApiResponse<{ user: UserProfile }>>('/auth/me');
}

/** POST /auth/refresh — Làm mới phiên đăng nhập */
export async function refreshToken() {
  return apiPost<ApiResponse<{ expires_at: string }>>('/auth/refresh');
}

// ─── Account (1 endpoint) ──────────────────────────────────

/** PUT /account/profile — Cập nhật thông tin tài khoản */
export async function updateProfile(data: UpdateProfileRequest) {
  return apiPut<{ message: string }>('/account/profile', data);
}

// ─── Exchange (3 endpoints) ────────────────────────────────

/** POST /exchange/api-keys — Lưu/cập nhật API Key sàn */
export async function saveApiKeys(data: SaveApiKeysRequest) {
  return apiPost<ApiResponse<ApiKeyInfo>>('/exchange/api-keys', data);
}

/** GET /exchange/api-keys — Lấy thông tin API Key hiện tại */
export async function getApiKeys() {
  return apiGet<ApiResponse<ApiKeyInfo | null>>('/exchange/api-keys');
}

/** DELETE /exchange/api-keys — Xóa cấu hình API Key */
export async function deleteApiKeys() {
  return apiDelete<{ message: string }>('/exchange/api-keys');
}

// ─── Strategies (7 endpoints) ──────────────────────────────

/** GET /strategies — Lấy danh sách chiến lược */
export async function listStrategies(params?: {
  page?: number;
  limit?: number;
  search?: string;
}) {
  const query = new URLSearchParams();
  if (params?.page) query.set('page', String(params.page));
  if (params?.limit) query.set('limit', String(params.limit));
  if (params?.search) query.set('search', params.search);

  const qs = query.toString();
  return apiGet<ApiPaginatedResponse<StrategySummary>>(
    `/strategies${qs ? `?${qs}` : ''}`
  );
}

/** POST /strategies — Tạo mới chiến lược */
export async function createStrategy(data: CreateStrategyRequest) {
  return apiPost<ApiResponse<StrategyCreated>>('/strategies', data);
}

/** GET /strategies/:id — Lấy chi tiết chiến lược */
export async function getStrategy(id: string) {
  return apiGet<ApiResponse<StrategyDetail>>(`/strategies/${id}`);
}

/** PUT /strategies/:id — Cập nhật chiến lược */
export async function updateStrategy(id: string, data: UpdateStrategyRequest) {
  return apiPut<ApiResponse<StrategyUpdated>>(`/strategies/${id}`, data);
}

/** DELETE /strategies/:id — Xóa chiến lược */
export async function deleteStrategy(id: string) {
  return apiDelete<{ message: string }>(`/strategies/${id}`);
}

/** POST /strategies/import — Nhập chiến lược từ JSON */
export async function importStrategy(data: ImportStrategyRequest) {
  return apiPost<ApiResponse<StrategyCreated>>('/strategies/import', data);
}

/** GET /strategies/:id/export — Xuất chiến lược ra JSON */
export async function exportStrategy(id: string) {
  return apiGet<StrategyExport>(`/strategies/${id}/export`);
}

// ─── Backtests (3 endpoints) ───────────────────────────────

/** POST /backtests — Khởi tạo phiên Backtest mới */
export async function createBacktest(data: CreateBacktestRequest) {
  return apiPost<ApiResponse<BacktestCreated>>('/backtests', data);
}

/** GET /backtests/:id — Lấy kết quả Backtest */
export async function getBacktestResult(id: string) {
  return apiGet<ApiResponse<BacktestResult>>(`/backtests/${id}`);
}

/** POST /backtests/:id/cancel — Hủy phiên Backtest đang chạy */
export async function cancelBacktest(id: string) {
  return apiPost<{ message: string }>(`/backtests/${id}/cancel`);
}

// ─── Bots (7 endpoints) ────────────────────────────────────

/** GET /bots — Lấy danh sách Bot */
export async function listBots(params?: { status?: string }) {
  const query = new URLSearchParams();
  if (params?.status) query.set('status', params.status);

  const qs = query.toString();
  return apiGet<ApiResponse<BotSummary[]>>(`/bots${qs ? `?${qs}` : ''}`);
}

/** POST /bots — Khởi tạo Bot mới */
export async function createBot(data: CreateBotRequest) {
  return apiPost<ApiResponse<BotCreated>>('/bots', data);
}

/** GET /bots/:id — Lấy chi tiết Bot */
export async function getBot(id: string) {
  return apiGet<ApiResponse<BotDetail>>(`/bots/${id}`);
}

/** DELETE /bots/:id — Xóa Bot */
export async function deleteBot(id: string) {
  return apiDelete<{ message: string }>(`/bots/${id}`);
}

/** POST /bots/:id/start — Khởi động lại Bot */
export async function startBot(id: string) {
  return apiPost<ApiResponse<BotStatusUpdate>>(`/bots/${id}/start`);
}

/** POST /bots/:id/stop — Dừng Bot */
export async function stopBot(id: string, data?: StopBotRequest) {
  return apiPost<ApiResponse<BotStopResult>>(`/bots/${id}/stop`, data);
}

/** GET /bots/:id/logs — Lấy nhật ký Bot (cursor-based) */
export async function getBotLogs(
  botId: string,
  params?: { cursor?: string; limit?: number }
) {
  const query = new URLSearchParams();
  if (params?.cursor) query.set('cursor', params.cursor);
  if (params?.limit) query.set('limit', String(params.limit));

  const qs = query.toString();
  return apiGet<{ data: BotLogEntry[]; pagination: CursorPagination }>(
    `/bots/${botId}/logs${qs ? `?${qs}` : ''}`
  );
}

// ─── Trades (2 endpoints) ──────────────────────────────────

/** GET /trades — Lấy lịch sử giao dịch (cursor-based) */
export async function listTrades(params?: {
  bot_id?: string;
  symbol?: string;
  side?: string;
  status?: string;
  start_date?: string;
  end_date?: string;
  cursor?: string;
  limit?: number;
}) {
  const query = new URLSearchParams();
  if (params?.bot_id) query.set('bot_id', params.bot_id);
  if (params?.symbol) query.set('symbol', params.symbol);
  if (params?.side) query.set('side', params.side);
  if (params?.status) query.set('status', params.status);
  if (params?.start_date) query.set('start_date', params.start_date);
  if (params?.end_date) query.set('end_date', params.end_date);
  if (params?.cursor) query.set('cursor', params.cursor);
  if (params?.limit) query.set('limit', String(params.limit));

  const qs = query.toString();
  return apiGet<ApiCursorResponse<TradeRecord>>(
    `/trades${qs ? `?${qs}` : ''}`
  );
}

/** GET /trades/export — Xuất lịch sử giao dịch CSV */
export async function exportTrades(params?: {
  bot_id?: string;
  symbol?: string;
  start_date?: string;
  end_date?: string;
}) {
  const query = new URLSearchParams();
  if (params?.bot_id) query.set('bot_id', params.bot_id);
  if (params?.symbol) query.set('symbol', params.symbol);
  if (params?.start_date) query.set('start_date', params.start_date);
  if (params?.end_date) query.set('end_date', params.end_date);

  const qs = query.toString();
  const response = await apiFetchRaw(
    `/trades/export${qs ? `?${qs}` : ''}`,
    {
      method: 'GET',
      headers: { Accept: 'text/csv' },
    }
  );

  // Trigger file download
  const blob = await response.blob();
  const disposition = response.headers.get('Content-Disposition');
  const filename = disposition
    ? disposition.split('filename=')[1]?.replace(/"/g, '') || 'trades.csv'
    : 'trades.csv';

  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = filename;
  link.click();
  URL.revokeObjectURL(link.href);
}

// ─── Market (2 endpoints) ──────────────────────────────────

/** GET /market/symbols — Lấy danh sách cặp tiền */
export async function listMarketSymbols(params?: { search?: string }) {
  const query = new URLSearchParams();
  if (params?.search) query.set('search', params.search);

  const qs = query.toString();
  return apiGet<ApiResponse<MarketSymbol[]>>(
    `/market/symbols${qs ? `?${qs}` : ''}`
  );
}

/** GET /market/candles — Lấy dữ liệu nến OHLCV */
export async function getMarketCandles(params: {
  symbol: string;
  timeframe: string;
  start?: string;
  end?: string;
  limit?: number;
}) {
  const query = new URLSearchParams();
  query.set('symbol', params.symbol);
  query.set('timeframe', params.timeframe);
  if (params.start) query.set('start', params.start);
  if (params.end) query.set('end', params.end);
  if (params.limit) query.set('limit', String(params.limit));

  return apiGet<ApiResponse<CandleData>>(`/market/candles?${query.toString()}`);
}
