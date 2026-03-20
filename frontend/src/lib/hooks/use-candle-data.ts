// ===================================================================
// QuantFlow — useCandleData Hook
// Task 3.3.2 — Candle Chart Component
// ===================================================================
//
// Responsibilities:
//   - Fetch historical candle data from GET /market/candles (with mock fallback)
//   - Manage timeframe state (default: 15m)
//   - Provide trade markers for chart overlay
//   - Simulate real-time candle updates until WS Manager (Task 3.4.4) is ready
//
// Integration point for Task 3.4.4:
//   Replace the simulated interval with actual WS subscription
//   to `market_ticker` channel → `market_candle` event.
// ===================================================================

"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import type { Timeframe, CandleData, TradeMarker } from "@/types";
import { marketApi } from "@/lib/api-client";

// -----------------------------------------------------------------
// Constants
// -----------------------------------------------------------------

const DEFAULT_TIMEFRAME: Timeframe = "15m";
const CANDLE_LIMIT = 500;
const MOCK_UPDATE_INTERVAL_MS = 2000;

// -----------------------------------------------------------------
// Mock candle generator — realistic OHLCV data for demo
// -----------------------------------------------------------------

function generateMockCandles(
  symbol: string,
  timeframe: Timeframe,
  count: number,
): CandleData[] {
  const basePrice = symbol.includes("BTC")
    ? 67000
    : symbol.includes("ETH")
      ? 3400
      : symbol.includes("SOL")
        ? 142
        : symbol.includes("BNB")
          ? 610
          : 100;

  const intervalMs: Record<string, number> = {
    "1m": 60_000,
    "5m": 300_000,
    "15m": 900_000,
    "30m": 1_800_000,
    "1h": 3_600_000,
    "4h": 14_400_000,
    "1d": 86_400_000,
  };

  const interval = intervalMs[timeframe] ?? 900_000;
  const now = Math.floor(Date.now() / interval) * interval;
  const startTime = now - (count - 1) * interval;

  const candles: CandleData[] = [];
  let price = basePrice;

  for (let i = 0; i < count; i++) {
    const changePercent = (Math.random() - 0.48) * 2;
    const open = price;
    const close = open * (1 + changePercent / 100);
    const high = Math.max(open, close) * (1 + Math.random() * 0.3 / 100);
    const low = Math.min(open, close) * (1 - Math.random() * 0.3 / 100);
    const volume = basePrice * (50 + Math.random() * 200);

    candles.push({
      time: (startTime + i * interval) / 1000,
      open: Math.round(open * 100) / 100,
      high: Math.round(high * 100) / 100,
      low: Math.round(low * 100) / 100,
      close: Math.round(close * 100) / 100,
      volume: Math.round(volume * 100) / 100,
    });

    price = close;
  }

  return candles;
}

function generateMockMarkers(candles: CandleData[]): TradeMarker[] {
  if (candles.length < 20) return [];

  const markers: TradeMarker[] = [];
  const markerCount = 3 + Math.floor(Math.random() * 4);

  for (let i = 0; i < markerCount; i++) {
    const idx = 20 + Math.floor(Math.random() * (candles.length - 40));
    const candle = candles[idx];
    if (!candle) continue;

    const isLong = Math.random() > 0.5;
    markers.push({
      time: candle.time,
      position: isLong ? "belowBar" : "aboveBar",
      color: isLong ? "#26A69A" : "#EF5350",
      shape: isLong ? "arrowUp" : "arrowDown",
      text: isLong ? "Long" : "Short",
    });
  }

  // Sort markers by time to satisfy Lightweight Charts requirement
  return markers.sort((a, b) => (a.time as number) - (b.time as number));
}

// -----------------------------------------------------------------
// Hook implementation
// -----------------------------------------------------------------

