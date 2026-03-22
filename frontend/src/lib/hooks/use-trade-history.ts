// ===================================================================
// QuantFlow — useTradeHistory Hook
// Task 3.3.6 — Trade History Table
// ===================================================================
//
// Responsibilities:
//   - Fetch trades from GET /trades (cursor pagination, multi-filter)
//   - Infinite scroll — loadMore appends to existing list
//   - Client-side CSV fallback when API export unavailable
//   - Fetch bot list from GET /bots for dynamic filter dropdown
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  tradeApi,
  botApi,
  type TradeRecordResponse,
  type TradeFilterParams,
  type BotSummaryResponse,
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

/** Bot option for the filter dropdown. */
export interface BotOption {
  id: string;
  name: string;
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
  const [bots, setBots] = useState<BotOption[]>([]);
  const [filters, setFilters] = useState<TradeFilters>({
    botId: "",
    symbol: "",
    side: "",
    status: "",
  });
  const cursorRef = useRef<string | null>(null);

  // ------- Fetch bot list for filter dropdown -------
  useEffect(() => {
    botApi
      .list()
      .then((list: BotSummaryResponse[]) => {
        setBots(list.map((b) => ({ id: b.id, name: b.bot_name })));
      })
      .catch(() => {
        // If bots API fails, leave dropdown empty
        setBots([]);
      });
  }, []);

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
    } catch {
      // API unavailable — show empty state instead of mock data
      setTrades([]);
      setHasMore(false);
    } finally {
      setIsLoading(false);
    }
  }, [buildParams]);

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
      // Silently fail — don't break infinite scroll
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
      // Client-side CSV fallback using currently loaded trades
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
    bots,
    updateFilter,
    loadMore,
    exportCSV,
  };
}
