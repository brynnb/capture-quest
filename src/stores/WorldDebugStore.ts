import { create } from "zustand";

interface WorldDebugState {
  tileInfo: string;
  viewMode: string;
  setTileInfo: (tileInfo: string) => void;
  setViewMode: (viewMode: string) => void;
  reset: () => void;
}

const useWorldDebugStore = create<WorldDebugState>()((set) => ({
  tileInfo: "Move over the map to inspect a tile.",
  viewMode: "Overworld View",
  setTileInfo: (tileInfo) => set({ tileInfo }),
  setViewMode: (viewMode) => set({ viewMode }),
  reset: () =>
    set({
      tileInfo: "Move over the map to inspect a tile.",
      viewMode: "Overworld View",
    }),
}));

export default useWorldDebugStore;
