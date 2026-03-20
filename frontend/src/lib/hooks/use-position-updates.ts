// ===================================================================
// QuantFlow — usePositionUpdates Hook
// Task 3.3.5 — Position and PnL Display (Unrealized PnL real-time)
// ===================================================================
//
// Simulates WebSocket `position_update` events:
//   - Jitters unrealized_pnl and total_pnl every 2 seconds
//   - Integration point for Task 3.4.4 (WS Manager)
//
// WS Channel: position_update (websocket.md §3.3)
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import type { BotItem } from "@/lib/hooks/use-bot-data";

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

  // ------- Mock simulation (replace with WS in Task 3.4.4) -------
  useEffect(() => {
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
  }, [bots]);

  // ------- Get live PnL for a specific bot -------
  const getLivePnl = useCallback(
    (botId: string) => {
      return pnlOverrides.get(botId) ?? null;
    },
    [pnlOverrides],
  );

  return { pnlOverrides, getLivePnl };
}
