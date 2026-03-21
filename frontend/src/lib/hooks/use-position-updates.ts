// ===================================================================
// QuantFlow — usePositionUpdates Hook
// Task 3.3.5 + 3.4.5 — Position and PnL Display + Real-time WS
// ===================================================================
//
// Responsibilities:
//   - Subscribe to WS `position_update` channel for real-time PnL
//   - Fallback to mock PnL jitter when WS is not connected
//
// WS Channel: position_update (websocket.md §3.3)
// SRS: FR-MON-03
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import type { BotItem } from "@/lib/hooks/use-bot-data";
import { useWebSocket } from "@/lib/hooks/use-websocket";
import { parsePositionUpdate } from "@/lib/websocket/ws-channels";

// -----------------------------------------------------------------
// Types for position updates
// -----------------------------------------------------------------

export interface PositionUpdate {
  botId: string;
  totalPnl: number;
  unrealizedPnl: number | null;
  timestamp: string;
}

// -----------------------------------------------------------------
// Hook: subscribes to real-time position updates for a set of bots
// -----------------------------------------------------------------

export function usePositionUpdates(bots: BotItem[]) {
  const [pnlOverrides, setPnlOverrides] = useState<
    Map<string, { totalPnl: number; unrealizedPnl: number | null }>
  >(new Map());

  const botsRef = useRef(bots);
  botsRef.current = bots;
  const { connectionState, subscribe, unsubscribe, on, off } = useWebSocket();

  // ------- WS real-time position updates (Task 3.4.5) -------
  useEffect(() => {
    if (connectionState !== "connected") return;

    // position_update channel does not require params — server sends
    // updates for all running bots owned by the authenticated user.
    subscribe("position_update");

    const handleUpdate = (data: unknown) => {
      const parsed = parsePositionUpdate(data);
      if (!parsed) return;

      setPnlOverrides((prev) => {
        const next = new Map(prev);
        next.set(parsed.botId, {
          totalPnl: parsed.totalPnl,
          unrealizedPnl: parsed.position?.unrealizedPnl ?? null,
        });
        return next;
      });
    };
    on("position_update", handleUpdate);

    return () => {
      off("position_update", handleUpdate);
      unsubscribe("position_update");
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connectionState]);

  // ------- Mock simulation (fallback when WS disconnected) -------
  useEffect(() => {
    if (connectionState === "connected") return;
    const runningBots = bots.filter((b) => b.status === "Running");
    if (runningBots.length === 0) return;

    const interval = setInterval(() => {
      setPnlOverrides((prev) => {
        const next = new Map(prev);
        const currentBots = botsRef.current.filter(
          (b) => b.status === "Running",
        );

        for (const bot of currentBots) {
          const prevData = next.get(bot.id);
          const baseTotalPnl = prevData?.totalPnl ?? bot.totalPnl;
          const baseUnrealizedPnl =
            prevData?.unrealizedPnl ?? bot.position?.unrealizedPnl ?? 0;

          // Jitter: ±0.5% of current value or ±0.5 USDT minimum
          const jitterTotal =
            (Math.random() - 0.5) * Math.max(Math.abs(baseTotalPnl) * 0.005, 0.5);
          const jitterUnrealized =
            (Math.random() - 0.5) *
            Math.max(Math.abs(baseUnrealizedPnl) * 0.01, 0.3);

          next.set(bot.id, {
            totalPnl: Math.round((baseTotalPnl + jitterTotal) * 100) / 100,
            unrealizedPnl: bot.position
              ? Math.round((baseUnrealizedPnl + jitterUnrealized) * 100) / 100
              : null,
          });
        }

        return next;
      });
    }, 2000);

    return () => clearInterval(interval);
  }, [connectionState, bots]);

  // ------- Get live PnL for a specific bot -------
  const getLivePnl = useCallback(
    (botId: string) => {
      return pnlOverrides.get(botId) ?? null;
    },
    [pnlOverrides],
  );

  return { pnlOverrides, getLivePnl };
}
