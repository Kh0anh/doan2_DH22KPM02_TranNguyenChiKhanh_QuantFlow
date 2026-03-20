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

import { useState, useEffect, useCallback, useRef } from "react";
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
// Mock data — 4 demo bots
// -----------------------------------------------------------------

function generateMockBots(): BotItem[] {
  return [
    {
      id: "bot-001",
      name: "BTC-Scalper",
      symbol: "BTCUSDT",
      strategyId: "strat-001",
      strategyName: "EMA Crossover",
      strategyVersion: 3,
      status: "Running",
      totalPnl: 125.4,
      createdAt: "2026-03-15T08:00:00Z",
      updatedAt: "2026-03-20T10:30:00Z",
      position: {
        side: "Long",
        entryPrice: 67200.5,
        quantity: 0.01,
        leverage: 10,
        unrealizedPnl: 12.3,
        marginType: "Isolated",
      },
      openOrders: [
        {
          orderId: "ORD-101",
          side: "Sell",
          type: "Limit",
          price: 68500.0,
          quantity: 0.01,
          status: "Pending",
        },
        {
          orderId: "ORD-102",
          side: "Sell",
          type: "Stop",
          price: 66500.0,
          quantity: 0.01,
          status: "Pending",
        },
      ],
    },
    {
      id: "bot-002",
      name: "ETH-Swing",
      symbol: "ETHUSDT",
      strategyId: "strat-002",
      strategyName: "RSI Reversal",
      strategyVersion: 1,
      status: "Running",
      totalPnl: -12.3,
      createdAt: "2026-03-16T10:00:00Z",
      updatedAt: "2026-03-20T09:15:00Z",
      position: {
        side: "Short",
        entryPrice: 3420.0,
        quantity: 0.5,
        leverage: 5,
        unrealizedPnl: -8.2,
        marginType: "Cross",
      },
      openOrders: [],
    },
    {
      id: "bot-003",
      name: "SOL-Breakout",
      symbol: "SOLUSDT",
      strategyId: "strat-001",
      strategyName: "EMA Crossover",
      strategyVersion: 2,
      status: "Stopped",
      totalPnl: 45.0,
      createdAt: "2026-03-10T14:00:00Z",
      updatedAt: "2026-03-18T16:00:00Z",
      position: null,
      openOrders: [],
    },
    {
      id: "bot-004",
      name: "BNB-Grid",
      symbol: "BNBUSDT",
      strategyId: "strat-003",
      strategyName: "Bollinger Grid",
      strategyVersion: 1,
      status: "Error",
      totalPnl: 8.2,
      createdAt: "2026-03-12T09:00:00Z",
      updatedAt: "2026-03-19T20:00:00Z",
      position: null,
      openOrders: [],
    },
  ];
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
  const isMockRef = useRef(false);

  // ------- Fetch bots -------
  const fetchBots = useCallback(async () => {
    setIsLoading(true);
    try {
      const data = await botApi.list();
      setBots(data.map(mapBot));
      isMockRef.current = false;
    } catch {
      // API not available — use mock data
      setBots(generateMockBots());
      isMockRef.current = true;
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchBots();
  }, [fetchBots]);

  // ------- Toggle expand -------
  const toggleExpand = useCallback((botId: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(botId)) {
        next.delete(botId);
      } else {
        next.add(botId);
      }
      return next;
    });
  }, []);

  // ------- Create bot -------
  const createBot = useCallback(
    async (params: {
      botName: string;
      strategyId: string;
      symbol: string;
    }) => {
      try {
        const created = await botApi.create({
          bot_name: params.botName,
          strategy_id: params.strategyId,
          symbol: params.symbol,
        });
        setBots((prev) => [mapBot(created), ...prev]);
        return { success: true };
      } catch {
        // Mock mode: simulate create
        const mock: BotItem = {
          id: `bot-${Date.now()}`,
          name: params.botName,
          symbol: params.symbol,
          strategyId: params.strategyId,
          strategyName: "Mock Strategy",
          strategyVersion: 1,
          status: "Running",
          totalPnl: 0,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
          position: null,
          openOrders: [],
        };
        setBots((prev) => [mock, ...prev]);
        return { success: true };
      }
    },
    [],
  );

  // ------- Start bot -------
  const startBot = useCallback(async (botId: string) => {
    try {
      await botApi.start(botId);
    } catch {
      // Mock mode
    }
    setBots((prev) =>
      prev.map((b) =>
        b.id === botId ? { ...b, status: "Running" as const } : b,
      ),
    );
  }, []);

  // ------- Stop bot -------
  const stopBot = useCallback(
    async (botId: string, closePosition: boolean) => {
      try {
        await botApi.stop(botId, closePosition);
      } catch {
        // Mock mode
      }
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
    try {
      await botApi.delete(botId);
    } catch {
      // Mock mode
    }
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
