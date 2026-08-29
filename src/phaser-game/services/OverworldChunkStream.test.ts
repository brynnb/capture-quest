import type { PhaserTile } from "@/net/generated/world_api";
import { describe, expect, test, vi } from "vitest";

import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import {
  overworldChunkKey,
  type CameraWorldView,
  type InclusiveTileBounds,
} from "./OverworldChunkPlanner";
import { OverworldChunkStream } from "./OverworldChunkStream";

const mapBounds = { minX: -256, minY: -256, maxX: 511, maxY: 511 };

function tileCamera(
  x: number,
  y: number,
  width = 50,
  height = 50,
): CameraWorldView {
  return {
    x: x * TILE_SIZE,
    y: y * TILE_SIZE,
    width: width * TILE_SIZE,
    height: height * TILE_SIZE,
  };
}

function chunkKeyForBounds(bounds: InclusiveTileBounds): string {
  return overworldChunkKey(
    Math.floor(bounds.minX / 64),
    Math.floor(bounds.minY / 64),
  );
}

function makeTile(bounds: InclusiveTileBounds, id: number): PhaserTile {
  return {
    id,
    x: bounds.minX,
    y: bounds.minY,
    tileImageId: id,
    mapId: UNIFIED_OVERWORLD_MAP_ID,
    collisionType: 0,
    talkOverTile: false,
    isNativeGameData: true,
    coordinateOrigin: "native",
    contentOrigin: "native",
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

function createHarness(viewOnly = false) {
  const events: string[] = [];
  const cache = new Map<
    string,
    { bounds: InclusiveTileBounds; tiles: PhaserTile[] }
  >();
  const emittedTiles: PhaserTile[][] = [];
  let nextTileId = 1;

  const fetchTilesInBounds = vi.fn(
    async (mapId: number, bounds: InclusiveTileBounds) => {
      expect(mapId).toBe(UNIFIED_OVERWORLD_MAP_ID);
      const key = chunkKeyForBounds(bounds);
      events.push(`fetch:${key}`);
      return [makeTile(bounds, nextTileId++)];
    },
  );
  const invalidateOverworldTileChunkAt = vi.fn((x: number, y: number) => {
    for (const [key, chunk] of cache) {
      if (
        x >= chunk.bounds.minX &&
        x <= chunk.bounds.maxX &&
        y >= chunk.bounds.minY &&
        y <= chunk.bounds.maxY
      ) {
        cache.delete(key);
      }
    }
  });
  const mapDataService = {
    getOverworldTileChunk: vi.fn((key: string) => cache.get(key)),
    setOverworldTileChunk: vi.fn(
      (key: string, bounds: InclusiveTileBounds, tiles: PhaserTile[]) => {
        cache.set(key, { bounds, tiles });
      },
    ),
    invalidateOverworldTileChunkAt,
    fetchTilesInBounds,
    fetchTileImages: vi.fn(async () => []),
  };
  const tileManager = {
    loadTileImages: vi.fn(async () => undefined),
  };
  const mapRenderer = {
    upsertTileChunk: vi.fn((key: string) => {
      events.push(`render:${key}`);
    }),
    unloadTileChunk: vi.fn(),
  };
  const movementController = {
    addCollisionTiles: vi.fn(),
    removeCollisionTiles: vi.fn(),
    getCurrentPosition: vi.fn(() => ({ x: 7, y: 7 })),
  };
  const overviewLayer = {
    sync: vi.fn(
      async (
        chunks: readonly { key: string; column: number; row: number }[],
        revisionForKey: (key: string) => number,
        onBackgroundFailure?: (error: unknown) => void,
      ) => {
        void chunks;
        void revisionForKey;
        void onBackgroundFailure;
      },
    ),
    setVisible: vi.fn(),
    invalidate: vi.fn(),
    clear: vi.fn(),
  };
  const onTilesChanged = vi.fn((tiles: PhaserTile[]) => {
    emittedTiles.push([...tiles]);
  });

  const stream = new OverworldChunkStream({
    mapBounds,
    viewOnly,
    mapDataService,
    tileManager,
    mapRenderer,
    movementController,
    overviewLayer,
    onTilesChanged,
  } as unknown as ConstructorParameters<typeof OverworldChunkStream>[0]);

  return {
    stream,
    cache,
    emittedTiles,
    events,
    fetchTilesInBounds,
    mapDataService,
    tileManager,
    mapRenderer,
    movementController,
    overviewLayer,
    onTilesChanged,
  };
}

async function waitForExactPlanToSettle(stream: OverworldChunkStream) {
  await vi.waitFor(() => {
    const status = stream.getStatus();
    expect(status.mode).toBe("exact");
    expect(status.loadedExactChunkKeys).toEqual(
      [...status.requestedChunkKeys].sort(),
    );
  });
}

describe("OverworldChunkStream", () => {
  test("renders required chunks before prefetch and keeps the live plan at nine chunks", async () => {
    const harness = createHarness();

    await harness.stream.initialize(tileCamera(7, 7));
    await waitForExactPlanToSettle(harness.stream);

    const status = harness.stream.getStatus();
    expect(status.requestedChunkKeys).toHaveLength(9);
    expect(status.loadedExactChunkKeys).toHaveLength(9);
    expect(new Set(status.loadedExactChunkKeys)).toHaveLength(9);
    expect(harness.fetchTilesInBounds).toHaveBeenCalledTimes(9);
    expect(harness.mapRenderer.upsertTileChunk).toHaveBeenCalledTimes(9);
    expect(Math.max(...harness.emittedTiles.map((tiles) => tiles.length))).toBe(
      9,
    );

    const requiredRender = harness.events.indexOf("render:overworld:0:0");
    const firstPrefetch = harness.events.findIndex(
      (event) => event.startsWith("fetch:") && event !== "fetch:overworld:0:0",
    );
    expect(requiredRender).toBeGreaterThanOrEqual(0);
    expect(firstPrefetch).toBeGreaterThan(requiredRender);
  });

  test("unloads old GPU chunks and collision data after the camera moves", async () => {
    const harness = createHarness();
    await harness.stream.initialize(tileCamera(7, 7));
    await waitForExactPlanToSettle(harness.stream);

    const oldKeys = [...harness.stream.getStatus().loadedExactChunkKeys];
    const initialUpsertCount =
      harness.mapRenderer.upsertTileChunk.mock.calls.length;
    const oldTiles = harness.movementController.addCollisionTiles.mock.calls
      .flatMap(([tiles]) => tiles as PhaserTile[])
      .slice();

    harness.stream.update(tileCamera(263, 7));
    await waitForExactPlanToSettle(harness.stream);

    const nextKeys = harness.stream.getStatus().loadedExactChunkKeys;
    expect(nextKeys).toHaveLength(9);
    expect(nextKeys.some((key) => oldKeys.includes(key))).toBe(false);
    expect(
      harness.mapRenderer.unloadTileChunk.mock.calls.map(([key]) => key).sort(),
    ).toEqual(oldKeys.sort());

    const removedTiles =
      harness.movementController.removeCollisionTiles.mock.calls.flatMap(
        ([tiles]) => tiles as PhaserTile[],
      );
    expect(removedTiles).toEqual(expect.arrayContaining(oldTiles));
    expect(removedTiles).toHaveLength(oldTiles.length);
    const firstReplacementUpsertOrder =
      harness.mapRenderer.upsertTileChunk.mock.invocationCallOrder[
        initialUpsertCount
      ];
    expect(
      Math.min(...harness.mapRenderer.unloadTileChunk.mock.invocationCallOrder),
    ).toBeLessThan(firstReplacementUpsertOrder);
  });

  test("uses overview imagery without requesting exact tiles for a view-only wide view", async () => {
    const harness = createHarness(true);

    await harness.stream.initialize(tileCamera(0, 0, 256, 256));

    expect(harness.stream.getStatus().mode).toBe("overview");
    expect(harness.stream.getStatus().loadedExactChunkKeys).toEqual([]);
    expect(harness.stream.getStatus().overviewChunkKeys.length).toBeGreaterThan(
      0,
    );
    expect(harness.overviewLayer.sync).toHaveBeenCalledOnce();
    expect(harness.overviewLayer.sync.mock.calls[0][0][0]).toEqual({
      key: "overworld:2:2",
      column: 2,
      row: 2,
    });
    expect(harness.overviewLayer.setVisible).toHaveBeenCalledWith(true);
    expect(
      harness.overviewLayer.setVisible.mock.invocationCallOrder[0],
    ).toBeLessThan(harness.overviewLayer.sync.mock.invocationCallOrder[0]);
    expect(harness.fetchTilesInBounds).not.toHaveBeenCalled();
    expect(harness.tileManager.loadTileImages).not.toHaveBeenCalled();
    expect(harness.mapRenderer.upsertTileChunk).not.toHaveBeenCalled();
    expect(harness.movementController.addCollisionTiles).not.toHaveBeenCalled();
  });

  test("keeps a bounded player-centered exact footprint beneath gameplay overview", async () => {
    const harness = createHarness();

    await harness.stream.initialize(tileCamera(128, 128, 256, 256));

    const status = harness.stream.getStatus();
    expect(status.mode).toBe("overview");
    expect(status.loadedExactChunkKeys).toContain("overworld:0:0");
    expect(status.loadedExactChunkKeys.length).toBeGreaterThan(0);
    expect(status.loadedExactChunkKeys.length).toBeLessThanOrEqual(9);
    expect(harness.movementController.getCurrentPosition).toHaveBeenCalled();
    expect(harness.movementController.addCollisionTiles).toHaveBeenCalled();
    expect(harness.overviewLayer.sync).toHaveBeenCalledOnce();
  });

  test("schedules bounded retry when overview background loading reports a hole", async () => {
    const harness = createHarness(true);
    await harness.stream.initialize(tileCamera(0, 0, 256, 256));
    const backgroundFailure = harness.overviewLayer.sync.mock.calls[0][2] as
      ((error: unknown) => void) | undefined;
    expect(backgroundFailure).toBeTypeOf("function");

    backgroundFailure?.(new Error("background overview failed"));

    const internal = harness.stream as unknown as {
      planSignature: string;
      failedPlanSignature: string;
      planRetryNotBefore: number;
    };
    expect(internal.planSignature).toBe("");
    expect(internal.failedPlanSignature).not.toBe("");
    expect(internal.planRetryNotBefore).toBeGreaterThan(Date.now());
  });

  test("promotes preload chunks when the required partition changes inside the same desired grid", async () => {
    const harness = createHarness();
    const preloadGate = deferred<void>();
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        const key = chunkKeyForBounds(bounds);
        harness.events.push(`fetch:${key}`);
        if (key !== "overworld:0:0") await preloadGate.promise;
        return [makeTile(bounds, key === "overworld:0:0" ? 1 : 2)];
      },
    );

    await harness.stream.initialize(tileCamera(7, 7));
    harness.stream.update(tileCamera(-50, -50, 150, 150));

    const internal = harness.stream as unknown as {
      lastPlan: { mode: string; requiredChunks: unknown[] };
      planSignature: string;
    };
    await vi.waitFor(() => {
      expect(internal.lastPlan.mode).toBe("exact");
      expect(internal.lastPlan.requiredChunks).toHaveLength(9);
      expect(internal.planSignature).toContain("required=");
    });

    harness.stream.stop();
    preloadGate.resolve();
  });

  test("keeps exact chunks until the first view-only overview chunk is ready", async () => {
    const harness = createHarness(true);
    await harness.stream.initialize(tileCamera(7, 7));
    await waitForExactPlanToSettle(harness.stream);
    const overviewReady = deferred<void>();
    harness.overviewLayer.sync.mockImplementationOnce(
      async () => overviewReady.promise,
    );

    harness.stream.update(tileCamera(0, 0, 256, 256));
    await vi.waitFor(() =>
      expect(harness.overviewLayer.sync).toHaveBeenCalledOnce(),
    );

    expect(harness.stream.getStatus().mode).toBe("overview");
    expect(harness.overviewLayer.setVisible).toHaveBeenLastCalledWith(true);
    expect(harness.mapRenderer.unloadTileChunk).not.toHaveBeenCalled();
    const visibleCall =
      harness.overviewLayer.setVisible.mock.invocationCallOrder.at(-1);
    expect(visibleCall).toBeLessThan(
      harness.overviewLayer.sync.mock.invocationCallOrder[0],
    );

    overviewReady.resolve();
    await vi.waitFor(() =>
      expect(harness.mapRenderer.unloadTileChunk).toHaveBeenCalledTimes(9),
    );
  });

  test("reuses cached chunks when returning to an earlier camera region", async () => {
    const harness = createHarness();
    await harness.stream.initialize(tileCamera(7, 7));
    await waitForExactPlanToSettle(harness.stream);
    const initialKeys = [...harness.stream.getStatus().loadedExactChunkKeys];

    harness.stream.update(tileCamera(263, 7));
    await waitForExactPlanToSettle(harness.stream);
    harness.stream.update(tileCamera(7, 7));
    await waitForExactPlanToSettle(harness.stream);

    const fetchedKeys = harness.fetchTilesInBounds.mock.calls.map(
      ([, bounds]) => chunkKeyForBounds(bounds),
    );
    for (const key of initialKeys) {
      expect(
        fetchedKeys.filter((fetchedKey) => fetchedKey === key),
      ).toHaveLength(1);
    }
    expect(harness.fetchTilesInBounds).toHaveBeenCalledTimes(18);
  });

  test("suppresses an exact response that becomes stale after switching to overview", async () => {
    const harness = createHarness(true);
    let resolveExact!: (tiles: PhaserTile[]) => void;
    const exactResponse = new Promise<PhaserTile[]>((resolve) => {
      resolveExact = resolve;
    });
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        harness.events.push(`fetch:${chunkKeyForBounds(bounds)}`);
        return exactResponse;
      },
    );

    const initialLoad = harness.stream.initialize(tileCamera(7, 7));
    await vi.waitFor(() =>
      expect(harness.fetchTilesInBounds).toHaveBeenCalledOnce(),
    );

    harness.stream.update(tileCamera(7, 7), true);
    await vi.waitFor(() =>
      expect(harness.overviewLayer.sync).toHaveBeenCalled(),
    );
    resolveExact([makeTile({ minX: 0, minY: 0, maxX: 63, maxY: 63 }, 99)]);
    await initialLoad;

    expect(harness.stream.getStatus().mode).toBe("overview");
    expect(harness.mapRenderer.upsertTileChunk).not.toHaveBeenCalled();
    expect(harness.movementController.addCollisionTiles).not.toHaveBeenCalled();
    expect(harness.onTilesChanged).toHaveBeenLastCalledWith([]);
  });

  test("keeps one global two-request ceiling while camera plans change rapidly", async () => {
    const harness = createHarness();
    const gates: Array<ReturnType<typeof deferred<void>>> = [];
    let activeRequests = 0;
    let maximumActiveRequests = 0;
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        const gate = deferred<void>();
        gates.push(gate);
        activeRequests += 1;
        maximumActiveRequests = Math.max(maximumActiveRequests, activeRequests);
        await gate.promise;
        activeRequests -= 1;
        return [makeTile(bounds, gates.indexOf(gate) + 1)];
      },
    );

    const initialLoad = harness.stream.initialize(tileCamera(7, 7));
    await vi.waitFor(() => expect(gates).toHaveLength(1));
    harness.stream.update(tileCamera(263, 7));
    harness.stream.update(tileCamera(327, 7));
    harness.stream.update(tileCamera(391, 7));
    await vi.waitFor(() => expect(gates).toHaveLength(2));
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(activeRequests).toBe(2);
    expect(maximumActiveRequests).toBe(2);
    expect(gates).toHaveLength(2);

    harness.stream.stop();
    for (const gate of gates) gate.resolve();
    await initialLoad;
    await vi.waitFor(() => expect(activeRequests).toBe(0));
    expect(gates).toHaveLength(2);
  });

  test("does not repopulate the shared chunk cache after a stopped stream resolves", async () => {
    const harness = createHarness();
    const lateResponse = deferred<PhaserTile[]>();
    harness.fetchTilesInBounds.mockImplementation(
      async () => lateResponse.promise,
    );

    const initialLoad = harness.stream.initialize(tileCamera(7, 7));
    await vi.waitFor(() =>
      expect(harness.fetchTilesInBounds).toHaveBeenCalledOnce(),
    );
    harness.stream.stop();
    lateResponse.resolve([
      makeTile({ minX: 0, minY: 0, maxX: 63, maxY: 63 }, 55),
    ]);
    await initialLoad;

    expect(harness.mapDataService.setOverworldTileChunk).not.toHaveBeenCalled();
    expect(harness.mapRenderer.upsertTileChunk).not.toHaveBeenCalled();
    expect(harness.stream.getStatus().loadedExactChunkKeys).toEqual([]);
  });

  test("keeps resident lookup tiles coherent with committed paint and erase broadcasts", async () => {
    const harness = createHarness();
    await harness.stream.initialize(tileCamera(7, 7));
    await waitForExactPlanToSettle(harness.stream);
    const existing = harness.emittedTiles
      .at(-1)
      ?.find((tile) => tile.x === 0 && tile.y === 0);
    expect(existing).toBeDefined();

    harness.stream.applyCommittedTileUpdates([
      {
        x: 0,
        y: 0,
        tileImageId: 0,
        collisionType: 0,
        erased: true,
      },
      {
        x: 1,
        y: 1,
        tileImageId: 77,
        collisionType: 1,
        talkOverTile: true,
      },
    ]);

    const latest = harness.emittedTiles.at(-1) ?? [];
    expect(latest.some((tile) => tile.x === 0 && tile.y === 0)).toBe(false);
    expect(latest).toContainEqual(
      expect.objectContaining({
        x: 1,
        y: 1,
        tileImageId: 77,
        collisionType: 1,
        talkOverTile: true,
        contentOrigin: "user",
      }),
    );
    expect(
      harness.mapDataService.invalidateOverworldTileChunkAt,
    ).toHaveBeenCalledTimes(2);
    expect(harness.overviewLayer.invalidate).toHaveBeenCalledOnce();
  });

  test("retries a bounded response invalidated by a concurrent committed edit", async () => {
    const harness = createHarness();
    const stale = deferred<PhaserTile[]>();
    const current = deferred<PhaserTile[]>();
    let requiredRequests = 0;
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        const key = chunkKeyForBounds(bounds);
        harness.events.push(`fetch:${key}`);
        if (key !== "overworld:0:0") return [makeTile(bounds, 10)];
        requiredRequests += 1;
        return requiredRequests === 1 ? stale.promise : current.promise;
      },
    );

    const initialLoad = harness.stream.initialize(tileCamera(7, 7));
    await vi.waitFor(() => expect(requiredRequests).toBe(1));
    harness.stream.applyCommittedTileUpdates([
      { x: 0, y: 0, tileImageId: 99, collisionType: 1 },
    ]);
    stale.resolve([makeTile({ minX: 0, minY: 0, maxX: 63, maxY: 63 }, 1)]);
    await vi.waitFor(() => expect(requiredRequests).toBe(2));
    current.resolve([makeTile({ minX: 0, minY: 0, maxX: 63, maxY: 63 }, 99)]);
    await initialLoad;

    expect(harness.mapRenderer.upsertTileChunk).toHaveBeenCalledWith(
      "overworld:0:0",
      0,
      0,
      [expect.objectContaining({ tileImageId: 99 })],
    );
  });

  test("does not partially commit a required plan when a later staged batch fails", async () => {
    const harness = createHarness();
    let requestCount = 0;
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        requestCount += 1;
        if (requestCount === 3) throw new Error("required batch failed");
        return [makeTile(bounds, requestCount)];
      },
    );

    await expect(
      harness.stream.initialize(tileCamera(-50, -50, 150, 150)),
    ).rejects.toThrow("required batch failed");

    expect(harness.mapRenderer.upsertTileChunk).not.toHaveBeenCalled();
    expect(harness.movementController.addCollisionTiles).not.toHaveBeenCalled();
    expect(harness.stream.getStatus().loadedExactChunkKeys).toEqual([]);
  });

  test("refetches when a committed edit lands during tile texture loading", async () => {
    const harness = createHarness();
    const textureGate = deferred<void>();
    let requiredRequests = 0;
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        const key = chunkKeyForBounds(bounds);
        if (key !== "overworld:0:0") return [makeTile(bounds, 10)];
        requiredRequests += 1;
        return [makeTile(bounds, requiredRequests === 1 ? 1 : 99)];
      },
    );
    harness.tileManager.loadTileImages.mockImplementationOnce(async () => {
      await textureGate.promise;
      return undefined;
    });

    const initialLoad = harness.stream.initialize(tileCamera(7, 7));
    await vi.waitFor(() =>
      expect(harness.tileManager.loadTileImages).toHaveBeenCalledOnce(),
    );
    harness.stream.applyCommittedTileUpdates([
      { x: 0, y: 0, tileImageId: 99, collisionType: 1 },
    ]);
    textureGate.resolve();
    await initialLoad;

    expect(requiredRequests).toBe(2);
    const requiredUpserts =
      harness.mapRenderer.upsertTileChunk.mock.calls.filter(
        ([key]) => key === "overworld:0:0",
      );
    expect(requiredUpserts).toHaveLength(1);
    const renderedTiles = (
      requiredUpserts[0] as unknown as [string, number, number, PhaserTile[]]
    )[3];
    expect(renderedTiles).toEqual([
      expect.objectContaining({ tileImageId: 99 }),
    ]);
  });

  test("retries a failed prefetch with bounded plan backoff", async () => {
    const harness = createHarness();
    const failedKey = "overworld:-1:-1";
    let failedKeyAttempts = 0;
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        const key = chunkKeyForBounds(bounds);
        if (key === failedKey) {
          failedKeyAttempts += 1;
          if (failedKeyAttempts === 1) throw new Error("prefetch failed");
        }
        return [makeTile(bounds, failedKeyAttempts + 1)];
      },
    );

    try {
      await harness.stream.initialize(tileCamera(7, 7));
      const internal = harness.stream as unknown as {
        failedPlanSignature: string;
        planRetryNotBefore: number;
      };
      await vi.waitFor(() => {
        expect(failedKeyAttempts).toBe(1);
        expect(internal.failedPlanSignature).not.toBe("");
      });

      harness.stream.update(tileCamera(7, 7));
      await Promise.resolve();
      expect(failedKeyAttempts).toBe(1);

      internal.planRetryNotBefore = 0;
      harness.stream.update(tileCamera(7, 7));
      await waitForExactPlanToSettle(harness.stream);
      expect(failedKeyAttempts).toBe(2);
    } finally {
      warn.mockRestore();
    }
  });

  test("keeps only the latest queued ad-hoc tile verification", async () => {
    const harness = createHarness(true);
    const gates = new Map<string, ReturnType<typeof deferred<PhaserTile[]>>>();
    harness.fetchTilesInBounds.mockImplementation(
      async (_mapId: number, bounds: InclusiveTileBounds) => {
        const key = chunkKeyForBounds(bounds);
        const gate = deferred<PhaserTile[]>();
        gates.set(key, gate);
        return gate.promise;
      },
    );

    const first = harness.stream.ensureTileAvailable(0, 0);
    const second = harness.stream.ensureTileAvailable(64, 0);
    await vi.waitFor(() => expect(gates.size).toBe(2));
    const obsoleteQueued = harness.stream.ensureTileAvailable(128, 0);
    const latest = harness.stream.ensureTileAvailable(192, 0);

    gates
      .get("overworld:0:0")
      ?.resolve([makeTile({ minX: 0, minY: 0, maxX: 63, maxY: 63 }, 1)]);
    gates
      .get("overworld:1:0")
      ?.resolve([makeTile({ minX: 64, minY: 0, maxX: 127, maxY: 63 }, 2)]);
    await vi.waitFor(() => expect(gates.has("overworld:3:0")).toBe(true));
    expect(gates.has("overworld:2:0")).toBe(false);
    gates
      .get("overworld:3:0")
      ?.resolve([makeTile({ minX: 192, minY: 0, maxX: 255, maxY: 63 }, 3)]);

    await expect(
      Promise.all([first, second, obsoleteQueued, latest]),
    ).resolves.toEqual([false, false, false, true]);
  });
});
