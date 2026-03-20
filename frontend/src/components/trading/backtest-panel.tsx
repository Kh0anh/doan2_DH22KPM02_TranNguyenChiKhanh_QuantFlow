// ===================================================================
// QuantFlow — Backtest Panel
// Task 3.4.1 + 3.4.2 — Backtest Panel Container (3 states)
// ===================================================================
//
// States (frontend_flows.md §3.2.5):
//   1. Config Form — BacktestConfigForm (Task 3.4.1)
//   2. Running — Progress bar + Cancel (polls GET /backtests/{id})
//   3. Results — Stats + Equity Curve (Task 3.4.2)
//
// SRS: UC-06
// ===================================================================

"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { BacktestConfigForm } from "@/components/trading/backtest-config-form";
import {
  BacktestResultDisplay,
  type BacktestResultData,
} from "@/components/trading/backtest-result-display";
import { backtestApi, type BacktestResultResponse } from "@/lib/api-client";
import { toast } from "sonner";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

type BacktestState = "config" | "running" | "completed" | "failed";

// -----------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------

/** Convert the backend BacktestResultResponse (completed) to the shape
 *  expected by BacktestResultDisplay. Backend sends Decimal values as
 *  strings (shopspring/decimal JSON serialisation). */
function mapResultData(res: BacktestResultResponse): BacktestResultData {
  const s = res.summary;
  return {
    totalPnl: s ? parseFloat(s.total_pnl) : 0,
    winRate: s ? parseFloat(s.win_rate) : 0,
    maxDrawdown: s ? parseFloat(s.max_drawdown_percent) : 0,
    profitFactor: s ? parseFloat(s.profit_factor) : 0,
    totalTrades: s?.total_trades ?? 0,
    equityCurve: (res.equity_curve ?? []).map((pt) => ({
      time: pt.timestamp,
      equity: parseFloat(pt.equity),
    })),
  };
}

// Polling interval in milliseconds
const POLL_INTERVAL_MS = 2000;

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function BacktestPanel() {
  const [state, setState] = useState<BacktestState>("config");
  const [backtestId, setBacktestId] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<BacktestResultData | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  // ------- Poll backtest status -------
  const startPolling = useCallback((id: string) => {
    // Clear any existing interval
    if (intervalRef.current) clearInterval(intervalRef.current);

    intervalRef.current = setInterval(async () => {
      try {
        const res = await backtestApi.getResult(id);

        if (res.status === "processing") {
          setProgress(res.progress ?? 0);
          return; // keep polling
        }

        // Terminal state — stop polling
        if (intervalRef.current) clearInterval(intervalRef.current);

        if (res.status === "completed") {
          setProgress(100);
          setResult(mapResultData(res));
          setState("completed");
        } else if (res.status === "failed") {
          const msg = res.error_message ?? "Lỗi không xác định.";
          setErrorMsg(msg);
          toast.error(`Backtest thất bại: ${msg}`);
          setState("failed");
        } else {
          // canceled
          toast.info("Phiên Backtest đã bị hủy.");
          setState("config");
          setBacktestId(null);
          setProgress(0);
        }
      } catch (err) {
        if (intervalRef.current) clearInterval(intervalRef.current);
        const msg =
          err instanceof Error ? err.message : "Lỗi không xác định.";
        setErrorMsg(msg);
        toast.error(`Backtest lỗi: ${msg}`);
        setState("failed");
      }
    }, POLL_INTERVAL_MS);
  }, []);

  // ------- Start backtest (called by config form with backtest_id) -------
  const handleSubmit = useCallback(
    (id: string) => {
      setBacktestId(id);
      setState("running");
      setProgress(0);
      setResult(null);
      setErrorMsg(null);
      startPolling(id);
    },
    [startPolling],
  );

  // ------- Cancel -------
  const handleCancel = useCallback(async () => {
    if (intervalRef.current) clearInterval(intervalRef.current);
    if (backtestId) {
      try {
        await backtestApi.cancel(backtestId);
        toast.info("Đã hủy phiên Backtest.");
      } catch {
        toast.error("Không thể hủy phiên Backtest.");
      }
    }
    setState("config");
    setBacktestId(null);
    setProgress(0);
    setResult(null);
  }, [backtestId]);

  // ------- Rerun (reset to config form) -------
  const handleRerun = useCallback(() => {
    if (intervalRef.current) clearInterval(intervalRef.current);
    setState("config");
    setProgress(0);
    setResult(null);
    setErrorMsg(null);
    // Keep backtestId so config form retains context
  }, []);

  // ------- Reset to config -------
  const handleReset = useCallback(() => {
    if (intervalRef.current) clearInterval(intervalRef.current);
    setState("config");
    setBacktestId(null);
    setProgress(0);
    setResult(null);
    setErrorMsg(null);
  }, []);

  // ------- State 1: Config Form -------
  if (state === "config") {
    return (
      <div className="h-full overflow-y-auto">
        <BacktestConfigForm onSubmit={handleSubmit} isRunning={false} />
      </div>
    );
  }

  // ------- State 2: Running -------
  if (state === "running") {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-4 px-8">
        <div className="flex items-center gap-2 text-sm text-foreground">
          <Loader2 className="h-4 w-4 animate-spin text-primary" />
          <span>Đang chạy mô phỏng...</span>
        </div>

        {/* Progress bar */}
        <div className="w-full max-w-md">
          <div className="h-2 w-full bg-muted rounded-full overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-300 rounded-full"
              style={{ width: `${progress}%` }}
            />
          </div>
          <div className="flex justify-between mt-1.5">
            <span className="text-xs text-muted-foreground">{progress}%</span>
            <span className="text-xs text-muted-foreground">
              ID: {backtestId?.slice(0, 8)}...
            </span>
          </div>
        </div>

        <Button variant="outline" size="sm" onClick={handleCancel}>
          Hủy
        </Button>
      </div>
    );
  }

  // ------- State 3: Completed — Task 3.4.2 -------
  if (state === "completed" && result) {
    return (
      <BacktestResultDisplay
        result={result}
        onRerun={handleRerun}
        onReset={handleReset}
      />
    );
  }

  // ------- State 4: Failed -------
  return (
    <div className="flex flex-col items-center justify-center h-full gap-3">
      <p className="text-sm text-danger">
        {errorMsg ?? "Lỗi không xác định."}
      </p>
      <Button variant="outline" size="sm" onClick={handleReset}>
        Quay về cấu hình
      </Button>
    </div>
  );
}
