import { afterEach, beforeEach, expect, test, vi } from "vitest";
const network = vi.hoisted(() => ({ listeners: new Set<(data: any) => void>(), send: vi.fn() }));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onActors: (receive: (data: any) => void) => { network.listeners.add(receive); return () => network.listeners.delete(receive); },
  requestActors: network.send,
}));
import { MapDataService } from "./MapDataService";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
beforeEach(() => { usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 }); network.send.mockReset(); });
afterEach(() => { vi.useRealTimers(); network.listeners.clear(); });
const receive = (value: unknown) => network.listeners.forEach(listener => listener(value));

test("actor replies require matching correlation, map and character", async () => {
  const promise = new MapDataService().fetchActors(50);
  const request = network.send.mock.calls[0][0];
  receive({ success: true, requestId: "old", characterId: 42, mapId: 50, actors: [] });
  expect(network.listeners.size).toBe(1);
  receive({ success: true, ...request, actors: [] });
  expect(await promise).toEqual([]);
  expect(network.listeners.size).toBe(0);
});

test("actor timeout and cancellation remove their response listeners", async () => {
  vi.useFakeTimers();
  const read = new MapDataService().fetchActors(50);
  const rejected = expect(read).rejects.toThrow("Timeout");
  await vi.advanceTimersByTimeAsync(10_000); await rejected;
  expect(network.listeners.size).toBe(0);
  const abort = new AbortController();
  const cancelled = new MapDataService().fetchActors(50, abort.signal);
  abort.abort(); await expect(cancelled).rejects.toMatchObject({ name: "AbortError" });
  expect(network.listeners.size).toBe(0);
});

test("foreign or replaced-character actor views cannot resolve as current", async () => {
  const first = new MapDataService().fetchActors(50);
  const request = network.send.mock.calls[0][0];
  receive({ success: true, ...request, characterId: 43, actors: [] });
  await expect(first).rejects.toThrow("ownership mismatch");
  const second = new MapDataService().fetchActors(50);
  const next = network.send.mock.calls[1][0];
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 43 });
  receive({ success: true, ...next, actors: [] });
  await expect(second).rejects.toMatchObject({ name: "AbortError" });
});
