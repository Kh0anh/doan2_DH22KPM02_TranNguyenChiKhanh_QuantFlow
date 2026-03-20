/**
 * [3.3.x] Trading Dashboard — Market Watch + Candle Chart + Bot Panel + History.
 *
 * Layout (frontend_flows.md §3.2.1):
 *   ┌──────────┬─────────────────────────────────────────────────┐
 *   │  Market  │  [Chart Area]                                    │
 *   │  Watch   │                                                  │
 *   │  220px   │  ─ ─ ─ Resizable Splitter ─ ─ ─                 │
 *   │          │  [Bottom Panel: Bot / Backtest / History]         │
 *   └──────────┴─────────────────────────────────────────────────┘
 *
 * Task 3.3.1: MarketWatch component (implemented)
 * Tasks 3.3.2–3.3.6: Chart, Bot, History (future tasks — placeholder)
 */

import { MarketWatch } from "@/components/trading/market-watch";

export default function TradingPage() {
  return (
    <div className="h-full flex overflow-hidden">
      {/* Left panel: Market Watch (220px fixed) — Task 3.3.1 */}
      <MarketWatch />

      {/* Right panel: Chart + Bottom Panel — Tasks 3.3.2–3.3.6 */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Chart area placeholder — Task 3.3.2 */}
        <div className="flex-1 flex items-center justify-center border-b border-border">
          <div className="text-center">
            <p className="text-sm text-muted-foreground">
              📊 Candlestick Chart — Task 3.3.2
            </p>
            <p className="text-xs text-muted-foreground/50 mt-1">
              Lightweight Charts will be rendered here
            </p>
          </div>
        </div>

        {/* Bottom panel placeholder — Tasks 3.3.3–3.3.6 */}
        <div className="h-[280px] min-h-[200px] flex items-center justify-center">
          <div className="text-center">
            <p className="text-sm text-muted-foreground">
              📋 Bottom Panel — Tasks 3.3.3–3.3.6
            </p>
            <p className="text-xs text-muted-foreground/50 mt-1">
              Bot / Backtest / Trade History tabs
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
