// ===================================================================
// QuantFlow — Backtest Panel
// Task 3.4.1 + 3.4.2 — Backtest Panel Container (3 states)
// ===================================================================
//
// States (frontend_flows.md §3.2.5):
//   1. Config Form — BacktestConfigForm (Task 3.4.1)
//   2. Running — Progress bar + Cancel
//   3. Results — Stats + Equity Curve (Task 3.4.2)
//
// SRS: UC-06
// ===================================================================

"use client";

import { useState, useCallback, useRef } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { BacktestConfigForm } from "@/components/trading/backtest-config-form";
import {
  BacktestResultDisplay,
  type BacktestResultData,
} from "@/components/trading/backtest-result-display";
import { backtestApi } from "@/lib/api-client";
import { toast } from "sonner";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

type BacktestState = "config" | "running" | "completed" | "failed";

// -----------------------------------------------------------------
// Mock result generator
// -----------------------------------------------------------------

function generateMockResult(): BacktestResultData {
  const totalPnl = Math.round((Math.random() * 800 - 200) * 100) / 100;
  const winRate = Math.round((40 + Math.random() * 30) * 10) / 10;
  const maxDrawdown = -Math.round((3 + Math.random() * 12) * 10) / 10;
  const profitFactor = Math.round((0.8 + Math.random() * 1.5) * 100) / 100;
  const totalTrades = Math.floor(50 + Math.random() * 200);

  // Generate equity curve
  const equityCurve: { time: string; equity: number }[] = [];
  let equity = 1000;
  const now = Date.now();
  const dayMs = 86400000;

  for (let i = 0; i < 60; i++) {
    const change = (Math.random() - 0.45) * 20;
    equity += change;
    equityCurve.push({
      time: new Date(now - (60 - i) * dayMs).toISOString(),
      equity: Math.round(equity * 100) / 100,
    });
  }

  return { totalPnl, winRate, maxDrawdown, profitFactor, totalTrades, equityCurve };
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function BacktestPanel() {
  const [state, setState] = useState<BacktestState>("config");
  const [backtestId, setBacktestId] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<BacktestResultData | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // ------- Start backtest -------
  const handleSubmit = useCallback((id: string) => {
    setBacktestId(id);
    setState("running");
    setProgress(0);
    setResult(null);

    // Mock progress simulation
    let p = 0;
    intervalRef.current = setInterval(() => {
      p += Math.random() * 15 + 5;
      if (p >= 100) {
        p = 100;
        if (intervalRef.current) clearInterval(intervalRef.current);

        // Generate mock result
        setResult(generateMockResult());
        setState("completed");
      }
      setProgress(Math.min(Math.round(p), 100));
    }, 800);
  }, []);

  // ------- Cancel -------
  const handleCancel = useCallback(async () => {
    if (intervalRef.current) clearInterval(intervalRef.current);
    if (backtestId) {
      try {
        await backtestApi.cancel(backtestId);
      } catch {
        // Mock: just reset
      }
    }
    toast.info("Đã hủy phiên Backtest.");
    setState("config");
    setBacktestId(null);
    setProgress(0);
    setResult(null);
  }, [backtestId]);

  // ------- Rerun (keep config) -------
  const handleRerun = useCallback(() => {
    if (backtestId) {
      setState("running");
      setProgress(0);
      setResult(null);

      let p = 0;
      intervalRef.current = setInterval(() => {
        p += Math.random() * 15 + 5;
        if (p >= 100) {
          p = 100;
          if (intervalRef.current) clearInterval(intervalRef.current);
          setResult(generateMockResult());
          setState("completed");
        }
        setProgress(Math.min(Math.round(p), 100));
      }, 800);
    }
  }, [backtestId]);

  // ------- Reset to config -------
  const handleReset = useCallback(() => {
    if (intervalRef.current) clearInterval(intervalRef.current);
    setState("config");
    setBacktestId(null);
    setProgress(0);
    setResult(null);
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

  // ------- Fallback -------
  return (
    <div className="flex items-center justify-center h-full">
      <p className="text-sm text-muted-foreground">Lỗi không xác định.</p>
    </div>
  );
}
