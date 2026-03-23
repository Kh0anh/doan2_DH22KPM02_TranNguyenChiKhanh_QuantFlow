// ===================================================================
// QuantFlow — useWebSocket Hook
// Task 3.4.4 — WebSocket Client Manager
// ===================================================================
//
// React hook wrapping WSManager singleton. Provides:
//   - connectionState: reactive connection status
//   - subscribe/unsubscribe: channel management
//   - on/off: event listener registration
//
// Auto-connects when user is authenticated, disconnects on logout.
// Redirects to /login on AUTH_FAILED close code (4001).
//
// Spec: docs/api/websocket.md §1.2–1.3
// SRS: NFR-SEC-04, FR-MON
// ===================================================================

"use client";

import { useEffect, useMemo, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { WSManager } from "@/lib/websocket/ws-manager";
import type {
  WSConnectionState,
  WSChannelName,
  WSEventCallback,
} from "@/types/websocket";

// -----------------------------------------------------------------
// WebSocket Endpoint
// -----------------------------------------------------------------

/**
 * Compute the WS endpoint URL based on the current browser location.
 * - Development: ws://localhost:8080/ws/v1
 * - Production:  wss://<host>/ws/v1
 *
 * Uses Next.js rewrites in dev to proxy /ws/v1 → localhost:8080.
 * In production, the same domain is used (Nginx routes /ws/v1 to backend).
 */
function getWSUrl(): string {
  if (typeof window === "undefined") return "";

  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/ws/v1`;
}

// -----------------------------------------------------------------
// Hook Return Type
// -----------------------------------------------------------------

export interface UseWebSocketReturn {
  /** Current connection state (reactive via useSyncExternalStore). */
  connectionState: WSConnectionState;

  /** Subscribe to a WebSocket channel (tracked for re-subscribe). */
  subscribe: (channel: WSChannelName, params?: Record<string, string>) => void;

  /** Unsubscribe from a WebSocket channel. */
  unsubscribe: (channel: WSChannelName, params?: Record<string, string>) => void;

  /** Register an event listener. */
  on: (event: string, callback: WSEventCallback) => void;

  /** Remove an event listener. */
  off: (event: string, callback: WSEventCallback) => void;
}

// -----------------------------------------------------------------
// Hook Implementation
// -----------------------------------------------------------------

/**
 * React hook for WebSocket connectivity. Auto-connects when
 * the user is authenticated and auto-disconnects on logout.
 *
 * Uses `useSyncExternalStore` to provide a reactive connection state
 * that triggers re-renders only when the state actually changes.
 *
 * Example:
 * ```tsx
 * const { connectionState, subscribe, on, off } = useWebSocket();
 *
 * useEffect(() => {
 *   const handler = (data) => console.log(data);
 *   on("market_ticker", handler);
 *   subscribe("market_ticker", { symbol: "BTCUSDT" });
 *   return () => off("market_ticker", handler);
 * }, []);
 * ```
 */
export function useWebSocket(): UseWebSocketReturn {
  const { user } = useAuth();
  const router = useRouter();
  const manager = WSManager.getInstance();

  // ── Reactive connection state via useSyncExternalStore ───────────
  const connectionState = useSyncExternalStore(
    (cb) => manager.subscribeState(cb),
    () => manager.getState(),
    () => "disconnected" as WSConnectionState, // SSR snapshot
  );

  // ── Auto-connect when authenticated, disconnect on logout ───────
  useEffect(() => {
    if (!user) {
      manager.disconnect();
      return;
    }

    const wsUrl = getWSUrl();
    if (!wsUrl) return; // SSR guard

    manager.connect(wsUrl);

    // Handle auth failure → redirect to login
    const handleAuthFailed = () => {
      manager.disconnect();
      router.replace("/login?from=logout");
    };
    manager.on("auth_failed", handleAuthFailed);

    return () => {
      manager.off("auth_failed", handleAuthFailed);
      // Note: do NOT disconnect here. The WSManager singleton outlives
      // individual component mounts. Disconnection happens on logout
      // (user becomes null) or via explicit disconnect().
    };
  }, [user, manager, router]);

  // ── Stable API references ──────────────────────────────────────
  const api = useMemo<UseWebSocketReturn>(
    () => ({
      connectionState,
      subscribe: (channel, params) => manager.subscribe(channel, params),
      unsubscribe: (channel, params) => manager.unsubscribe(channel, params),
      on: (event, callback) => manager.on(event, callback),
      off: (event, callback) => manager.off(event, callback),
    }),
    [connectionState, manager],
  );

  return api;
}
