// ===================================================================
// QuantFlow — Bot Logs Console
// Task 3.3.4 — Bot Logs Console (Terminal-style + Floating + Auto-scroll)
// ===================================================================
//
// Spec (frontend_flows.md §3.2.4):
//   ┌── Console: BTC-Scalper ─────────────────────── [×] ─┐
//   │ [10:00:01] Session #142 triggered                    │
//   │ [10:00:01] RSI(15m,14) = 32.5                        │
//   │ [10:00:02] → LONG BTCUSDT Qty=0.01 → ORDER_PLACED   │
//   │ [10:00:02] Unit used: 23/1000                        │
//   │                                             [↓]      │
//   └──────────────────────────────────────────────────────┘
//
// Features:
//   - Floating popup (600×400, fixed position)
//   - Draggable via title bar (pointer events)
//   - JetBrains Mono 12px terminal font
//   - Color-coded: order=success, skip=warning, error=danger
//   - Auto-scroll (disable on scroll up, ↓ button to re-enable)
//   - Max 1000 lines (virtual scroll trimming)
//
// SRS refs: UC-09, FR-MONITOR-03
// ===================================================================

"use client";

import { useRef, useEffect, useState, useCallback } from "react";
import { X, ArrowDown, Terminal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useBotLogs, type LogEntry } from "@/lib/hooks/use-bot-logs";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

interface BotLogsConsoleProps {
  botId: string;
  botName: string;
  onClose: () => void;
}

// -----------------------------------------------------------------
// Log level → color class mapping
// -----------------------------------------------------------------

const LEVEL_COLORS: Record<LogEntry["level"], string> = {
  info: "text-muted-foreground",
  order: "text-success",
  skip: "text-warning",
  error: "text-danger",
};

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function BotLogsConsole({ botId, botName, onClose }: BotLogsConsoleProps) {
  const { logs, isLoading } = useBotLogs(botId);

  // ------- Auto-scroll state -------
  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const lastLogCountRef = useRef(0);

  // Auto-scroll when new logs arrive
  useEffect(() => {
    if (!autoScroll || !scrollRef.current) return;
    if (logs.length > lastLogCountRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
    lastLogCountRef.current = logs.length;
  }, [logs.length, autoScroll]);

  // Detect manual scroll up → disable auto-scroll
  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const el = scrollRef.current;
    const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30;
    setAutoScroll(isAtBottom);
  }, []);

  // ↓ button → re-enable auto-scroll
  const scrollToBottom = useCallback(() => {
    if (!scrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    setAutoScroll(true);
  }, []);

  // ------- Draggable state -------
  const [pos, setPos] = useState({ x: -1, y: -1 });
  const dragRef = useRef<{ startX: number; startY: number; origX: number; origY: number } | null>(null);

  // Center on mount
  useEffect(() => {
    if (pos.x === -1 && pos.y === -1) {
      const x = Math.max(60, (window.innerWidth - 600) / 2);
      const y = Math.max(60, (window.innerHeight - 400) / 2);
      setPos({ x, y });
    }
  }, [pos]);

  const handlePointerDown = useCallback(
    (e: React.PointerEvent) => {
      dragRef.current = {
        startX: e.clientX,
        startY: e.clientY,
        origX: pos.x,
        origY: pos.y,
      };
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
    },
    [pos],
  );

  const handlePointerMove = useCallback((e: React.PointerEvent) => {
    if (!dragRef.current) return;
    const dx = e.clientX - dragRef.current.startX;
    const dy = e.clientY - dragRef.current.startY;
    setPos({
      x: Math.max(0, dragRef.current.origX + dx),
      y: Math.max(0, dragRef.current.origY + dy),
    });
  }, []);

  const handlePointerUp = useCallback(() => {
    dragRef.current = null;
  }, []);

  return (
    <div
      className="fixed z-50 flex flex-col rounded-lg border border-border shadow-2xl overflow-hidden"
      style={{
        width: 600,
        height: 400,
        left: pos.x,
        top: pos.y,
        background: "#0D1117",
      }}
    >
      {/* Title Bar — drag handle */}
      <div
        className="flex items-center justify-between px-3 py-1.5 select-none cursor-move"
        style={{ background: "#161B22", borderBottom: "1px solid #30363D" }}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
      >
        <div className="flex items-center gap-2 text-xs">
          <Terminal className="h-3.5 w-3.5 text-primary" />
          <span className="font-medium text-foreground">
            Console: {botName}
          </span>
          <span className="text-muted-foreground">
            ({logs.length} lines)
          </span>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="h-5 w-5 text-muted-foreground hover:text-foreground"
          onClick={onClose}
        >
          <X className="h-3 w-3" />
        </Button>
      </div>

      {/* Log Content */}
      <div
        ref={scrollRef}
        className="flex-1 overflow-y-auto overflow-x-hidden p-2"
        onScroll={handleScroll}
        style={{
          fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
          fontSize: 12,
          lineHeight: "18px",
        }}
      >
        {isLoading ? (
          <div className="flex items-center justify-center h-full">
            <span className="text-muted-foreground text-xs">
              Đang tải logs...
            </span>
          </div>
        ) : logs.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <span className="text-muted-foreground/50 text-xs">
              Chưa có log nào
            </span>
          </div>
        ) : (
          logs.map((log) => (
            <LogLine key={log.id} entry={log} />
          ))
        )}
      </div>

      {/* Auto-scroll ↓ button */}
      {!autoScroll && logs.length > 0 && (
        <button
          type="button"
          onClick={scrollToBottom}
          className="
            absolute bottom-3 right-4
            flex items-center gap-1 px-2 py-1
            rounded-md text-xs font-medium
            bg-primary text-primary-foreground
            shadow-lg hover:bg-primary/90 transition-colors
          "
        >
          <ArrowDown className="h-3 w-3" />
          ↓
        </button>
      )}
    </div>
  );
}

// -----------------------------------------------------------------
// LogLine — single log row
// -----------------------------------------------------------------

function LogLine({ entry }: { entry: LogEntry }) {
  const colorClass = LEVEL_COLORS[entry.level] ?? LEVEL_COLORS.info;

  return (
    <div className={`whitespace-pre-wrap break-all ${colorClass}`}>
      <span className="text-muted-foreground/60">{entry.formattedTime}</span>
      {" "}
      {entry.message}
    </div>
  );
}
