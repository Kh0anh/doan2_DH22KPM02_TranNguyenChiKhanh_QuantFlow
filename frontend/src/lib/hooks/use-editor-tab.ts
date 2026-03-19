// ===================================================================
// QuantFlow — useEditorTab Hook
// Task 3.2.4 — Save/Load Strategy via API
// Task 3.2.8 — Multi-tab editor load/save/dirty logic per tab
// ===================================================================
//
// Custom React hook that manages Save and Load logic for a single
// editor tab. Used by EditorShell to:
//   - Load strategy data (GET API → Blockly workspace) on tab open
//   - Save strategy data (Blockly workspace → POST/PUT API) on save
//
// Dependencies:
//   - api-client.ts — REST API calls
//   - strategy-serializer.ts — Blockly serialization
//   - editor-store.ts — Zustand tab state (markClean, updateTabName)
//
// SRS: FR-DESIGN-11 (Save), UC-04 luồng 6-9
// ===================================================================

"use client";

import { useCallback, useRef, useState } from "react";
import { toast } from "sonner";
import type * as Blockly from "blockly";
import { strategyApi, ApiError } from "@/lib/api-client";
import {
  serializeWorkspace,
  deserializeToWorkspace,
  hasEventTriggerBlock,
} from "@/components/editor/strategy-serializer";
import { useEditorStore } from "@/store/editor-store";

// -----------------------------------------------------------------
// Hook interface
// -----------------------------------------------------------------

interface UseEditorTabReturn {
  /** Save the active tab's workspace to backend via API */
  saveStrategy: (tabId: string) => Promise<void>;
  /** Load a strategy from backend into a workspace */
  loadStrategy: (tabId: string, strategyId: string) => Promise<void>;
  /** Whether a save operation is currently in progress */
  isSaving: boolean;
  /** Set of tabIds currently loading */
  loadingTabs: Set<string>;
}

// -----------------------------------------------------------------
// Hook implementation
// -----------------------------------------------------------------

/**
 * useEditorTab — manages Save/Load strategy logic for the editor.
 *
 * @param workspacesRef — mutable Map of tabId → Blockly WorkspaceSvg,
 *   maintained by EditorShell's workspace lifecycle callbacks.
 */
