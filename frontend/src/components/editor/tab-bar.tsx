/**
 * [3.2.7] TabBar — Horizontal tab strip for the Multi-tab Strategy Editor.
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
 *   - Consumes state directly from useEditorStore (Zustand)
 */
"use client";

import { useCallback, useEffect } from "react";
import { Plus } from "lucide-react";
import { TabItem } from "./tab-item";
import { useEditorStore } from "@/store/editor-store";

// ---------------------------------------------------------------------------
// TabBar — main export
// ---------------------------------------------------------------------------

export function TabBar() {
  const {
    tabs,
    activeTabId,
    setActiveTab,
    closeTab,
    openNewTab,
  } = useEditorStore();

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

      // Ctrl+W — close active tab
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "w") {
        e.preventDefault();
        if (activeTabId) {
          closeTab(activeTabId);
        }
        return;
      }
    },
    [tabs, activeTabId, setActiveTab, closeTab],
  );

  useEffect(() => {
    document.addEventListener("keydown", handleKeyboard);
    return () => document.removeEventListener("keydown", handleKeyboard);
  }, [handleKeyboard]);

  // ------------------------------------------------------------------
  // Render
  // ------------------------------------------------------------------
  return (
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
          onClose={() => closeTab(tab.id)}
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
  );
}
