"use client";

/**
 * auth.tsx — AuthContext, AuthProvider, and useAuth hook (Task 3.1.3).
 *
 * Pattern:
 *  - AuthProvider wraps the entire app (mounted in root layout.tsx).
 *  - On mount: GET /auth/me to validate session cookie and hydrate user state.
 *    - 200 → set user, schedule refresh timer (23h)
 *    - 401 → redirect /login
 *  - logout(): POST /auth/logout → clear user → redirect /login
 *  - Refresh timer: setTimeout(23h) → POST /auth/refresh
 *    - 200 → reschedule next refresh
 *    - 401 → session expired → redirect /login
 *
 * Security: no token is ever stored in localStorage/sessionStorage.
 * The HttpOnly cookie is managed entirely by the browser and backend.
 * This context only holds the user profile for display purposes.
 *
 * WBS 3.1.3 · SRS NFR-SEC-04 · api.yaml §/auth/me, §/auth/refresh, §/auth/logout
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { useRouter } from "next/navigation";
import { authMe, authLogout, authRefresh, UserProfile } from "@/lib/api-client";

// ─── Context shape ────────────────────────────────────────────────────────────

interface AuthContextValue {
  /** Authenticated user profile. null = unauthenticated or loading. */
  user: UserProfile | null;
  /** True while the initial /auth/me call is in-flight. */
  isLoading: boolean;
  /** True once /auth/me returned 200. */
  isAuthenticated: boolean;
  /** Sign the user out: POST /auth/logout → redirect /login. */
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

// ─── Hook ─────────────────────────────────────────────────────────────────────

/** useAuth — consume AuthContext. Must be used inside <AuthProvider>. */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within <AuthProvider>");
  return ctx;
}

// ─── Refresh timer helpers ────────────────────────────────────────────────────

/** 23 hours in ms — safe margin before the 24h JWT expires. */
const REFRESH_INTERVAL_MS = 23 * 60 * 60 * 1000;

// ─── Provider ─────────────────────────────────────────────────────────────────

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Keep a ref to the refresh timer so we can clear it on logout/unmount.
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ── Refresh logic ───────────────────────────────────────────────────────────
  /**
   * scheduleRefresh — sets a 23h timer to call /auth/refresh.
   * If refresh succeeds, reschedule. If it fails (401), the session
   * has expired: clear user and redirect to /login.
   */
  const scheduleRefresh = useCallback(() => {
    // Clear any existing timer before scheduling a new one.
    if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);

    refreshTimerRef.current = setTimeout(async () => {
      const result = await authRefresh();
      if (result.ok) {
        // Token rotated successfully — schedule the next refresh.
        scheduleRefresh();
      } else {
        // Token expired or invalid — force re-login.
        setUser(null);
        router.replace("/login");
      }
    }, REFRESH_INTERVAL_MS);
  }, [router]);

  // ── App-init: validate session ──────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;

    (async () => {
      const result = await authMe();

      if (cancelled) return;

      if (result.ok) {
        setUser(result.user);
        scheduleRefresh();
      } else {
        // No valid session — only redirect if we're not already on /login.
        if (!window.location.pathname.startsWith("/login")) {
          router.replace("/login");
        }
      }

      setIsLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // intentionally run once on mount

  // ── Cleanup refresh timer on unmount ───────────────────────────────────────
  useEffect(() => {
    return () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
    };
  }, []);

  // ── Logout ─────────────────────────────────────────────────────────────────
  const logout = useCallback(async () => {
    // Cancel refresh timer.
    if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);

    // Best-effort server-side cookie clear.
    await authLogout();

    // Clear local user state and redirect.
    setUser(null);
    router.replace("/login");
  }, [router]);

  // ── Context value ──────────────────────────────────────────────────────────
  const value: AuthContextValue = {
    user,
    isLoading,
    isAuthenticated: user !== null,
    logout,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
