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

export const strategyApi = {
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
};
