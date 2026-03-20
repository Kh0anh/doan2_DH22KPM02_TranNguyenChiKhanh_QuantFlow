// ===================================================================
// QuantFlow — useTradeHistory Hook
// Task 3.3.6 — Trade History Table
// ===================================================================
//
// Responsibilities:
//   - Fetch trades from GET /trades (cursor pagination, multi-filter)
//   - Infinite scroll — loadMore appends to existing list
//   - Client-side CSV fallback when API export unavailable
//   - Mock data fallback when API unavailable
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  tradeApi,
  type TradeRecordResponse,
  type TradeFilterParams,
} from "@/lib/api-client";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

export interface TradeItem {
  id: string;
  botId: string;
  botName: string;
  symbol: string;
  side: "Long" | "Short";
  quantity: number;
  fillPrice: number;
  fee: number;
  realizedPnl: number;
  status: "Filled" | "Canceled";
  executedAt: string;
}

export interface TradeFilters {
  botId: string; // "" = all
  symbol: string; // "" = all
  side: string; // "" = all
  status: string; // "" = all
}

// -----------------------------------------------------------------
// Mock data
// -----------------------------------------------------------------

const MOCK_BOTS = ["BTC-Scalper", "ETH-Swing", "SOL-Breakout", "BNB-Grid"];
const MOCK_SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];
const MOCK_SIDES: ("Long" | "Short")[] = ["Long", "Short"];
const MOCK_PRICES: Record<string, number> = {
  BTCUSDT: 67400, ETHUSDT: 3420, SOLUSDT: 142, BNBUSDT: 580,
};

function generateMockTrades(count: number): TradeItem[] {
  const trades: TradeItem[] = [];
  const now = Date.now();

  for (let i = 0; i < count; i++) {
    const botIdx = i % MOCK_BOTS.length;
    const symbol = MOCK_SYMBOLS[botIdx];
    const side = MOCK_SIDES[i % 2];
    const basePrice = MOCK_PRICES[symbol] ?? 100;
    const fillPrice = basePrice + (Math.random() - 0.5) * basePrice * 0.02;
    const qty = symbol === "BTCUSDT" ? 0.01 : symbol === "ETHUSDT" ? 0.5 : 1;
    const pnl = (Math.random() - 0.4) * 20;

    trades.push({
      id: `trade-${i + 1}`,
      botId: `bot-00${botIdx + 1}`,
      botName: MOCK_BOTS[botIdx],
      symbol,
      side,
      quantity: qty,
      fillPrice: Math.round(fillPrice * 100) / 100,
      fee: Math.round(fillPrice * qty * 0.0004 * 100) / 100,
      realizedPnl: Math.round(pnl * 100) / 100,
      status: Math.random() > 0.1 ? "Filled" : "Canceled",
      executedAt: new Date(now - i * 3600000 - Math.random() * 3600000).toISOString(),
    });
  }

  return trades;
}

// -----------------------------------------------------------------
// Map API response
// -----------------------------------------------------------------

function mapTrade(t: TradeRecordResponse): TradeItem {
  return {
    id: t.id,
    botId: t.bot_id,
    botName: t.bot_name,
    symbol: t.symbol,
    side: t.side,
    quantity: t.quantity,
    fillPrice: t.fill_price,
    fee: t.fee,
    realizedPnl: t.realized_pnl,
    status: t.status,
    executedAt: t.executed_at,
  };
}

// -----------------------------------------------------------------
// Hook
// -----------------------------------------------------------------

export function useTradeHistory() {
  const [trades, setTrades] = useState<TradeItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [filters, setFilters] = useState<TradeFilters>({
    botId: "",
    symbol: "",
    side: "",
    status: "",
  });
  const cursorRef = useRef<string | null>(null);
  const isMockRef = useRef(false);

  // ------- Build API params from filters -------
  const buildParams = useCallback(
    (cursor?: string): TradeFilterParams => {
      const p: TradeFilterParams = { limit: 50 };
      if (filters.botId) p.bot_id = filters.botId;
      if (filters.symbol) p.symbol = filters.symbol;
      if (filters.side) p.side = filters.side;
      if (filters.status) p.status = filters.status;
      if (cursor) p.cursor = cursor;
      return p;
    },
    [filters],
  );

  // ------- Fetch initial trades -------
  const fetchTrades = useCallback(async () => {
    setIsLoading(true);
    cursorRef.current = null;
    try {
      const res = await tradeApi.list(buildParams());
      setTrades(res.data.map(mapTrade));
      setHasMore(res.pagination.has_more);
      cursorRef.current = res.pagination.next_cursor;
      isMockRef.current = false;
    } catch {
      // Mock fallback
      const allMock = generateMockTrades(30);
      // Apply client-side filter
      const filtered = allMock.filter((t) => {
        if (filters.botId && t.botId !== filters.botId) return false;
        if (filters.symbol && t.symbol !== filters.symbol) return false;
        if (filters.side && t.side !== filters.side) return false;
        if (filters.status && t.status !== filters.status) return false;
        return true;
      });
      setTrades(filtered);
      setHasMore(false);
      isMockRef.current = true;
    } finally {
      setIsLoading(false);
    }
  }, [buildParams, filters]);

  useEffect(() => {
    fetchTrades();
  }, [fetchTrades]);

  // ------- Load more (infinite scroll) -------
  const loadMore = useCallback(async () => {
    if (!hasMore || !cursorRef.current || isLoadingMore) return;
    setIsLoadingMore(true);
    try {
      const res = await tradeApi.list(buildParams(cursorRef.current));
      setTrades((prev) => [...prev, ...res.data.map(mapTrade)]);
      setHasMore(res.pagination.has_more);
      cursorRef.current = res.pagination.next_cursor;
    } catch {
      // Silently fail in mock mode
    } finally {
      setIsLoadingMore(false);
    }
  }, [hasMore, isLoadingMore, buildParams]);

  // ------- Export CSV -------
  const exportCSV = useCallback(async () => {
    try {
      const blob = await tradeApi.exportCSV({
        bot_id: filters.botId || undefined,
        symbol: filters.symbol || undefined,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `trade-history-${new Date().toISOString().slice(0, 10)}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // Client-side CSV fallback
      const headers = [
        "ID",
        "Bot",
        "Symbol",
        "Side",
        "Quantity",
        "Fill Price",
        "Fee",
        "Realized PnL",
        "Status",
        "Executed At",
      ];
      const rows = trades.map((t) =>
        [
          t.id,
          t.botName,
          t.symbol,
          t.side,
          t.quantity,
          t.fillPrice,
          t.fee,
          t.realizedPnl,
          t.status,
          t.executedAt,
        ].join(","),
      );
      const csv = [headers.join(","), ...rows].join("\n");
      const blob = new Blob([csv], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `trade-history-${new Date().toISOString().slice(0, 10)}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    }
  }, [filters, trades]);

  // ------- Update filter -------
  const updateFilter = useCallback(
    (key: keyof TradeFilters, value: string) => {
      setFilters((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  return {
    trades,
    isLoading,
    isLoadingMore,
    hasMore,
    filters,
    updateFilter,
    loadMore,
    exportCSV,
  };
}
