import { create } from "zustand";

export interface DebugSceneEntry {
  seqNum: number;
  label: string;
  description: string;
  scenarioName: string;
  scenarioJson?: string;
  triggerType: string;
  mapName: string;
  scriptLabel?: string;
  category?: string;
}

interface DebugSceneState {
  scenes: DebugSceneEntry[];
  isOpen: boolean;
  powerPokemonMessage: string | null;
  setScenes: (scenes: DebugSceneEntry[]) => void;
  toggleOpen: () => void;
  setOpen: (open: boolean) => void;
  setPowerPokemonMessage: (message: string | null) => void;
}

const useDebugSceneStore = create<DebugSceneState>((set) => ({
  scenes: [],
  isOpen: false,
  powerPokemonMessage: null,
  setScenes: (scenes) => set({ scenes }),
  toggleOpen: () => set((s) => ({ isOpen: !s.isOpen })),
  setOpen: (open) => set({ isOpen: open }),
  setPowerPokemonMessage: (message) => set({ powerPokemonMessage: message }),
}));

export default useDebugSceneStore;
