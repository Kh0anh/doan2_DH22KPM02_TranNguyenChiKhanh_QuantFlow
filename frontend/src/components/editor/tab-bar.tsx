/**
 * [3.2.7] TabBar — Horizontal tab strip for the Multi-tab Strategy Editor.
 * [3.2.9] Wired CloseTabDialog for isDirty close-tab warning.
 *
 * Visual (frontend_flows.md §3.4.2):
 *   ┌─────────────────────────────────────────────────────────────┐
 *   │ [EMA Crossover 15m ● ✕] [RSI Reversal ✕] [Tạo mới * ✕] [+] │
 *   └─────────────────────────────────────────────────────────────┘
 *
 * Features:
 *   - Renders a list of TabItem components + [+] new-tab button
 *   - Horizontal scroll when tabs overflow viewport
 *   - Keyboard shortcuts: Ctrl+Tab (next), Ctrl+W (close active)
 *   - isDirty close-tab warning dialog (3 actions)
 *   - Consumes state directly from useEditorStore (Zustand)
 */
"use client";

import { useCallback, useEffect, useState } from "react";
import { Plus } from "lucide-react";
import { TabItem } from "./tab-item";
import { CloseTabDialog } from "./close-tab-dialog";
import { useEditorStore } from "@/store/editor-store";
import { useEditorTab } from "@/lib/hooks/use-editor-tab";
import type * as Blockly from "blockly";

// ---------------------------------------------------------------------------
// TabBar — main export
// ---------------------------------------------------------------------------

interface TabBarProps {
  /** Workspace registry ref for save-on-close (passed from EditorShell) */
  workspacesRef?: React.RefObject<Map<string, Blockly.WorkspaceSvg>>;
}

export function TabBar({ workspacesRef }: TabBarProps) {
  const {
    tabs,
    activeTabId,
    setActiveTab,
    closeTab,
    openNewTab,
  } = useEditorStore();

  // [3.2.9] CloseTabDialog state
  const [pendingCloseTabId, setPendingCloseTabId] = useState<string | null>(null);
  const pendingTab = pendingCloseTabId
    ? tabs.find((t) => t.id === pendingCloseTabId) ?? null
    : null;

  // [3.2.9] Save strategy hook (for "Lưu & Đóng" in CloseTabDialog)
  const { saveStrategy } = useEditorTab(
    workspacesRef ?? { current: new Map() },
  );

  // ------------------------------------------------------------------
  // [3.2.9] Request close — intercepts isDirty tabs
  // ------------------------------------------------------------------
  const requestClose = useCallback(
    (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId);
      if (tab?.isDirty) {
        // Show CloseTabDialog instead of closing directly
        setPendingCloseTabId(tabId);
      } else {
        // Clean tab — close immediately
        closeTab(tabId);
      }
    },
    [tabs, closeTab],
  );

  // ------------------------------------------------------------------
  // [3.2.9] CloseTabDialog action handlers
  // ------------------------------------------------------------------
  const handleSaveAndClose = useCallback(async () => {
    if (!pendingCloseTabId) return;
    await saveStrategy(pendingCloseTabId);
    // Check if save succeeded (tab should now be clean)
    const tab = useEditorStore.getState().tabs.find((t) => t.id === pendingCloseTabId);
    if (tab && !tab.isDirty) {
      closeTab(pendingCloseTabId);
    }
    // If still dirty (save failed), keep dialog open → user sees toast error
    setPendingCloseTabId(null);
  }, [pendingCloseTabId, saveStrategy, closeTab]);

  const handleDiscardAndClose = useCallback(() => {
    if (!pendingCloseTabId) return;
    closeTab(pendingCloseTabId);
    setPendingCloseTabId(null);
  }, [pendingCloseTabId, closeTab]);

  const handleCancelClose = useCallback(() => {
    setPendingCloseTabId(null);
  }, []);

  // ------------------------------------------------------------------
  // Keyboard shortcuts: Ctrl+Tab (next tab), Ctrl+W (close active)
  // ------------------------------------------------------------------
  const handleKeyboard = useCallback(
    (e: KeyboardEvent) => {
      // Ctrl+Tab — cycle to next tab
      if ((e.ctrlKey || e.metaKey) && e.key === "Tab") {
        e.preventDefault();
        if (tabs.length <= 1 || !activeTabId) return;
        const currentIndex = tabs.findIndex((t) => t.id === activeTabId);
        const nextIndex = (currentIndex + 1) % tabs.length;
        setActiveTab(tabs[nextIndex].id);
        return;
      }

      // Ctrl+W — close active tab (with isDirty check)
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "w") {
        e.preventDefault();
        if (activeTabId) {
          requestClose(activeTabId);
        }
        return;
      }
    },
    [tabs, activeTabId, setActiveTab, requestClose],
  );

  useEffect(() => {
    document.addEventListener("keydown", handleKeyboard);
    return () => document.removeEventListener("keydown", handleKeyboard);
  }, [handleKeyboard]);

  // ------------------------------------------------------------------
  // Render
  // ------------------------------------------------------------------
  return (
    <>
      <div
        role="tablist"
        aria-label="Các tab chiến lược"
        className="flex h-9 shrink-0 items-stretch overflow-x-auto border-b border-border bg-secondary scrollbar-none"
      >
        {/* Tab items */}
        {tabs.map((tab) => (
          <TabItem
            key={tab.id}
            tab={tab}
            isActive={tab.id === activeTabId}
            onSelect={() => setActiveTab(tab.id)}
            onClose={() => requestClose(tab.id)}
          />
        ))}

        {/* New tab [+] button */}
        <button
          type="button"
          aria-label="Mở tab mới"
          onClick={openNewTab}
          className="flex h-full shrink-0 items-center px-2.5 text-muted-foreground hover:bg-background/60 hover:text-foreground transition-colors"
        >
          <Plus className="size-3.5" />
        </button>
      </div>

      {/* [3.2.9] CloseTabDialog — shown when closing a dirty tab */}
      <CloseTabDialog
        open={pendingCloseTabId !== null}
        strategyName={pendingTab?.name ?? ""}
        onSaveAndClose={handleSaveAndClose}
        onDiscardAndClose={handleDiscardAndClose}
        onCancel={handleCancelClose}
      />
    </>
  );
}
