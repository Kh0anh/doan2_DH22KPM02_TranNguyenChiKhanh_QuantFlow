// ===================================================================
// QuantFlow — useSystemAlerts Hook
// Task 3.4.6 — System Alerts / Toast Notifications
// ===================================================================
//
// Global hook that listens to WebSocket events and connection state
// changes, then fires Sonner toasts for critical system alerts.
//
// Alert categories:
//   1. Disconnection: WS goes to reconnecting/disconnected
//   2. Bot Errors: ORDER_REJECTED, UNIT_COST_EXCEEDED, API_CONNECTION_LOST,
//                  EXECUTION_ERROR, LIQUIDATION_ALERT
//   3. Bot Status: Running → Error / Stopped transitions
//
// Deduplication: Same error_type + bot_id throttled within 10 seconds.
//
// Mount this hook once at the top-level layout (TopBar) so it runs
// globally when the user is authenticated.
//
// Spec: docs/api/websocket.md §3.3
// SRS: NFR-UX-02, FR-MON
// ===================================================================

"use client";

import { useEffect, useRef } from "react";
import { toast } from "sonner";
import { useWebSocket } from "@/lib/hooks/use-websocket";
import {
  parseBotError,
  parseBotStatusChange,
  type BotErrorPayload,
  type BotStatusChangePayload,
} from "@/lib/websocket/ws-channels";

// -----------------------------------------------------------------
// Constants
// -----------------------------------------------------------------

/** Deduplication window in milliseconds. */
const DEDUP_WINDOW_MS = 10_000;

// -----------------------------------------------------------------
// Error type → toast config mapping
// -----------------------------------------------------------------

interface AlertConfig {
  title: string;
  level: "error" | "warning";
  icon?: string;
}

const ERROR_TYPE_MAP: Record<string, AlertConfig> = {
  ORDER_REJECTED: {
    title: "⛔ Lệnh bị từ chối",
    level: "error",
  },
  UNIT_COST_EXCEEDED: {
    title: "⚠️ Vượt giới hạn Unit Cost",
    level: "error",
  },
  API_CONNECTION_LOST: {
    title: "🔌 Mất kết nối API sàn",
    level: "warning",
  },
  EXECUTION_ERROR: {
    title: "❌ Lỗi thực thi chiến lược",
    level: "error",
  },
  LIQUIDATION_ALERT: {
    title: "🚨 Cảnh báo Liquidation",
    level: "error",
  },
};

// -----------------------------------------------------------------
// Hook
// -----------------------------------------------------------------

/**
 * Global system alerts hook. Must be mounted once at app top-level
 * (inside AuthProvider). Listens to WS events and fires toasts.
 */
export function useSystemAlerts(): void {
  const { connectionState, on, off } = useWebSocket();

  // Deduplication set: stores "errorType:botId" keys
  const recentAlerts = useRef<Set<string>>(new Set());

  // Track previous connection state for transition detection
  const prevState = useRef(connectionState);

  // ------- Deduplicate helper -------
  function isDuplicate(key: string): boolean {
    if (recentAlerts.current.has(key)) return true;
    recentAlerts.current.add(key);
    setTimeout(() => {
      recentAlerts.current.delete(key);
    }, DEDUP_WINDOW_MS);
    return false;
  }

  // ------- Connection state change → toast -------
  useEffect(() => {
    const prev = prevState.current;
    prevState.current = connectionState;

    // Skip initial mount (prev === current)
    if (prev === connectionState) return;

    if (connectionState === "reconnecting" && prev === "connected") {
      toast.warning("Mất kết nối WebSocket", {
        description: "Đang thử kết nối lại...",
        duration: 5000,
      });
    }

    if (connectionState === "disconnected" && prev !== "disconnected") {
      toast.error("Ngắt kết nối WebSocket", {
        description: "Không thể kết nối đến máy chủ. Dữ liệu real-time tạm ngưng.",
        duration: 8000,
      });
    }

    if (connectionState === "connected" && prev === "reconnecting") {
      toast.success("Đã kết nối lại", {
        description: "Kết nối WebSocket đã được khôi phục.",
        duration: 3000,
      });
    }
  }, [connectionState]);

  // ------- Bot error events → toast -------
  useEffect(() => {
    const handleBotError = (data: unknown) => {
      const parsed = parseBotError(data);
      if (!parsed) return;

      const dedupKey = `${parsed.errorType}:${parsed.botId}`;
      if (isDuplicate(dedupKey)) return;

      fireBotErrorToast(parsed);
    };

    on("bot_error", handleBotError);
    return () => off("bot_error", handleBotError);
  }, [on, off]);

  // ------- Bot status change events → toast -------
  useEffect(() => {
    const handleStatusChange = (data: unknown) => {
      const parsed = parseBotStatusChange(data);
      if (!parsed) return;

      const dedupKey = `status:${parsed.newStatus}:${parsed.botId}`;
      if (isDuplicate(dedupKey)) return;

      fireBotStatusToast(parsed);
    };

    on("bot_status_change", handleStatusChange);
    return () => off("bot_status_change", handleStatusChange);
  }, [on, off]);
}

// -----------------------------------------------------------------
// Toast fire helpers
// -----------------------------------------------------------------

function fireBotErrorToast(payload: BotErrorPayload): void {
  const config = ERROR_TYPE_MAP[payload.errorType] ?? {
    title: "⚠️ Lỗi Bot",
    level: "error" as const,
  };

  const description = `[${payload.botName}] ${payload.message}`;

  if (config.level === "warning") {
    toast.warning(config.title, {
      description,
      duration: 6000,
    });
  } else {
    toast.error(config.title, {
      description,
      duration: 8000,
    });
  }
}

function fireBotStatusToast(payload: BotStatusChangePayload): void {
  if (payload.newStatus === "Error") {
    toast.error(`🔴 Bot "${payload.botName}" gặp lỗi`, {
      description: payload.reason || `Trạng thái: ${payload.previousStatus} → Error`,
      duration: 8000,
    });
  } else if (payload.newStatus === "Stopped" && payload.previousStatus === "Running") {
    toast.info(`Bot "${payload.botName}" đã dừng`, {
      description: payload.reason || "Bot đã chuyển sang trạng thái Stopped.",
      duration: 5000,
    });
  }
}
