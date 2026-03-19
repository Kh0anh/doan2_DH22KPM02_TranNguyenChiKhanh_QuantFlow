import { create } from "zustand";
import { persist } from "zustand/middleware";

/**
 * [3.1.2] UIStore — Shared UI state across the dashboard.
 *
 * State:
 *   activeSymbol           — symbol currently selected in Market Watch
 *   chartSplitterHeight    — bottom panel height (px), persisted to localStorage
 *   marketWatchWidth       — market watch panel width (px), persisted
 *   settingsOpen           — whether Settings Dialog is visible
 *
 * Actions (full implementation: Task 3.1.2):
 *   setActiveSymbol(symbol)         — change active chart symbol
 *   setChartSplitterHeight(height)  — drag-resize bottom panel
 *   setMarketWatchWidth(width)      — drag-resize market watch
 *   openSettings()                  — open Settings Dialog
 *   closeSettings()                 — close Settings Dialog
 */

interface UIStoreState {
  // Market Watch
  activeSymbol: string;
  setActiveSymbol: (symbol: string) => void;

  // Splitter positions (persisted to localStorage)
  chartSplitterHeight: number;
  setChartSplitterHeight: (height: number) => void;
  marketWatchWidth: number;
  setMarketWatchWidth: (width: number) => void;

  // Settings dialog
  settingsOpen: boolean;
  openSettings: () => void;
  closeSettings: () => void;
}

export const useUIStore = create<UIStoreState>()(
  persist(
    (set) => ({
      activeSymbol: "BTCUSDT",
      setActiveSymbol: (symbol) => set({ activeSymbol: symbol }),

      chartSplitterHeight: 280,
      setChartSplitterHeight: (height) => set({ chartSplitterHeight: height }),

      marketWatchWidth: 220,
      setMarketWatchWidth: (width) => set({ marketWatchWidth: width }),

      settingsOpen: false,
      openSettings: () => set({ settingsOpen: true }),
      closeSettings: () => set({ settingsOpen: false }),
    }),
    {
      name: "quantflow-ui",
      // Only persist splitter positions — not dialog state
      partialize: (state) => ({
        chartSplitterHeight: state.chartSplitterHeight,
        marketWatchWidth: state.marketWatchWidth,
        activeSymbol: state.activeSymbol,
      }),
    }
  )
);
