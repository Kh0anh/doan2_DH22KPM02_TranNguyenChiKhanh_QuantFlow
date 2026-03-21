// ===================================================================
// QuantFlow — useBotLogs Hook
// Task 3.3.4 + 3.4.5 — Bot Logs Console + Real-time WS
// ===================================================================
//
// Responsibilities:
//   - Fetch historical logs from GET /bots/{id}/logs (with mock fallback)
//   - Subscribe to WS `bot_logs` channel for real-time log append
//   - Fallback to mock simulation when WS is not connected
//   - Limit buffer to 1000 lines (virtual scroll requirement)
//   - Provide cursor for loading older logs
//
// WS Channel: bot_logs (websocket.md §3.2)
// SRS: FR-RUN-05
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { botApi } from "@/lib/api-client";
import { useWebSocket } from "@/lib/hooks/use-websocket";
import { parseBotLog } from "@/lib/websocket/ws-channels";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

export interface LogEntry {
  id: number;
  timestamp: string;        // ISO string
  formattedTime: string;    // [HH:MM:SS]
  level: "info" | "order" | "skip" | "error";
  message: string;
}

const MAX_LOG_LINES = 1000;

// -----------------------------------------------------------------
// Time formatter
// -----------------------------------------------------------------

function formatTime(date: Date): string {
  const h = String(date.getHours()).padStart(2, "0");
  const m = String(date.getMinutes()).padStart(2, "0");
  const s = String(date.getSeconds()).padStart(2, "0");
  return `[${h}:${m}:${s}]`;
}



// -----------------------------------------------------------------
// Hook
// -----------------------------------------------------------------

export function useBotLogs(botId: string) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);
  const cursorRef = useRef<string | null>(null);
  const { connectionState, subscribe, unsubscribe, on, off } = useWebSocket();

  // ------- Fetch initial logs -------
  useEffect(() => {
    let cancelled = false;

    async function fetchLogs() {
      setIsLoading(true);
      try {
        const res = await botApi.getLogs(botId, { limit: 50 });
        if (cancelled) return;

        const mapped: LogEntry[] = res.data.map((entry) => {
          const time = new Date(entry.created_at);
          let level: LogEntry["level"] = "info";
          if (entry.action_decision?.includes("lệnh")) level = "order";
          else if (entry.action_decision?.includes("Bỏ qua")) level = "skip";
          else if (entry.message.toLowerCase().includes("lỗi") || entry.message.toLowerCase().includes("error")) level = "error";

          return {
            id: entry.id,
            timestamp: entry.created_at,
            formattedTime: formatTime(time),
            level,
            message: entry.action_decision
              ? `[${entry.action_decision}] ${entry.message}`
              : entry.message,
          };
        });

        setLogs(mapped);
        setHasMore(res.pagination.has_more);
        cursorRef.current = res.pagination.next_cursor;
      } catch {
        // API unavailable — show empty state
        if (cancelled) return;
        setLogs([]);
        setHasMore(false);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }

    fetchLogs();
    return () => { cancelled = true; };
  }, [botId]);

  // ------- WS real-time log append (Task 3.4.5) -------
  useEffect(() => {
    if (connectionState !== "connected" || !botId) return;

    subscribe("bot_logs", { bot_id: botId });

    const handleLog = (data: unknown) => {
      const parsed = parseBotLog(data);
      if (!parsed || parsed.botId !== botId) return;

      const time = new Date(parsed.log.createdAt);
      let level: LogEntry["level"] = "info";
      if (parsed.log.actionDecision.includes("lệnh")) level = "order";
      else if (parsed.log.actionDecision.includes("Bỏ qua")) level = "skip";
      else if (parsed.log.message.toLowerCase().includes("lỗi") || parsed.log.message.toLowerCase().includes("error")) level = "error";

      const newEntry: LogEntry = {
        id: parsed.log.id,
        timestamp: parsed.log.createdAt,
        formattedTime: formatTime(time),
        level,
        message: parsed.log.actionDecision
          ? `[${parsed.log.actionDecision}] ${parsed.log.message}`
          : parsed.log.message,
      };

      setLogs((prev) => {
        const next = [...prev, newEntry];
        if (next.length > MAX_LOG_LINES) {
          return next.slice(next.length - MAX_LOG_LINES);
        }
        return next;
      });
    };
    on("bot_log", handleLog);

    return () => {
      off("bot_log", handleLog);
      unsubscribe("bot_logs", { bot_id: botId });
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connectionState, botId]);



  // ------- Load more (older logs) -------
  const loadMore = useCallback(async () => {
    if (!cursorRef.current || !hasMore) return;
    try {
      const res = await botApi.getLogs(botId, {
        cursor: cursorRef.current,
        limit: 50,
      });
      const mapped: LogEntry[] = res.data.map((entry) => {
        const time = new Date(entry.created_at);
        return {
          id: entry.id,
          timestamp: entry.created_at,
          formattedTime: formatTime(time),
          level: "info" as const,
          message: entry.message,
        };
      });
      setLogs((prev) => [...mapped, ...prev].slice(-MAX_LOG_LINES));
      setHasMore(res.pagination.has_more);
      cursorRef.current = res.pagination.next_cursor;
    } catch {
      // Silently fail in mock mode
    }
  }, [botId, hasMore]);

  return { logs, isLoading, hasMore, loadMore };
}
