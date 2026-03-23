// ===================================================================
// QuantFlow — Strategy Serializer
// Task 3.2.4 — Save/Load Strategy via API
// Task 3.2.5 — Import/Export JSON (shared utilities)
// ===================================================================
//
// Provides Blockly workspace serialization/deserialization utilities.
//
// Uses Blockly 12's JSON serialization system:
//   - Blockly.serialization.workspaces.save(workspace)
//   - Blockly.serialization.workspaces.load(state, workspace)
//
// SRS FR-DESIGN-11: "Lưu dưới dạng JSON"
// SRS FR-DESIGN-13: "Kiểm tra tính hợp lệ của cấu trúc JSON"
// ===================================================================

import * as Blockly from "blockly";

// -----------------------------------------------------------------
// Known event trigger block types (SRS FR-DESIGN-03)
// -----------------------------------------------------------------
const EVENT_TRIGGER_TYPES = new Set([
  "event_on_candle",
]);

// -----------------------------------------------------------------
// Type for Blockly JSON state
// -----------------------------------------------------------------

/**
 * The JSON state object produced by `Blockly.serialization.workspaces.save()`.
 * Kept intentionally loose — the Blockly library defines the internal structure.
 */
export type BlocklyJsonState = Record<string, unknown>;

// -----------------------------------------------------------------
// Serialize: Workspace → JSON
// -----------------------------------------------------------------

/**
 * Serialize the entire Blockly workspace to a JSON state object.
 *
 * This captures all top-level blocks, their positions, field values,
 * connections, and any workspace-level data (variables, etc.).
 *
 * @param workspace — A live Blockly WorkspaceSvg instance
 * @returns JSON-serializable state object
 */
export function serializeWorkspace(
  workspace: Blockly.WorkspaceSvg,
): BlocklyJsonState {
  return Blockly.serialization.workspaces.save(workspace);
}

// -----------------------------------------------------------------
// Deserialize: JSON → Workspace
// -----------------------------------------------------------------

/**
 * Load a JSON state object into a Blockly workspace.
 *
 * Clears the workspace first, then loads the state. The workspace
 * will fire a FINISHED_LOADING event when done — callers should
 * ignore dirty tracking during this operation.
 *
 * @param workspace — Target Blockly WorkspaceSvg to load into
 * @param state — JSON state object (from serializeWorkspace or API)
 */
export function deserializeToWorkspace(
  workspace: Blockly.WorkspaceSvg,
  state: BlocklyJsonState,
): void {
  // Blockly.serialization.workspaces.load() clears and loads atomically,
  // wrapping the operation in a FINISHED_LOADING event.
  Blockly.serialization.workspaces.load(state, workspace);
}

// -----------------------------------------------------------------
// Validation: Check for Event Trigger block
// -----------------------------------------------------------------

/**
 * Check whether the serialized JSON state contains at least one
 * Event Trigger block (event_on_candle_close or event_on_candle_open).
 *
 * SRS FR-DESIGN-11: "Kiểm tra lỗi nối khối, cảnh báo nếu thiếu khối Sự kiện."
 *
 * @param state — JSON state object from serializeWorkspace
 * @returns true if at least one event trigger block is found
 */
export function hasEventTriggerBlock(state: BlocklyJsonState): boolean {
  // Top-level blocks are stored under state.blocks.blocks (array)
  const blocks = state?.blocks as { blocks?: Array<{ type?: string }> } | undefined;
  if (!blocks?.blocks || !Array.isArray(blocks.blocks)) {
    return false;
  }

  return blocks.blocks.some((block) =>
    block.type ? EVENT_TRIGGER_TYPES.has(block.type) : false,
  );
}

/**
 * Check whether a strategy is "valid" (ready to run) or "draft":
 *
 *   - **valid**: the workspace has exactly ONE top-level block, and that
 *     block is an event trigger (event_on_candle). This means there are
 *     no orphan/detached blocks floating in the workspace.
 *   - **draft**: any other case — missing event trigger, multiple top-level
 *     blocks (orphans), or empty workspace.
 *
 * In Blockly's serialized JSON, `state.blocks.blocks` is the array of
 * top-level (root) blocks. Blocks connected via next/input connections
 * are nested inside their parent — they are NOT separate top-level entries.
 * So multiple entries = orphan blocks = draft.
 *
 * @param state — JSON state from serializeWorkspace
 * @returns true if the strategy is valid (can be used by bots/backtest)
 */
export function isStrategyValid(state: BlocklyJsonState): boolean {
  const blocks = state?.blocks as { blocks?: Array<{ type?: string }> } | undefined;
  if (!blocks?.blocks || !Array.isArray(blocks.blocks)) {
    return false;
  }

  // Must have exactly one top-level block and it must be an event trigger.
  return (
    blocks.blocks.length === 1 &&
    blocks.blocks[0].type !== undefined &&
    EVENT_TRIGGER_TYPES.has(blocks.blocks[0].type)
  );
}

// -----------------------------------------------------------------
// Export: Workspace → .json file download (Blob API)
// -----------------------------------------------------------------

/**
 * Slugify a strategy name into a safe filename base.
 * Example: "EMA Crossover 15m" → "ema-crossover-15m"
 */
function slugifyFilename(name: string): string {
  let slug = name
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  if (!slug) slug = "strategy";
  return slug;
}

/**
 * Export a Blockly workspace state to a downloadable `.json` file.
 *
 * Uses the Blob API + URL.createObjectURL to trigger a browser-side
 * file download. No API call is made — this is purely client-side.
 *
 * SRS FR-DESIGN-12: "Xuất chiến lược ra file JSON"
 * frontend_flows.md: "Serialize → Blob download .json. Không đánh dấu clean"
 *
 * @param state — JSON state from serializeWorkspace()
 * @param strategyName — human-readable name (used to generate filename)
 */
export function exportToJsonFile(
  state: BlocklyJsonState,
  strategyName: string,
): void {
  // Wrap in the same {name, logic_json} format as GET /strategies/{id}/export
  // so the exported file can be re-imported via POST /strategies/import.
  const exportData = {
    name: strategyName,
    logic_json: state,
  };
  const jsonString = JSON.stringify(exportData, null, 2);
  const blob = new Blob([jsonString], { type: "application/json" });
  const url = URL.createObjectURL(blob);

  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `${slugifyFilename(strategyName)}.json`;
  document.body.appendChild(anchor);
  anchor.click();

  // Cleanup: remove the temporary anchor and revoke the blob URL
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
}
