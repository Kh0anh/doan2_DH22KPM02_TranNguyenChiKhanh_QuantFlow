/**
 * [3.2.1] EditorShell — Outer container for the Multi-tab Strategy Editor.
 * [3.2.4] Save/Load Strategy via API wired into this component.
 *
 * Architecture:
 *   ┌──────────────────────────────────────────────────────────┐
 *   │  Inline Tab Bar  (to be replaced by <TabBar/> in 3.2.7)  │
 *   ├──────────────────────────────────────────────────────────┤
 *   │  EditorControlBar (name, undo/redo, zoom, save, export)  │
 *   ├──────────────────────────────────────────────────────────┤
 *   │  Workspace Area (all tabs rendered simultaneously)        │
 *   │   Tab 0: display:flex  ← active                          │
 *   │   Tab 1: display:none  ← hidden (NOT unmounted)          │
 *   │   Tab 2: display:none  ← hidden                          │
 *   └──────────────────────────────────────────────────────────┘
 *
 * CSS display:none strategy:
 *   Blockly workspaces are NEVER unmounted between tab switches.
 *   Switching tabs only changes the CSS `display` property.
 *   Blockly.svgResize() is called inside BlocklyWorkspace itself when
 *   `isActive` changes (50ms delay to allow DOM paint).
 *
 * WorkspaceSvg registry:
 *   workspacesRef (Map<tabId, WorkspaceSvg>) stores each injected workspace.
 *   activeWorkspace state is derived from the registry when active tab changes.
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import * as Blockly from "blockly";
import {
  LayoutGrid,
  Plus,
  X,
  FileCode2,
} from "lucide-react";
import { toast } from "sonner";
import { EditorControlBar } from "./editor-control-bar";
import { useEditorStore } from "@/store/editor-store";
import { useEditorTab } from "@/lib/hooks/use-editor-tab";
import type { EditorTab } from "@/types";

// ---------------------------------------------------------------------------
// BlocklyWorkspace loaded with ssr:false — safe browser-only Blockly init
// ---------------------------------------------------------------------------
const BlocklyWorkspace = dynamic(() => import("./blockly-workspace"), {
  ssr: false,
  loading: () => (
    <div className="absolute inset-0 flex items-center justify-center bg-[#0D1117]">
      <div className="flex flex-col items-center gap-3 text-muted-foreground">
        <FileCode2 className="size-8 animate-pulse" />
        <p className="text-sm">Đang tải workspace...</p>
      </div>
    </div>
  ),
});

// ---------------------------------------------------------------------------
// Blockly WorkspaceSvg facade (method calls only — no Blockly import needed)
// ---------------------------------------------------------------------------
interface WorkspaceFacade {
  undo(redo: boolean): void;
  zoom(x: number, y: number, amount: number): void;
  zoomToFit(): void;
  scrollCenter(): void;
  getMetrics(): {
    viewLeft: number;
    viewTop: number;
    viewWidth: number;
    viewHeight: number;
  };
}

// ---------------------------------------------------------------------------
// Empty state — shown when no tabs are open
// ---------------------------------------------------------------------------
function EditorEmptyState({ onNewTab }: { onNewTab: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
      <div className="rounded-xl border border-border bg-secondary/50 p-6">
        <LayoutGrid className="mx-auto mb-3 size-10 text-muted-foreground" />
        <h3 className="mb-1 text-sm font-semibold text-foreground">
          Chưa có chiến lược nào được mở
        </h3>
        <p className="mb-4 text-xs text-muted-foreground">
          Mở một chiến lược từ danh sách hoặc tạo mới để bắt đầu thiết kế.
        </p>
        <button
          type="button"
          onClick={onNewTab}
          className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          <Plus className="size-3.5" />
          Tạo chiến lược mới
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Inline Tab Bar — minimal implementation; replaced by <TabBar/> in Task 3.2.7
// ---------------------------------------------------------------------------
interface InlineTabBarProps {
  tabs: EditorTab[];
  activeTabId: string | null;
  onTabSelect: (tabId: string) => void;
  onTabClose: (tabId: string) => void;
  onNewTab: () => void;
}

function InlineTabBar({
  tabs,
  activeTabId,
  onTabSelect,
  onTabClose,
  onNewTab,
}: InlineTabBarProps) {
  return (
    <div className="flex h-9 shrink-0 items-stretch overflow-x-auto border-b border-border bg-secondary scrollbar-none">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTabId;
        return (
          <div
            key={tab.id}
            role="tab"
            aria-selected={isActive}
            onClick={() => onTabSelect(tab.id)}
            className={[
              "group relative flex h-full min-w-0 max-w-48 shrink-0 cursor-pointer select-none items-center gap-1.5 border-r border-border px-3 text-xs transition-colors",
              isActive
                ? "border-t-2 border-t-[var(--color-accent)] bg-background text-foreground font-medium pt-0.5"
                : "text-muted-foreground hover:bg-background/60 hover:text-foreground",
            ].join(" ")}
          >
            {/* isDirty dot */}
            {tab.isDirty && (
              <span
                className="size-1.5 shrink-0 rounded-full bg-[var(--color-warning)]"
                aria-label="Chưa lưu"
              />
            )}
            {/* Tab name */}
            <span className="truncate">
              {tab.name}
              {tab.strategyId === null && !tab.isDirty ? " *" : ""}
            </span>
            {/* Close button — visible on hover */}
            <button
              type="button"
              aria-label={`Đóng tab ${tab.name}`}
              onClick={(e) => {
                e.stopPropagation();
                onTabClose(tab.id);
              }}
              className="ml-0.5 shrink-0 rounded p-0.5 opacity-0 transition-opacity hover:bg-muted group-hover:opacity-100 focus:opacity-100"
            >
              <X className="size-3" />
            </button>
          </div>
        );
      })}

      {/* New tab button */}
      <button
        type="button"
        aria-label="Mở tab mới"
        onClick={onNewTab}
        className="flex h-full shrink-0 items-center px-2.5 text-muted-foreground hover:bg-background/60 hover:text-foreground transition-colors"
      >
        <Plus className="size-3.5" />
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// EditorShell — main export
// ---------------------------------------------------------------------------

