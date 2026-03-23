// ===================================================================
// QuantFlow — Market Watch Component
// Task 3.3.1 — Symbol list + real-time price flash
// ===================================================================
//
// Layout (frontend_flows.md §3.2.2):
//   ┌─────────────────────┐
//   │  🔍 Tìm kiếm...     │  ← sticky search filter
//   ├─────────────────────┤
//   │  Symbol     Giá      │
//   ├─────────────────────┤
//   │▶ BTCUSDT  67,432.5  │  ← active row
//   │  ETHUSDT   3,421.2  │
//   │  SOLUSDT     142.8  │
//   └─────────────────────┘
//
// Features:
//   - Client-side instant search filter
//   - Price flash animation (green up / red down, 300ms)
//   - Active row highlight via Zustand activeSymbol
//   - Bot badge indicator (hasRunningBot)
//   - Click row → setActiveSymbol
//
// SRS refs: FR-MONITOR-01, UC-10
// ===================================================================

"use client";

import { useState, useMemo } from "react";
import { Search, Bot, TrendingUp, TrendingDown, ArrowUpDown, ArrowUp, ArrowDown } from "lucide-react";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useUIStore } from "@/store/ui-store";
import { useMarketData, type MarketSymbol } from "@/lib/hooks/use-market-data";

// -----------------------------------------------------------------
// Price formatter — locale-aware number formatting
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
// SymbolRow sub-component
// -----------------------------------------------------------------

interface SymbolRowProps {
  symbol: MarketSymbol;
  isActive: boolean;
  onSelect: (symbol: string) => void;
}

function SymbolRow({ symbol, isActive, onSelect }: SymbolRowProps) {
  const isPositive = (Number(symbol.priceChangePercent) || 0) >= 0;

  // Flash animation class (300ms defined in globals.css)
  const flashClass =
    symbol.flashDirection === "up"
      ? "animate-price-flash-up"
      : symbol.flashDirection === "down"
        ? "animate-price-flash-down"
        : "";

  return (
    <button
      id={`market-watch-row-${symbol.symbol}`}
      type="button"
      onClick={() => onSelect(symbol.symbol)}
      className={`
        w-full flex items-center justify-between px-3 py-2
        text-left text-sm transition-colors duration-150
        border-b border-border/50
        hover:bg-[var(--bg-tertiary,#21262D)]
        ${isActive ? "bg-[#21262D] border-l-2 border-l-primary" : "border-l-2 border-l-transparent"}
        ${flashClass}
      `}
    >
      {/* Left: Symbol name + Bot badge */}
      <div className="flex items-center gap-2 min-w-0">
        {/* Active indicator */}
        {isActive && (
          <span className="text-primary text-xs">▶</span>
        )}

        {/* Symbol name */}
        <span
          className={`font-medium truncate ${
            isActive ? "text-foreground" : "text-muted-foreground"
          }`}
        >
          {symbol.baseAsset}
          <span className="text-muted-foreground/60 font-normal">
            /{symbol.quoteAsset}
          </span>
        </span>

        {/* Bot running badge */}
        {symbol.hasRunningBot && (
          <Bot className="h-3 w-3 text-primary flex-shrink-0" />
        )}
      </div>

      {/* Right: Price + Change % */}
      <div className="flex flex-col items-end flex-shrink-0 ml-2">
        <span
          className={`font-mono text-xs font-medium ${
            isPositive ? "text-success" : "text-danger"
          }`}
        >
          {formatPrice(symbol.lastPrice)}
        </span>
        <span
          className={`font-mono text-[10px] flex items-center gap-0.5 ${
            isPositive ? "text-success/80" : "text-danger/80"
          }`}
        >
          {isPositive ? (
            <TrendingUp className="h-2.5 w-2.5" />
          ) : (
            <TrendingDown className="h-2.5 w-2.5" />
          )}
          {formatChangePercent(symbol.priceChangePercent)}
        </span>
      </div>
    </button>
  );
}

// -----------------------------------------------------------------
// MarketWatch main component
// -----------------------------------------------------------------

type SortField = "price" | "change";
type SortDir = "asc" | "desc";

