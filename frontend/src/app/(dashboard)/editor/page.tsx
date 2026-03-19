/**
 * [3.2.1] Multi-tab Strategy Editor — single page, no :id route.
 * Tab state managed by GlobalEditorStore (Zustand).
 * openTab() called from /strategies or Sidebar [+] button.
 *
 * EditorShell renders:
 *   - Inline Tab Bar (task 3.2.7 will replace with <TabBar/>)
 *   - EditorControlBar (name, undo/redo, zoom, save, export)
 *   - All Blockly workspaces simultaneously (CSS display:none switching)
 */
import { EditorShell } from "@/components/editor/editor-shell";

export default function EditorPage() {
  return <EditorShell />;
}
