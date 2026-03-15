/**
 * api-client.ts — Centralized REST API client (minimal bootstrap).
 *
 * This file grows as new tasks are implemented (3.1.3 Auth Context, etc.).
 * For now, only authLogin() is needed by the Login Page (WBS 3.1.1).
 *
 * Base URL resolution:
 * - In Docker (Nginx reverse proxy): requests to /api/v1/... are proxied to backend.
 * - In local dev (no Nginx): use NEXT_PUBLIC_API_URL (default http://localhost:8080).
 *
 * Credentials: "include" is required so the browser sends / receives the
 * HttpOnly JWT cookie set by the backend on login (api.yaml §POST /auth/login).
 */

// Normalize: strip trailing slash and /api/v1 suffix (if env var already includes it)
// so fetch calls can always safely append /api/v1/...
const BASE_URL = (
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
)
  .replace(/\/api\/v1\/?$/, "")  // strip /api/v1 if already present
  .replace(/\/$/, "");           // strip trailing slash

/** Raw API response envelope types for POST /auth/login */
export interface LoginSuccessResponse {
  message: string;
  data: {
    user: {
      id: string;
      username: string;
      created_at: string;
    };
  };
}

export interface LoginErrorResponse {
  error: {
    code: string;
    message: string;
    // 401: wrong password
    remaining_attempts?: number;
    // 403: account locked
    locked_until?: string; // ISO 8601
  };
}

export type LoginResult =
  | { ok: true; status: 200; data: LoginSuccessResponse }
  | { ok: false; status: 401 | 403 | 500; data: LoginErrorResponse };

/**
 * authLogin — calls POST /api/v1/auth/login.
 *
 * Returns a discriminated union so callers can exhaustively handle
 * all response scenarios described in api.yaml §POST /auth/login.
 */
export async function authLogin(
  username: string,
  password: string
): Promise<LoginResult> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    credentials: "include", // ← required for HttpOnly cookie
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });

  const data = await res.json();

  if (res.ok) {
    return { ok: true, status: 200, data: data as LoginSuccessResponse };
  }

  const status = res.status === 401 || res.status === 403 ? res.status : 500;
  return { ok: false, status, data: data as LoginErrorResponse };
}
