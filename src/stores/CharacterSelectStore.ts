import { create } from "zustand";
import { enterWorld, deleteCharacter } from "@/services/authService";
import useGameScreenStore from "./GameScreenStore";
import useChatStore, { MessageType } from "./ChatStore";
import useGameStatusStore from "./GameStatusStore";

export interface CharacterSelectEntry {
  id: number;
  name: string;
  factionId: number;
  class: number;
  mapId: number;
  zoneId?: number;
  gender: number;
  lastLogin: number;
}

interface CharacterSelectStore {
  characters: CharacterSelectEntry[];
  selectedCharacter: CharacterSelectEntry | null;
  isLoading: boolean;
  pendingSelectName: string | null; // Name of character to auto-select after creation

  setCharacters: (characters: CharacterSelectEntry[]) => void;
  setSelectedCharacter: (character: CharacterSelectEntry | null) => void;
  setIsLoading: (loading: boolean) => void;
  removeCharacter: (characterName: string) => void;
  setPendingSelectName: (name: string | null) => void;
  enterWorld: (characterName: string) => Promise<boolean>;
  deleteCharacter: (characterName: string) => Promise<boolean>;
  reset: () => void;
}

const useCharacterSelectStore = create<CharacterSelectStore>((set, get) => ({
  characters: [],
  selectedCharacter: null,
  isLoading: false,
  pendingSelectName: null,

  setCharacters: (characters) => {
    const { pendingSelectName } = get();
    const normalized = characters.map((character) => ({
      ...character,
      factionId: character.factionId ?? 0,
      mapId: character.mapId ?? character.zoneId ?? 0,
    }));

    // Sort by lastLogin descending (newest first, oldest at bottom)
    const sorted = normalized.sort(
      (a, b) => (b.lastLogin || 0) - (a.lastLogin || 0),
    );

    // If we have a pending character name to select (from character creation), find and select it
    let selectedChar: CharacterSelectEntry | null = null;
    if (pendingSelectName) {
      selectedChar = sorted.find((c) => c.name === pendingSelectName) || null;
    }
    // Otherwise auto-select first character (most recently created/logged in)
    if (!selectedChar && sorted.length > 0) {
      selectedChar = sorted[0];
    }

    set({
      characters: sorted,
      selectedCharacter: selectedChar,
      pendingSelectName: null, // Clear the pending name after use
    });
  },

  setPendingSelectName: (name) => set({ pendingSelectName: name }),

  setSelectedCharacter: (character) => set({ selectedCharacter: character }),

  setIsLoading: (loading) => set({ isLoading: loading }),

  removeCharacter: (characterName) => {
    const { characters, selectedCharacter } = get();
    const filtered = characters.filter((c) => c.name !== characterName);
    set({
      characters: filtered,
      // Clear selection if deleted character was selected
      selectedCharacter:
        selectedCharacter?.name === characterName
          ? filtered.length > 0
            ? filtered[0]
            : null
          : selectedCharacter,
    });
  },

  enterWorld: async (characterName: string) => {
    const { cancelActiveCutscene } = await import(
      "@/phaser-game/services/CutsceneService"
    );
    cancelActiveCutscene("entering world");

    const success = await enterWorld(characterName);
    if (!success) return false;

    // After PostEnterWorld success, the server will send CharacterState.
    // We can rely on NetworkBridge routing it to PlayerCharacterStore.
    // But we need to handle the UI transition once we have the initial data.

    // We'll set a small timeout or wait for the store to have data,
    // but better yet, we just trigger the UI side effects here that are and
    // then switch screens once we know the handshake is complete.

    // Clear chat and add welcome messages
    const chatStore = useChatStore.getState();
    chatStore.clearMessages();
    chatStore.addMessage("Welcome to CaptureQuest!", MessageType.SYSTEM);

    // Add zone entry message
    const { selectedCharacter } = get();
    if (selectedCharacter) {
      const mapName = useGameStatusStore
        .getState()
        .getMapNameById(selectedCharacter.mapId);
      chatStore.addMessage(
        `You have entered ${mapName}.`,
        MessageType.ZONE_ENTRY,
      );
    }

    // Reset UI panel states
    useGameStatusStore.setState({ currentMap: null });
    useGameStatusStore.getState().resetPanelStates();

    // Switch to game screen
    useGameScreenStore.getState().setScreen("game");

    // Clear the map loading state since Phaser handles its own loading
    useGameStatusStore.getState().setIsMapLoading(false);

    return true;
  },

  deleteCharacter: async (characterName: string) => {
    const success = await deleteCharacter(characterName);
    if (success) {
      get().removeCharacter(characterName);
    }
    return success;
  },

  reset: () =>
    set({
      characters: [],
      selectedCharacter: null,
      isLoading: false,
    }),
}));

export default useCharacterSelectStore;
