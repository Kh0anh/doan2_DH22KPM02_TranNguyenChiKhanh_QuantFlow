// ===================================================================
// QuantFlow — Bot Panel Component
// Task 3.3.3 — Bot Management Panel (Tree View + CRUD)
// ===================================================================
//
// Layout (frontend_flows.md §3.2.4):
//   ┌────────────────────────────────────────────────────────────────┐
//   │ [● Bot]  [ Backtest]  [ Lịch sử GD]           [+ Tạo Bot mới] │
//   ├────────────────────────────────────────────────────────────────┤
//   │ ▸ BTC-Scalper    BTCUSDT  🟢 Running   +125.40 USDT   [⏹][⋮] │
//   │ ▾ ETH-Swing      ETHUSDT  🟢 Running    -12.30 USDT   [⏹][⋮] │
//   │   ├─ Vị thế: Long │ Entry: 3,380.5 │ ...                     │
//   │   └─ Lệnh chờ: Limit Sell @ 3,500.0 (Qty: 0.5)              │
//   │ ▸ SOL-Breakout   SOLUSDT  🔴 Stopped    +45.00 USDT   [▶][⋮] │
//   └────────────────────────────────────────────────────────────────┘
//
// SRS refs: FR-MONITOR-03, UC-08, UC-09
// ===================================================================

"use client";

import { useState, useCallback } from "react";
import {
  ChevronRight,
  ChevronDown,
  Plus,
  Square,
  Play,
  MoreVertical,
  Trash2,
  Bot,
  Loader2,
  Terminal,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useBotData, type BotItem } from "@/lib/hooks/use-bot-data";
import { CreateBotDialog } from "@/components/trading/create-bot-dialog";
import { StopBotDialog } from "@/components/trading/stop-bot-dialog";
import { BotLogsConsole } from "@/components/trading/bot-logs-console";
import { PositionDisplay } from "@/components/trading/position-display";
import { usePositionUpdates } from "@/lib/hooks/use-position-updates";
import { toast } from "sonner";

// -----------------------------------------------------------------
// Status badge
// -----------------------------------------------------------------

const STATUS_CONFIG: Record<
  string,
  { label: string; emoji: string; color: string }
> = {
  Running: { label: "Running", emoji: "🟢", color: "text-success" },
  Stopped: { label: "Stopped", emoji: "🔴", color: "text-muted-foreground" },
  Error: { label: "Error", emoji: "⚠️", color: "text-danger" },
  Reconnecting: {
    label: "Reconnect",
    emoji: "🟡",
    color: "text-warning",
  },
};

function StatusBadge({ status }: { status: string }) {
  const config = STATUS_CONFIG[status] ?? STATUS_CONFIG.Error;
  return (
    <span
      className={`text-xs font-medium ${config.color} flex items-center gap-1`}
    >
      <span className="text-[10px]">{config.emoji}</span>
      {config.label}
    </span>
  );
}

// -----------------------------------------------------------------
// PnL display
// -----------------------------------------------------------------

function PnlDisplay({ pnl }: { pnl: number }) {
  const val = Number(pnl) || 0;
  const isPositive = val >= 0;
  const sign = isPositive ? "+" : "";
  return (
    <span
      className={`font-mono text-xs font-medium ${
        isPositive ? "text-success" : "text-danger"
      }`}
    >
      {sign}
      {val.toFixed(2)} USDT
    </span>
  );
}

// -----------------------------------------------------------------
// LivePnl type
// -----------------------------------------------------------------

interface LivePnlData {
  totalPnl: number;
  unrealizedPnl: number | null;
}

// -----------------------------------------------------------------
// Bot Row — Tree Level 1
// -----------------------------------------------------------------

function BotRow({
  bot,
  isExpanded,
  onToggle,
  onStop,
  onStart,
  onDelete,
  onViewLogs,
  livePnl,
}: {
  bot: BotItem;
  isExpanded: boolean;
  onToggle: () => void;
  onStop: () => void;
  onStart: () => void;
  onDelete: () => void;
  onViewLogs: () => void;
  livePnl: LivePnlData | null;
}) {
  const isRunning = bot.status === "Running";
  const isStopped = bot.status === "Stopped";

  return (
    <div className="border-b border-border last:border-b-0">
      {/* Row Level 1 */}
      <div
        className="flex items-center gap-2 px-3 py-2 hover:bg-accent/40 transition-colors cursor-pointer group"
        onClick={onToggle}
      >
        {/* Expand/Collapse */}
        <button type="button" className="text-muted-foreground shrink-0">
          {isExpanded ? (
            <ChevronDown className="h-3.5 w-3.5" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5" />
          )}
        </button>

        {/* Bot Name */}
        <span className="text-sm font-medium text-foreground min-w-[100px] truncate">
          {bot.name}
        </span>

        {/* Symbol */}
        <span className="text-xs text-muted-foreground min-w-[65px]">
          {bot.symbol}
        </span>

        {/* Status */}
        <div className="min-w-[80px]">
          <StatusBadge status={bot.status} />
        </div>

        {/* PnL — live override if available */}
        <div className="flex-1 text-right">
          <PnlDisplay pnl={livePnl?.totalPnl ?? bot.totalPnl} />
        </div>

        {/* Actions */}
        <div
          className="flex items-center gap-1 ml-2 opacity-0 group-hover:opacity-100 transition-opacity"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Start / Stop button */}
          {isRunning ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              title="Dừng Bot"
              onClick={onStop}
            >
              <Square className="h-3 w-3 text-danger" />
            </Button>
          ) : isStopped ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              title="Khởi động Bot"
              onClick={onStart}
            >
              <Play className="h-3 w-3 text-success" />
            </Button>
          ) : null}

          {/* More menu */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-6 w-6">
                <MoreVertical className="h-3 w-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-32">
              <DropdownMenuItem onClick={onViewLogs}>
                <Terminal className="mr-2 h-3.5 w-3.5" />
                Logs
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={onDelete}
                disabled={isRunning}
                className="text-destructive focus:text-destructive"
              >
                <Trash2 className="mr-2 h-3.5 w-3.5" />
                Xóa
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Row Level 2 — Expanded Detail (Task 3.3.5: PositionDisplay) */}
      {isExpanded && (
        <PositionDisplay bot={bot} livePnl={livePnl} />
      )}
    </div>
  );
}

