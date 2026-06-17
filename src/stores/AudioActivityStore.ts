import { create } from "zustand";

interface AudioActivityStore {
  isSurfing: boolean;
  isBicycleActive: boolean;
  travelMapId: number | null;
  travelMapName: string | null;
  battleVictoryTrack: string | null;
  setSurfing: (isSurfing: boolean) => void;
  setBicycleActive: (isBicycleActive: boolean) => void;
  setTravelMap: (mapId: number | null, mapName?: string | null) => void;
  setBattleVictoryTrack: (battleVictoryTrack: string | null) => void;
  resetTravelAudio: () => void;
}

const useAudioActivityStore = create<AudioActivityStore>()((set) => ({
  isSurfing: false,
  isBicycleActive: false,
  travelMapId: null,
  travelMapName: null,
  battleVictoryTrack: null,
  setSurfing: (isSurfing) => set({ isSurfing }),
  setBicycleActive: (isBicycleActive) => set({ isBicycleActive }),
  setTravelMap: (travelMapId, travelMapName = null) =>
    set({ travelMapId, travelMapName }),
  setBattleVictoryTrack: (battleVictoryTrack) => set({ battleVictoryTrack }),
  resetTravelAudio: () =>
    set({
      isSurfing: false,
      isBicycleActive: false,
      travelMapId: null,
      travelMapName: null,
      battleVictoryTrack: null,
    }),
}));

export default useAudioActivityStore;
