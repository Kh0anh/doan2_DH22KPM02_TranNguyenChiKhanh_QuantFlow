// ===================================================================
// QuantFlow — Backtest Panel
// Task 3.4.1 — Backtest Panel Container (3 states)
// ===================================================================
//
// States (frontend_flows.md §3.2.5):
//   1. Config Form — BacktestConfigForm (this task)
//   2. Running — Progress bar + Cancel (Task 3.4.2)
//   3. Results — Stats + Equity Curve (Task 3.4.3)
//
// SRS: UC-06
// ===================================================================

"use client";

import { useState, useCallback } from "react";
import { FlaskConical, Loader2, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { BacktestConfigForm } from "@/components/trading/backtest-config-form";
import { backtestApi } from "@/lib/api-client";
import { toast } from "sonner";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

type BacktestState = "config" | "running" | "completed" | "failed";

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function BacktestPanel() {
  const [state, setState] = useState<BacktestState>("config");
  const [backtestId, setBacktestId] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);

  // ------- Start backtest -------
  const handleSubmit = useCallback((id: string) => {
    setBacktestId(id);
    setState("running");
    setProgress(0);

    // Mock progress simulation (replace with polling in Task 3.4.2)
    let p = 0;
    const interval = setInterval(() => {
      p += Math.random() * 15 + 5;
      if (p >= 100) {
        p = 100;
        clearInterval(interval);
        setState("completed");
      }
      setProgress(Math.min(Math.round(p), 100));
    }, 800);
  }, []);

  // ------- Cancel -------
  const handleCancel = useCallback(async () => {
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
  }, [backtestId]);

  // ------- Reset to config -------
  const handleReset = useCallback(() => {
    setState("config");
    setBacktestId(null);
    setProgress(0);
  }, []);

  // ------- State 1: Config Form -------
  if (state === "config") {
    return (
      <div className="h-full overflow-y-auto">
        <BacktestConfigForm
          onSubmit={handleSubmit}
          isRunning={false}
        />
      </div>
    );
  }

  // ------- State 2: Running (placeholder — Task 3.4.2) -------
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

  // ------- State 3: Completed (placeholder — Task 3.4.3) -------
  return (
    <div className="flex flex-col items-center justify-center h-full gap-4">
      <FlaskConical className="h-8 w-8 text-success" />
      <p className="text-sm text-foreground font-medium">
        Backtest hoàn tất!
      </p>
      <p className="text-xs text-muted-foreground">
        📊 Kết quả chi tiết — Task 3.4.2 / 3.4.3
      </p>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" onClick={handleReset}>
          <RotateCcw className="h-3 w-3 mr-1" />
          Quay về cấu hình
        </Button>
      </div>
    </div>
  );
}
