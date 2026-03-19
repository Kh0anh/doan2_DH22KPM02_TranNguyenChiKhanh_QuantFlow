/**
 * [3.2.1] EditorControlBar — Control bar for the Strategy Editor.
 *
 * Layout (left → right):
 *   [← Trở về] | [Tên chiến lược (editable)] | [Undo][Redo] | [Zoom-][Zoom+][Fit] | [💾 Lưu] [📤 Xuất]
 *
 * Keyboard shortcuts (captured at document level):
 *   Ctrl+Z  → Undo
 *   Ctrl+Y  → Redo
 *   Ctrl+S  → Save (preventDefault)
 *
 * Props:
 *   activeTab       — current EditorTab (name, isDirty, id, strategyId)
 *   activeWorkspace — raw Blockly WorkspaceSvg object (method calls only, no Blockly import)
 *   onSave          — called when Save is triggered
 *   onExport        — called when Export is triggered (stub; wired in Task 3.2.5)
 *   onBack          — navigate back to /strategies
 *   onNameChange    — update tab name in Zustand store
 *
 * Note: This component does NOT import Blockly. It calls methods on the
 * workspace object directly (undo, zoom, zoomToFit, scrollCenter) using a
 * local facade type — keeping the control bar SSR-safe.
 */
"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ChevronLeft,
  Undo2,
  Redo2,
  ZoomIn,
  ZoomOut,
  Maximize2,
  Save,
  Upload,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { EditorTab } from "@/types";

// ---------------------------------------------------------------------------
// Blockly workspace facade — only the methods we call (no blockly import)
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
// Props
// ---------------------------------------------------------------------------

interface EditorControlBarProps {
  activeTab: EditorTab | null;
  /** Raw Blockly WorkspaceSvg — may be null if workspace not yet initialised */
  activeWorkspace: WorkspaceFacade | null;
  isSaving?: boolean;
  onSave: () => void;
  onExport: () => void;
  onNameChange: (name: string) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EditorControlBar({
  activeTab,
  activeWorkspace,
  isSaving = false,
  onSave,
  onExport,
  onNameChange,
}: EditorControlBarProps) {
  const router = useRouter();
  const [localName, setLocalName] = useState(activeTab?.name ?? "");
  const nameRef = useRef(localName);

  // Keep localName in sync when tab switches
  useEffect(() => {
    const next = activeTab?.name ?? "";
    setLocalName(next);
    nameRef.current = next;
  }, [activeTab?.id, activeTab?.name]);

  // ------------------------------------------------------------------
  // Keyboard shortcuts: Ctrl+S, Ctrl+Z, Ctrl+Y
  // ------------------------------------------------------------------
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!e.ctrlKey && !e.metaKey) return;
      if (!activeWorkspace) return;

      switch (e.key.toLowerCase()) {
        case "s":
          e.preventDefault();
          onSave();
          break;
        case "z":
          if (!e.shiftKey) {
            e.preventDefault();
            activeWorkspace.undo(false);
          }
          break;
        case "y":
          e.preventDefault();
          activeWorkspace.undo(true);
          break;
        default:
          break;
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [activeWorkspace, onSave]);

  // ------------------------------------------------------------------
  // Zoom helpers — zoom relative to workspace center
  // ------------------------------------------------------------------
  function zoomWorkspace(factor: number) {
    if (!activeWorkspace) return;
    const m = activeWorkspace.getMetrics();
    const cx = m.viewLeft + m.viewWidth / 2;
    const cy = m.viewTop + m.viewHeight / 2;
    activeWorkspace.zoom(cx, cy, factor);
  }

  function fitWorkspace() {
    if (!activeWorkspace) return;
    activeWorkspace.zoomToFit();
    activeWorkspace.scrollCenter();
  }

  // ------------------------------------------------------------------
  // Name commit
  // ------------------------------------------------------------------
  function commitName() {
    const trimmed = localName.trim() || (activeTab?.strategyId ? "Chiến lược" : "Chiến lược mới");
    setLocalName(trimmed);
    nameRef.current = trimmed;
    onNameChange(trimmed);
  }

