import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { GameplayStateResponse } from "@/net/generated/world_api";
const state = vi.hoisted(() => ({
  listeners: new Set<(data: unknown) => void>(), send: vi.fn(), apply: vi.fn(), dispatch: vi.fn(), cutscene: vi.fn(),
  current: null as unknown as { restoreGameplay: (snapshot: unknown) => void },
}));
vi.mock("@/net", () => ({ OpCodes: { TrainerEncounterNotify: 75 } }));
vi.mock("@/stores/PokeBattleStore", () => ({ default: { getState: () => state.current } }));
vi.mock("./CutsceneService", () => ({ handleCutsceneStart: state.cutscene }));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onGameplayState: (receive: (data: unknown) => void) => { state.listeners.add(receive); return () => state.listeners.delete(receive); },
  requestGameplayState: state.send, dispatchPhaserResponse: state.dispatch,
}));
import { recoverGameplayState, readCurrentGameplayState } from "./GameplayRecoveryService";
const snapshot = (requestId: string): GameplayStateResponse => ({ success: true, requestId, position: { success: true, requestId, mapId: 50, x: 7, y: 8, direction: "UP", serverMovementPending: false }, battle: null, safari: null, trainer: null, cutscene: null });
const receive = (data: unknown) => state.listeners.forEach(listener => listener(data));
beforeEach(() => { state.current = { restoreGameplay: state.apply }; });
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); state.listeners.clear(); });

test("one correlated current snapshot clears stale state and redelivers its issued plans", async () => {
  const safari = vi.fn(); window.addEventListener("safariZoneEnter", safari);
  const result = recoverGameplayState(50);
  const request = state.send.mock.calls[0][0];
  receive(snapshot("obsolete")); expect(state.apply).not.toHaveBeenCalled();
  const reply = snapshot(request.requestId);
  reply.trainer = { encounterToken: "issued", trainerActorId: 17, trainerX: 7, trainerY: 6, playerX: 7, playerY: 8, approachToX: 7, approachToY: 7, walkToX: 7, walkToY: 8, trainerClass: "TEST", trainerName: "Trainer" };
  receive(reply); await result;
  expect(state.apply).toHaveBeenCalledOnce(); expect(state.apply).toHaveBeenCalledWith(reply);
  expect(state.dispatch).toHaveBeenCalledWith(75, reply.trainer);
  expect((safari.mock.calls[0][0] as CustomEvent).detail).toEqual({ success: false });
  expect(state.listeners.size).toBe(0); window.removeEventListener("safariZoneEnter", safari);
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
  const current = snapshot(second.requestId); current.safari = { active: true, ballsLeft: 7, stepsLeft: 93, pokemon: undefined };
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
