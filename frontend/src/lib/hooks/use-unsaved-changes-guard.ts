// ===================================================================
// QuantFlow — useUnsavedChangesGuard Hook
// Task 3.2.10 — Navigate-away protection (multiple dirty tabs)
// ===================================================================
//
// 2 layers of protection:
//   1. beforeunload — native browser prompt on tab close / refresh
//   2. In-app navigation — intercept back button / sidebar links
//      via requestNavigateAway(targetPath) → show dialog if dirty
//
// Usage in EditorShell:
//   const guard = useUnsavedChangesGuard();
//   <UnsavedChangesDialog {...guard.dialogProps} />
//   <EditorControlBar onBack={() => guard.requestNavigateAway("/strategies")} />
// ===================================================================

"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useEditorStore } from "@/store/editor-store";

// -----------------------------------------------------------------
// Hook
// -----------------------------------------------------------------

export function useUnsavedChangesGuard() {
  const router = useRouter();
  const tabs = useEditorStore((s) => s.tabs);

  // Derive dirty tab info
  const dirtyTabs = tabs.filter((t) => t.isDirty);
  const hasDirtyTabs = dirtyTabs.length > 0;
  const dirtyTabNames = dirtyTabs.map((t) => t.name);

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [pendingPath, setPendingPath] = useState<string | null>(null);

  // ---------------------------------------------------------------
  // Layer 1: beforeunload — browser close / refresh protection
  // ---------------------------------------------------------------
  useEffect(() => {
    if (!hasDirtyTabs) return;

    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };

    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [hasDirtyTabs]);

  // ---------------------------------------------------------------
  // Layer 2: In-app navigation interception
  // ---------------------------------------------------------------

  /**
   * Call this instead of router.push() when leaving the editor.
   * If dirty tabs exist, shows the dialog. Otherwise navigates directly.
   */
  const requestNavigateAway = useCallback(
    (targetPath: string) => {
      if (hasDirtyTabs) {
        setPendingPath(targetPath);
        setDialogOpen(true);
      } else {
        router.push(targetPath);
      }
    },
    [hasDirtyTabs, router],
  );

  /** User confirmed leaving — navigate to pending path */
  const confirmLeave = useCallback(() => {
    setDialogOpen(false);
    if (pendingPath) {
      router.push(pendingPath);
      setPendingPath(null);
    }
  }, [pendingPath, router]);

  /** User cancelled — stay on editor */
  const cancelLeave = useCallback(() => {
    setDialogOpen(false);
    setPendingPath(null);
  }, []);

  return {
    /** Props to spread onto UnsavedChangesDialog */
    dialogProps: {
      open: dialogOpen,
      dirtyTabNames,
      onLeave: confirmLeave,
      onStay: cancelLeave,
    },
    /** Call this to request navigation — shows dialog if dirty */
    requestNavigateAway,
  };
}
