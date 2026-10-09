import {bindSafariVisitView} from "./SafariVisitService";
import useGameScreenStore from "@/stores/GameScreenStore";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { GameplayStateResponse } from "@/net/generated/world_api";
const state = vi.hoisted(() => ({
  listeners: new Set<(data: unknown) => void>(), send: vi.fn(), apply: vi.fn(), dispatch: vi.fn(), cutscene: vi.fn(),
  wallet: vi.fn(), flags: vi.fn(),
  character: null as unknown as { characterProfile:{id:number}; handleCharacterWalletData: (wallet: unknown) => void; setEventFlags: (flags: string[]) => void },
  current: null as unknown as { restoreGameplay: (snapshot: unknown) => void },
}));
vi.mock("@/net", () => ({ WorldSocket:{sessionGeneration:0,subscribeSessionRetirement:()=>()=>{}},OpCodes: { TrainerEncounterNotify: 75 } }));
vi.mock("@/stores/PokeBattleStore", () => ({ default: { getState: () => state.current } }));
vi.mock("@/stores/PlayerCharacterStore", () => ({ default: { getState: () => state.character, subscribe:()=>()=>{} } }));
vi.mock("./CutsceneService", () => ({ handleCutsceneStart: state.cutscene }));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onGameplayState: (receive: (data: unknown) => void) => { state.listeners.add(receive); return () => state.listeners.delete(receive); },
  requestGameplayState: state.send, dispatchPhaserResponse: state.dispatch,
}));
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import { applyGameplaySnapshot, applyGameplayResourceSnapshot, recoverGameplayState, readCurrentGameplayState } from "./GameplayRecoveryService";
const snapshot = (requestId: string): GameplayStateResponse => ({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: [], eventFlags: [], success: true, requestId, position: { success: true, requestId, mapId: 50, x: 7, y: 8, direction: "UP", serverMovementPending: false }, battle: null, safari: null, trainer: null, cutscene: null });
const receive = (data: unknown) => state.listeners.forEach(listener => listener(data));
beforeEach(() => { useGameScreenStore.setState({currentScreen:"game"}); useCQInventoryStore.getState().setInventory([], 0); usePokemonPartyStore.getState().clearParty(); state.current = { restoreGameplay: state.apply }; state.character = { characterProfile:{id:42},handleCharacterWalletData: state.wallet, setEventFlags: state.flags }; });
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); state.listeners.clear(); });

test("resource-only publication leaves battle and issued-plan presentation with their owners",()=>{
  const reply=snapshot("resources");
  applyGameplayResourceSnapshot(reply);
  expect(state.wallet).toHaveBeenCalledWith(reply.wallet);
  expect(usePokemonPartyStore.getState().party).toBe(reply.party);
  expect(state.apply).not.toHaveBeenCalled(); expect(state.flags).not.toHaveBeenCalled();
  expect(state.dispatch).not.toHaveBeenCalled(); expect(state.cutscene).not.toHaveBeenCalled();
});

test("owned recovery updates the selected box and source identity without reopening a closed PC", async () => {
  usePokemonPCStore.getState().closePC();
  const run = recoverGameplayState(50);
  const request = state.send.mock.calls[0][0];
  const reply = snapshot(request.requestId);
  reply.pc.currentBox = 3;
  reply.pc.sources = [{id:17,mapId:50,x:15,y:7,direction:"UP"}];
  receive(reply); await run;
  expect(usePokemonPCStore.getState()).toMatchObject({isOpen:false,currentBox:3,sources:reply.pc.sources,boxPokemon:[],party:reply.party});
});

test("a PC box refresh overtaking a gameplay read requires a fresh current snapshot", async () => {
  const run = recoverGameplayState(50);
  const first = state.send.mock.calls[0][0];
  usePokemonPCStore.getState().setBox(2,[]);
  receive(snapshot(first.requestId)); await Promise.resolve(); await Promise.resolve();
  expect(state.apply).not.toHaveBeenCalled(); expect(state.send).toHaveBeenCalledTimes(2);
  const reply = snapshot(state.send.mock.calls[1][0].requestId); reply.pc.currentBox=2;
  receive(reply); await run;
  expect(usePokemonPCStore.getState().currentBox).toBe(2);
});