// -----------------------------------------------------------------
// BotPanel — Main Export
// -----------------------------------------------------------------

export function BotPanel() {
  const {
    bots,
    isLoading,
    expandedIds,
    toggleExpand,
    createBot,
    startBot,
    stopBot,
    deleteBot,
  } = useBotData();

  // Task 3.3.5: Real-time PnL simulation
  const { getLivePnl } = usePositionUpdates(bots);

  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [stopTarget, setStopTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [logsTarget, setLogsTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);

  // ------- Handlers -------

  const handleCreate = useCallback(
    async (params: {
      botName: string;
      strategyId: string;
      symbol: string;
    }) => {
      try {
        const result = await createBot(params);
        if (result.success) {
          toast.success(`Bot "${params.botName}" đã khởi chạy thành công`);
        }
        return result;
      } catch {
        toast.error("Không thể tạo Bot. Vui lòng thử lại.");
        throw new Error("Create bot failed");
      }
    },
    [createBot],
  );

  const handleStart = useCallback(
    async (bot: BotItem) => {
      try {
        await startBot(bot.id);
        toast.success(`Bot "${bot.name}" đã khởi động lại`);
      } catch {
        toast.error(`Không thể khởi động Bot "${bot.name}". Vui lòng thử lại.`);
      }
    },
    [startBot],
  );

  const handleStopConfirm = useCallback(
    async (botId: string, closePosition: boolean) => {
      try {
        await stopBot(botId, closePosition);
        const bot = bots.find((b) => b.id === botId);
        toast.success(
          `Bot "${bot?.name ?? botId}" đã dừng${
            closePosition ? " và đóng vị thế" : ""
          }`,
        );
      } catch {
        toast.error("Không thể dừng Bot. Vui lòng thử lại.");
      }
    },
    [stopBot, bots],
  );

  const handleDelete = useCallback(
    async (bot: BotItem) => {
      if (bot.status === "Running") {
        toast.error("Không thể xóa Bot đang chạy. Vui lòng dừng Bot trước.");
        return;
      }
      try {
        await deleteBot(bot.id);
        toast.success(`Bot "${bot.name}" đã được xóa`);
      } catch {
        toast.error(`Không thể xóa Bot "${bot.name}". Vui lòng thử lại.`);
      }
    },
    [deleteBot],
  );

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-1.5 border-b border-border bg-card/50">
        <div className="flex items-center gap-2 text-sm">
          <Bot className="h-3.5 w-3.5 text-primary" />
          <span className="font-medium text-foreground">
            Bot ({bots.length})
          </span>
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 gap-1 text-xs"
          onClick={() => setShowCreateDialog(true)}
        >
          <Plus className="h-3 w-3" />
          Tạo Bot mới
        </Button>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : bots.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 gap-2">
            <Bot className="h-8 w-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">Chưa có Bot nào</p>
            <Button
              variant="outline"
              size="sm"
              className="gap-1"
              onClick={() => setShowCreateDialog(true)}
            >
              <Plus className="h-3 w-3" />
              Tạo Bot đầu tiên
            </Button>
          </div>
        ) : (
          bots.map((bot) => (
            <BotRow
              key={bot.id}
              bot={bot}
              isExpanded={expandedIds.has(bot.id)}
              onToggle={() => toggleExpand(bot.id)}
              onStop={() => setStopTarget({ id: bot.id, name: bot.name })}
              onStart={() => handleStart(bot)}
              onDelete={() => handleDelete(bot)}
              onViewLogs={() => setLogsTarget({ id: bot.id, name: bot.name })}
              livePnl={getLivePnl(bot.id)}
            />
          ))
        )}
      </div>

      {/* Dialogs */}
      <CreateBotDialog
        open={showCreateDialog}
        onOpenChange={setShowCreateDialog}
        onSubmit={handleCreate}
      />

      {stopTarget && (
        <StopBotDialog
          open={!!stopTarget}
          onOpenChange={(open) => !open && setStopTarget(null)}
          botName={stopTarget.name}
          botId={stopTarget.id}
          onConfirm={handleStopConfirm}
        />
      )}

      {logsTarget && (
        <BotLogsConsole
          botId={logsTarget.id}
          botName={logsTarget.name}
          onClose={() => setLogsTarget(null)}
        />
      )}
    </div>
  );
}
