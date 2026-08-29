import { beforeEach, describe, expect, test, vi } from "vitest";

const network = vi.hoisted(() => ({
  tileHandler: undefined as ((data: unknown) => void) | undefined,
  requestTiles: vi.fn(),
}));

vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onTiles: (handler: (data: unknown) => void) => {
    network.tileHandler = handler;
    return () => {
      if (network.tileHandler === handler) network.tileHandler = undefined;
    };
  },
  requestTiles: network.requestTiles,
}));

vi.mock("./RuntimeAssetCompatibility", () => ({
  ensureRuntimeTileCatalogCurrent: vi.fn(async () => undefined),
}));

import { MapDataService } from "./MapDataService";

describe("MapDataService correlated tile responses", () => {
  beforeEach(() => {
    network.tileHandler = undefined;
    network.requestTiles.mockReset();
  });

  test("accepts a legacy null tile slice as a completed empty chunk", async () => {
    network.requestTiles.mockImplementation((request: { requestId: string }) => {
      queueMicrotask(() => {
        network.tileHandler?.({
          mapId: 9999,
          requestId: request.requestId,
          tiles: null,
          nextAfterId: 0,
          hasMore: false,
        });
      });
    });

    const tiles = await new MapDataService().fetchTilesInBounds(9999, {
      minX: 0,
      minY: 0,
      maxX: 63,
      maxY: 63,
    });

    expect(tiles).toEqual([]);
    expect(network.tileHandler).toBeUndefined();
  });

  test("rejects a malformed matching response instead of hanging", async () => {
    network.requestTiles.mockImplementation((request: { requestId: string }) => {
      queueMicrotask(() => {
        network.tileHandler?.({
          mapId: 9999,
          requestId: request.requestId,
          tiles: { unexpected: true },
          nextAfterId: 0,
          hasMore: false,
        });
      });
    });

    await expect(
      new MapDataService().fetchTilesInBounds(9999, {
        minX: 0,
        minY: 0,
        maxX: 63,
        maxY: 63,
      }),
    ).rejects.toThrow("tiles must be an array");
    expect(network.tileHandler).toBeUndefined();
  });
});
