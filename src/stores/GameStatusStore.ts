import { create } from "zustand";
import { persist, subscribeWithSelector } from "zustand/middleware";
import { MapData } from "@/services/characterService";
import { WorldSocket, OpCodes } from "@/net";
import { OptionId } from "@/constants/optionId";
import { displayLocationNameForMap } from "@utils/locationNames";
import type { InstantWarpTarget } from "@/phaser-game/instantWarp";
import AudioManager from "@/services/audio/AudioManager";

interface GameStatusStore {
  maps: MapData[];
  currentMap: number | null;
  playerTileContext: {
    x: number;
    y: number;
    mapId: number;
    direction: string;
  } | null;
  setPlayerTileContext: (context: {
    x: number;
    y: number;
    mapId: number;
    direction: string;
  }) => void;
  clearPlayerTileContext: () => void;

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
  isHudSidebarCollapsed: boolean;
  setHudSidebarCollapsed: (collapsed: boolean) => void;
  toggleHudSidebar: () => void;
  isMobileChatOpen: boolean;
  setMobileChatOpen: (open: boolean) => void;
  toggleMobileChat: () => void;

  syncOptions: (options: string | GameOptions | null | undefined) => void;
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
  pendingInstantWarpTarget: InstantWarpTarget | null;
  setPendingInstantWarpTarget: (target: InstantWarpTarget) => void;
  clearPendingInstantWarpTarget: () => void;
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

interface GameOptions {
  allowTrainerRebattles?: boolean;
}

const useGameStatusStore = create<GameStatusStore>()(
  subscribeWithSelector(
    persist(
      (set, get) => ({
        maps: [],
        currentMap: null,
        playerTileContext: null,
        setPlayerTileContext: (context) => set({ playerTileContext: context }),
        clearPlayerTileContext: () => set({ playerTileContext: null }),

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
          set((state) => {
            const nextMuted = !state.isMuted;
            // Apply the native media mute and resume Web Audio synchronously
            // within the tap/click. iOS may reject a later effect-driven resume
            // because it is no longer part of the user gesture.
            AudioManager.setMuted(nextMuted);
            return { isMuted: nextMuted };
          });
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
              isPokedexOpen: nextValue ? false : state.isPokedexOpen,
              isTrainerCardOpen: nextValue ? false : state.isTrainerCardOpen,
              isOptionsOpen: nextValue ? false : state.isOptionsOpen,
              isHelpOpen: nextValue ? false : state.isHelpOpen,
              isGroupOpen: nextValue ? false : state.isGroupOpen,
              isTileManagerOpen: nextValue ? false : state.isTileManagerOpen,
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
              isInventoryOpen: nextValue ? false : state.isInventoryOpen,
              isOptionsOpen: nextValue ? false : state.isOptionsOpen,
              isHelpOpen: nextValue ? false : state.isHelpOpen,
              isGroupOpen: nextValue ? false : state.isGroupOpen,
              isTileManagerOpen: nextValue ? false : state.isTileManagerOpen,
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
              isInventoryOpen: nextValue ? false : state.isInventoryOpen,
              isOptionsOpen: nextValue ? false : state.isOptionsOpen,
              isHelpOpen: nextValue ? false : state.isHelpOpen,
              isGroupOpen: nextValue ? false : state.isGroupOpen,
              isTileManagerOpen: nextValue ? false : state.isTileManagerOpen,
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
              isInventoryOpen: nextValue ? false : state.isInventoryOpen,
              isPokedexOpen: nextValue ? false : state.isPokedexOpen,
              isTrainerCardOpen: nextValue ? false : state.isTrainerCardOpen,
              isGroupOpen: nextValue ? false : state.isGroupOpen,
              isTileManagerOpen: nextValue ? false : state.isTileManagerOpen,
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
              isInventoryOpen: nextValue ? false : state.isInventoryOpen,
              isPokedexOpen: nextValue ? false : state.isPokedexOpen,
              isTrainerCardOpen: nextValue ? false : state.isTrainerCardOpen,
              isGroupOpen: nextValue ? false : state.isGroupOpen,
              isTileManagerOpen: nextValue ? false : state.isTileManagerOpen,
            };
          });
        },
        isGroupOpen: true,
        toggleGroup: () => {
          set((state) => {
            const nextValue = !state.isGroupOpen;
            return {
              isGroupOpen: nextValue,
              isInventoryOpen: nextValue ? false : state.isInventoryOpen,
              isPokedexOpen: nextValue ? false : state.isPokedexOpen,
              isTrainerCardOpen: nextValue ? false : state.isTrainerCardOpen,
              isOptionsOpen: nextValue ? false : state.isOptionsOpen,
              isHelpOpen: nextValue ? false : state.isHelpOpen,
              isTileManagerOpen: nextValue ? false : state.isTileManagerOpen,
            };
          });
        },
        isHudSidebarCollapsed: false,
        setHudSidebarCollapsed: (collapsed) => {
          set({ isHudSidebarCollapsed: collapsed });
        },
        toggleHudSidebar: () => {
          set((state) => ({
            isHudSidebarCollapsed: !state.isHudSidebarCollapsed,
            isMobileChatOpen: state.isHudSidebarCollapsed
              ? false
              : state.isMobileChatOpen,
          }));
        },
        isMobileChatOpen: false,
        setMobileChatOpen: (open) => {
          set({ isMobileChatOpen: open });
        },
        toggleMobileChat: () => {
          set((state) => ({
            isMobileChatOpen: !state.isMobileChatOpen,
            isHudSidebarCollapsed: state.isMobileChatOpen
              ? state.isHudSidebarCollapsed
              : true,
          }));
        },

        syncOptions: (options) => {
          if (!options) return;
          let parsed: GameOptions;
          if (typeof options === "string") {
            try {
              parsed = JSON.parse(options) as GameOptions;
            } catch {
              console.warn("[GameStatusStore] Ignoring malformed character options");
              return;
            }
          } else {
            parsed = options;
          }
          set({
            allowTrainerRebattles: !!parsed.allowTrainerRebattles,
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
            isMobileChatOpen: false,
            isWarpMode: false,
            pendingInstantWarpTarget: null,
            isTileManagerOpen: false,
            isArtStudioOpen: false,
          });
        },

        isMapLoading: false,
        setIsMapLoading: (isLoading) => {
          set({
            isMapLoading: isLoading,
            ...(isLoading ? { pendingInstantWarpTarget: null } : {}),
          });
        },
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
          set({
            isWarpMode: enabled,
            ...(enabled ? {} : { pendingInstantWarpTarget: null }),
          });
        },
        toggleWarpMode: () => {
          set((state) => ({
            isWarpMode: !state.isWarpMode,
            pendingInstantWarpTarget: null,
          }));
        },
        pendingInstantWarpTarget: null,
        setPendingInstantWarpTarget: (target) => {
          set({ pendingInstantWarpTarget: target });
        },
        clearPendingInstantWarpTarget: () => {
          set({ pendingInstantWarpTarget: null });
        },
        isTileManagerOpen: false,
        toggleTileManager: () => {
          set((state) => {
            const nextValue = !state.isTileManagerOpen;
            return {
              isTileManagerOpen: nextValue,
              isInventoryOpen: nextValue ? false : state.isInventoryOpen,
              isPokedexOpen: nextValue ? false : state.isPokedexOpen,
              isTrainerCardOpen: nextValue ? false : state.isTrainerCardOpen,
              isOptionsOpen: nextValue ? false : state.isOptionsOpen,
              isHelpOpen: nextValue ? false : state.isHelpOpen,
              isGroupOpen: nextValue ? false : state.isGroupOpen,
              isMobileChatOpen: nextValue ? false : state.isMobileChatOpen,
              isWarpMode: nextValue ? false : state.isWarpMode,
              pendingInstantWarpTarget: nextValue
                ? null
                : state.pendingInstantWarpTarget,
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