export function useCandleData(activeSymbol: string) {
  const [timeframe, setTimeframe] = useState<Timeframe>(DEFAULT_TIMEFRAME);
  const [candles, setCandles] = useState<CandleData[]>([]);
  const [markers, setMarkers] = useState<TradeMarker[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastPrice, setLastPrice] = useState<number>(0);
  const [priceChangePercent, setPriceChangePercent] = useState<number>(0);

  const candlesRef = useRef<CandleData[]>([]);

  // ------- Fetch candle data when symbol or timeframe changes -------
  useEffect(() => {
    let cancelled = false;

    async function fetchCandles() {
      setIsLoading(true);
      setError(null);

      try {
        const data = await marketApi.getCandles({
          symbol: activeSymbol,
          timeframe,
          limit: CANDLE_LIMIT,
        });

        if (cancelled) return;

        // Map API response (snake_case) to frontend types (camelCase)
        const mappedCandles: CandleData[] = data.candles.map((c) => ({
          time: new Date(c.open_time).getTime() / 1000,
          open: c.open,
          high: c.high,
          low: c.low,
          close: c.close,
          volume: c.volume,
        }));

        const mappedMarkers: TradeMarker[] = data.markers.map((m) => ({
          time: new Date(m.time).getTime() / 1000,
          position: m.side === "Long" ? "belowBar" as const : "aboveBar" as const,
          color: m.side === "Long" ? "#26A69A" : "#EF5350",
          shape: m.side === "Long" ? "arrowUp" as const : "arrowDown" as const,
          text: `${m.side} (${m.bot_name})`,
        }));

        setCandles(mappedCandles);
        candlesRef.current = mappedCandles;
        setMarkers(mappedMarkers);

        if (mappedCandles.length > 0) {
          const last = mappedCandles[mappedCandles.length - 1];
          setLastPrice(last.close);
          const first = mappedCandles[0];
          const pct = ((last.close - first.open) / first.open) * 100;
          setPriceChangePercent(Math.round(pct * 100) / 100);
        }
      } catch {
        // API not available — fall back to mock data
        if (cancelled) return;

        const mockCandles = generateMockCandles(activeSymbol, timeframe, 200);
        const mockMarkers = generateMockMarkers(mockCandles);

        setCandles(mockCandles);
        candlesRef.current = mockCandles;
        setMarkers(mockMarkers);
        setError(null);

        if (mockCandles.length > 0) {
          const last = mockCandles[mockCandles.length - 1];
          setLastPrice(last.close);
          const first = mockCandles[0];
          const pct = ((last.close - first.open) / first.open) * 100;
          setPriceChangePercent(Math.round(pct * 100) / 100);
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }

    fetchCandles();
    return () => {
      cancelled = true;
    };
  }, [activeSymbol, timeframe]);

  // ------- Mock real-time candle updates (remove when Task 3.4.4) -------
  useEffect(() => {
    if (candlesRef.current.length === 0) return;

    const interval = setInterval(() => {
      const current = candlesRef.current;
      if (current.length === 0) return;

      const lastCandle = { ...current[current.length - 1] };

      // Simulate small price movement
      const changePct = (Math.random() - 0.48) * 0.4;
      const newClose = lastCandle.close * (1 + changePct / 100);
      lastCandle.close = Math.round(newClose * 100) / 100;
      lastCandle.high = Math.max(lastCandle.high, lastCandle.close);
      lastCandle.low = Math.min(lastCandle.low, lastCandle.close);

      const updated = [...current.slice(0, -1), lastCandle];
      candlesRef.current = updated;
      setCandles(updated);
      setLastPrice(lastCandle.close);

      const first = current[0];
      const pct = ((lastCandle.close - first.open) / first.open) * 100;
      setPriceChangePercent(Math.round(pct * 100) / 100);
    }, MOCK_UPDATE_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [candles.length]); // eslint-disable-line react-hooks/exhaustive-deps

  // ------- Change timeframe callback -------
  const changeTimeframe = useCallback((tf: Timeframe) => {
    setTimeframe(tf);
  }, []);

  return {
    candles,
    markers,
    timeframe,
    setTimeframe: changeTimeframe,
    isLoading,
    error,
    lastPrice,
    priceChangePercent,
  };
}
