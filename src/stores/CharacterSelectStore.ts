import { create } from "zustand";
import {
  enterWorld, deleteCharacter as requestCharacterDeletion, createCharacter as requestCharacterCreation,
  connectForAuthentication, login, loginGuest, registerAccount, readCharacterList,
  type CharacterCreateRequest,
} from "@/services/authService";
import type { CharacterListResponse } from "@/net/generated/world_api";
import { CorrelatedResponseError } from "@/phaser-game/services/CorrelatedRequest";
import type { GameScreen } from "./GameScreenStore";
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

export type AuthenticationRequest = { kind: "guest"; token: string } | { kind: "login" | "register"; email: string; password: string };
export type AccountOperation = "authenticate" | "create" | "delete" | "roster";

interface CharacterSelectStore {
  characters: CharacterSelectEntry[];
  selectedCharacter: CharacterSelectEntry | null;
  isLoading: boolean;
  isEnteringWorld: boolean;
  entryError: string | null;
  pendingSelectName: string | null;
  accountOperation: AccountOperation | null;
  accountGeneration: number | null;
  accountId: number | null;
  rosterReady: boolean;
  accountError: string | null;
  rosterError: string | null;
  createError: string | null;
  deleteError: string | null;

  setCharacters: (characters: CharacterSelectEntry[]) => void;
  setSelectedCharacter: (character: CharacterSelectEntry | null) => void;
  setIsLoading: (loading: boolean) => void;
  setPendingSelectName: (name: string | null) => void;
  enterWorld: (characterName: string) => Promise<boolean>;
  authenticate: (request: AuthenticationRequest) => Promise<boolean>;
  refreshCharacters: () => Promise<boolean>;
  createCharacter: (request: CharacterCreateRequest) => Promise<boolean>;
  deleteCharacter: (characterId: number) => Promise<boolean>;
  logout: () => void;
  reset: () => void;
}

type EntryAttempt = {
  kind: "entry";
  characterName: string;
  generation: number;
  promise: Promise<boolean>;
  retire: (closeTransport?: boolean) => void;
};
type AccountAttempt = {
  kind: AccountOperation;
  generation: number;
  screen: GameScreen;
  controller: AbortController;
  promise: Promise<boolean>;
  closeOnRetire: boolean;
  replacingSession: boolean;
  current: () => boolean;
  retire: (closeTransport?: boolean) => void;
  complete: (screen?: GameScreen) => void;
  replaceSession: <T>(start: () => T) => T;
};
// Entry and account actions share one admission slot. No account mutation can
// overlap entry or another mutation, including after a component remount.
let activeOperation: EntryAttempt | AccountAttempt | null = null;

