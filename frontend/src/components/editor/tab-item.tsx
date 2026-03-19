/**
 * [3.2.7] TabItem — Single tab chip in the Tab Bar.
 *
 * Anatomy (per frontend_flows.md §3.4.2):
 *   ┌────────────────────────────┐
 *   │ [●] [Tên chiến lược] [✕]  │
 *   └────────────────────────────┘
 *   ● = isDirty dot (vàng #FFAB40)
 *   ✕ = close button (visible on hover only)
 *
 * Visual states:
 *   Active:   bg-background, border-top 2px accent, text-foreground, font-medium
 *   Inactive: bg-transparent, text-muted-foreground, hover → bg-background/60
 */
"use client";

import { X, FileCode2 } from "lucide-react";
import type { EditorTab } from "@/types";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface TabItemProps {
  tab: EditorTab;
  isActive: boolean;
  onSelect: () => void;
  onClose: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function TabItem({ tab, isActive, onSelect, onClose }: TabItemProps) {
  return (
    <div
      role="tab"
      aria-selected={isActive}
      tabIndex={isActive ? 0 : -1}
      onClick={onSelect}
      className={[
        "group relative flex h-full min-w-0 max-w-52 shrink-0 cursor-pointer select-none items-center gap-1.5 border-r border-border px-3 text-xs transition-colors",
        isActive
          ? "border-t-2 border-t-[var(--color-accent)] bg-background text-foreground font-medium pt-0.5"
          : "text-muted-foreground hover:bg-background/60 hover:text-foreground",
      ].join(" ")}
    >
      {/* File icon */}
      <FileCode2
        className={[
          "size-3.5 shrink-0",
          isActive ? "text-[var(--color-accent)]" : "text-muted-foreground/60",
        ].join(" ")}
      />

      {/* isDirty dot — yellow indicator (frontend_flows §3.4.2) */}
      {tab.isDirty && (
        <span
          className="size-1.5 shrink-0 rounded-full bg-[#FFAB40]"
          aria-label="Chưa lưu"
        />
      )}

      {/* Tab name — truncated with ellipsis */}
      <span className="truncate">
        {tab.name}
        {tab.strategyId === null && !tab.isDirty ? " *" : ""}
      </span>

      {/* Close button — visible on hover only */}
      <button
        type="button"
        aria-label={`Đóng tab ${tab.name}`}
        onClick={(e) => {
          e.stopPropagation();
          onClose();
        }}
        className="ml-auto shrink-0 rounded p-0.5 opacity-0 transition-opacity hover:bg-muted group-hover:opacity-100 focus:opacity-100"
      >
        <X className="size-3" />
      </button>
    </div>
  );
}
