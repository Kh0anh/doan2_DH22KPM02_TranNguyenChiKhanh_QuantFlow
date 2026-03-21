// ===================================================================
// QuantFlow — useBotData Hook
// Task 3.3.3 — Bot Management Panel
// ===================================================================
//
// Responsibilities:
//   - Fetch bot list from GET /bots (with mock fallback)
//   - CRUD operations: create, start, stop, delete
//   - Expand/collapse state per bot
//   - Fetch detail (position + orders) when expanded
//
// Integration point for Task 3.4.4:
//   Replace mock data with real API responses and WS updates.
// ===================================================================

"use client";

import { useState, useEffect, useCallback } from "react";
import { botApi, type BotSummaryResponse } from "@/lib/api-client";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

export interface BotItem {
  id: string;
  name: string;
  symbol: string;
  strategyId: string;
  strategyName: string;
  strategyVersion: number;
  status: "Running" | "Stopped" | "Error";
  totalPnl: number;
  createdAt: string;
  updatedAt: string;
  // Expanded detail (loaded on demand)
  position?: {
    side: "Long" | "Short";
    entryPrice: number;
    quantity: number;
    leverage: number;
    unrealizedPnl: number;
    marginType: string;
  } | null;
  openOrders?: {
    orderId: string;
    side: string;
    type: string;
    price: number;
    quantity: number;
    status: string;
  }[];
}



// -----------------------------------------------------------------
// Map API response to frontend type
// -----------------------------------------------------------------

function mapBot(s: BotSummaryResponse): BotItem {
  return {
    id: s.id,
    name: s.bot_name,
    symbol: s.symbol,
    strategyId: s.strategy_id,
    strategyName: s.strategy_name,
    strategyVersion: s.strategy_version,
    status: s.status,
    totalPnl: s.total_pnl,
    createdAt: s.created_at,
    updatedAt: s.updated_at,
  };
}

// -----------------------------------------------------------------
// Hook
// -----------------------------------------------------------------

export function useBotData() {
  const [bots, setBots] = useState<BotItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  // ------- Fetch bots -------
  const fetchBots = useCallback(async () => {
    setIsLoading(true);
    try {
      const data = await botApi.list();
      setBots(data.map(mapBot));
    } catch {
      // API unavailable — show empty list (no mock data)
      setBots([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchBots();
  }, [fetchBots]);

  // ------- Toggle expand (fetches detail on expand) -------
  const toggleExpand = useCallback(
    async (botId: string) => {
      setExpandedIds((prev) => {
        const next = new Set(prev);
        if (next.has(botId)) {
          next.delete(botId);
        } else {
          next.add(botId);
        }
        return next;
      });

      // If expanding, fetch full bot detail (position + open orders)
      if (!expandedIds.has(botId)) {
        try {
          const detail = await botApi.getDetail(botId);
          setBots((prev) =>
            prev.map((b) => {
              if (b.id !== botId) return b;
              return {
                ...b,
                totalPnl: Number(detail.total_pnl) || b.totalPnl,
                position: detail.position
                  ? {
                      side: detail.position.side,
                      entryPrice: detail.position.entry_price,
                      quantity: detail.position.quantity,
                      leverage: detail.position.leverage,
                      unrealizedPnl: detail.position.unrealized_pnl,
                      marginType: detail.position.margin_type,
                    }
                  : null,
                openOrders: (detail.open_orders ?? []).map((o) => ({
                  orderId: o.order_id,
                  side: o.side,
                  type: o.type,
                  price: o.price,
                  quantity: o.quantity,
                  status: o.status,
                })),
              };
            }),
          );
        } catch {
          // Detail fetch failed — expanded view will show empty data
        }
      }
    },
    [expandedIds],
  );

  // ------- Create bot -------
  const createBot = useCallback(
    async (params: {
      botName: string;
      strategyId: string;
      symbol: string;
    }) => {
      const created = await botApi.create({
        bot_name: params.botName,
        strategy_id: params.strategyId,
        symbol: params.symbol,
      });
      setBots((prev) => [mapBot(created), ...prev]);
      return { success: true };
    },
    [],
  );

  // ------- Start bot -------
  const startBot = useCallback(async (botId: string) => {
    await botApi.start(botId);
    setBots((prev) =>
      prev.map((b) =>
        b.id === botId ? { ...b, status: "Running" as const } : b,
      ),
    );
  }, []);

  // ------- Stop bot -------
  const stopBot = useCallback(
    async (botId: string, closePosition: boolean) => {
      await botApi.stop(botId, closePosition);
      setBots((prev) =>
        prev.map((b) =>
          b.id === botId
            ? {
                ...b,
                status: "Stopped" as const,
                position: closePosition ? null : b.position,
              }
            : b,
        ),
      );
    },
    [],
  );

  // ------- Delete bot -------
  const deleteBot = useCallback(async (botId: string) => {
    await botApi.delete(botId);
    setBots((prev) => prev.filter((b) => b.id !== botId));
    setExpandedIds((prev) => {
      const next = new Set(prev);
      next.delete(botId);
      return next;
    });
  }, []);

  return {
    bots,
    isLoading,
    expandedIds,
    toggleExpand,
    createBot,
    startBot,
    stopBot,
    deleteBot,
    refetch: fetchBots,
  };
}