test("malformed PC identity and a foreign source map cannot publish any gameplay state", () => {
  const missing = snapshot("old-server"); delete (missing as Partial<GameplayStateResponse>).pc;
  expect(()=>applyGameplaySnapshot(missing)).toThrow("Incomplete PC snapshot");
  const invalid = snapshot("invalid-pc"); invalid.pc.currentBox=12;
  expect(()=>applyGameplaySnapshot(invalid)).toThrow("Incomplete PC snapshot");
  const foreign = snapshot("foreign-source"); foreign.pc.sources=[{id:17,mapId:51,x:13,y:3,direction:"UP"}];
  expect(()=>applyGameplaySnapshot(foreign)).toThrow("PC source belongs to another map");
  expect(state.apply).not.toHaveBeenCalled(); expect(state.wallet).not.toHaveBeenCalled();
});

test("one correlated current snapshot clears stale state and redelivers its issued plans", async () => {
  const safari = vi.fn();const stop=bindSafariVisitView(safari);safari.mockClear();
  const result = recoverGameplayState(50);
  const request = state.send.mock.calls[0][0];
  receive(snapshot("obsolete")); expect(state.apply).not.toHaveBeenCalled();
  const reply = snapshot(request.requestId);
  reply.trainer = { encounterToken: "issued", trainerActorId: 17, trainerX: 7, trainerY: 6, playerX: 7, playerY: 8, approachToX: 7, approachToY: 7, walkToX: 7, walkToY: 8, trainerClass: "TEST", trainerName: "Trainer" };
  receive(reply); await result;
  expect(state.apply).toHaveBeenCalledOnce(); expect(state.apply).toHaveBeenCalledWith(reply);
  expect(state.dispatch).toHaveBeenCalledWith(75, reply.trainer);
  expect(safari).toHaveBeenCalledWith(null);
  expect(state.listeners.size).toBe(0); stop();
});

test("scene retirement removes the listener and cannot apply a late reply", async () => {
  const abort = new AbortController();
  const result = recoverGameplayState(50, abort.signal);
  const request = state.send.mock.calls[0][0];
  abort.abort(); await expect(result).rejects.toMatchObject({ name: "AbortError" });
  receive(snapshot(request.requestId));
  expect(state.apply).not.toHaveBeenCalled(); expect(state.dispatch).not.toHaveBeenCalled(); expect(state.cutscene).not.toHaveBeenCalled();
  expect(state.listeners.size).toBe(0);
});

test("a newer battle state overtaking a read requires a fresh read before publication", async () => {
  const result = recoverGameplayState(50);
  const first = state.send.mock.calls[0][0];
  state.current = { restoreGameplay: state.apply };
  receive(snapshot(first.requestId)); await Promise.resolve(); await Promise.resolve();
  expect(state.apply).not.toHaveBeenCalled(); expect(state.send).toHaveBeenCalledTimes(2);
  const second = state.send.mock.calls[1][0]; expect(second.requestId).not.toBe(first.requestId);
  const current = snapshot(second.requestId); current.safari = { visitId:"visit",visitRevision:1,active: true, ballsLeft: 7, stepsLeft: 93, pokemon: undefined };
  receive(current); await result;
  expect(state.apply).toHaveBeenCalledOnce(); expect(state.apply).toHaveBeenCalledWith(current);
});

test("rejection or a different map cannot clear gameplay presentation", async () => {
  const rejected = recoverGameplayState(50);
  const request = state.send.mock.calls[0][0];
  receive({ success: false, requestId: request.requestId, error: "storage unavailable" });
  await expect(rejected).rejects.toThrow("storage unavailable");
  const otherMap = recoverGameplayState(50); const other = state.send.mock.calls[1][0];
  const reply = snapshot(other.requestId); reply.position.mapId = 51; receive(reply);
  await expect(otherMap).rejects.toThrow("map differs"); expect(state.apply).not.toHaveBeenCalled();
});

