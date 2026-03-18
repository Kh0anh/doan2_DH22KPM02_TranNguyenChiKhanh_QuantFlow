// ============================================================
// QuantFlow — API Types
// Task: F-0.2 — Setup TypeScript Types
// Source: docs/api/api.yaml (components/schemas)
// ============================================================

// ─── Generic API Wrappers ──────────────────────────────────

/** Standard API success response wrapper */
export interface ApiResponse<T> {
  message?: string;
  data: T;
}

/** Standard API success response with page pagination */
export interface ApiPaginatedResponse<T> {
  data: T[];
  pagination: PagePagination;
}

/** Standard API success response with cursor pagination */
export interface ApiCursorResponse<T> {
  data: T[];
  pagination: CursorPagination;
  message?: string;
}

/** Standard API error response */
export interface ApiError {
  error: {
    code: string;
    message: string;
    /** Additional error-specific fields (e.g., remaining_attempts, locked_until, active_bot_ids) */
    [key: string]: unknown;
  };
}

// ─── Pagination ────────────────────────────────────────────

/** Page-based pagination (Strategies list) */
export interface PagePagination {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

/** Cursor-based pagination (Bot Logs, Trade History — Infinite Scroll) */
export interface CursorPagination {
  next_cursor: string | null;
  has_more: boolean;
}

// ─── Auth Types ────────────────────────────────────────────

export interface LoginRequest {
  username: string;
  password: string;
}

/** Extended error response for login — includes brute-force protection fields */
export interface LoginErrorResponse extends ApiError {
  error: ApiError['error'] & {
    remaining_attempts?: number;
    locked_until?: string; // ISO 8601
  };
}

// ─── User Types ────────────────────────────────────────────

/** Basic user info returned after login */
export interface User {
  id: string;
  username: string;
}

/** Full user profile returned from GET /auth/me */
export interface UserProfile extends User {
  created_at: string; // ISO 8601
}

// ─── Account Types ─────────────────────────────────────────

export interface UpdateProfileRequest {
  current_password: string;
  new_username?: string;
  new_password?: string;
  confirm_password?: string;
}

// ─── Exchange / API Key Types ──────────────────────────────

export type ApiKeyStatus = 'Connected' | 'Revoked';

export interface SaveApiKeysRequest {
  exchange?: string; // default: "Binance"
  api_key: string;
  secret_key: string;
}

/** API Key info returned from the server (Secret Key is NEVER returned) */
export interface ApiKeyInfo {
  id: string;
  exchange: string;
  api_key_masked: string;
  status: ApiKeyStatus;
  created_at?: string;
  updated_at?: string;
}
