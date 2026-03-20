/**
 * [3.3.x] Trading Dashboard — Market Watch + Candle Chart + Bot Panel + History.
 *
 * Layout (frontend_flows.md §3.2.1):
 *   ┌──────────┬─────────────────────────────────────────────────┐
 *   │  Market  │  [Chart Header + Timeframe Tabs]                 │
 *   │  Watch   │  [Candlestick Chart + Trade Markers]             │
 *   │  220px   │  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─                │
 *   │          │  [Bottom Panel: Bot / Backtest / History]         │
 *   └──────────┴─────────────────────────────────────────────────┘
 *
 * Task 3.3.1: MarketWatch component (implemented)
 * Task 3.3.2: CandleChart component (implemented)
 * Task 3.3.3: BotPanel component (implemented)
 * Tasks 3.3.4–3.3.6: Backtest, History (future tasks — placeholder)
 */

"use client";

import { useState } from "react";
import { MarketWatch } from "@/components/trading/market-watch";
import { CandleChart } from "@/components/trading/candle-chart";
import { BotPanel } from "@/components/trading/bot-panel";
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
// Trading Page
// -----------------------------------------------------------------

export default function TradingPage() {
  const [activeTab, setActiveTab] = useState<BottomTab>("bot");

  return (
    <div className="h-full flex overflow-hidden">
      {/* Left panel: Market Watch (220px fixed) — Task 3.3.1 */}
      <MarketWatch />

      {/* Right panel: Chart + Bottom Panel — Tasks 3.3.2–3.3.6 */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Chart area — Task 3.3.2 */}
        <CandleChart />

        {/* Bottom Panel — Tasks 3.3.3–3.3.6 */}
        <div className="h-[280px] min-h-[200px] flex flex-col border-t border-border">
          {/* Tab Header */}
          <div className="flex items-center border-b border-border bg-card/50 px-1">
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
            {activeTab === "backtest" && (
              <div className="flex items-center justify-center h-full">
                <div className="text-center">
                  <FlaskConical className="h-8 w-8 text-muted-foreground/30 mx-auto mb-2" />
                  <p className="text-sm text-muted-foreground">
                    📋 Backtest Panel — Task 3.3.4
                  </p>
                </div>
              </div>
            )}
            {activeTab === "history" && (
              <div className="flex items-center justify-center h-full">
                <div className="text-center">
                  <History className="h-8 w-8 text-muted-foreground/30 mx-auto mb-2" />
                  <p className="text-sm text-muted-foreground">
                    📋 Trade History Panel — Task 3.3.5
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
