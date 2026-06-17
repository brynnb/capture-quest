import { create } from "zustand";

interface AudioActivityStore {
  isSurfing: boolean;
  isBicycleActive: boolean;
  wantsBicycle: boolean;
  forcedBicycle: boolean;
  travelMapId: number | null;
  travelMapName: string | null;
  battleVictoryTrack: string | null;
  setSurfing: (isSurfing: boolean) => void;
  setBicycleActive: (isBicycleActive: boolean) => void;
  setBicycleState: (state: {
    wantsRiding?: boolean;
    activeRiding?: boolean;
    forcedRiding?: boolean;
  }) => void;
  setTravelMap: (mapId: number | null, mapName?: string | null) => void;
  setBattleVictoryTrack: (battleVictoryTrack: string | null) => void;
  resetTravelAudio: () => void;
}

const useAudioActivityStore = create<AudioActivityStore>()((set) => ({
  isSurfing: false,
  isBicycleActive: false,
  wantsBicycle: false,
  forcedBicycle: false,
  travelMapId: null,
  travelMapName: null,
  battleVictoryTrack: null,
  setSurfing: (isSurfing) => set({ isSurfing }),
  setBicycleActive: (isBicycleActive) => set({ isBicycleActive }),
  setBicycleState: (state) =>
    set((current) => ({
      wantsBicycle: state.wantsRiding ?? current.wantsBicycle,
      forcedBicycle: state.forcedRiding ?? current.forcedBicycle,
      isBicycleActive: state.activeRiding ?? current.isBicycleActive,
    })),
  setTravelMap: (travelMapId, travelMapName = null) =>
    set({ travelMapId, travelMapName }),
  setBattleVictoryTrack: (battleVictoryTrack) => set({ battleVictoryTrack }),
  resetTravelAudio: () =>
    set({
      isSurfing: false,
      isBicycleActive: false,
      wantsBicycle: false,
      forcedBicycle: false,
      travelMapId: null,
      travelMapName: null,
      battleVictoryTrack: null,
    }),
}));

export default useAudioActivityStore;
