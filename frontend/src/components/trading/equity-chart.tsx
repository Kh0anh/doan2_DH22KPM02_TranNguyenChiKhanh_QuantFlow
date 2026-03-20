// ===================================================================
// QuantFlow — Equity Chart (Lightweight Charts Area)
// Task 3.4.3 — Equity Chart growth visualization
// ===================================================================
//
// Renders an area chart showing equity curve over time using
// Lightweight Charts library (already used by CandleChart).
//
// frontend_flows.md §3.2.5:
//   Equity Curve: Area chart, biến động số dư theo thời gian
//
// SRS: UC-06
// ===================================================================

"use client";

import { useEffect, useRef, useMemo } from "react";
import { createChart, type IChartApi, ColorType, AreaSeries } from "lightweight-charts";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

interface EquityChartProps {
  data: { time: string; equity: number }[];
  height?: number;
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function EquityChart({ data, height = 120 }: EquityChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);

  // Convert ISO timestamps to YYYY-MM-DD for Lightweight Charts
  const chartData = useMemo(() => {
    return data
      .map((d) => ({
        time: d.time.slice(0, 10) as string,
        value: d.equity,
      }))
      .sort((a, b) => a.time.localeCompare(b.time));
  }, [data]);

  // Determine if positive performance
  const isPositive = useMemo(() => {
    if (chartData.length < 2) return true;
    return chartData[chartData.length - 1].value >= chartData[0].value;
  }, [chartData]);

  useEffect(() => {
    if (!containerRef.current || chartData.length < 2) return;

    const lineColor = isPositive ? "#22C55E" : "#EF4444";
    const topColor = isPositive
      ? "rgba(34, 197, 94, 0.25)"
      : "rgba(239, 68, 68, 0.25)";
    const bottomColor = isPositive
      ? "rgba(34, 197, 94, 0.02)"
      : "rgba(239, 68, 68, 0.02)";

    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height,
      layout: {
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: "#888",
        fontSize: 10,
      },
      grid: {
        vertLines: { visible: false },
        horzLines: { color: "rgba(255,255,255,0.04)" },
      },
      rightPriceScale: {
        borderVisible: false,
        scaleMargins: { top: 0.1, bottom: 0.05 },
      },
      timeScale: {
        borderVisible: false,
        timeVisible: false,
        fixLeftEdge: true,
        fixRightEdge: true,
      },
      crosshair: {
        horzLine: { visible: false, labelVisible: false },
        vertLine: { labelVisible: false },
      },
      handleScroll: false,
      handleScale: false,
    });

    chartRef.current = chart;

    const areaSeries = chart.addSeries(AreaSeries, {
      lineColor,
      topColor,
      bottomColor,
      lineWidth: 2,
      crosshairMarkerVisible: true,
      crosshairMarkerRadius: 3,
      priceFormat: { type: "price", precision: 2, minMove: 0.01 },
    });

    areaSeries.setData(chartData);
    chart.timeScale().fitContent();

    // Resize observer
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width } = entry.contentRect;
        chart.applyOptions({ width });
      }
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.remove();
      chartRef.current = null;
    };
  }, [chartData, height, isPositive]);

  if (chartData.length < 2) {
    return (
      <div
        className="flex items-center justify-center text-xs text-muted-foreground"
        style={{ height }}
      >
        Không đủ dữ liệu để vẽ biểu đồ
      </div>
    );
  }

  return <div ref={containerRef} style={{ height }} />;
}
