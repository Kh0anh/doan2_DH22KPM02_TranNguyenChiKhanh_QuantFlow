// ===================================================================
// QuantFlow — Candle Chart Component
// Task 3.3.2 — Candle Chart (Lightweight Charts + Trade Markers + Timeframe)
// ===================================================================
//
// Layout (frontend_flows.md §3.2.3):
//   ┌────────────────────────────────────────────────────────────┐
//   │  BTCUSDT  67,432.50 (+2.31%)  │  1m  5m  15m  1h  4h  1D  │
//   ├────────────────────────────────────────────────────────────┤
//   │                                                            │
//   │  Lightweight Charts Candlestick + Trade Markers            │
//   │                                                            │
//   └────────────────────────────────────────────────────────────┘
//
// Features:
//   - Candlestick chart via Lightweight Charts v5
//   - Chart header: symbol + last price + 24h change %
//   - Timeframe radio tabs (1m, 5m, 15m, 1h, 4h, 1D)
//   - Trade markers overlay (▲ Long / ▼ Short)
//   - Real-time last candle updates (mock until Task 3.4.4)
//   - Dark theme matching QuantFlow palette
//   - Error overlay with retry
//
// SRS refs: FR-MONITOR-02, UC-10
// ===================================================================

"use client";

import { useEffect, useRef, useCallback } from "react";
import {
  createChart,
  createSeriesMarkers,
  CandlestickSeries,
  ColorType,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type CandlestickSeriesOptions,
  type DeepPartial,
} from "lightweight-charts";
import {
  TrendingUp,
  TrendingDown,
  AlertTriangle,
  RefreshCw,
  Loader2,
} from "lucide-react";
import { useUIStore } from "@/store/ui-store";
import { useCandleData } from "@/lib/hooks/use-candle-data";
import type { Timeframe } from "@/types";

// -----------------------------------------------------------------
// Constants
// -----------------------------------------------------------------

const TIMEFRAMES: { value: Timeframe; label: string }[] = [
  { value: "1m", label: "1m" },
  { value: "5m", label: "5m" },
  { value: "15m", label: "15m" },
  { value: "1h", label: "1h" },
  { value: "4h", label: "4h" },
  { value: "1d", label: "1D" },
];

/** QuantFlow dark theme chart colors */
const CHART_COLORS = {
  background: "#0D1117",
  textColor: "#8B949E",
  gridColor: "#21262D",
  borderColor: "#30363D",
  crosshairColor: "#58A6FF",
  upColor: "#26A69A",
  downColor: "#EF5350",
  wickUpColor: "#26A69A",
  wickDownColor: "#EF5350",
};

// -----------------------------------------------------------------
// Price formatter
// -----------------------------------------------------------------

function formatPrice(price: number): string {
  const val = Number(price) || 0;
  if (val >= 1000) {
    return val.toLocaleString("en-US", {
      minimumFractionDigits: 1,
      maximumFractionDigits: 1,
    });
  }
  if (val >= 1) {
    return val.toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  }
  return val.toLocaleString("en-US", {
    minimumFractionDigits: 4,
    maximumFractionDigits: 4,
  });
}

function formatChangePercent(pct: number): string {
  const val = Number(pct) || 0;
  const sign = val >= 0 ? "+" : "";
  return `${sign}${val.toFixed(2)}%`;
}

// -----------------------------------------------------------------
// CandleChart Component
// -----------------------------------------------------------------

