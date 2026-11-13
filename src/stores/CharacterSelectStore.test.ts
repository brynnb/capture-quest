import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  enter: vi.fn(),
  cancelCutscene: vi.fn(),
  addMessage: vi.fn(),
  clearMessages: vi.fn(),
  resetPanels: vi.fn(),
  listeners: new Set<() => void>(),
  socket: { isConnected: true, sessionGeneration: 1, close: vi.fn(), subscribeSessionRetirement: vi.fn() },
}));
vi.mock("@/net", () => ({ WorldSocket: mocks.socket }));
vi.mock("@/services/authService", () => ({ enterWorld: mocks.enter, deleteCharacter: vi.fn() }));
vi.mock("@/phaser-game/services/CutsceneService", () => ({ cancelActiveCutscene: mocks.cancelCutscene }));
vi.mock("./ChatStore", () => ({
  default: { getState: () => ({ addMessage: mocks.addMessage, clearMessages: mocks.clearMessages }) },
  MessageType: { SYSTEM: "system", ZONE_ENTRY: "zone" },
}));
vi.mock("./GameStatusStore", async () => {
  const { create } = await import("zustand");
  return { default: create<{ isMapLoading: boolean; currentMap: number | null; setIsMapLoading: (value: boolean) => void; getMapNameById: (id: number) => string; resetPanelStates: () => void }>(set => ({
    isMapLoading: false, currentMap: 1,
    setIsMapLoading: value => set({ isMapLoading: value }),
    getMapNameById: id => `Map ${id}`, resetPanelStates: mocks.resetPanels,
  })) };
});

import useCharacterSelectStore, { type CharacterSelectEntry } from "./CharacterSelectStore";
import useGameScreenStore from "./GameScreenStore";
import useGameStatusStore from "./GameStatusStore";

const character: CharacterSelectEntry = { id: 42, name: "Entrytester", factionId: 1, class: 1, mapId: 1, gender: 0, lastLogin: 2 };
const other: CharacterSelectEntry = { ...character, id: 43, name: "Otherentry", lastLogin: 1 };
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function retireTransport() {
  mocks.socket.isConnected = false;
  mocks.socket.sessionGeneration++;
  for (const receive of [...mocks.listeners]) receive();
}

beforeEach(() => {
  useCharacterSelectStore.getState().reset();
  mocks.listeners.clear();
  vi.clearAllMocks();
  mocks.socket.isConnected = true;
  mocks.socket.sessionGeneration = 1;
  mocks.socket.close.mockImplementation(retireTransport);
  mocks.socket.subscribeSessionRetirement.mockImplementation((receive: () => void) => {
    mocks.listeners.add(receive);
    return () => mocks.listeners.delete(receive);
  });
  useGameScreenStore.getState().setScreen("characterSelect");
  useGameStatusStore.setState({ isMapLoading: false, currentMap: 1 });
  useCharacterSelectStore.getState().setCharacters([character, other]);
});

describe("owned character entry", () => {
  it("coalesces duplicate entry and switches screens only after confirmed success", async () => {
    const reply = deferred<boolean>(); mocks.enter.mockReturnValue(reply.promise);
    const first = useCharacterSelectStore.getState().enterWorld(character.name);
    const second = useCharacterSelectStore.getState().enterWorld(character.name);
    expect(second).toBe(first);
    expect(useCharacterSelectStore.getState().isEnteringWorld).toBe(true);
    expect(useGameStatusStore.getState().isMapLoading).toBe(true);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
    reply.resolve(true);
    await expect(first).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("game");
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(useCharacterSelectStore.getState().isEnteringWorld).toBe(false);
    expect(mocks.socket.close).not.toHaveBeenCalled();
    expect(mocks.clearMessages).toHaveBeenCalledTimes(1);
    expect(mocks.listeners.size).toBe(0);
  });

  it("settles an explicit rejection, clears loading and permits an explicit retry", async () => {
    mocks.enter.mockResolvedValueOnce(false);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({ isEnteringWorld: false, entryError: "Could not enter world. Please try again." });
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
    expect(mocks.clearMessages).not.toHaveBeenCalled();
    mocks.enter.mockResolvedValueOnce(true);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(true);
    expect(useCharacterSelectStore.getState().entryError).toBeNull();
  });

  it("retires an unacknowledged entry before a retry can reuse its transport", async () => {
    mocks.enter.mockRejectedValueOnce(new Error("Request timed out"));
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    expect(mocks.socket.close).toHaveBeenCalledOnce();
    expect(mocks.socket.close).toHaveBeenCalledWith(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({ isEnteringWorld: false, entryError: null });
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(mocks.clearMessages).not.toHaveBeenCalled();
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    expect(mocks.enter).toHaveBeenCalledOnce();
    mocks.socket.isConnected = true;
    mocks.enter.mockResolvedValueOnce(true);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(true);
  });

  it("does not dispatch when the selection is abandoned before asynchronous preparation finishes", async () => {
    const result = useCharacterSelectStore.getState().enterWorld(character.name);
    useGameScreenStore.getState().setScreen("title");
    await expect(result).resolves.toBe(false);
    expect(mocks.enter).not.toHaveBeenCalled();
    expect(mocks.socket.close).not.toHaveBeenCalled();
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(useGameScreenStore.getState().currentScreen).toBe("title");
  });

  it.each(["screen", "selection", "reset"] as const)("retires a sent entry on %s replacement without publishing its late success", async cause => {
    const reply = deferred<boolean>(); mocks.enter.mockReturnValue(reply.promise);
    const result = useCharacterSelectStore.getState().enterWorld(character.name);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    if (cause === "screen") useGameScreenStore.getState().setScreen("register");
    else if (cause === "selection") useCharacterSelectStore.getState().setSelectedCharacter(other);
    else useCharacterSelectStore.getState().reset();
    expect(mocks.socket.close).toHaveBeenCalledOnce();
    expect(mocks.socket.close).toHaveBeenCalledWith(false);
    reply.resolve(true);
    await expect(result).resolves.toBe(false);
    expect(useGameScreenStore.getState().currentScreen).not.toBe("game");
    expect(mocks.clearMessages).not.toHaveBeenCalled();
    expect(useCharacterSelectStore.getState().entryError).toBeNull();
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(mocks.listeners.size).toBe(0);
  });

  it("an old failure cannot clear a replacement entry or close its new transport", async () => {
    const old = deferred<boolean>(); const next = deferred<boolean>();
    mocks.enter.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise);
    const first = useCharacterSelectStore.getState().enterWorld(character.name);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    retireTransport();
    mocks.socket.isConnected = true;
    const second = useCharacterSelectStore.getState().enterWorld(character.name);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(2));
    old.reject(new Error("Old connection failed"));
    await expect(first).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({ isEnteringWorld: true, entryError: null });
    expect(useGameStatusStore.getState().isMapLoading).toBe(true);
    expect(mocks.socket.close).not.toHaveBeenCalled();
    next.resolve(true); await expect(second).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("game");
  });

  it("rejects another character while the selected entry is pending without a second command", async () => {
    const reply = deferred<boolean>(); mocks.enter.mockReturnValue(reply.promise);
    const first = useCharacterSelectStore.getState().enterWorld(character.name);
    await expect(useCharacterSelectStore.getState().enterWorld(other.name)).resolves.toBe(false);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    reply.resolve(true); await first;
  });
});
