// ===================================================================
// QuantFlow — Trade History Panel
// Task 3.3.6 — Trade History Table (Infinite Scroll + Multi-filter + CSV)
// ===================================================================
//
// Layout (frontend_flows.md §3.2.6):
//   ┌────────────────────────────────────────────────────────────────────┐
//   │ Bot:[All▼] Symbol:[All▼] Side:[All▼] Status:[All▼]   [📥 CSV]   │
//   ├──────────┬──────────┬────────┬──────┬──────┬──────────┬───────────┤
//   │ Thời gian│ Bot      │ Symbol │ Side │ Qty  │ Giá khớp │ PnL      │
//   ├──────────┼──────────┼────────┼──────┼──────┼──────────┼───────────┤
//   │ ...      │          │        │      │      │          │          │
//   │    (Cuộn xuống để tải thêm — Infinite Scroll)                    │
//   └──────────────────────────────────────────────────────────────────┘
//
// SRS: UC-11
// ===================================================================

"use client";

import { useRef, useCallback, useEffect } from "react";
import { Download, History, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTradeHistory, type TradeItem } from "@/lib/hooks/use-trade-history";

// -----------------------------------------------------------------
// Constants
// -----------------------------------------------------------------

const SYMBOLS = ["", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];
const SIDES = ["", "Long", "Short"];
const STATUSES = ["", "Filled", "Canceled"];

// -----------------------------------------------------------------
// Format helpers
// -----------------------------------------------------------------

function formatTime(iso: string): string {
  const d = new Date(iso);
  const dd = String(d.getDate()).padStart(2, "0");
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mi = String(d.getMinutes()).padStart(2, "0");
  return `${dd}/${mm} ${hh}:${mi}`;
}

// -----------------------------------------------------------------
// TradeRow
// -----------------------------------------------------------------

function TradeRow({ trade }: { trade: TradeItem }) {
  const pnl = trade.realizedPnl;
  const isPositive = pnl >= 0;

  return (
    <tr className="border-b border-border/50 hover:bg-accent/30 transition-colors text-xs">
      <td className="py-1.5 px-2 text-muted-foreground whitespace-nowrap">
        {formatTime(trade.executedAt)}
      </td>
      <td className="py-1.5 px-2 font-medium text-foreground truncate max-w-[100px]">
        {trade.botName}
      </td>
      <td className="py-1.5 px-2 text-muted-foreground">
        {trade.symbol}
      </td>
      <td className="py-1.5 px-2">
        <span
          className={`font-medium ${
            trade.side === "Long" ? "text-success" : "text-danger"
          }`}
        >
          {trade.side}
        </span>
      </td>
      <td className="py-1.5 px-2 font-mono text-muted-foreground text-right">
        {trade.quantity}
      </td>
      <td className="py-1.5 px-2 font-mono text-foreground text-right">
        {trade.fillPrice.toLocaleString()}
      </td>
      <td className="py-1.5 px-2 font-mono text-right">
        <span className={isPositive ? "text-success" : "text-danger"}>
          {isPositive ? "+" : ""}
          {pnl.toFixed(2)}
        </span>
      </td>
    </tr>
  );
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function TradeHistoryPanel() {
  const {
    trades,
    isLoading,
    isLoadingMore,
    hasMore,
    filters,
    bots,
    updateFilter,
    loadMore,
    exportCSV,
  } = useTradeHistory();

  const scrollRef = useRef<HTMLDivElement>(null);

  // ------- Infinite scroll -------
  const handleScroll = useCallback(() => {
    if (!scrollRef.current || isLoadingMore || !hasMore) return;
    const el = scrollRef.current;
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 60) {
      loadMore();
    }
  }, [isLoadingMore, hasMore, loadMore]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener("scroll", handleScroll);
    return () => el.removeEventListener("scroll", handleScroll);
  }, [handleScroll]);

  return (
    <div className="flex flex-col h-full">
      {/* Filter bar */}
      <div className="flex items-center gap-2 px-3 py-1.5 border-b border-border bg-card/50 flex-wrap">
        {/* Bot filter */}
        <Select
          value={filters.botId}
          onValueChange={(v) => updateFilter("botId", v === "__all__" ? "" : v)}
        >
          <SelectTrigger className="h-7 w-[130px] text-xs">
            <SelectValue placeholder="Tất cả Bot" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Tất cả Bot</SelectItem>
            {bots.map((b) => (
              <SelectItem key={b.id} value={b.id}>
                {b.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Symbol filter */}
        <Select
          value={filters.symbol}
          onValueChange={(v) =>
            updateFilter("symbol", v === "__all__" ? "" : v)
          }
        >
          <SelectTrigger className="h-7 w-[110px] text-xs">
            <SelectValue placeholder="Tất cả Cặp tiền" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Tất cả Cặp tiền</SelectItem>
            {SYMBOLS.filter(Boolean).map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Side filter */}
        <Select
          value={filters.side}
          onValueChange={(v) => updateFilter("side", v === "__all__" ? "" : v)}
        >
          <SelectTrigger className="h-7 w-[90px] text-xs">
            <SelectValue placeholder="Tất cả Chiều" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Tất cả Chiều</SelectItem>
            {SIDES.filter(Boolean).map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Status filter */}
        <Select
          value={filters.status}
          onValueChange={(v) =>
            updateFilter("status", v === "__all__" ? "" : v)
          }
        >
          <SelectTrigger className="h-7 w-[100px] text-xs">
            <SelectValue placeholder="Tất cả Trạng thái" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Tất cả Trạng thái</SelectItem>
            {STATUSES.filter(Boolean).map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="flex-1" />

        {/* CSV export */}
        <Button
          variant="outline"
          size="sm"
          className="h-7 gap-1 text-xs"
          onClick={exportCSV}
        >
          <Download className="h-3 w-3" />
          CSV
        </Button>
      </div>

      {/* Table */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto overflow-x-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : trades.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 gap-2">
            <History className="h-8 w-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">
              Chưa có lịch sử giao dịch nào.
            </p>
          </div>
        ) : (
          <table className="w-full min-w-[600px]">
            <thead className="sticky top-0 bg-card z-10">
              <tr className="text-xs text-muted-foreground border-b border-border">
                <th className="py-1.5 px-2 text-left font-medium">
                  Thời gian
                </th>
                <th className="py-1.5 px-2 text-left font-medium">Bot</th>
                <th className="py-1.5 px-2 text-left font-medium">Cặp tiền</th>
                <th className="py-1.5 px-2 text-left font-medium">Chiều</th>
                <th className="py-1.5 px-2 text-right font-medium">Khối lượng</th>
                <th className="py-1.5 px-2 text-right font-medium">
                  Giá khớp
                </th>
                <th className="py-1.5 px-2 text-right font-medium">Lợi nhuận</th>
              </tr>
            </thead>
            <tbody>
              {trades.map((trade) => (
                <TradeRow key={trade.id} trade={trade} />
              ))}
            </tbody>
          </table>
        )}

        {/* Loading more indicator */}
        {isLoadingMore && (
          <div className="flex items-center justify-center py-3">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground mr-2" />
            <span className="text-xs text-muted-foreground">
              Đang tải thêm...
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