export function CandleChart() {
  const activeSymbol = useUIStore((s) => s.activeSymbol);
  const {
    candles,
    markers,
    timeframe,
    setTimeframe,
    isLoading,
    error,
    lastPrice,
    priceChangePercent,
  } = useCandleData(activeSymbol);

  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const seriesApiRef = useRef<any>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const markersRef = useRef<any>(null);

  const isPositive = (Number(priceChangePercent) || 0) >= 0;

  // ------- Create / destroy chart -------
  const initChart = useCallback(() => {
    if (!chartContainerRef.current) return;

    // Destroy existing chart
    if (chartRef.current) {
      chartRef.current.remove();
      chartRef.current = null;
      seriesRef.current = null;
    }

    const container = chartContainerRef.current;

    const chart = createChart(container, {
      width: container.clientWidth,
      height: container.clientHeight,
      layout: {
        background: { type: ColorType.Solid, color: CHART_COLORS.background },
        textColor: CHART_COLORS.textColor,
        fontFamily: "'Inter', system-ui, sans-serif",
        fontSize: 11,
      },
      grid: {
        vertLines: { color: CHART_COLORS.gridColor },
        horzLines: { color: CHART_COLORS.gridColor },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
        vertLine: { color: CHART_COLORS.crosshairColor, width: 1, style: 2 },
        horzLine: { color: CHART_COLORS.crosshairColor, width: 1, style: 2 },
      },
      rightPriceScale: {
        borderColor: CHART_COLORS.borderColor,
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
      timeScale: {
        borderColor: CHART_COLORS.borderColor,
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: { vertTouchDrag: false },
    });

    const candlestickOptions: DeepPartial<CandlestickSeriesOptions> = {
      upColor: CHART_COLORS.upColor,
      downColor: CHART_COLORS.downColor,
      wickUpColor: CHART_COLORS.wickUpColor,
      wickDownColor: CHART_COLORS.wickDownColor,
      borderVisible: false,
    };

    const series = chart.addSeries(CandlestickSeries, candlestickOptions);

    chartRef.current = chart;
    seriesRef.current = series;
    seriesApiRef.current = series;

    // Handle resize
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        chart.applyOptions({ width, height });
      }
    });
    resizeObserver.observe(container);

    return () => {
      resizeObserver.disconnect();
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, []);

  // ------- Initialize chart on mount -------
  useEffect(() => {
    const cleanup = initChart();
    return () => {
      cleanup?.();
    };
  }, [initChart]);

  // ------- Update chart data when candles change -------
  useEffect(() => {
    if (!seriesApiRef.current || candles.length === 0) return;

    // Cast candles to LC v5 format (time as UTCTimestamp)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const lcData = candles.map((c) => ({
      time: c.time as any,
      open: c.open,
      high: c.high,
      low: c.low,
      close: c.close,
    }));

    // Set candle data
    seriesApiRef.current.setData(lcData);

    // Set trade markers (LC v5: use createSeriesMarkers instead of removed setMarkers)
    // Remove previous markers primitive before creating a new one
    if (markersRef.current) {
      markersRef.current.detach();
      markersRef.current = null;
    }
    if (markers.length > 0 && seriesRef.current) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const lcMarkers = markers.map((m) => ({
        time: m.time as any,
        position: m.position,
        color: m.color,
        shape: m.shape,
        text: m.text,
      }));
      markersRef.current = createSeriesMarkers(seriesRef.current, lcMarkers);
    }

    // Fit content to view
    if (chartRef.current) {
      chartRef.current.timeScale().fitContent();
    }
  }, [candles, markers]);

  // ------- Retry handler -------
  const handleRetry = useCallback(() => {
    // Force re-fetch by toggling timeframe back and forth
    setTimeframe(timeframe);
  }, [timeframe, setTimeframe]);

  return (
    <div
      id="candle-chart-panel"
      className="h-full w-full flex flex-col overflow-hidden"
    >
      {/* Chart Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border bg-card">
        {/* Symbol + Price */}
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-foreground">
            {activeSymbol}
          </span>
          {!isLoading && (
            <>
              <span
                className={`font-mono text-sm font-semibold ${
                  isPositive ? "text-success" : "text-danger"
                }`}
              >
                {formatPrice(lastPrice)}
              </span>
              <span
                className={`font-mono text-xs flex items-center gap-0.5 ${
                  isPositive ? "text-success/80" : "text-danger/80"
                }`}
              >
                {isPositive ? (
                  <TrendingUp className="h-3 w-3" />
                ) : (
                  <TrendingDown className="h-3 w-3" />
                )}
                {formatChangePercent(priceChangePercent)}
              </span>
            </>
          )}
        </div>

        {/* Timeframe Tabs */}
        <div className="flex items-center gap-0.5 bg-secondary rounded-md p-0.5">
          {TIMEFRAMES.map((tf) => (
            <button
              key={tf.value}
              id={`timeframe-${tf.value}`}
              type="button"
              onClick={() => setTimeframe(tf.value)}
              className={`
                px-2 py-1 text-xs font-medium rounded transition-colors
                ${
                  timeframe === tf.value
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-accent"
                }
              `}
            >
              {tf.label}
            </button>
          ))}
        </div>
      </div>

      {/* Chart Container */}
      <div className="flex-1 relative">
        <div
          ref={chartContainerRef}
          className="absolute inset-0"
        />

        {/* Loading Overlay */}
        {isLoading && (
          <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
            <div className="flex flex-col items-center gap-2">
              <Loader2 className="h-6 w-6 text-primary animate-spin" />
              <span className="text-xs text-muted-foreground">
                Đang tải dữ liệu...
              </span>
            </div>
          </div>
        )}

        {/* Error Overlay */}
        {error && !isLoading && (
          <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
            <div className="flex flex-col items-center gap-3 text-center p-4">
              <AlertTriangle className="h-8 w-8 text-warning" />
              <p className="text-sm text-foreground">
                Không thể tải dữ liệu biểu đồ
              </p>
              <p className="text-xs text-muted-foreground max-w-[240px]">
                {error}
              </p>
              <button
                type="button"
                onClick={handleRetry}
                className="
                  flex items-center gap-1.5 px-3 py-1.5
                  text-xs font-medium rounded-md
                  bg-primary text-primary-foreground
                  hover:bg-primary/90 transition-colors
                "
              >
                <RefreshCw className="h-3 w-3" />
                Thử lại
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
