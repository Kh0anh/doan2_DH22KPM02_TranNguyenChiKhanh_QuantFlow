/**
 * [3.2.9] CloseTabDialog — Warning dialog when closing a dirty tab.
 *
 * Triggered when user tries to close a tab with isDirty=true.
 *
 * Visual (frontend_flows.md §3.4.3):
 *   ┌─────────────────────────────────────────────────────────┐
 *   │  ⚠ Lưu thay đổi?                                       │
 *   │                                                         │
 *   │  Chiến lược "[name]" có thay đổi chưa được lưu.        │
 *   │  Bạn muốn làm gì?                                      │
 *   │                                                         │
 *   │  [Không lưu & Đóng]    [Hủy]    [Lưu & Đóng]           │
 *   └─────────────────────────────────────────────────────────┘
 *
 * 3 Actions:
 *   - "Lưu & Đóng": Save API → markClean → close tab
 *   - "Không lưu & Đóng": Discard changes → close tab immediately
 *   - "Hủy": Close dialog, keep tab open
 */
"use client";

import { useState } from "react";
import { AlertTriangle, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface CloseTabDialogProps {
  /** Whether the dialog is currently open */
  open: boolean;
  /** Name of the strategy being closed (shown in the message) */
  strategyName: string;
  /** Called when user clicks "Lưu & Đóng" — should save then close */
  onSaveAndClose: () => Promise<void>;
  /** Called when user clicks "Không lưu & Đóng" — discard and close */
  onDiscardAndClose: () => void;
  /** Called when user clicks "Hủy" or closes dialog — keep tab open */
  onCancel: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CloseTabDialog({
  open,
  strategyName,
  onSaveAndClose,
  onDiscardAndClose,
  onCancel,
}: CloseTabDialogProps) {
  const [isSaving, setIsSaving] = useState(false);

  const handleSaveAndClose = async () => {
    setIsSaving(true);
    try {
      await onSaveAndClose();
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onCancel()}>
      <DialogContent showCloseButton={false} className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="size-5 text-[#FFAB40]" />
            Lưu thay đổi?
          </DialogTitle>
          <DialogDescription>
            Chiến lược &quot;{strategyName}&quot; có thay đổi chưa được lưu.
            Bạn muốn làm gì?
          </DialogDescription>
        </DialogHeader>

        <DialogFooter className="gap-2 sm:gap-2">
          {/* Destructive — discard changes */}
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive hover:bg-destructive/10"
            onClick={onDiscardAndClose}
            disabled={isSaving}
          >
            Không lưu &amp; Đóng
          </Button>

          {/* Spacer between destructive and safe actions */}
          <div className="flex-1 hidden sm:block" />

          {/* Cancel — keep tab open */}
          <Button
            variant="outline"
            size="sm"
            onClick={onCancel}
            disabled={isSaving}
          >
            Hủy
          </Button>

          {/* Primary — save then close */}
          <Button
            variant="default"
            size="sm"
            onClick={handleSaveAndClose}
            disabled={isSaving}
          >
            {isSaving && <Loader2 className="mr-1.5 size-3.5 animate-spin" />}
            Lưu &amp; Đóng
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
