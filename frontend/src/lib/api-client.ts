/**
 * api-client.ts — Centralized REST API client (minimal bootstrap).
 *
 * Base URL resolution:
 * - In Docker (Nginx reverse proxy): requests to /api/v1/... are proxied to backend.
 * - In local dev (no Nginx): use NEXT_PUBLIC_API_URL (default http://localhost:8080).
 *
 * Credentials: "include" is required so the browser sends / receives the
 * HttpOnly JWT cookie set by the backend (api.yaml §POST /auth/login).
 */

// Normalize: strip trailing slash and /api/v1 suffix (if env var already includes it)
// so fetch calls can always safely append /api/v1/...
const BASE_URL = (
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
)
  .replace(/\/api\/v1\/?$/, "")  // strip /api/v1 if already present
  .replace(/\/$/, "");           // strip trailing slash

// ─── Shared domain types ─────────────────────────────────────────────────────

export interface UserProfile {
  id: string;
  username: string;
  created_at: string;
}

// ─── POST /auth/login ─────────────────────────────────────────────────────────

export interface LoginSuccessResponse {
  message: string;
  data: { user: UserProfile };
}

export interface LoginErrorResponse {
  error: {
    code: string;
    message: string;
    remaining_attempts?: number; // 401
    locked_until?: string;       // 403 — ISO 8601
  };
}

export type LoginResult =
  | { ok: true; status: 200; data: LoginSuccessResponse }
  | { ok: false; status: 401 | 403 | 500; data: LoginErrorResponse };

/**
 * authLogin — POST /api/v1/auth/login (api.yaml §Authentication).
 */
export async function authLogin(
  username: string,
  password: string
): Promise<LoginResult> {
  const res = await fetch(`${BASE_URL}/api/v1/auth/login`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const data = await res.json();
  if (res.ok) return { ok: true, status: 200, data: data as LoginSuccessResponse };
  const status = res.status === 401 || res.status === 403 ? res.status : 500;
  return { ok: false, status, data: data as LoginErrorResponse };
}

// ─── GET /auth/me ─────────────────────────────────────────────────────────────

export type AuthMeResult =
  | { ok: true; user: UserProfile }
  | { ok: false };

/**
 * authMe — GET /api/v1/auth/me.
 * Called on app init to validate the session cookie and hydrate user state.
 */
export async function authMe(): Promise<AuthMeResult> {
  try {
    const res = await fetch(`${BASE_URL}/api/v1/auth/me`, {
      method: "GET",
      credentials: "include",
    });
    if (!res.ok) return { ok: false };
    const data = await res.json();
    return { ok: true, user: data.data.user as UserProfile };
  } catch {
    return { ok: false };
  }
}

// ─── POST /auth/logout ────────────────────────────────────────────────────────

/**
 * authLogout — POST /api/v1/auth/logout.
 * Backend clears HttpOnly cookie (Max-Age=0).
 */
export async function authLogout(): Promise<void> {
  try {
    await fetch(`${BASE_URL}/api/v1/auth/logout`, {
      method: "POST",
      credentials: "include",
    });
  } catch {
    // Best-effort — redirect to login regardless.
  }
}

// ─── POST /auth/refresh ───────────────────────────────────────────────────────

export type AuthRefreshResult =
  | { ok: true; expiresAt: string }
  | { ok: false };

/**
 * authRefresh — POST /api/v1/auth/refresh.
 * Issues a new JWT (Token Rotation). Called automatically ~23h after login.
 */
export async function authRefresh(): Promise<AuthRefreshResult> {
  try {
    const res = await fetch(`${BASE_URL}/api/v1/auth/refresh`, {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) return { ok: false };
    const data = await res.json();
    return { ok: true, expiresAt: data.data.expires_at as string };
  } catch {
    return { ok: false };
  }
}