  const disabled = !activeTab;
  const wsDisabled = !activeWorkspace;

  return (
    <div className="flex h-10 shrink-0 items-center gap-1 border-b border-border bg-background/80 px-2 backdrop-blur-sm">
      {/* ── Back button ─────────────────────────────────────────────── */}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="size-7 shrink-0"
            onClick={() => router.push("/strategies")}
            aria-label="Trở về danh sách chiến lược"
          >
            <ChevronLeft className="size-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">Trở về (Chiến lược)</TooltipContent>
      </Tooltip>

      <Separator orientation="vertical" className="mx-1 h-5" />

      {/* ── Strategy name input ─────────────────────────────────────── */}
      <div className="flex items-center gap-1.5 min-w-0">
        <Input
          value={localName}
          onChange={(e) => setLocalName(e.target.value.slice(0, 30))}
          onBlur={commitName}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.currentTarget.blur();
            }
          }}
          disabled={disabled}
          placeholder="Tên chiến lược"
          className="h-7 w-52 min-w-0 text-sm font-medium bg-transparent border-transparent hover:border-border focus:border-border transition-colors"
          aria-label="Tên chiến lược"
        />
        {/* isDirty indicator */}
        {activeTab?.isDirty && (
          <span
            className="size-2 shrink-0 rounded-full bg-[var(--color-warning)]"
            title="Có thay đổi chưa lưu"
            aria-label="Chưa lưu"
          />
        )}
      </div>

      <Separator orientation="vertical" className="mx-1 h-5" />

      {/* ── Undo / Redo ─────────────────────────────────────────────── */}
      <div className="flex items-center gap-0.5">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              disabled={wsDisabled}
              onClick={() => activeWorkspace?.undo(false)}
              aria-label="Hoàn tác (Ctrl+Z)"
            >
              <Undo2 className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">Hoàn tác (Ctrl+Z)</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              disabled={wsDisabled}
              onClick={() => activeWorkspace?.undo(true)}
              aria-label="Làm lại (Ctrl+Y)"
            >
              <Redo2 className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">Làm lại (Ctrl+Y)</TooltipContent>
        </Tooltip>
      </div>

      <Separator orientation="vertical" className="mx-1 h-5" />

      {/* ── Zoom controls ─────────────────────────────────────────────── */}
      <div className="flex items-center gap-0.5">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              disabled={wsDisabled}
              onClick={() => zoomWorkspace(1.2)}
              aria-label="Phóng to"
            >
              <ZoomIn className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">Phóng to</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              disabled={wsDisabled}
              onClick={() => zoomWorkspace(0.8)}
              aria-label="Thu nhỏ"
            >
              <ZoomOut className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">Thu nhỏ</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              disabled={wsDisabled}
              onClick={fitWorkspace}
              aria-label="Vừa màn hình"
            >
              <Maximize2 className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">Vừa màn hình</TooltipContent>
        </Tooltip>
      </div>

      {/* ── Spacer ──────────────────────────────────────────────────── */}
      <div className="flex-1" />

      {/* ── Export ──────────────────────────────────────────────────── */}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            disabled={disabled}
            onClick={onExport}
            aria-label="Xuất JSON"
          >
            <Upload className="size-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">Xuất JSON</TooltipContent>
      </Tooltip>

      {/* ── Save ────────────────────────────────────────────────────── */}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="default"
            size="sm"
            className="h-7 gap-1.5 px-3 text-xs"
            disabled={disabled || isSaving}
            onClick={onSave}
            aria-label="Lưu chiến lược (Ctrl+S)"
          >
            {isSaving ? (
              <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
            ) : (
              <Save className="size-3.5" aria-hidden="true" />
            )}
            Lưu
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">Lưu chiến lược (Ctrl+S)</TooltipContent>
      </Tooltip>
    </div>
  );
}