export function EditorShell() {
  const {
    tabs,
    activeTabId,
    openNewTab,
    closeTab,
    setActiveTab,
    markDirty,
    markClean,
    updateTabName,
  } = useEditorStore();

  // Registry: tabId → WorkspaceSvg (for undo/redo/zoom calls in ControlBar)
  // Cast-safe: WorkspaceFacade is a subset of Blockly.WorkspaceSvg
  const workspacesRef = useRef<Map<string, Blockly.WorkspaceSvg>>(new Map());

  // Active workspace exposed to EditorControlBar
  const [activeWorkspace, setActiveWorkspace] = useState<WorkspaceFacade | null>(null);

  // [3.2.4] Save/Load strategy hook — wired to the workspace registry
  const { saveStrategy, loadStrategy, isSaving } = useEditorTab(workspacesRef);

  // ------------------------------------------------------------------
  // WorkspaceSvg lifecycle callbacks (stable via useCallback)
  // ------------------------------------------------------------------
  const handleWorkspaceReady = useCallback(
    (tabId: string, workspace: WorkspaceFacade) => {
      workspacesRef.current.set(tabId, workspace as Blockly.WorkspaceSvg);
      // If this is the currently active tab, expose it to the control bar
      if (tabId === activeTabId) {
        setActiveWorkspace(workspace);
      }

      // [3.2.4] Auto-load strategy data if tab has a strategyId
      const tab = tabs.find((t) => t.id === tabId);
      if (tab?.strategyId) {
        loadStrategy(tabId, tab.strategyId);
      }
    },
    [activeTabId, tabs, loadStrategy]
  );

  const handleWorkspaceDestroy = useCallback((tabId: string) => {
    workspacesRef.current.delete(tabId);
    setActiveWorkspace((prev) => {
      // If the destroyed workspace was active, clear it
      return workspacesRef.current.get(tabId) === prev ? null : prev;
    });
  }, []);

  // ------------------------------------------------------------------
  // Update activeWorkspace when active tab changes
  // ------------------------------------------------------------------
  useEffect(() => {
    const ws = activeTabId ? workspacesRef.current.get(activeTabId) ?? null : null;
    setActiveWorkspace(ws);
  }, [activeTabId]);

  // ------------------------------------------------------------------
  // Dirty tracking callback per tab (stable per tabId via closure)
  // ------------------------------------------------------------------
  const makeDirtyHandler = useCallback(
    (tabId: string) => () => markDirty(tabId),
    [markDirty]
  );

  // ------------------------------------------------------------------
  // Tab actions
  // ------------------------------------------------------------------
  const handleTabClose = useCallback(
    (tabId: string) => {
      // NOTE: isDirty check + CloseTabDialog wired in Task 3.2.9
      closeTab(tabId);
    },
    [closeTab]
  );

  // ------------------------------------------------------------------
  // Save handler — [3.2.4] wired to real save via useEditorTab
  // ------------------------------------------------------------------
  const handleSave = useCallback(async () => {
    if (!activeTabId) return;
    await saveStrategy(activeTabId);
  }, [activeTabId, saveStrategy]);

  // ------------------------------------------------------------------
  // Export handler (stub — full implementation in Task 3.2.5)
  // ------------------------------------------------------------------
  const handleExport = useCallback(() => {
    // TODO [3.2.5]: serialize workspace → Blob API → download .json
    toast.info("Tính năng Xuất JSON sẽ được triển khai trong Task 3.2.5.");
  }, []);

  // ------------------------------------------------------------------
  // Name change handler
  // ------------------------------------------------------------------
  const handleNameChange = useCallback(
    (name: string) => {
      if (activeTabId) updateTabName(activeTabId, name);
    },
    [activeTabId, updateTabName]
  );

  // ------------------------------------------------------------------
  // Derive active tab for ControlBar
  // ------------------------------------------------------------------
  const activeTab = tabs.find((t) => t.id === activeTabId) ?? null;

  // ------------------------------------------------------------------
  // Render
  // ------------------------------------------------------------------
  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* ── Tab Bar ────────────────────────────────────────────────── */}
      <InlineTabBar
        tabs={tabs}
        activeTabId={activeTabId}
        onTabSelect={setActiveTab}
        onTabClose={handleTabClose}
        onNewTab={openNewTab}
      />

      {/* ── Control Bar ────────────────────────────────────────────── */}
      <EditorControlBar
        activeTab={activeTab}
        activeWorkspace={activeWorkspace}
        isSaving={isSaving}
        onSave={handleSave}
        onExport={handleExport}
        onNameChange={handleNameChange}
      />

      {/* ── Workspace Area ─────────────────────────────────────────── */}
      <div className="relative flex-1 overflow-hidden bg-[#0D1117]">
        {tabs.length === 0 ? (
          /* Empty state */
          <EditorEmptyState onNewTab={openNewTab} />
        ) : (
          <>
            {tabs.map((tab) => {
              const isActive = tab.id === activeTabId;
              return (
                /*
                 * CRITICAL: Use `style` not className for display toggle.
                 * CSS `display:none` hides the element but keeps the DOM node
                 * (and Blockly workspace) alive. React never unmounts it.
                 * Blockly.svgResize() is called inside BlocklyWorkspace on
                 * isActive change.
                 */
                <div
                  key={tab.id}
                  style={{ display: isActive ? "flex" : "none" }}
                  className="absolute inset-0"
                >
                  <BlocklyWorkspace
                    tabId={tab.id}
                    isActive={isActive}
                    onWorkspaceReady={handleWorkspaceReady}
                    onWorkspaceDestroy={handleWorkspaceDestroy}
                    onDirty={makeDirtyHandler(tab.id)}
                  />
                </div>
              );
            })}
          </>
        )}
      </div>
    </div>
  );
}
