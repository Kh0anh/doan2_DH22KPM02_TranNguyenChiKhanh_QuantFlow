// ===================================================================
// QuantFlow — Connection Indicator
// Task 3.4.4 — WebSocket Client Manager
// ===================================================================
//
// Compact status chip displaying real-time WebSocket connection state.
// Renders in TopBar (top-bar.tsx) between logo and user menu.
//
// States (websocket.md §1.3):
//   Connected    → green  pulse dot + "Connected"
//   Connecting   → yellow pulse dot + "Connecting..."
//   Reconnecting → yellow pulse dot + "Reconnecting..."
//   Disconnected → red    static dot + "Disconnected"
//
// frontend_flows.md §3.2.5: Connection indicator in TopBar
// frontend_structure.md: src/components/shared/connection-indicator.tsx
// ===================================================================

"use client";

import { useWebSocket } from "@/lib/hooks/use-websocket";
import type { WSConnectionState } from "@/types/websocket";

// -----------------------------------------------------------------
// State configuration
// -----------------------------------------------------------------

interface StateConfig {
  label: string;
  dotClass: string;
  textClass: string;
}

const STATE_MAP: Record<WSConnectionState, StateConfig> = {
  connected: {
    label: "Connected",
    dotClass: "bg-emerald-500",
    textClass: "text-emerald-500",
  },
  connecting: {
    label: "Connecting...",
    dotClass: "bg-yellow-500",
    textClass: "text-yellow-500",
  },
  reconnecting: {
    label: "Reconnecting...",
    dotClass: "bg-yellow-500",
    textClass: "text-yellow-500",
  },
  disconnected: {
    label: "Disconnected",
    dotClass: "bg-red-500",
    textClass: "text-red-500",
  },
};

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function ConnectionIndicator() {
  const { connectionState } = useWebSocket();
  const config = STATE_MAP[connectionState];
  const isActive = connectionState === "connected" || connectionState === "connecting" || connectionState === "reconnecting";

  return (
    <div
      className="flex items-center gap-1.5 rounded-full border border-border bg-card/60 px-2.5 py-1"
      role="status"
      aria-label={`WebSocket: ${config.label}`}
    >
      {/* Dot with optional pulse animation */}
      <span className="relative flex size-2">
        {isActive && (
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
