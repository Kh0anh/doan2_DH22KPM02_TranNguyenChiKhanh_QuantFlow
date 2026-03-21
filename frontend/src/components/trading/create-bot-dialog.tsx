// ===================================================================
// QuantFlow — Create Bot Dialog
// Task 3.3.3 — Bot Management Panel
// ===================================================================
//
// UX Flow (frontend_flows.md §3.2.4):
//   Click "+ Tạo Bot mới" → Dialog popup:
//     Tên Bot: [_________]
//     Chiến lược: [Dropdown ▼]
//     Cặp tiền: [BTCUSDT ▼]  ← auto-fill từ Chart đang xem
//     [Hủy] [Khởi chạy]
// ===================================================================

"use client";

import { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Loader2 } from "lucide-react";
import { useUIStore } from "@/store/ui-store";
import { strategyApi } from "@/lib/api-client";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

interface StrategyOption {
  id: string;
  name: string;
}

interface CreateBotDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (params: {
    botName: string;
    strategyId: string;
    symbol: string;
  }) => Promise<{ success: boolean }>;
}

// -----------------------------------------------------------------
// Mock strategies for fallback
// -----------------------------------------------------------------

const MOCK_STRATEGIES: StrategyOption[] = [
  { id: "strat-001", name: "EMA Crossover Strategy" },
  { id: "strat-002", name: "RSI Reversal" },
  { id: "strat-003", name: "Bollinger Grid" },
];

const SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT"];

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function CreateBotDialog({
  open,
  onOpenChange,
  onSubmit,
}: CreateBotDialogProps) {
  const activeSymbol = useUIStore((s) => s.activeSymbol);
  const [botName, setBotName] = useState("");
  const [strategyId, setStrategyId] = useState("");
  const [symbol, setSymbol] = useState(activeSymbol);
  const [strategies, setStrategies] =
    useState<StrategyOption[]>(MOCK_STRATEGIES);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Auto-fill symbol from active chart
  useEffect(() => {
    if (open) {
      setSymbol(activeSymbol);
      setBotName("");
      setStrategyId("");
      setError(null);
    }
  }, [open, activeSymbol]);

  // Load strategies
  useEffect(() => {
    if (!open) return;
    (async () => {
      try {
        const res = await strategyApi.list({ page: 1, limit: 50 });
        if (res.data && res.data.length > 0) {
          // Only show Valid strategies — Draft strategies cannot be used by bots
          const validStrategies = res.data
            .filter((s: { status: string }) => s.status === "Valid")
            .map((s: { id: string; name: string }) => ({
              id: s.id,
              name: s.name,
            }));
          setStrategies(validStrategies);
        }
      } catch {
        // Use mock strategies
      }
    })();
  }, [open]);

  const handleSubmit = async () => {
    if (!botName.trim()) {
      setError("Vui lòng nhập tên Bot");
      return;
    }
    if (!strategyId) {
      setError("Vui lòng chọn chiến lược");
      return;
    }

    setIsSubmitting(true);
    setError(null);
    try {
      await onSubmit({ botName: botName.trim(), strategyId, symbol });
      onOpenChange(false);
    } catch {
      setError("Không thể tạo Bot. Vui lòng thử lại.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Tạo Bot mới</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          {/* Bot Name */}
          <div className="grid gap-2">
            <Label htmlFor="bot-name">Tên Bot</Label>
            <Input
              id="bot-name"
              placeholder="VD: BTC Scalper"
              value={botName}
              onChange={(e) => setBotName(e.target.value)}
              disabled={isSubmitting}
              autoFocus
            />
          </div>

          {/* Strategy */}
          <div className="grid gap-2">
            <Label htmlFor="strategy-select">Chiến lược</Label>
            <Select
              value={strategyId}
              onValueChange={setStrategyId}
              disabled={isSubmitting}
            >
              <SelectTrigger id="strategy-select">
                <SelectValue placeholder="Chọn chiến lược..." />
              </SelectTrigger>
              <SelectContent>
                {strategies.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Symbol */}
          <div className="grid gap-2">
            <Label htmlFor="symbol-select">Cặp tiền</Label>
            <Select
              value={symbol}
              onValueChange={setSymbol}
              disabled={isSubmitting}
            >
              <SelectTrigger id="symbol-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SYMBOLS.map((s) => (
                  <SelectItem key={s} value={s}>
                    {s}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Error */}
          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isSubmitting}
          >
            Hủy
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Đang tạo...
              </>
            ) : (
              "Khởi chạy"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
