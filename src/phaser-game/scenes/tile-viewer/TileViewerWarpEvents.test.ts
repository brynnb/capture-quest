import { afterEach, expect, test, vi } from "vitest";
import { TileViewerWarpEvents } from "./TileViewerWarpEvents";

const network = vi.hoisted(() => ({ sendPlayerPosition: vi.fn() }));
vi.mock("../../services/PhaserNetworkService", () => network);
vi.mock("@/services/audio/AudioManager", () => ({ default: { playSFX: vi.fn() } }));

afterEach(() => vi.clearAllMocks());

test.each([1, 2])("a committed teleport to map %s snaps immediately without a write or source wait", async (mapId) => {
  const registry = new Map<string, unknown>([["currentMapId", 1]]);
  const movement = {
    stopMovement: vi.fn(), syncMapId: vi.fn(), syncPosition: vi.fn(), syncDirection: vi.fn(),
  };
  const renderer = { waitForActorIdle: vi.fn(() => new Promise(() => {})), snapActorPosition: vi.fn() };
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
  await (handler as unknown as {
    handleWarpTileTeleport: (event: CustomEvent) => Promise<void>;
  }).handleWarpTileTeleport(new CustomEvent("warpTileTeleport", {
    detail: { mapId, x: 3, y: 4, direction: "UP", serverCommitted: true },
  }));
  expect(network.sendPlayerPosition).not.toHaveBeenCalled();
  expect(renderer.waitForActorIdle).not.toHaveBeenCalled();
  expect(movement.stopMovement).toHaveBeenCalledOnce();
  expect(movement.syncMapId).toHaveBeenCalledWith(mapId);
  expect(renderer.snapActorPosition).toHaveBeenCalledOnce();
  expect(renderer.snapActorPosition).toHaveBeenCalledWith(10, 3, 4, "UP", actor);
  if (mapId === 2) {
    expect(registry.get("destinationServerCommitted")).toBe(true);
    expect(resetScene).toHaveBeenCalledWith(false);
  } else {
    expect(resetScene).not.toHaveBeenCalled();
  }
});
