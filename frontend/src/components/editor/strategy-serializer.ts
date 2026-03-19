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
