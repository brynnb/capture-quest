import { create } from "zustand";
import { enterWorld, deleteCharacter } from "@/services/authService";
import useGameScreenStore from "./GameScreenStore";
import useChatStore, { MessageType } from "./ChatStore";
import useGameStatusStore from "./GameStatusStore";
import { WorldSocket } from "@/net";

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
  isEnteringWorld: boolean;
  entryError: string | null;
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

type EntryAttempt = {
  characterName: string;
  generation: number;
  promise: Promise<boolean>;
  retire: (closeTransport?: boolean) => void;
};
let activeEntry: EntryAttempt | null = null;

const useCharacterSelectStore = create<CharacterSelectStore>((set, get) => ({
  characters: [],
  selectedCharacter: null,
  isLoading: false,
  isEnteringWorld: false,
  entryError: null,
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

  enterWorld: (characterName: string) => {
    if (activeEntry) {
      return activeEntry.characterName === characterName
        ? activeEntry.promise
        : Promise.resolve(false);
    }
    const selected = get().selectedCharacter;
    if (useGameScreenStore.getState().currentScreen !== "characterSelect"
      || selected?.name !== characterName || !WorldSocket.isConnected) {
      return Promise.resolve(false);
    }

    let retired = false;
    let sent = false;
    let completed = false;
    const subscriptions: Array<() => void> = [];
    const cleanup = () => subscriptions.splice(0).forEach(stop => stop());
    const attempt: EntryAttempt = {
      characterName,
      generation: WorldSocket.sessionGeneration,
      promise: Promise.resolve(false),
      retire: (closeTransport = false) => {
        if (retired || completed) return;
        retired = true;
        cleanup();
        if (activeEntry === attempt) {
          activeEntry = null;
          set({ isEnteringWorld: false, entryError: null });
          if (useGameScreenStore.getState().currentScreen !== "game") {
            useGameStatusStore.getState().setIsMapLoading(false);
          }
        }
        // A sent entry can already own a server character. Retiring that view
        // must release the same transport, never a replacement connection.
        if (closeTransport && sent && WorldSocket.isConnected
          && WorldSocket.sessionGeneration === attempt.generation) {
          WorldSocket.close(false);
        }
      },
    };
    const current = () => activeEntry === attempt && !retired
      && WorldSocket.sessionGeneration === attempt.generation
      && useGameScreenStore.getState().currentScreen === "characterSelect"
      && get().selectedCharacter?.name === characterName;
    activeEntry = attempt;
    subscriptions.push(
      WorldSocket.subscribeSessionRetirement(() => attempt.retire()),
      useGameScreenStore.subscribe(state => {
        if (state.currentScreen !== "characterSelect") attempt.retire(true);
      }),
      useCharacterSelectStore.subscribe(state => {
        if (state.selectedCharacter?.name !== characterName) attempt.retire(true);
      }),
    );
    attempt.promise = (async () => {
      try {
        const { cancelActiveCutscene } = await import("@/phaser-game/services/CutsceneService");
        if (!current()) return false;
        cancelActiveCutscene("entering world");
        sent = true;
        const success = await enterWorld(characterName);
        if (!current()) return false;
        if (!success) {
          set({ entryError: "Could not enter world. Please try again." });
          return false;
        }

        // The server acknowledges only after all required restoration and
        // initial character streams are ready. Retire listeners before our
        // own transition so they cannot mistake it for an abandoned entry.
        completed = true;
        cleanup();
        const chatStore = useChatStore.getState();
        chatStore.clearMessages();
        chatStore.addMessage("Welcome to CaptureQuest!", MessageType.SYSTEM);
        const mapName = useGameStatusStore.getState().getMapNameById(selected.mapId);
        chatStore.addMessage(`You have entered ${mapName}.`, MessageType.ZONE_ENTRY);
        useGameStatusStore.setState({ currentMap: null });
        useGameStatusStore.getState().resetPanelStates();
        useGameStatusStore.getState().setIsMapLoading(false);
        useGameScreenStore.getState().setScreen("game");
        return true;
      } catch {
        if (current()) {
          // An unacknowledged entry can already own a character on the server.
          // Close only this attempt's transport before allowing another login;
          // an untagged late reply must never settle a later entry request.
          if (sent) attempt.retire(true);
          else set({ entryError: "Could not enter world. Please try again." });
        }
        return false;
      } finally {
        cleanup();
        if (activeEntry === attempt) {
          activeEntry = null;
          set({ isEnteringWorld: false });
          if (!completed && useGameScreenStore.getState().currentScreen !== "game") {
            useGameStatusStore.getState().setIsMapLoading(false);
          }
        }
      }
    })();
    set({ isEnteringWorld: true, entryError: null });
    useGameStatusStore.getState().setIsMapLoading(true);
    return attempt.promise;
  },

  deleteCharacter: async (characterName: string) => {
    const success = await deleteCharacter(characterName);
    if (success) {
      get().removeCharacter(characterName);
    }
    return success;
  },

  reset: () => {
    activeEntry?.retire(true);
    set({
      characters: [],
      selectedCharacter: null,
      isLoading: false,
      isEnteringWorld: false,
      entryError: null,
    });
  },
}));

export default useCharacterSelectStore;
