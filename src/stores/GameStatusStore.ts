import { create } from "zustand";
import { persist, subscribeWithSelector } from "zustand/middleware";
import { MapData } from "@/services/characterService";
import { WorldSocket, OpCodes } from "@/net";
import { OptionId } from "@/constants/optionId";
import { displayLocationNameForMap } from "@utils/locationNames";

interface GameStatusStore {
  maps: MapData[];
  currentMap: number | null;

  isMuted: boolean;
  sfxVolume: number;
  ambientVolume: number;
  musicVolume: number;
  currentVideoIndex: number;
  setInitialVideoIndex: (index: number) => void;
  cycleVideo: () => void;
  toggleMute: () => void;
  setSFXVolume: (volume: number) => void;
  setAmbientVolume: (volume: number) => void;
  setMusicVolume: (volume: number) => void;

  initializeMaps: (forceReload?: boolean) => Promise<void>;
  getMapNameById: (id: number) => string | undefined;
  getMapIdByName: (name: string) => number | undefined;
  setCurrentMap: (mapId: number | null) => Promise<void>;

  isInventoryOpen: boolean;
  toggleInventory: () => void;
  isPokedexOpen: boolean;
  togglePokedex: () => void;
  isTrainerCardOpen: boolean;
  toggleTrainerCard: () => void;
  isOptionsOpen: boolean;
  toggleOptions: () => void;
  isHelpOpen: boolean;
  toggleHelp: () => void;
  isGroupOpen: boolean;
  toggleGroup: () => void;

  syncOptions: (options: any) => void;
  resetPanelStates: () => void;

  isMapLoading: boolean;
  setIsMapLoading: (isLoading: boolean) => void;
  uiScale: number;
  setUIScale: (scale: number) => void;
  isCameraFollowEnabled: boolean;
  setCameraFollowEnabled: (enabled: boolean) => void;
  toggleCameraFollow: () => void;
  isWarpMode: boolean;
  setWarpMode: (enabled: boolean) => void;
  toggleWarpMode: () => void;
  isTileManagerOpen: boolean;
  toggleTileManager: () => void;
  isArtStudioOpen: boolean;
  toggleArtStudio: () => void;
  pendingBlackoutWarp: { mapId: number; x: number; y: number } | null;
  triggerBlackoutWarp: (mapId: number, x: number, y: number) => void;
  clearBlackoutWarp: () => void;
  allowTrainerRebattles: boolean;
  toggleAllowTrainerRebattles: () => void;
}

