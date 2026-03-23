/**
 * [3.3.x] Trading Dashboard — Market Watch + Candle Chart + Bot Panel + History.
 *
 * Layout (resizable panels):
 *   ┌──────────┬─────────────────────────────────────────────────┐
 *   │  Market  │  [Chart Header + Timeframe Tabs]                 │
 *   │  Watch   │  [Candlestick Chart + Trade Markers]             │
 *   │ resizable│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─                │
 *   │          │  [Bottom Panel: Bot / Backtest / History]         │
 *   └──────────┴─────────────────────────────────────────────────┘
 *
 * All panels are resizable via drag handles.
 */

"use client";

import { useState } from "react";
import {
  Group,
  Panel,
  Separator,
} from "react-resizable-panels";
import { MarketWatch } from "@/components/trading/market-watch";
import { CandleChart } from "@/components/trading/candle-chart";
import { BotPanel } from "@/components/trading/bot-panel";
import { TradeHistoryPanel } from "@/components/trading/trade-history-panel";
import { BacktestPanel } from "@/components/trading/backtest-panel";
import { FlaskConical, History, Bot } from "lucide-react";

// -----------------------------------------------------------------
// Bottom Panel Tab definitions
// -----------------------------------------------------------------

type BottomTab = "bot" | "backtest" | "history";

const TABS: { id: BottomTab; label: string; icon: React.ElementType }[] = [
  { id: "bot", label: "Bot", icon: Bot },
  { id: "backtest", label: "Backtest", icon: FlaskConical },
  { id: "history", label: "Lịch sử GD", icon: History },
];

// -----------------------------------------------------------------
// Styled Separator Component
// -----------------------------------------------------------------

function ResizeHandle({
  direction = "vertical",
}: {
  direction?: "vertical" | "horizontal";
}) {
  const isVertical = direction === "vertical";
  return (
    <Separator
      className={`group relative flex items-center justify-center
        ${isVertical ? "w-1 cursor-col-resize" : "h-1 cursor-row-resize"}
        bg-border/50 hover:bg-primary/30 active:bg-primary/50
        transition-colors duration-150`}
    >
      <div
        className={`rounded-full bg-muted-foreground/40 group-hover:bg-primary/60 transition-colors
          ${isVertical ? "h-8 w-0.5" : "w-8 h-0.5"}`}
      />
    </Separator>
  );
}

// -----------------------------------------------------------------
// Trading Page
// -----------------------------------------------------------------

export default function TradingPage() {
  const [activeTab, setActiveTab] = useState<BottomTab>("bot");

  return (
    <Group orientation="horizontal" className="h-full">
      {/* Left panel: Market Watch — resizable */}
      <Panel
        defaultSize="18%"
        minSize="10%"
        maxSize="30%"
      >
        <MarketWatch />
      </Panel>

      <ResizeHandle direction="vertical" />

      {/* Right panel: Chart + Bottom Panel */}
      <Panel defaultSize="82%" minSize="50%">
        <Group orientation="vertical" className="h-full">
          {/* Chart area */}
          <Panel defaultSize="65%" minSize="30%">
            <CandleChart />
          </Panel>

          <ResizeHandle direction="horizontal" />

          {/* Bottom Panel — Bot / Backtest / History */}
          <Panel defaultSize="35%" minSize="15%">
            <div className="h-full flex flex-col border-t border-border">
              {/* Tab Header */}
              <div className="flex items-center border-b border-border bg-card/50 px-1 shrink-0">
                {TABS.map((tab) => {
                  const Icon = tab.icon;
                  const isActive = activeTab === tab.id;
                  return (
                    <button
                      key={tab.id}
                      type="button"
                      onClick={() => setActiveTab(tab.id)}
                      className={`
                        flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium
                        border-b-2 transition-colors
                        ${
                          isActive
                            ? "border-primary text-foreground"
                            : "border-transparent text-muted-foreground hover:text-foreground"
                        }
                      `}
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {tab.label}
                    </button>
                  );
                })}
              </div>

              {/* Tab Content */}
              <div className="flex-1 overflow-hidden">
                {activeTab === "bot" && <BotPanel />}
                {activeTab === "backtest" && <BacktestPanel />}
                {activeTab === "history" && <TradeHistoryPanel />}
              </div>
            </div>
          </Panel>
        </Group>
      </Panel>
    </Group>
  );
}
