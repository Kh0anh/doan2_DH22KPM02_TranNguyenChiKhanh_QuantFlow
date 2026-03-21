// ===================================================================
// QuantFlow — useMarketData Hook
// Task 3.3.1 + 3.4.5 — Market Watch Component + Real-time WS
// ===================================================================
//
// Responsibilities:
//   - Fetch initial symbols from GET /market/symbols (with mock fallback)
//   - Maintain real-time price state per symbol
//   - Track price flash direction (up/down/null) for animation
//   - Subscribe to WS `market_ticker` channel for real-time prices
//   - Fallback to mock simulation when WS is not connected
//
// WS Channel: market_ticker (websocket.md §3.1)
// SRS: FR-MON-01
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import type { SymbolInfo } from "@/types";
import { marketApi } from "@/lib/api-client";
import { useWebSocket } from "@/lib/hooks/use-websocket";
import { parseMarketTicker } from "@/lib/websocket/ws-channels";

/** Direction of last price change — drives the flash animation */
export type FlashDirection = "up" | "down" | null;

/** Symbol data enriched with flash state for UI rendering */
export interface MarketSymbol extends SymbolInfo {
  flashDirection: FlashDirection;
}

// -----------------------------------------------------------------
// Mock data — used when backend API is unavailable
// -----------------------------------------------------------------

const MOCK_SYMBOLS: SymbolInfo[] = [
  {
    symbol: "BTCUSDT",
    baseAsset: "BTC",
    quoteAsset: "USDT",
    lastPrice: 67432.5,
    priceChangePercent: 2.31,
    volume24h: 28650432000.75,
    hasRunningBot: false,
  },
  {
    symbol: "ETHUSDT",
    baseAsset: "ETH",
    quoteAsset: "USDT",
    lastPrice: 3421.2,
    priceChangePercent: -1.15,
    volume24h: 12340567890.0,
    hasRunningBot: false,
  },
  {
    symbol: "SOLUSDT",
    baseAsset: "SOL",
    quoteAsset: "USDT",
    lastPrice: 142.85,
    priceChangePercent: 4.52,
    volume24h: 4560789012.3,
    hasRunningBot: false,
  },
  {
    symbol: "BNBUSDT",
    baseAsset: "BNB",
    quoteAsset: "USDT",
    lastPrice: 612.3,
    priceChangePercent: 0.87,
    volume24h: 1890234567.8,
    hasRunningBot: false,
  },
];

// -----------------------------------------------------------------
// Flash duration constant
// -----------------------------------------------------------------

/** Duration in ms for the price flash animation (frontend_flows §3.2.2) */
const FLASH_DURATION_MS = 300;

/**
 * Interval in ms for simulated price updates (fallback when WS disconnected).
 */
const MOCK_TICK_INTERVAL_MS = 2000;

// -----------------------------------------------------------------
// Hook implementation
// -----------------------------------------------------------------

export function useMarketData() {
  const [symbols, setSymbols] = useState<MarketSymbol[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const { connectionState, subscribe, unsubscribe, on, off } = useWebSocket();
  const [error, setError] = useState<string | null>(null);

  // Ref to hold flash timers so we can clear them
  const flashTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(
    new Map(),
  );

  // ------- Fetch initial data -------
  useEffect(() => {
    let cancelled = false;

    async function fetchSymbols() {
      try {
        const data = await marketApi.getSymbols();
        if (cancelled) return;

        const mapped: MarketSymbol[] = data.map((s) => ({
          symbol: s.symbol,
          baseAsset: s.base_asset ?? s.symbol.replace("USDT", ""),
          quoteAsset: s.quote_asset ?? "USDT",
          lastPrice: s.last_price ?? 0,
          priceChangePercent: s.price_change_percent ?? 0,
          volume24h: s.volume_24h ?? 0,
          hasRunningBot: s.has_running_bot ?? false,
          flashDirection: null,
        }));

        setSymbols(mapped);
        setError(null);
      } catch {
        // API not available — fall back to mock data for development
        if (cancelled) return;
        setSymbols(
          MOCK_SYMBOLS.map((s) => ({ ...s, flashDirection: null })),
        );
        setError(null); // Don't show error, just use mock data silently
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }

    fetchSymbols();
    return () => {
      cancelled = true;
    };
  }, []);

  // ------- Update price (called by WS handler or mock simulation) -------
  const updatePrice = useCallback(
    (symbol: string, newPrice: number, changePercent: number) => {
      setSymbols((prev) =>
        prev.map((s) => {
          if (s.symbol !== symbol) return s;

          const direction: FlashDirection =
            newPrice > s.lastPrice ? "up" : newPrice < s.lastPrice ? "down" : null;

          // Clear any existing flash timer for this symbol
          const existingTimer = flashTimers.current.get(symbol);
          if (existingTimer) clearTimeout(existingTimer);

          // Schedule flash reset after FLASH_DURATION_MS
          if (direction) {
            const timer = setTimeout(() => {
              setSymbols((current) =>
                current.map((cs) =>
                  cs.symbol === symbol
                    ? { ...cs, flashDirection: null }
                    : cs,
                ),
              );
              flashTimers.current.delete(symbol);
            }, FLASH_DURATION_MS);
            flashTimers.current.set(symbol, timer);
          }

          return {
            ...s,
            lastPrice: newPrice,
            priceChangePercent: changePercent,
            flashDirection: direction,
          };
        }),
      );
    },
    [],
  );

  // ------- WS real-time ticker (Task 3.4.5) -------
  useEffect(() => {
    if (connectionState !== "connected" || symbols.length === 0) return;

    // Subscribe to market_ticker for each loaded symbol
    const symbolNames = symbols.map((s) => s.symbol);
    for (const sym of symbolNames) {
      subscribe("market_ticker", { symbol: sym });
    }

    // Handle incoming ticker events
    const handleTicker = (data: unknown) => {
      const parsed = parseMarketTicker(data);
      if (!parsed) return;
      updatePrice(parsed.symbol, parsed.lastPrice, parsed.priceChangePercent);
    };
    on("market_ticker", handleTicker);

    return () => {
      off("market_ticker", handleTicker);
      for (const sym of symbolNames) {
        unsubscribe("market_ticker", { symbol: sym });
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connectionState, symbols.length]);

  // ------- Mock price simulation (fallback when WS disconnected) -------
  useEffect(() => {
    // Skip mock when WS is connected — real data flows via WS
    if (connectionState === "connected") return;
    if (symbols.length === 0) return;

    const interval = setInterval(() => {
      // Pick a random symbol to update
      const idx = Math.floor(Math.random() * symbols.length);
      const sym = symbols[idx];
      if (!sym) return;

      // Generate a small random price change (-0.3% to +0.3%)
      const changePct = (Math.random() - 0.5) * 0.6;
      const newPrice = sym.lastPrice * (1 + changePct / 100);
      const roundedPrice = Math.round(newPrice * 100) / 100;
      const newChangePercent =
        Math.round((sym.priceChangePercent + changePct * 0.1) * 100) / 100;

      updatePrice(sym.symbol, roundedPrice, newChangePercent);
    }, MOCK_TICK_INTERVAL_MS);

    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connectionState, symbols.length, updatePrice]);

  // ------- Cleanup flash timers on unmount -------
  useEffect(() => {
    return () => {
      flashTimers.current.forEach((timer) => clearTimeout(timer));
      flashTimers.current.clear();
    };
  }, []);

  return {
    symbols,
    isLoading,
    error,
    updatePrice,
  };
}