const useGameStatusStore = create<GameStatusStore>()(
  subscribeWithSelector(
    persist(
      (set, get) => ({
        maps: [],
        currentMap: null,

        isMuted: false,
        sfxVolume: 0.25,
        ambientVolume: 0.25,
        musicVolume: 1.0,
        currentVideoIndex: -1,

        setInitialVideoIndex: (index) => {
          if (get().currentVideoIndex === -1) {
            set({ currentVideoIndex: index });
          }
        },
        cycleVideo: () => {
          set((state) => ({ currentVideoIndex: state.currentVideoIndex + 1 }));
        },
        toggleMute: () => {
          set((state) => ({ isMuted: !state.isMuted }));
        },
        setSFXVolume: (volume) => set({ sfxVolume: volume }),
        setAmbientVolume: (volume) => set({ ambientVolume: volume }),
        setMusicVolume: (volume) => set({ musicVolume: volume }),

        initializeMaps: async (forceReload = false) => {
          if (!forceReload && get().maps.length > 0) return;

          try {
            const staticDataStore = await import("./StaticDataStore");
            const staticData = staticDataStore.default.getState();

            if (staticData.isLoaded && staticData.maps.length > 0) {
              set({ maps: staticData.maps });
              return;
            }

            console.warn(
              "[GameStatusStore] Static data not loaded yet, maps unavailable",
            );
          } catch (error) {
            console.error("Error initializing maps:", error);
          }
        },
        getMapNameById: (id) => displayLocationNameForMap(id, get().maps),
        getMapIdByName: (name) => get().maps.find((m) => m.name === name)?.id,
        setCurrentMap: async (mapId) => {
          set({ currentMap: mapId });
        },

        isInventoryOpen: false,
        toggleInventory: () => {
          set((state) => {
            const nextValue = !state.isInventoryOpen;
            return {
              isInventoryOpen: nextValue,
            };
          });
        },
        isPokedexOpen: false,
        togglePokedex: () => {
          set((state) => {
            const nextValue = !state.isPokedexOpen;
            return {
              isPokedexOpen: nextValue,
              isTrainerCardOpen: false,
            };
          });
        },
        isTrainerCardOpen: false,
        toggleTrainerCard: () => {
          set((state) => {
            const nextValue = !state.isTrainerCardOpen;
            return {
              isTrainerCardOpen: nextValue,
              isPokedexOpen: false,
            };
          });
        },
        isOptionsOpen: false,
        toggleOptions: () => {
          set((state) => {
            const nextValue = !state.isOptionsOpen;
            return {
              isOptionsOpen: nextValue,
              isHelpOpen: false,
            };
          });
        },
        isHelpOpen: false,
        toggleHelp: () => {
          set((state) => {
            const nextValue = !state.isHelpOpen;
            return {
              isHelpOpen: nextValue,
              isOptionsOpen: false,
            };
          });
        },
        isGroupOpen: true,
        toggleGroup: () => {
          set((state) => ({ isGroupOpen: !state.isGroupOpen }));
        },

        syncOptions: (options: any) => {
          if (!options) return;
          set({
            allowTrainerRebattles: !!options.allowTrainerRebattles,
          });
        },
        resetPanelStates: () => {
          set({
            isInventoryOpen: false,
            isPokedexOpen: false,
            isTrainerCardOpen: false,
            isOptionsOpen: false,
            isHelpOpen: false,
            isGroupOpen: true,
            isWarpMode: false,
            isTileManagerOpen: false,
            isArtStudioOpen: false,
          });
        },

        isMapLoading: false,
        setIsMapLoading: (isLoading) => set({ isMapLoading: isLoading }),
        uiScale: 1,
        setUIScale: (scale) => set({ uiScale: scale }),
        isCameraFollowEnabled: true,
        setCameraFollowEnabled: (enabled) => {
          set({ isCameraFollowEnabled: enabled });
        },
        toggleCameraFollow: () => {
          set((state) => ({
            isCameraFollowEnabled: !state.isCameraFollowEnabled,
          }));
        },
        isWarpMode: false,
        setWarpMode: (enabled) => {
          set({ isWarpMode: enabled });
        },
        toggleWarpMode: () => {
          set((state) => ({ isWarpMode: !state.isWarpMode }));
        },
        isTileManagerOpen: false,
        toggleTileManager: () => {
          set((state) => {
            const nextValue = !state.isTileManagerOpen;
            return {
              isTileManagerOpen: nextValue,
              isOptionsOpen: nextValue ? false : state.isOptionsOpen,
            };
          });
        },
        isArtStudioOpen: false,
        toggleArtStudio: () => {
          set((state) => ({ isArtStudioOpen: !state.isArtStudioOpen }));
        },
        pendingBlackoutWarp: null,
        triggerBlackoutWarp: (mapId, x, y) => {
          set({ pendingBlackoutWarp: { mapId, x, y } });
        },
        clearBlackoutWarp: () => {
          set({ pendingBlackoutWarp: null });
        },
        allowTrainerRebattles: false,
        toggleAllowTrainerRebattles: () => {
          const newVal = !get().allowTrainerRebattles;
          set({ allowTrainerRebattles: newVal });
          WorldSocket.sendJsonMessage(OpCodes.SetOption, {
            optionId: OptionId.AllowTrainerRebattles,
            value: newVal ? 1 : 0,
          });
        },
      }),
      {
        name: "game-status-storage",
        partialize: (state) => ({
          sfxVolume: state.sfxVolume,
          ambientVolume: state.ambientVolume,
          musicVolume: state.musicVolume,
          isMuted: state.isMuted,
        }),
      },
    ),
  ),
);

export default useGameStatusStore;
