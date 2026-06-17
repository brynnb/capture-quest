import { create } from "zustand";
import CharacterProfile from "@entities/CharacterProfile";
import { Item } from "@entities/Item";
import useChatStore, { MessageType } from "./ChatStore";
import useGameStatusStore from "./GameStatusStore";
import * as ModelTypes from "@/net/generated/models";
import useStaticDataStore from "./StaticDataStore";

function createDefaultCharacterProfile(): CharacterProfile {
  return {};
}

interface PlayerCharacterStore {
  characterProfile: CharacterProfile;
  characterProfileLoaded: boolean;
  setCharacterProfile: (profile: CharacterProfile) => void;
  hoveredItem: Item | null;
  setHoveredItem: (item: Item | null) => void;
  setCharacterMap: (mapId: number) => void;
  handleCharacterData: (data: ModelTypes.CharacterData) => void;
  handleCharacterWalletData: (wallet: ModelTypes.CharacterWallet) => void;
  handleCharacterBindData: (bind: ModelTypes.CharacterBind) => void;
}

const usePlayerCharacterStore = create<PlayerCharacterStore>()((set, get) => ({
  characterProfile: createDefaultCharacterProfile(),
  characterProfileLoaded: false,
  setCharacterProfile: (profile) => {
    set({
      characterProfile: {
        ...createDefaultCharacterProfile(),
        ...profile,
      },
    });
  },
  hoveredItem: null,
  setHoveredItem: (item) => set({ hoveredItem: item }),
  setCharacterMap: (mapId) => {
    const mapName = useGameStatusStore.getState().getMapNameById(mapId);
    useChatStore
      .getState()
      .addMessage(`Traveling to ${mapName}`, MessageType.ZONE_ENTRY);
    useGameStatusStore.getState().setCurrentMap(mapId);
    set((state) => ({
      characterProfile: {
        ...state.characterProfile,
        mapId,
        zoneId: mapId,
      },
    }));
  },
  handleCharacterData: (charData: ModelTypes.CharacterData) => {
    if (!charData) return;

    if (charData.options) {
      useGameStatusStore.getState().syncOptions(charData.options as string);
    }

    const sd = useStaticDataStore.getState();
    const factionData = (sd as any).factions?.find(
      (f: any) => f.id === charData.factionId,
    );
    const classData = (sd as any).classes?.find(
      (c: any) => c.id === charData.class,
    );
    const oldMapId =
      get().characterProfile.mapId ?? get().characterProfile.zoneId;
    const newMapId = charData.mapId;

    if (newMapId !== undefined && oldMapId !== newMapId) {
      useGameStatusStore.getState().setCurrentMap(newMapId);
    }

    set((state) => ({
      characterProfile: {
        ...state.characterProfile,
        ...(charData as any),
        mapId: charData.mapId,
        zoneId: charData.mapId,
        faction: factionData,
        class: classData,
      },
    }));
  },

  handleCharacterWalletData: (wallet: ModelTypes.CharacterWallet) => {
    if (!wallet) return;

    set((state) => ({
      characterProfile: {
        ...state.characterProfile,
        pokedollars: wallet.pokedollars,
      },
    }));
  },

  handleCharacterBindData: (b: ModelTypes.CharacterBind) => {
    if (!b) return;

    set((state) => ({
      characterProfile: {
        ...state.characterProfile,
        bind: b,
      },
    }));
  },
}));

if (typeof window !== "undefined") {
  (window as any).__PLAYER_STORE__ = usePlayerCharacterStore;
}

export default usePlayerCharacterStore;
