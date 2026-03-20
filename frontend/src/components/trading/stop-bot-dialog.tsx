// ===================================================================
// QuantFlow — Stop Bot Dialog
// Task 3.3.3 — Bot Management Panel
// ===================================================================
//
// UX Flow (frontend_flows.md §3.2.4):
//   Click ⏹ → Confirmation Dialog:
//     "Bạn muốn xử lý Bot [Tên] như thế nào?"
//     ○ Dừng Bot & Đóng vị thế (Market Close)
//     ○ Chỉ dừng Bot (Giữ nguyên vị thế)
//     [Hủy] [Xác nhận dừng]
// ===================================================================

"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Loader2, AlertTriangle } from "lucide-react";

// -----------------------------------------------------------------
// Types
// -----------------------------------------------------------------

interface StopBotDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  botName: string;
  botId: string;
  onConfirm: (botId: string, closePosition: boolean) => Promise<void>;
}

// -----------------------------------------------------------------
// Component
// -----------------------------------------------------------------

export function StopBotDialog({
  open,
  onOpenChange,
  botName,
  botId,
  onConfirm,
}: StopBotDialogProps) {
  const [closePosition, setClosePosition] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleConfirm = async () => {
    setIsSubmitting(true);
    try {
      await onConfirm(botId, closePosition);
      onOpenChange(false);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-warning" />
            Dừng Bot
          </DialogTitle>
          <DialogDescription>
            Bạn muốn xử lý Bot <strong>{botName}</strong> như thế nào?
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 py-4">
          {/* Option 1: Stop and close position */}
          <label
            htmlFor="stop-close"
            className={`
              flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors
              ${closePosition
                ? "border-primary bg-primary/5"
                : "border-border hover:border-muted-foreground/30"
              }
            `}
          >
            <input
              type="radio"
              id="stop-close"
              name="stop-option"
              checked={closePosition}
              onChange={() => setClosePosition(true)}
              className="mt-0.5"
            />
            <div>
              <Label htmlFor="stop-close" className="cursor-pointer font-medium">
                Dừng Bot &amp; Đóng vị thế
              </Label>
              <p className="text-xs text-muted-foreground mt-1">
                Dừng Bot, đóng tất cả vị thế đang mở và hủy tất cả lệnh chờ
                trên sàn Binance (Market Close).
              </p>
            </div>
          </label>

          {/* Option 2: Stop only */}
          <label
            htmlFor="stop-keep"
            className={`
              flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors
              ${!closePosition
                ? "border-primary bg-primary/5"
                : "border-border hover:border-muted-foreground/30"
              }
            `}
          >
            <input
              type="radio"
              id="stop-keep"
              name="stop-option"
              checked={!closePosition}
              onChange={() => setClosePosition(false)}
              className="mt-0.5"
            />
            <div>
              <Label htmlFor="stop-keep" className="cursor-pointer font-medium">
                Chỉ dừng Bot
              </Label>
              <p className="text-xs text-muted-foreground mt-1">
                Dừng Bot nhưng giữ nguyên vị thế hiện tại trên sàn. Bạn có thể
                khởi động lại Bot sau.
              </p>
            </div>
          </label>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isSubmitting}
          >
            Hủy
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={isSubmitting}
          >
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Đang xử lý...
              </>
            ) : (
              "Xác nhận dừng"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
