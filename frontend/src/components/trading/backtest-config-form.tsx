// ===================================================================
// QuantFlow — Backtest Configuration Form
// Task 3.4.1 — Backtest Configuration Form
// ===================================================================
//
// Layout (frontend_flows.md §3.2.5 — State 1):
//   ┌───────────────────────────────────────────────────────────────┐
//   │  Chiến lược: [Dropdown ▼      ]    Symbol:   [BTCUSDT ▼]      │
//   │  Từ ngày:    [📅 2024-01-01]       Đến ngày: [📅 2024-12-31]  │
//   │  Vốn ban đầu:[1000 USDT]           Phí GD:   [0.04 %]         │
//   │  Unit/Ses:   [1000]                                            │
//   │               [======  Bắt đầu Backtest  ======]               │
//   └───────────────────────────────────────────────────────────────┘
//
// API: POST /backtests — CreateBacktestRequest
// SRS: UC-06
// ===================================================================

"use client";

import { useState, useEffect, useCallback } from "react";
import { Play, Loader2 } from "lucide-react";
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
import {
  backtestApi,
  strategyApi,
  type CreateBacktestParams,
} from "@/lib/api-client";
import { toast } from "sonner";

// -----------------------------------------------------------------
// Constants
// -----------------------------------------------------------------

const SYMBOLS = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

// Default date range: past 6 months
const now = new Date();
const sixMonthsAgo = new Date(now);
sixMonthsAgo.setMonth(sixMonthsAgo.getMonth() - 6);

const DEFAULT_START = sixMonthsAgo.toISOString().slice(0, 10);
const DEFAULT_END = now.toISOString().slice(0, 10);


// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

interface BacktestConfigFormProps {
  onSubmit: (backtestId: string) => void;
  isRunning: boolean;
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function BacktestConfigForm({
  onSubmit,
  isRunning,
}: BacktestConfigFormProps) {
  // ------- Strategies dropdown -------
  const [strategies, setStrategies] = useState<{ id: string; name: string }[]>(
    [],
  );

  useEffect(() => {
    async function fetchStrategies() {
      try {
        const res = await strategyApi.list({ limit: 100 });
        // Only show Valid strategies — Draft strategies cannot be backtested
        const validStrategies = res.data
          .filter((s: { status: string }) => s.status === "Valid")
          .map(
            (s: { id: string; name: string }) => ({
              id: s.id,
              name: s.name,
            }),
          );
        setStrategies(validStrategies);
      } catch {
        setStrategies([]);
      }
    }
    fetchStrategies();
  }, []);

  // ------- Form state -------
  const [strategyId, setStrategyId] = useState("");
  const [symbol, setSymbol] = useState("BTCUSDT");
  const [startDate, setStartDate] = useState(DEFAULT_START);
  const [endDate, setEndDate] = useState(DEFAULT_END);
  const [initialCapital, setInitialCapital] = useState("1000");
  const [feeRate, setFeeRate] = useState("0.04");
  const [maxUnit, setMaxUnit] = useState("1000");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState("");

  // ------- Submit -------
  const handleSubmit = useCallback(async () => {
    setError("");

    // Validation
    if (!strategyId) {
      setError("Vui lòng chọn chiến lược.");
      return;
    }
    if (!startDate || !endDate) {
      setError("Vui lòng chọn khoảng thời gian.");
      return;
    }
    if (new Date(startDate) >= new Date(endDate)) {
      setError("Ngày bắt đầu phải trước ngày kết thúc.");
      return;
    }
    const cap = parseFloat(initialCapital);
    if (isNaN(cap) || cap <= 0) {
      setError("Vốn ban đầu phải lớn hơn 0.");
      return;
    }
    const fee = parseFloat(feeRate);
    if (isNaN(fee) || fee < 0) {
      setError("Phí giao dịch không hợp lệ.");
      return;
    }

    setIsSubmitting(true);
    const params: CreateBacktestParams = {
      strategy_id: strategyId,
      symbol,
      start_time: new Date(startDate).toISOString(),
      end_time: new Date(endDate + "T23:59:59").toISOString(),
      initial_capital: cap,
      fee_rate: fee,
      max_unit: parseInt(maxUnit) || 1000,
    };

    try {
      const result = await backtestApi.create(params);
      toast.success("Phiên Backtest đã được khởi tạo!");
      onSubmit(result.backtest_id);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Lỗi không xác định.";
      toast.error(`Khởi tạo Backtest thất bại: ${msg}`);
      setError(msg);
    } finally {
      setIsSubmitting(false);
    }
  }, [
    strategyId,
    symbol,
    startDate,
    endDate,
    initialCapital,
    feeRate,
    maxUnit,
    onSubmit,
  ]);

  const disabled = isRunning || isSubmitting;

  return (
    <div className="p-4 space-y-4 max-w-2xl mx-auto">
      {/* Row 1: Strategy + Symbol */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="bt-strategy" className="text-xs">
            Chiến lược
          </Label>
          <Select
            value={strategyId}
            onValueChange={setStrategyId}
            disabled={disabled}
          >
            <SelectTrigger id="bt-strategy" className="h-8 text-xs">
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

        <div className="space-y-1.5">
          <Label htmlFor="bt-symbol" className="text-xs">
            Symbol
          </Label>
          <Select
            value={symbol}
            onValueChange={setSymbol}
            disabled={disabled}
          >
            <SelectTrigger id="bt-symbol" className="h-8 text-xs">
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
      </div>

      {/* Row 2: Start Date + End Date */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="bt-start" className="text-xs">
            Từ ngày
          </Label>
          <Input
            id="bt-start"
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            disabled={disabled}
            className="h-8 text-xs"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="bt-end" className="text-xs">
            Đến ngày
          </Label>
          <Input
            id="bt-end"
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            disabled={disabled}
            className="h-8 text-xs"
          />
        </div>
      </div>

      {/* Row 3: Initial Capital + Fee Rate */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="bt-capital" className="text-xs">
            Vốn ban đầu (USDT)
          </Label>
          <Input
            id="bt-capital"
            type="number"
            min="1"
            step="100"
            value={initialCapital}
            onChange={(e) => setInitialCapital(e.target.value)}
            disabled={disabled}
            className="h-8 text-xs font-mono"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="bt-fee" className="text-xs">
            Phí GD (%)
          </Label>
          <Input
            id="bt-fee"
            type="number"
            min="0"
            step="0.01"
            value={feeRate}
            onChange={(e) => setFeeRate(e.target.value)}
            disabled={disabled}
            className="h-8 text-xs font-mono"
          />
        </div>
      </div>

      {/* Row 4: Max Unit */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="bt-unit" className="text-xs">
            Unit/Session
          </Label>
          <Input
            id="bt-unit"
            type="number"
            min="1"
            step="100"
            value={maxUnit}
            onChange={(e) => setMaxUnit(e.target.value)}
            disabled={disabled}
            className="h-8 text-xs font-mono"
          />
        </div>
      </div>

      {/* Error */}
      {error && (
        <p className="text-xs text-danger">{error}</p>
      )}

      {/* Submit */}
      <Button
        className="w-full h-9 gap-2"
        onClick={handleSubmit}
        disabled={disabled}
      >
        {isSubmitting ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin" />
            Đang khởi tạo...
          </>
        ) : (
          <>
            <Play className="h-4 w-4" />
            Bắt đầu Backtest
          </>
        )}
      </Button>
    </div>
  );
}
