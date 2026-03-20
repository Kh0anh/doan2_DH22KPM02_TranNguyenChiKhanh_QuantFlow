// ===================================================================
// QuantFlow — Backtest Result Display
// Task 3.4.2 + 3.4.3 — Performance Report + Equity Chart
// ===================================================================
//
// Layout (frontend_flows.md §3.2.5 — State 3):
//   ┌───────────────────────────────────────────────────────────────┐
//   │  📊 Kết quả: EMA Crossover trên BTCUSDT (15m)                │
//   │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
//   │  │+342.50   │ │ 62.5%    │ │  -8.2%   │ │  1.84    │        │
//   │  │Total PnL │ │ Win Rate │ │Max Drawdn│ │Profit Fac│        │
//   │  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
//   │  Equity Curve: [mini area chart]                              │
//   │  [Chạy lại]  [Quay về cấu hình]                              │
//   └───────────────────────────────────────────────────────────────┘
//
// SRS: UC-06
// ===================================================================

"use client";

import {
  TrendingUp,
  TrendingDown,
  Target,
  BarChart3,
  RotateCcw,
  Settings2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import dynamic from "next/dynamic";

// Dynamic import for Lightweight Charts (SSR incompatible)
const EquityChart = dynamic(
  () =>
    import("@/components/trading/equity-chart").then((m) => m.EquityChart),
  { ssr: false, loading: () => <div className="h-[120px] animate-pulse bg-muted/30 rounded" /> },
);

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

export interface BacktestResultData {
  totalPnl: number;
  winRate: number;
  maxDrawdown: number;
  profitFactor: number;
  totalTrades: number;
  equityCurve: { time: string; equity: number }[];
}

interface BacktestResultDisplayProps {
  result: BacktestResultData;
  onRerun: () => void;
  onReset: () => void;
}

// -----------------------------------------------------------------
// Stat Card
// -----------------------------------------------------------------

function StatCard({
  label,
  value,
  icon: Icon,
  colorClass,
}: {
  label: string;
  value: string;
  icon: React.ElementType;
  colorClass: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-border bg-card/80 p-2.5 min-w-[110px]">
      <div className="flex items-center gap-1 mb-1">
        <Icon className={`h-3.5 w-3.5 ${colorClass}`} />
      </div>
      <span className={`text-lg font-bold font-mono ${colorClass}`}>
        {value}
      </span>
      <span className="text-[10px] text-muted-foreground mt-0.5">
        {label}
      </span>
    </div>
  );
}


// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function BacktestResultDisplay({
  result,
  onRerun,
  onReset,
}: BacktestResultDisplayProps) {
  const pnlPositive = result.totalPnl >= 0;

  return (
    <div className="flex flex-col h-full p-3 gap-3 overflow-y-auto">
      {/* 4 Stat Cards */}
      <div className="grid grid-cols-4 gap-2">
        <StatCard
          label="Total PnL"
          value={`${pnlPositive ? "+" : ""}${result.totalPnl.toFixed(2)}`}
          icon={pnlPositive ? TrendingUp : TrendingDown}
          colorClass={pnlPositive ? "text-success" : "text-danger"}
        />
        <StatCard
          label="Win Rate"
          value={`${result.winRate.toFixed(1)}%`}
          icon={Target}
          colorClass={
            result.winRate >= 50 ? "text-success" : "text-warning"
          }
        />
        <StatCard
          label="Max Drawdown"
          value={`${result.maxDrawdown.toFixed(1)}%`}
          icon={TrendingDown}
          colorClass="text-danger"
        />
        <StatCard
          label="Profit Factor"
          value={result.profitFactor.toFixed(2)}
          icon={BarChart3}
          colorClass={
            result.profitFactor >= 1.5 ? "text-success" : "text-warning"
          }
        />
      </div>

      {/* Equity Curve — Task 3.4.3: Lightweight Charts */}
      <div className="rounded-lg border border-border bg-card/50 p-2 flex-1 min-h-[100px]">
        <div className="flex items-center justify-between mb-1">
          <span className="text-[10px] text-muted-foreground font-medium">
            Equity Curve
          </span>
          <span className="text-[10px] text-muted-foreground">
            {result.totalTrades} lệnh
          </span>
        </div>
        <EquityChart data={result.equityCurve} height={100} />
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          className="gap-1 text-xs"
          onClick={onRerun}
        >
          <RotateCcw className="h-3 w-3" />
          Chạy lại
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="gap-1 text-xs"
          onClick={onReset}
        >
          <Settings2 className="h-3 w-3" />
          Quay về cấu hình
        </Button>
      </div>
    </div>
  );
}