const useCharacterSelectStore = create<CharacterSelectStore>((set, get) => ({
  characters: [],
  selectedCharacter: null,
  isLoading: false,
  isEnteringWorld: false,
  entryError: null,
  pendingSelectName: null,
  accountOperation: null,
  accountGeneration: null,
  accountId: null,
  rosterReady: false,
  accountError: null,
  rosterError: null,
  createError: null,
  deleteError: null,

  setCharacters: (characters) => {
    const { pendingSelectName, selectedCharacter } = get();
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
    if (!selectedChar && selectedCharacter) selectedChar = sorted.find(character => character.id === selectedCharacter.id) ?? null;
    // Otherwise auto-select first character (most recently created/logged in)
    if (!selectedChar && sorted.length > 0) {
      selectedChar = sorted[0];
    }

    set({
      characters: sorted,
      rosterReady: true,
      selectedCharacter: selectedChar,
      pendingSelectName: null, // Clear the pending name after use
    });
  },

  setPendingSelectName: (name) => set({ pendingSelectName: name }),

  setSelectedCharacter: (character) => set({ selectedCharacter: character }),

  setIsLoading: (loading) => set({ isLoading: loading }),

  enterWorld: (characterName: string) => {
    if (activeOperation) {
      return activeOperation.kind === "entry" && activeOperation.characterName === characterName
        ? activeOperation.promise
        : Promise.resolve(false);
    }
    const selected = get().selectedCharacter;
    if (useGameScreenStore.getState().currentScreen !== "characterSelect"
      || selected?.name !== characterName || !get().rosterReady || !WorldSocket.isConnected) {
      return Promise.resolve(false);
    }

    let retired = false;
    let sent = false;
    let completed = false;
    const subscriptions: Array<() => void> = [];
    const cleanup = () => subscriptions.splice(0).forEach(stop => stop());
    const attempt: EntryAttempt = {
      kind: "entry",
      characterName,
      generation: WorldSocket.sessionGeneration,
      promise: Promise.resolve(false),
      retire: (closeTransport = false) => {
        if (retired || completed) return;
        retired = true;
        cleanup();
        if (activeOperation === attempt) {
          activeOperation = null;
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
    const current = () => activeOperation === attempt && !retired
      && WorldSocket.sessionGeneration === attempt.generation
      && useGameScreenStore.getState().currentScreen === "characterSelect"
      && get().selectedCharacter?.name === characterName;
    activeOperation = attempt;
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
        // The acknowledgement owns the server character, but the browser still
        // needs terrain, textures and the current world snapshot. MapLoader
        // releases the loading screen only after that presentation is ready.
        set({ rosterReady: false });
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
        if (activeOperation === attempt) {
          activeOperation = null;
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

  authenticate: request => authenticateAccount(request),
  refreshCharacters: () => refreshCharacters(),
  createCharacter: request => createOwnedCharacter(request),
  deleteCharacter: characterId => deleteOwnedCharacter(characterId),
  logout: () => {
    get().reset();
    WorldSocket.close(false);
    useGameScreenStore.getState().setScreen("title");
  },

  reset: () => {
    activeOperation?.retire(true);
    set({
      characters: [], selectedCharacter: null, pendingSelectName: null,
      isLoading: false, isEnteringWorld: false, entryError: null,
      accountOperation: null, accountGeneration: null, accountId: null,
      rosterReady: false, accountError: null, rosterError: null, createError: null, deleteError: null,
    });
  },
}));

export default useCharacterSelectStore;


// Account operations use the same admission slot as entry and the existing
// correlated request helper. A retired screen cannot publish into its successor.
function runAccountOperation(kind: AccountOperation, work: (owner: AccountAttempt) => Promise<boolean>): Promise<boolean> {
  const screen = useGameScreenStore.getState().currentScreen;
  const state = useCharacterSelectStore.getState();
  const allowed = kind === "authenticate"
    ? ["title", "login", "register"].includes(screen)
    : screen === (kind === "create" ? "characterCreate" : "characterSelect");
  if (activeOperation || !allowed) return Promise.resolve(false);
  if (kind !== "authenticate" && (!WorldSocket.isConnected || state.accountGeneration !== WorldSocket.sessionGeneration)) return Promise.resolve(false);
  const subscriptions: Array<() => void> = [];
  let retired = false;
  const cleanup = () => subscriptions.splice(0).forEach(stop => stop());
  const owner: AccountAttempt = {
    kind, screen, generation: WorldSocket.sessionGeneration,
    controller: new AbortController(), promise: Promise.resolve(false),
    closeOnRetire: false, replacingSession: false,
    current: () => activeOperation === owner && !retired && !owner.controller.signal.aborted
      && WorldSocket.sessionGeneration === owner.generation
      && useGameScreenStore.getState().currentScreen === screen,
    retire: (closeTransport = false) => {
      if (retired) return;
      retired = true;
      cleanup();
      owner.controller.abort();
      if (activeOperation === owner) {
        activeOperation = null;
        useCharacterSelectStore.setState({accountOperation: null, isLoading: false});
      }
      // Authentication/creation still use untagged legacy replies. Abandoning
      // a sent request retires only that connection before any later attempt.
      if (closeTransport && owner.closeOnRetire && WorldSocket.sessionGeneration === owner.generation) WorldSocket.close(false);
    },
    complete: nextScreen => {
      if (activeOperation !== owner || retired) return;
      cleanup();
      activeOperation = null;
      useCharacterSelectStore.setState({accountOperation: null, isLoading: false});
      if (nextScreen) useGameScreenStore.getState().setScreen(nextScreen);
    },
    replaceSession: start => {
      owner.replacingSession = true;
      try { return start(); }
      finally { owner.generation = WorldSocket.sessionGeneration; owner.replacingSession = false; }
    },
  };
  activeOperation = owner;
  subscriptions.push(
    WorldSocket.subscribeSessionRetirement(() => {
      if (owner.replacingSession) { owner.generation = WorldSocket.sessionGeneration; return; }
      owner.retire();
      useCharacterSelectStore.setState({accountGeneration: null, accountId: null, rosterReady: false, characters: [], selectedCharacter: null, pendingSelectName: null});
    }),
    useGameScreenStore.subscribe(value => { if (value.currentScreen !== screen) owner.retire(true); }),
  );
  // Publish admission before any synchronous subscriber can start another action.
  owner.promise = Promise.resolve().then(async () => {
    if (!owner.current()) return false;
    try { return await work(owner); }
    catch (error) {
      if (owner.current()) {
        const message = error instanceof Error ? error.message : "Account request could not be completed.";
        useCharacterSelectStore.setState(kind === "authenticate" ? {accountError: message}
          : kind === "create" ? {createError: message}
          : kind === "delete" ? {deleteError: message}
          : {rosterError: "Characters could not be loaded. Please try again.", rosterReady: false});
      }
      return false;
    } finally { owner.complete(); }
  });
  useCharacterSelectStore.setState({
    accountOperation: kind, isLoading: kind === "authenticate" || kind === "roster",
    ...(kind === "authenticate" ? {accountError: null, rosterError: null, createError: null, deleteError: null, accountGeneration: null, accountId: null, rosterReady: false, characters: [], selectedCharacter: null, pendingSelectName: null}
      : kind === "create" ? {createError: null}
      : kind === "delete" ? {deleteError: null}
      : {rosterError: null}),
  });
  return owner.promise;
}

function applyOwnedRoster(owner: AccountAttempt, reply: CharacterListResponse, preferredName?: string): void {
  const state = useCharacterSelectStore.getState();
  if (!owner.current()) throw new DOMException("Account operation retired", "AbortError");
  if (!Number.isSafeInteger(reply.accountId) || reply.accountId <= 0
    || (state.accountId !== null && state.accountId !== reply.accountId)
    || !Array.isArray(reply.characters)
    || reply.characters.some(character => !Number.isSafeInteger(character.id) || character.id <= 0
      || typeof character.name !== "string" || !character.name
      || ![character.factionId, character.class, character.mapId, character.gender, character.lastLogin].every(Number.isSafeInteger))
    || new Set(reply.characters.map(character => character.id)).size !== reply.characters.length) {
    throw new Error("Invalid current character list");
  }
  if (preferredName) state.setPendingSelectName(preferredName);
  state.setCharacters(reply.characters);
  useCharacterSelectStore.setState({accountId: reply.accountId, accountGeneration: owner.generation, rosterReady: true, rosterError: null});
}

async function readOwnedRoster(owner: AccountAttempt, preferredName?: string): Promise<void> {
  const reply = await readCharacterList(owner.controller.signal);
  applyOwnedRoster(owner, reply, preferredName);
}

function authenticateAccount(request: AuthenticationRequest): Promise<boolean> {
  return runAccountOperation("authenticate", async owner => {
    owner.closeOnRetire = true;
    const connected = await owner.replaceSession(connectForAuthentication);
    if (!owner.current()) return false;
    if (!connected) throw new Error("Could not connect. Please try again.");
    const response = owner.replaceSession(() => request.kind === "guest"
      ? loginGuest(request.token)
      : request.kind === "register" ? registerAccount(request.email, request.password)
      : login(request.email, request.password));
    await response;
    if (!owner.current()) return false;
    useCharacterSelectStore.setState({accountGeneration: owner.generation});
    try { await readOwnedRoster(owner); }
    catch {
      if (!owner.current()) return false;
      useCharacterSelectStore.setState({rosterReady: false, rosterError: "Characters could not be loaded. Please try again."});
    }
    if (!owner.current()) return false;
    // Authentication is settled even when the separate bounded roster read
    // fails. The selection screen offers a read Retry without authenticating again.
    owner.complete("characterSelect");
    return true;
  });
}

function refreshCharacters(): Promise<boolean> {
  return runAccountOperation("roster", async owner => {
    await readOwnedRoster(owner);
    return owner.current();
  });
}

function createOwnedCharacter(request: CharacterCreateRequest): Promise<boolean> {
  if (!useCharacterSelectStore.getState().rosterReady) return Promise.resolve(false);
  const payload = {...request};
  return runAccountOperation("create", async owner => {
    owner.closeOnRetire = true;
    const success = await requestCharacterCreation(payload);
    if (!owner.current()) return false;
    if (!success) {
      useCharacterSelectStore.setState({createError: "Character creation was rejected. Check the name and try again."});
      return false;
    }
    try { await readOwnedRoster(owner, payload.name); }
    catch {
      if (!owner.current()) return false;
      useCharacterSelectStore.setState({rosterReady: false, rosterError: "Characters could not be loaded. Please try again.", pendingSelectName: payload.name});
    }
    if (!owner.current()) return false;
    owner.complete("characterSelect");
    return true;
  });
}

function deleteOwnedCharacter(characterId: number): Promise<boolean> {
  const state = useCharacterSelectStore.getState();
  if (!state.rosterReady || !Number.isSafeInteger(characterId) || characterId <= 0
    || !state.characters.some(character => character.id === characterId)) return Promise.resolve(false);
  return runAccountOperation("delete", async owner => {
    try {
      const reply = await requestCharacterDeletion(characterId, owner.controller.signal);
      if (!owner.current()) return false;
      if (reply.characterId !== characterId || reply.characters.some(character => character.id === characterId)) throw new Error("Invalid character deletion result");
      applyOwnedRoster(owner, reply);
      return true;
    } catch (error) {
      if (!owner.current()) return false;
      if (error instanceof CorrelatedResponseError) {
        // Explicit rejection leaves the current row and selection intact.
        useCharacterSelectStore.setState({deleteError: error.message});
        return false;
      }
      // An unknown result may have committed. Read once; never replay deletion.
      try {
        await readOwnedRoster(owner);
        if (!owner.current()) return false;
        if (!useCharacterSelectStore.getState().characters.some(character => character.id === characterId)) return true;
        useCharacterSelectStore.setState({deleteError: "Deletion could not be confirmed. The character is still listed. Please try again."});
      } catch {
        if (owner.current()) useCharacterSelectStore.setState({
          rosterReady: false, rosterError: "Characters could not be loaded. Please try again.",
          deleteError: "Could not check whether the character was deleted. Retry the character list before another change.",
        });
      }
      return false;
    }
  });
}
