// ===================================================================
// QuantFlow — Position Display Component
// Task 3.3.5 — Position and PnL Display (Unrealized PnL real-time)
// ===================================================================
//
// Renders a polished position card in the Bot tree view Level 2:
//   ├─ Vị thế: Long │ Entry: 67,200 │ Size: 0.01 │ Leverage: 10x
//   ├─ Unrealized PnL: +12.30 USDT (flashing)
//   ├─ Margin: Isolated │ Total PnL: +125.40 USDT
//   ├─ Lệnh chờ: Limit Sell @ 68,500.0 (Qty: 0.01)
//   └─ Lệnh chờ: Stop Sell @ 66,500.0 (Qty: 0.01)
//
// SRS: FR-MONITOR-03, UC-09
// ===================================================================

"use client";

import { useEffect, useRef, useState } from "react";
import type { BotItem } from "@/lib/hooks/use-bot-data";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

interface PositionDisplayProps {
  bot: BotItem;
  livePnl: {
    totalPnl: number;
    unrealizedPnl: number | null;
  } | null;
}

// -----------------------------------------------------------------
// PnL flash animation hook
// -----------------------------------------------------------------

function usePnlFlash(value: number | null | undefined): "flash-up" | "flash-down" | "" {
  const prevRef = useRef<number | null>(null);
  const [flash, setFlash] = useState<"flash-up" | "flash-down" | "">("");

  useEffect(() => {
    if (value == null || prevRef.current == null) {
      prevRef.current = value ?? null;
      return;
    }

    if (value > prevRef.current) {
      setFlash("flash-up");
    } else if (value < prevRef.current) {
      setFlash("flash-down");
    }

    prevRef.current = value;
    const timeout = setTimeout(() => setFlash(""), 600);
    return () => clearTimeout(timeout);
  }, [value]);

  return flash;
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function PositionDisplay({ bot, livePnl }: PositionDisplayProps) {
  const totalPnl = Number(livePnl?.totalPnl ?? bot.totalPnl) || 0;
  const unrealizedPnl =
    livePnl?.unrealizedPnl ?? bot.position?.unrealizedPnl ?? null;

  const totalFlash = usePnlFlash(totalPnl);
  const unrealizedFlash = usePnlFlash(unrealizedPnl);

  return (
    <div className="pl-10 pr-3 pb-2 space-y-1">
      {/* Position info */}
      {bot.position ? (
        <>
          {/* Row 1: Side + Entry + Size + Leverage */}
          <div className="text-xs text-muted-foreground flex items-center gap-1 flex-wrap">
            <span className="text-muted-foreground/60">├─</span>
            <span>
              Vị thế:{" "}
              <span
                className={`font-semibold ${
                  bot.position.side === "Long"
                    ? "text-success"
                    : "text-danger"
                }`}
              >
                {bot.position.side}
              </span>
            </span>
            <span className="text-muted-foreground/40">│</span>
            <span>
              Entry:{" "}
              <span className="font-mono">
                {bot.position.entryPrice.toLocaleString()}
              </span>
            </span>
            <span className="text-muted-foreground/40">│</span>
            <span>
              Size:{" "}
              <span className="font-mono">{bot.position.quantity}</span>
            </span>
            <span className="text-muted-foreground/40">│</span>
            <span>
              Leverage:{" "}
              <span className="font-mono">{bot.position.leverage}x</span>
            </span>
          </div>

          {/* Row 2: Unrealized PnL (with flash) */}
          <div className="text-xs flex items-center gap-1">
            <span className="text-muted-foreground/60">├─</span>
            <span className="text-muted-foreground">Unrealized PnL:</span>
            <span
              className={`
                font-mono font-semibold transition-colors duration-300
                ${unrealizedPnl != null && unrealizedPnl >= 0 ? "text-success" : "text-danger"}
                ${unrealizedFlash === "flash-up" ? "animate-flash-green" : ""}
                ${unrealizedFlash === "flash-down" ? "animate-flash-red" : ""}
              `}
              style={{
                ...(unrealizedFlash === "flash-up"
                  ? { background: "rgba(34,197,94,0.15)", borderRadius: 2 }
                  : {}),
                ...(unrealizedFlash === "flash-down"
                  ? { background: "rgba(239,68,68,0.15)", borderRadius: 2 }
                  : {}),
                padding: "0 4px",
                transition: "background 0.3s, color 0.3s",
              }}
            >
              {unrealizedPnl != null
                ? `${unrealizedPnl >= 0 ? "+" : ""}${unrealizedPnl.toFixed(2)} USDT`
                : "—"}
            </span>
          </div>

          {/* Row 3: Margin + Total PnL */}
          <div className="text-xs flex items-center gap-1">
            <span className="text-muted-foreground/60">├─</span>
            <span className="text-muted-foreground">
              Margin:{" "}
              <span className="font-medium text-foreground/80">
                {bot.position.marginType}
              </span>
            </span>
            <span className="text-muted-foreground/40">│</span>
            <span className="text-muted-foreground">Total PnL:</span>
            <span
              className={`
                font-mono font-semibold transition-colors duration-300
                ${totalPnl >= 0 ? "text-success" : "text-danger"}
              `}
              style={{
                ...(totalFlash === "flash-up"
                  ? { background: "rgba(34,197,94,0.15)", borderRadius: 2 }
                  : {}),
                ...(totalFlash === "flash-down"
                  ? { background: "rgba(239,68,68,0.15)", borderRadius: 2 }
                  : {}),
                padding: "0 4px",
                transition: "background 0.3s, color 0.3s",
              }}
            >
              {totalPnl >= 0 ? "+" : ""}
              {totalPnl.toFixed(2)} USDT
            </span>
          </div>
        </>
      ) : (
        <div className="text-xs text-muted-foreground/60 flex items-center gap-1">
          <span className="text-muted-foreground/60">├─</span>
          <span>Không có vị thế mở</span>
          <span className="text-muted-foreground/40">│</span>
          <span className="text-muted-foreground">Total PnL:</span>
          <span
            className={`font-mono font-semibold ${
              totalPnl >= 0 ? "text-success" : "text-danger"
            }`}
          >
            {totalPnl >= 0 ? "+" : ""}
            {totalPnl.toFixed(2)} USDT
          </span>
        </div>
      )}

      {/* Open Orders */}
      {bot.openOrders && bot.openOrders.length > 0 ? (
        bot.openOrders.map((order, idx) => (
          <div
            key={order.orderId}
            className="text-xs text-muted-foreground flex items-center gap-1"
          >
            <span className="text-muted-foreground/60">
              {idx === bot.openOrders!.length - 1 ? "└─" : "├─"}
            </span>
            <span>
              Lệnh chờ:{" "}
              <span className="font-medium">{order.type}</span>{" "}
              <span
                className={
                  order.side === "Buy" ? "text-success" : "text-danger"
                }
              >
                {order.side}
              </span>{" "}
              @{" "}
              <span className="font-mono">
                {order.price.toLocaleString()}
              </span>{" "}
              (Qty: <span className="font-mono">{order.quantity}</span>)
            </span>
          </div>
        ))
      ) : (
        <div className="text-xs text-muted-foreground/60 flex items-center gap-1">
          <span className="text-muted-foreground/60">└─</span>
          <span>Không có lệnh chờ</span>
        </div>
      )}
    </div>
  );
}
