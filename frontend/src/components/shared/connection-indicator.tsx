// ===================================================================
// QuantFlow — Connection Indicator
// Task 3.4.4 — WebSocket Client Manager
// ===================================================================
//
// Compact status chip displaying the *effective* connection state,
// combining both the WebSocket link to the backend AND the Binance
// exchange API-key status.
//
// Display logic:
//   WS connecting / reconnecting → yellow "Connecting…" / "Reconnecting…"
//   WS disconnected              → red    "Disconnected"
//   WS connected + exchange loading → yellow "Checking…"
//   WS connected + no API key       → red    "No Exchange"
//   WS connected + exchange ok      → green  "Connected"
//
// frontend_flows.md §3.2.5: Connection indicator in TopBar
// frontend_structure.md: src/components/shared/connection-indicator.tsx
// ===================================================================

"use client";

import { useEffect, useState } from "react";
import { useWebSocket } from "@/lib/hooks/use-websocket";

// -----------------------------------------------------------------
// State configuration
// -----------------------------------------------------------------

interface StateConfig {
  label: string;
  dotClass: string;
  textClass: string;
  pulse: boolean;
}

const STATES: Record<string, StateConfig> = {
  connected: {
    label: "Connected",
    dotClass: "bg-emerald-500",
    textClass: "text-emerald-500",
    pulse: true,
  },
  connecting: {
    label: "Connecting...",
    dotClass: "bg-yellow-500",
    textClass: "text-yellow-500",
    pulse: true,
  },
  reconnecting: {
    label: "Reconnecting...",
    dotClass: "bg-yellow-500",
    textClass: "text-yellow-500",
    pulse: true,
  },
  checking: {
    label: "Checking...",
    dotClass: "bg-yellow-500",
    textClass: "text-yellow-500",
    pulse: true,
  },
  no_exchange: {
    label: "No Exchange",
    dotClass: "bg-red-500",
    textClass: "text-red-500",
    pulse: false,
  },
  disconnected: {
    label: "Disconnected",
    dotClass: "bg-red-500",
    textClass: "text-red-500",
    pulse: false,
  },
};

// -----------------------------------------------------------------
// Hook: fetch Binance exchange API-key connection status
// -----------------------------------------------------------------

type ExchangeStatus = "loading" | "connected" | "disconnected";

function useExchangeStatus(): ExchangeStatus {
  const [status, setStatus] = useState<ExchangeStatus>("loading");

  useEffect(() => {
    let cancelled = false;

    async function check() {
      try {
        const res = await fetch("/api/v1/exchange/api-keys", {
          credentials: "include",
        });
        if (cancelled) return;
        if (!res.ok) {
          setStatus("disconnected");
          return;
        }
        const body = await res.json();
        const raw = body?.data;
        if (raw && raw.status === "Connected") {
          setStatus("connected");
        } else {
          setStatus("disconnected");
        }
      } catch {
        if (!cancelled) setStatus("disconnected");
      }
    }

    check();
    return () => { cancelled = true; };
  }, []);

  return status;
}

// -----------------------------------------------------------------
// Derive effective display state
// -----------------------------------------------------------------

function deriveDisplayState(
  wsState: string,
  exchangeStatus: ExchangeStatus,
): StateConfig {
  // WS not yet connected — show WS state directly
  if (wsState === "connecting") return STATES.connecting;
  if (wsState === "reconnecting") return STATES.reconnecting;
  if (wsState === "disconnected") return STATES.disconnected;

  // WS is connected — factor in exchange status
  if (exchangeStatus === "loading") return STATES.checking;
  if (exchangeStatus === "connected") return STATES.connected;
  return STATES.no_exchange;
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function ConnectionIndicator() {
  const { connectionState } = useWebSocket();
  const exchangeStatus = useExchangeStatus();
  const config = deriveDisplayState(connectionState, exchangeStatus);

  return (
    <div
      className="flex items-center gap-1.5 rounded-full border border-border bg-card/60 px-2.5 py-1"
      role="status"
      aria-label={`Connection: ${config.label}`}
    >
      {/* Dot with optional pulse animation */}
      <span className="relative flex size-2">
        {config.pulse && (
          <span
            className={`absolute inline-flex size-full animate-ping rounded-full opacity-75 ${config.dotClass}`}
          />
        )}
        <span
          className={`relative inline-flex size-2 rounded-full ${config.dotClass}`}
        />
      </span>

      {/* Label */}
      <span className={`text-[10px] font-medium leading-none ${config.textClass}`}>
        {config.label}
      </span>
    </div>
  );
}
