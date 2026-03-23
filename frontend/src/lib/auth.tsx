/**
 * [3.1.3] AuthContext — session management, protected route guard, token refresh.
 *
 * Flow on mount:
 *   1. GET /api/v1/auth/me  — validates current session
 *      → 200: set user, schedule token refresh
 *      → 401/error: redirect to /login
 *   2. POST /api/v1/auth/refresh — refreshes the JWT cookie; returns expires_at
 *      → schedules the next refresh at (expires_at - 5 min)
 *      → 401: redirect to /login (session expired)
 *
 * While loading: renders a full-screen spinner.
 * Not authenticated: renders nothing (redirect is in-flight).
 * Authenticated: renders children.
 */
"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import type { UserProfile } from "@/types";

// ---------------------------------------------------------------------------
// Context shape
// ---------------------------------------------------------------------------

interface AuthContextValue {
  user: UserProfile | null;
  isLoading: boolean;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used inside <AuthProvider>");
  }
  return ctx;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Schedule the next token refresh at `expires_at - offsetMs` (default 5 min). */
function scheduleRefresh(
  expiresAt: string,
  refreshFn: () => void,
  offsetMs = 5 * 60 * 1000,
): ReturnType<typeof setTimeout> | null {
  const msUntilExpiry = new Date(expiresAt).getTime() - Date.now();
  const delay = msUntilExpiry - offsetMs;
  if (delay <= 0) {
    // Token is already close to expiring — refresh immediately
    refreshFn();
    return null;
  }
  return setTimeout(refreshFn, delay);
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  /** Clear any pending refresh timer. */
  const clearRefreshTimer = useCallback(() => {
    if (refreshTimerRef.current !== null) {
      clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }
  }, []);

  /** POST /auth/refresh — rotate cookie, schedule next refresh. */
  const doRefresh = useCallback(async () => {
    try {
      const res = await fetch("/api/v1/auth/refresh", {
        method: "POST",
        credentials: "include",
      });
      if (res.status === 401) {
        clearRefreshTimer();
        setUser(null);
        router.replace("/login?from=logout");
        return;
      }
      if (res.ok) {
        const body = await res.json();
        const expiresAt: string | undefined = body?.data?.expires_at;
        if (expiresAt) {
          clearRefreshTimer();
          refreshTimerRef.current = scheduleRefresh(expiresAt, doRefresh);
        }
      }
    } catch {
      // Network error — keep user logged in until next natural expiry
    }
  }, [clearRefreshTimer, router]);

  /** GET /auth/me — check session on mount. */
  useEffect(() => {
    let cancelled = false;

    async function checkSession() {
      try {
        const res = await fetch("/api/v1/auth/me", {
          credentials: "include",
        });

        if (cancelled) return;

        if (!res.ok) {
          router.replace("/login?from=logout");
          return;
        }

        const body = await res.json();
        const raw = body?.data?.user;
        if (!raw) {
          router.replace("/login?from=logout");
          return;
        }

        const profile: UserProfile = {
          id: raw.id,
          username: raw.username,
          createdAt: raw.created_at,
          updatedAt: raw.updated_at ?? raw.created_at,
        };
        setUser(profile);

        // Kick off first token refresh to get expires_at
        doRefresh();
      } catch {
        if (!cancelled) {
          router.replace("/login?from=logout");
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    checkSession();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /** Logout — hits API then clears local state. */
  const logout = useCallback(async () => {
    clearRefreshTimer();
    try {
      await fetch("/api/v1/auth/logout", {
        method: "POST",
        credentials: "include",
      });
    } finally {
      setUser(null);
      router.replace("/login?from=logout");
    }
  }, [clearRefreshTimer, router]);

  // Clean up timer on unmount
  useEffect(() => {
    return () => clearRefreshTimer();
  }, [clearRefreshTimer]);

  // ---------------------------------------------------------------------------
  // Render gate
  // ---------------------------------------------------------------------------

  if (isLoading) {
    return (
      <div className="flex h-screen w-full items-center justify-center bg-background">
        <Loader2 className="size-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!user) {
    // Redirect is in-flight — render nothing
    return null;
  }

  return (
    <AuthContext.Provider value={{ user, isLoading, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