export function MarketWatch() {
  const [searchQuery, setSearchQuery] = useState("");
  const [sortField, setSortField] = useState<SortField | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const { symbols, isLoading } = useMarketData();
  const activeSymbol = useUIStore((s) => s.activeSymbol);
  const setActiveSymbol = useUIStore((s) => s.setActiveSymbol);

  // Toggle sort: click same field → flip direction, click different → set desc
  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    } else {
      setSortField(field);
      setSortDir("desc");
    }
  };

  // Client-side instant filter + sort
  const filteredSymbols = useMemo(() => {
    let result = symbols;

    // Search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toUpperCase().trim();
      result = result.filter(
        (s) =>
          s.symbol.includes(q) ||
          s.baseAsset.toUpperCase().includes(q),
      );
    }

    // Sort
    if (sortField) {
      result = [...result].sort((a, b) => {
        const valA = sortField === "price" ? a.lastPrice : a.priceChangePercent;
        const valB = sortField === "price" ? b.lastPrice : b.priceChangePercent;
        return sortDir === "desc" ? valB - valA : valA - valB;
      });
    }

    return result;
  }, [symbols, searchQuery, sortField, sortDir]);

  // Sort indicator icon
  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) {
      return <ArrowUpDown className="h-2.5 w-2.5 opacity-40" />;
    }
    return sortDir === "desc" ? (
      <ArrowDown className="h-2.5 w-2.5 text-primary" />
    ) : (
      <ArrowUp className="h-2.5 w-2.5 text-primary" />
    );
  };

  return (
    <div
      id="market-watch-panel"
      className="
        h-full flex flex-col
        border-r border-border
        bg-card
      "
    >
      {/* Header */}
      <div className="px-3 py-2 border-b border-border">
        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          Theo dõi Thị trường
        </h2>

        {/* Search input — sticky top */}
        <div className="relative">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            id="market-watch-search"
            type="text"
            placeholder="Tìm kiếm..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="
              h-7 pl-7 text-xs
              bg-secondary border-border
              placeholder:text-muted-foreground/60
              focus-visible:ring-1 focus-visible:ring-primary
            "
          />
        </div>
      </div>

      {/* Column headers — clickable for sorting */}
      <div className="flex items-center justify-between px-3 py-1.5 text-[10px] text-muted-foreground/70 uppercase tracking-wider border-b border-border/50">
        <span>Cặp tiền</span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => handleSort("price")}
            className="flex items-center gap-0.5 hover:text-foreground transition-colors cursor-pointer"
          >
            Giá <SortIcon field="price" />
          </button>
          <span className="text-muted-foreground/30">/</span>
          <button
            type="button"
            onClick={() => handleSort("change")}
            className="flex items-center gap-0.5 hover:text-foreground transition-colors cursor-pointer"
          >
            24h <SortIcon field="change" />
          </button>
        </div>
      </div>

      {/* Symbol list */}
      <ScrollArea className="flex-1">
        {isLoading ? (
          // Loading skeleton
          <div className="space-y-0">
            {[...Array(4)].map((_, i) => (
              <div
                key={i}
                className="flex items-center justify-between px-3 py-2 border-b border-border/50"
              >
                <div className="h-3.5 w-16 bg-secondary rounded animate-pulse" />
                <div className="flex flex-col items-end gap-1">
                  <div className="h-3 w-14 bg-secondary rounded animate-pulse" />
                  <div className="h-2.5 w-10 bg-secondary rounded animate-pulse" />
                </div>
              </div>
            ))}
          </div>
        ) : filteredSymbols.length === 0 ? (
          // Empty state
          <div className="flex flex-col items-center justify-center py-8 px-4 text-center">
            <Search className="h-6 w-6 text-muted-foreground/40 mb-2" />
            <p className="text-xs text-muted-foreground">
              Không tìm thấy cặp tiền
            </p>
          </div>
        ) : (
          // Symbol rows
          <div>
            {filteredSymbols.map((symbol) => (
              <SymbolRow
                key={symbol.symbol}
                symbol={symbol}
                isActive={symbol.symbol === activeSymbol}
                onSelect={setActiveSymbol}
              />
            ))}
          </div>
        )}
      </ScrollArea>

      {/* Footer: symbol count */}
      <div className="px-3 py-1.5 border-t border-border text-[10px] text-muted-foreground/50 text-center">
        {filteredSymbols.length} / {symbols.length} cặp tiền
      </div>
    </div>
  );
}

