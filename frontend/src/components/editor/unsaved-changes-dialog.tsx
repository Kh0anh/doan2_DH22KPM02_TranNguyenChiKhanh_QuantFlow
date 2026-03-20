/**
 * [3.2.10] UnsavedChangesDialog — Navigate-away protection for dirty tabs.
 *
 * Shown when user tries to leave the Editor page while ≥1 tab has
 * unsaved changes (isDirty=true).
 *
 * Different from CloseTabDialog (3.2.9) which handles closing a SINGLE tab.
 * This dialog handles leaving the ENTIRE editor with MULTIPLE dirty tabs.
 *
 * Visual:
 *   ┌─────────────────────────────────────────────────────────┐
 *   │  ⚠ Rời khỏi Editor?                                    │
 *   │                                                         │
 *   │  Bạn có N chiến lược chưa được lưu:                    │
 *   │    • EMA Crossover 15m                                  │
 *   │    • RSI Reversal                                       │
 *   │  Các thay đổi sẽ bị mất nếu bạn rời đi.               │
 *   │                                                         │
 *   │                        [Ở lại]    [Rời đi]              │
 *   └─────────────────────────────────────────────────────────┘
 *
 * 2 Actions:
 *   - "Rời đi": Discard all unsaved changes → navigate away
 *   - "Ở lại": Cancel navigation → stay on editor
 */
"use client";

import { AlertTriangle } from "lucide-react";
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

interface UnsavedChangesDialogProps {
  /** Whether the dialog is currently open */
  open: boolean;
  /** Names of tabs with unsaved changes */
  dirtyTabNames: string[];
  /** Called when user confirms leaving — discard all changes */
  onLeave: () => void;
  /** Called when user cancels — stay on editor */
  onStay: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function UnsavedChangesDialog({
  open,
  dirtyTabNames,
  onLeave,
  onStay,
}: UnsavedChangesDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onStay()}>
      <DialogContent showCloseButton={false} className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="size-5 text-[#FFAB40]" />
            Rời khỏi Editor?
          </DialogTitle>
          <DialogDescription asChild>
            <div>
              <p>
                Bạn có {dirtyTabNames.length} chiến lược chưa được lưu:
              </p>
              <ul className="mt-2 list-disc pl-5 space-y-0.5">
                {dirtyTabNames.map((name) => (
                  <li key={name} className="text-sm text-foreground">
                    {name}
                  </li>
                ))}
              </ul>
              <p className="mt-2">
                Các thay đổi sẽ bị mất nếu bạn rời đi.
              </p>
            </div>
          </DialogDescription>
        </DialogHeader>

        <DialogFooter className="gap-2 sm:gap-2">
          {/* Stay — keep editing */}
          <Button variant="outline" size="sm" onClick={onStay}>
            Ở lại
          </Button>

          {/* Leave — discard all and navigate */}
          <Button
            variant="destructive"
            size="sm"
            onClick={onLeave}
          >
            Rời đi
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
