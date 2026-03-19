import { create } from "zustand";
import { EditorTab } from "@/types";
import { v4 as uuidv4 } from "uuid";

/**
 * [3.2.7] GlobalEditorStore — Multi-tab Editor state.
 *
 * State:
 *   tabs[]        — list of currently open editor tabs
 *   activeTabId   — ID of the visible tab
 *
 * Actions (full implementation: Task 3.2.7):
 *   openTab(strategyId)   — open new tab or focus existing
 *   closeTab(tabId)       — close tab (with isDirty check)
 *   setActiveTab(tabId)   — switch visible tab
 *   markDirty(tabId)      — mark tab as having unsaved changes
 *   markClean(tabId)      — clear isDirty after successful save
 *   updateTabName(tabId)  — rename tab when strategy name changes
 *   closeAllTabs()        — close all tabs (navigate away)
 *
 * Persistence:
 *   Tab IDs/names in sessionStorage.
 *   Blockly XML NOT persisted — reconstructed from API on re-open.
 */

const MAX_TABS = 7;

interface EditorStoreState {
  tabs: EditorTab[];
  activeTabId: string | null;
  openTab: (strategyId: string, strategyName: string) => void;
  openNewTab: () => void;
  closeTab: (tabId: string) => void;
  setActiveTab: (tabId: string) => void;
  markDirty: (tabId: string) => void;
  markClean: (tabId: string) => void;
  updateTabName: (tabId: string, name: string) => void;
  closeAllTabs: () => void;
}

export const useEditorStore = create<EditorStoreState>((set, get) => ({
  tabs: [],
  activeTabId: null,

  openTab: (strategyId: string, strategyName: string) => {
    const { tabs } = get();
    const existing = tabs.find((t) => t.strategyId === strategyId);
    if (existing) {
      set({ activeTabId: existing.id });
      return;
    }
    // TODO [3.2.7]: enforce MAX_TABS = 7, show toast warning if exceeded
    // TODO [3.2.7]: enforce MAX_TABS = 7, show toast warning if exceeded
    if (tabs.length >= MAX_TABS) return;
    const newTab: EditorTab = {
      id: strategyId,
      strategyId,
      name: strategyName.slice(0, 20),
      isDirty: false,
    };
    set((state) => ({
      tabs: [...state.tabs, newTab],
      activeTabId: newTab.id,
    }));
  },

  openNewTab: () => {
    const { tabs } = get();
    // TODO [3.2.7]: show toast warning when limit reached
    if (tabs.length >= MAX_TABS) return;
    const id = uuidv4();
    const newTab: EditorTab = {
      id,
      strategyId: null,
      name: "Chiến lược mới",
      isDirty: false,
    };
    set((state) => ({
      tabs: [...state.tabs, newTab],
      activeTabId: newTab.id,
    }));
  },

  closeTab: (tabId: string) => {
    // NOTE [3.2.9]: isDirty check + CloseTabDialog will be wired in task 3.2.9
    set((state) => {
      const filtered = state.tabs.filter((t) => t.id !== tabId);
      const newActive =
        state.activeTabId === tabId
          ? (filtered[filtered.length - 1]?.id ?? null)
          : state.activeTabId;
      return { tabs: filtered, activeTabId: newActive };
    });
  },

  setActiveTab: (tabId: string) => set({ activeTabId: tabId }),

  markDirty: (tabId: string) =>
    set((state) => ({
      tabs: state.tabs.map((t) =>
        t.id === tabId ? { ...t, isDirty: true } : t
      ),
    })),

  markClean: (tabId: string) =>
    set((state) => ({
      tabs: state.tabs.map((t) =>
        t.id === tabId ? { ...t, isDirty: false } : t
      ),
    })),

  updateTabName: (tabId: string, name: string) =>
    set((state) => ({
      tabs: state.tabs.map((t) =>
        t.id === tabId ? { ...t, name: name.slice(0, 20) } : t
      ),
    })),

  closeAllTabs: () => set({ tabs: [], activeTabId: null }),
}));