test("timeout leaves state untouched and a later explicit read can recover", async () => {
  vi.useFakeTimers();
  const result = recoverGameplayState(50); const rejected = expect(result).rejects.toThrow("Timeout");
  await vi.advanceTimersByTimeAsync(10000); await rejected;
  expect(state.listeners.size).toBe(0); expect(state.apply).not.toHaveBeenCalled();
  const retry = recoverGameplayState(50); receive(snapshot(state.send.mock.calls[1][0].requestId)); await retry;
  expect(state.apply).toHaveBeenCalledOnce();
});

test("current-owned read accepts a changed map without applying presentation or independently delivering plans", async () => {
  const result = readCurrentGameplayState();
  const request = state.send.mock.calls[0][0];
  expect(request).toEqual({ current: true, requestId: expect.any(String) });
  const reply = snapshot(request.requestId); reply.position.mapId = 99;
  receive(reply);
  await expect(result).resolves.toEqual(reply);
  expect(state.apply).not.toHaveBeenCalled(); expect(state.dispatch).not.toHaveBeenCalled(); expect(state.cutscene).not.toHaveBeenCalled();
  expect(state.listeners.size).toBe(0);
});

test("current-owned read cancellation removes its listener and ignores late authority", async () => {
  const abort = new AbortController();
  const result = readCurrentGameplayState(abort.signal);
  const request = state.send.mock.calls[0][0];
  abort.abort(); await expect(result).rejects.toMatchObject({ name: "AbortError" });
  receive(snapshot(request.requestId));
  expect(state.listeners.size).toBe(0); expect(state.apply).not.toHaveBeenCalled();
});


test("owned recovery clears consumed inventory and empty party and replaces both money views and flags", () => {
  useCQInventoryStore.getState().setInventory([{} as never], 999);
  usePokemonPartyStore.getState().setParty([{} as never]);
  const reply = snapshot("settled"); reply.wallet.pokedollars = 123; reply.eventFlags = ["EVENT_OAK_ASKED_TO_CHOOSE_MON"];
  applyGameplaySnapshot(reply);
  expect(useCQInventoryStore.getState()).toMatchObject({ items: [], money: 123 });
  expect(usePokemonPartyStore.getState().party).toEqual([]);
  expect(state.wallet).toHaveBeenCalledWith({ characterId: 42, pokedollars: 123 });
  expect(state.flags).toHaveBeenCalledWith(reply.eventFlags);
});

test.each(["scene", "current"])("a newer wallet notification forces a fresh %s read and never publishes the old snapshot", async mode => {
  const result = mode === "scene" ? recoverGameplayState(50) : readCurrentGameplayState();
  const first = state.send.mock.calls.at(-1)![0];
  useCQInventoryStore.getState().setMoney(456);
  receive(snapshot(first.requestId));
  await Promise.resolve(); await Promise.resolve();
  expect(state.send).toHaveBeenCalledTimes(2); expect(state.wallet).not.toHaveBeenCalled();
  const second = state.send.mock.calls.at(-1)![0];
  const reply = snapshot(second.requestId); reply.wallet.pokedollars = 456;
  receive(reply); await expect(result).resolves.toEqual(reply);
  expect(state.listeners.size).toBe(0);
  if (mode === "scene") expect(useCQInventoryStore.getState().money).toBe(456);
});

test("an incomplete success packet rejects before clearing any owned store", () => {
  useCQInventoryStore.getState().setMoney(99);
  const reply = snapshot("incomplete"); delete (reply as Partial<GameplayStateResponse>).wallet;
  expect(() => applyGameplaySnapshot(reply)).toThrow("Incomplete owned gameplay snapshot");
  expect(useCQInventoryStore.getState().money).toBe(99); expect(state.apply).not.toHaveBeenCalled(); expect(state.flags).not.toHaveBeenCalled();
});


test("an owned position change overtaking a current snapshot requires a fresh read", async () => {
  let positionGeneration = 0;
  const run = readCurrentGameplayState(undefined, () => positionGeneration);
  const first = state.send.mock.calls[0][0];
  positionGeneration++;
  receive(snapshot(first.requestId));
  await Promise.resolve(); await Promise.resolve();
  expect(state.send).toHaveBeenCalledTimes(2);
  expect(state.wallet).not.toHaveBeenCalled();
  const current = snapshot(state.send.mock.calls[1][0].requestId);
  current.position.x = 9;
  receive(current);
  expect((await run).position.x).toBe(9);
});
