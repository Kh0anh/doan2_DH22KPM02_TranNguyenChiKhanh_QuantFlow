/**
 * [3.2.1] BlocklyWorkspace — Single Blockly workspace instance wrapper.
 *
 * Lifecycle:
 *   - Initialised once on first render (Blockly.inject into container div).
 *   - Never unmounted while parent tab exists — CSS display:none used instead.
 *   - Calls Blockly.svgResize() 50ms after becoming active (display:block).
 *   - Dirty tracking: addChangeListener → props.onDirty().
 *   - Disposes workspace + removes listener on tab close (component unmount).
 *
 * SSR: This file is loaded with next/dynamic { ssr: false } from editor-shell,
 *      so top-level Blockly imports execute only in the browser.
 *
 * Toolbox: Empty placeholder — custom blocks registered in Task 3.2.2/3.2.3.
 */
"use client";

import { useEffect, useRef } from "react";
import * as Blockly from "blockly";
import * as EnMsg from "blockly/msg/en";
// [3.2.2] Register all 26 custom blocks before any Blockly.inject() call
import "@/lib/blockly/blocks";
// [3.2.3] Toolbox configuration — 6 categorized groups
import { QUANTFLOW_TOOLBOX } from "./toolbox-config";

// ---------------------------------------------------------------------------
// Locale — set once at module load (client-only, safe)
// ---------------------------------------------------------------------------
Blockly.setLocale(EnMsg as unknown as { [key: string]: string });

// ---------------------------------------------------------------------------
// Custom dark theme matching QuantFlow palette
// ---------------------------------------------------------------------------
const QUANTFLOW_THEME = Blockly.Theme.defineTheme("quantflow-dark", {
  name: "quantflow-dark",
  base: Blockly.Themes.Classic,
  fontStyle: {
    family: "'JetBrains Mono', monospace",
    weight: "500",
    size: 11,
  },
  componentStyles: {
    workspaceBackgroundColour: "#0D1117",
    toolboxBackgroundColour: "#161B22",
    toolboxForegroundColour: "#E6EDF3",
    flyoutBackgroundColour: "#161B22",
    flyoutForegroundColour: "#C9D1D9",
    flyoutOpacity: 1,
    scrollbarColour: "#30363D",
    insertionMarkerColour: "#58A6FF",
    insertionMarkerOpacity: 0.3,
    scrollbarOpacity: 0.6,
    cursorColour: "#58A6FF",
  },
});

// ---------------------------------------------------------------------------
// Blockly inject options
// ---------------------------------------------------------------------------
const INJECT_OPTIONS: Blockly.BlocklyOptions = {
  toolbox: QUANTFLOW_TOOLBOX,
  zoom: {
    controls: false, // custom zoom buttons in EditorControlBar
    wheel: true,
    startScale: 1.0,
    maxScale: 3,
    minScale: 0.3,
    scaleSpeed: 1.2,
  },
  trashcan: true,
  grid: {
    spacing: 24,
    length: 3,
    colour: "#21262D",
    snap: true,
  },
  theme: QUANTFLOW_THEME,
  renderer: "geras",
  sounds: false,
};

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface BlocklyWorkspaceProps {
  /** Unique tab ID — used for workspace registry and dirty tracking */
  tabId: string;
  /** Whether this workspace's parent tab is currently the active tab */
  isActive: boolean;
  /** Called once when the Blockly workspace has been injected */
  onWorkspaceReady: (tabId: string, workspace: Blockly.WorkspaceSvg) => void;
  /** Called on unmount so parent can clean up the workspace reference */
  onWorkspaceDestroy: (tabId: string) => void;
  /** Called when a non-UI change is detected (to mark the tab as dirty) */
  onDirty: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function BlocklyWorkspace({
  tabId,
  isActive,
  onWorkspaceReady,
  onWorkspaceDestroy,
  onDirty,
}: BlocklyWorkspaceProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const workspaceRef = useRef<Blockly.WorkspaceSvg | null>(null);

  // ------------------------------------------------------------------
  // Mount: inject Blockly once
  // ------------------------------------------------------------------
  useEffect(() => {
    if (!containerRef.current || workspaceRef.current) return;

    const workspace = Blockly.inject(containerRef.current, INJECT_OPTIONS);
    workspaceRef.current = workspace;

    // ── Disable flyout auto-close when clicking the workspace ─────────
    const toolbox = workspace.getToolbox();
    if (toolbox) {
      const flyout = toolbox.getFlyout();
      if (flyout) {
        flyout.autoClose = false;
      }
    }

    // ── Dirty tracking ────────────────────────────────────────────────
    const changeHandler = (event: Blockly.Events.Abstract) => {
      // Ignore purely visual events (scroll, zoom, select, click)
      if (event.isUiEvent) return;
      // Ignore workspace load completion
      if (event.type === Blockly.Events.FINISHED_LOADING) return;
      onDirty();
    };
    workspace.addChangeListener(changeHandler);

    // ── Initial resize to fill container ─────────────────────────────
    Blockly.svgResize(workspace);

    // ── Notify parent ─────────────────────────────────────────────────
    onWorkspaceReady(tabId, workspace);

    // ── Cleanup: dispose workspace on tab close (component unmount) ───
    return () => {
      workspace.removeChangeListener(changeHandler);
      workspace.dispose();
      workspaceRef.current = null;
      onWorkspaceDestroy(tabId);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // run exactly once on mount

  // ------------------------------------------------------------------
  // Workaround: suppress Blockly v12 FocusManager "unregistered tree"
  // error (google/blockly#9599). Occurs when a workspace inside a
  // display:none container receives a focus event before re-registration.
  // ------------------------------------------------------------------
  useEffect(() => {
    const handler = (event: ErrorEvent) => {
      if (
        event.error?.message?.includes?.(
          "Attempted to focus unregistered tree",
        )
      ) {
        event.preventDefault();
        event.stopImmediatePropagation();
      }
    };
    window.addEventListener("error", handler);
    return () => window.removeEventListener("error", handler);
  }, []);

  // ------------------------------------------------------------------
  // Resize when tab becomes visible (display:none → display:block)
  // ------------------------------------------------------------------
  useEffect(() => {
    if (!isActive || !workspaceRef.current) return;
    // Allow the parent's CSS display update to flush before measuring
    const id = setTimeout(() => {
      if (workspaceRef.current) {
        Blockly.svgResize(workspaceRef.current);
      }
    }, 50);
    return () => clearTimeout(id);
  }, [isActive]);

  // ------------------------------------------------------------------
  // Render: a single full-size div that Blockly injects into
  // ------------------------------------------------------------------
  return <div ref={containerRef} className="absolute inset-0" />;
}
