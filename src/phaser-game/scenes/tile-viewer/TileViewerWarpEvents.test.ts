import { afterEach, expect, test, vi } from "vitest";
import useGameStatusStore from "@/stores/GameStatusStore";
import { TileViewerWarpEvents } from "./TileViewerWarpEvents";

const network = vi.hoisted(() => ({ send: vi.fn() }));
vi.mock("@/net/NetworkBridge", () => ({ NetworkBridge: network }));
import * as OpCodes from "@/net/generated/opcodes";
vi.mock("@/services/audio/AudioManager", () => ({ default: { playSFX: vi.fn() } }));

afterEach(() => vi.clearAllMocks());

function warpFixture() {
  const registry = new Map<string, unknown>([["currentMapId", 1]]);
  const movement = {
    getCurrentMapId: () => 1, stopMovement: vi.fn(), syncMapId: vi.fn(), syncPosition: vi.fn(), syncDirection: vi.fn(), beginServerMovement: vi.fn(),
  };
  const renderer = { waitForActorIdle: vi.fn(() => new Promise(() => { })), snapActorPosition: vi.fn() };
  const resetScene = vi.fn();
  const actor = { id: 10, mapId: 1, x: 1, y: 1 };
  const handler = new TileViewerWarpEvents({
    scene: { game: { registry: { get: (key: string) => registry.get(key), set: (key: string, value: unknown) => registry.set(key, value), remove: (key: string) => registry.delete(key) } } },
    mapDataService: { isOverworld: () => false },
    mapRenderer: () => renderer,
    playerMovementController: () => movement,
    getPlayerActor: () => actor,
    setPlayerActor: vi.fn(),
    resetScene,
  } as never);
  return { handler, registry, movement, renderer, resetScene, actor };
}

test.each([1, 2])("a committed teleport to map %s snaps immediately without a write or source wait", async (mapId) => {
  const { handler, registry, movement, renderer, resetScene, actor } = warpFixture();
  await (handler as unknown as {
    handleWarpTileTeleport: (event: CustomEvent) => Promise<void>;
  }).handleWarpTileTeleport(new CustomEvent("warpTileTeleport", {
    detail: { mapId, x: 3, y: 4, direction: "UP", serverCommitted: true },
  }));
  expect(network.send).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
  expect(renderer.waitForActorIdle).not.toHaveBeenCalled();
  expect(movement.stopMovement).toHaveBeenCalledOnce();
  expect(movement.syncMapId).toHaveBeenCalledWith(mapId);
  expect(renderer.snapActorPosition).toHaveBeenCalledOnce();
  expect(renderer.snapActorPosition).toHaveBeenCalledWith(10, 3, 4, "UP", actor);
  if (mapId === 2) {
    expect(registry.get("destinationX")).toBe(3);
    expect(registry.get("destinationY")).toBe(4);
    expect(resetScene).toHaveBeenCalledWith(false);
  } else {
    expect(resetScene).not.toHaveBeenCalled();
  }
});


test("blackout store presentation uses the committed path without echoing coordinates", () => {
  const { handler, registry, resetScene } = warpFixture();
  handler.register();
  try {
    useGameStatusStore.getState().triggerBlackoutWarp(2, 3, 4);
    expect(network.send).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
    expect(registry.get("destinationX")).toBe(3);
    expect(registry.get("destinationY")).toBe(4);
    expect(resetScene).toHaveBeenCalledWith(false);
    expect(useGameStatusStore.getState().pendingBlackoutWarp).toBeNull();
  } finally { handler.cleanup(); }
});


test("owned position reconciliation uses movement map identity without reloading the viewed scene", async () => {
  const { handler, registry, movement, renderer, resetScene, actor } = warpFixture();
  registry.set("currentMapId", 9999); // View registry can differ from owned movement.
  await handler.reconcileOwnedPosition({ mapId: 1, x: 3, y: 4, direction: "UP", serverMovementPending: false });
  expect(resetScene).not.toHaveBeenCalled();
  expect(network.send).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
  expect(movement.syncPosition).toHaveBeenCalledWith(3, 4);
  expect(renderer.snapActorPosition).toHaveBeenCalledWith(10, 3, 4, "UP", actor);
});


test("owned correction preserves an unfinished server movement phase", async () => {
  const { handler, movement, resetScene } = warpFixture();
  await handler.reconcileOwnedPosition({ mapId: 1, x: 3, y: 4, direction: "UP", serverMovementPending: true });
  expect(movement.beginServerMovement).toHaveBeenCalledWith(false);
  expect(resetScene).not.toHaveBeenCalled();
});


test("a teleport event without a committed result cannot mutate presentation or send a destination", async () => {
  const { handler, movement, renderer, resetScene, actor } = warpFixture();
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  await (handler as unknown as { handleWarpTileTeleport: (event: CustomEvent) => Promise<void> }).handleWarpTileTeleport(new CustomEvent("warpTileTeleport", { detail: { mapId: 2, x: 3, y: 4 } }));
  expect(error).toHaveBeenCalledWith("[WarpTile] Ignored teleport without a committed server result");
  expect(movement.stopMovement).not.toHaveBeenCalled();
  expect(renderer.snapActorPosition).not.toHaveBeenCalled();
  expect(resetScene).not.toHaveBeenCalled();
  expect(actor).toEqual({ id: 10, mapId: 1, x: 1, y: 1 });
  expect(network.send).not.toHaveBeenCalled(); error.mockRestore();
});