export function useEditorTab(
  workspacesRef: React.RefObject<Map<string, Blockly.WorkspaceSvg>>,
): UseEditorTabReturn {
  const [isSaving, setIsSaving] = useState(false);
  const [loadingTabs, setLoadingTabs] = useState<Set<string>>(new Set());

  // Prevent duplicate loads
  const loadedRef = useRef<Set<string>>(new Set());

  // Access Zustand store actions directly (non-reactive)
  const store = useEditorStore;

  // ----------------------------------------------------------------
  // saveStrategy — serialize workspace → validate → POST/PUT API
  // ----------------------------------------------------------------
  const saveStrategy = useCallback(
    async (tabId: string) => {
      const workspace = workspacesRef.current?.get(tabId);
      if (!workspace) {
        toast.error("Workspace chưa sẵn sàng. Vui lòng thử lại.");
        return;
      }

      // Get current tab info from store
      const tab = store.getState().tabs.find((t) => t.id === tabId);
      if (!tab) return;

      // Validate: strategy name must not be empty
      const name = tab.name.trim();
      if (!name || name === "Chiến lược mới") {
        toast.error("Vui lòng đặt tên cho chiến lược trước khi lưu.");
        return;
      }

      // Serialize workspace to JSON
      const logicJson = serializeWorkspace(workspace);

      // Validate: must contain Event Trigger block (SRS FR-DESIGN-11)
      if (!hasEventTriggerBlock(logicJson)) {
        toast.error(
          "Chiến lược phải bắt đầu bằng khối Sự kiện (Event Trigger).",
        );
        return;
      }

      setIsSaving(true);
      try {
        if (tab.strategyId === null) {
          // ── CREATE (new strategy) ─────────────────────────────────
          const created = await strategyApi.create({
            name,
            logic_json: logicJson,
            status: "valid",
          });

          // Update tab with the new strategyId from backend
          // We need to update both the strategyId and the tab id
          const { tabs, activeTabId } = store.getState();
          const updatedTabs = tabs.map((t) =>
            t.id === tabId
              ? { ...t, strategyId: created.id, isDirty: false }
              : t,
          );
          store.setState({ tabs: updatedTabs, activeTabId });

          toast.success("Đã tạo chiến lược mới thành công.");
        } else {
          // ── UPDATE (existing strategy) ────────────────────────────
          const result = await strategyApi.update(tab.strategyId, {
            name,
            logic_json: logicJson,
            status: "valid",
          });

          // Mark tab as clean
          store.getState().markClean(tabId);

          // Show warning if bots are using this strategy
          if (result.warning) {
            toast.warning(result.warning);
          } else {
            toast.success("Đã cập nhật chiến lược thành công.");
          }
        }
      } catch (err) {
        if (err instanceof ApiError) {
          switch (err.code) {
            case "MISSING_EVENT_TRIGGER":
              toast.error(
                "Chiến lược phải bắt đầu bằng khối Sự kiện (Event Trigger).",
              );
              break;
            case "INVALID_JSON_STRUCTURE":
              toast.error(
                "Cấu trúc JSON không hợp lệ. Vui lòng kiểm tra lại.",
              );
              break;
            case "STRATEGY_NOT_FOUND":
              toast.error("Chiến lược không tồn tại hoặc đã bị xóa.");
              break;
            default:
              toast.error(err.message || "Lưu thất bại. Vui lòng thử lại.");
          }
        } else {
          toast.error("Lỗi kết nối. Vui lòng kiểm tra mạng và thử lại.");
        }
      } finally {
        setIsSaving(false);
      }
    },
    [workspacesRef, store],
  );

  // ----------------------------------------------------------------
  // loadStrategy — GET API → deserialize into workspace
  // ----------------------------------------------------------------
  const loadStrategy = useCallback(
    async (tabId: string, strategyId: string) => {
      // Prevent duplicate loading for the same tab
      if (loadedRef.current.has(tabId)) return;
      loadedRef.current.add(tabId);

      setLoadingTabs((prev) => new Set(prev).add(tabId));

      try {
        // Fetch strategy detail from backend
        const detail = await strategyApi.get(strategyId);

        // Wait for workspace to be available (it may still be injecting)
        const workspace = await waitForWorkspace(workspacesRef, tabId, 5000);
        if (!workspace) {
          toast.error("Workspace không sẵn sàng. Vui lòng thử lại.");
          loadedRef.current.delete(tabId);
          return;
        }

        // Load blocks into workspace
        // The FINISHED_LOADING event from Blockly prevents dirty tracking
        const logicJson =
          typeof detail.logic_json === "string"
            ? JSON.parse(detail.logic_json)
            : detail.logic_json;

        deserializeToWorkspace(workspace, logicJson);

        // Update tab name from the server data
        store.getState().updateTabName(tabId, detail.name);

        // Show warning banner if active bots exist
        if (detail.warning) {
          toast.warning(detail.warning);
        }
      } catch (err) {
        loadedRef.current.delete(tabId);
        if (err instanceof ApiError) {
          if (err.code === "STRATEGY_NOT_FOUND") {
            toast.error("Chiến lược không tồn tại hoặc đã bị xóa.");
            // Close the broken tab
            store.getState().closeTab(tabId);
          } else {
            toast.error(err.message || "Tải chiến lược thất bại.");
          }
        } else {
          toast.error("Lỗi kết nối. Vui lòng kiểm tra mạng và thử lại.");
        }
      } finally {
        setLoadingTabs((prev) => {
          const next = new Set(prev);
          next.delete(tabId);
          return next;
        });
      }
    },
    [workspacesRef, store],
  );

  return { saveStrategy, loadStrategy, isSaving, loadingTabs };
}

// -----------------------------------------------------------------
// Helper: wait for workspace to appear in the registry
// -----------------------------------------------------------------

/**
 * Poll the workspaces registry until the workspace for the given tabId
 * appears, or timeout is reached.
 */
function waitForWorkspace(
  workspacesRef: React.RefObject<Map<string, Blockly.WorkspaceSvg>>,
  tabId: string,
  timeoutMs: number,
): Promise<Blockly.WorkspaceSvg | null> {
  return new Promise((resolve) => {
    const start = Date.now();
    const check = () => {
      const ws = workspacesRef.current?.get(tabId);
      if (ws) {
        resolve(ws);
        return;
      }
      if (Date.now() - start > timeoutMs) {
        resolve(null);
        return;
      }
      requestAnimationFrame(check);
    };
    check();
  });
}
