import { describe, expect, it, vi } from "vitest";

import { MapLoader } from "./MapLoader";

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

function createPendingLoader() {
  const compatibility = deferred();
  const raw = Object.create(MapLoader.prototype) as Record<string, unknown>;
  const scene = { mapLoadInProgress: false };
  const clear = vi.fn();
  const rendererClear = vi.fn();
  raw.mapLoadGeneration = 0;
  raw.overworldChunkStream = null;
  raw.lastOverworldStreamUpdateAt = 0;
  raw.preferOverworldOverviewForZoom = false;
  raw.overworldOverviewLayer = { clear };
  raw.scene = scene;
  raw.mapDataService = {
    isReady: () => true,
    ensureRuntimeTileCatalogCurrent: vi.fn(() => compatibility.promise),
  };
  raw.uiManager = { setLoadingText: vi.fn() };
  raw.mapRenderer = {
    clearLocalActorPositionOverrides: vi.fn(),
    clear: rendererClear,
  };

  return {
    loader: raw as unknown as MapLoader,
    raw,
    scene,
    compatibility,
    clear,
    rendererClear,
  };
}

function createFailedOverworldLoader() {
  const raw = Object.create(MapLoader.prototype) as Record<string, unknown>;
  const scene = {
    mapLoadInProgress: false,
    game: {
      registry: { get: vi.fn(() => undefined), remove: vi.fn() },
    },
    cameras: {
      main: {
        width: 800,
        height: 600,
        getWorldPoint: (x: number, y: number) => ({ x, y }),
        fadeIn: vi.fn(),
      },
    },
    time: { now: 0 },
  };
  const state: Record<string, unknown> = {};
  const overviewClear = vi.fn();
  const buildCollisionMap = vi.fn();
  const setLoadingText = vi.fn();
  const fetchTilesInBounds = vi.fn(async () => {
    throw new Error("required exact chunk timed out");
  });

  Object.assign(raw, {
    scene,
    mapLoadGeneration: 0,
    overworldChunkStream: null,
    lastOverworldStreamUpdateAt: 0,
    preferOverworldOverviewForZoom: false,
    overworldOverviewLayer: {
      clear: overviewClear,
      sync: vi.fn(async () => undefined),
      setVisible: vi.fn(),
      invalidate: vi.fn(),
    },
    mapDataService: {
      isReady: () => true,
      ensureRuntimeTileCatalogCurrent: vi.fn(async () => undefined),
      getSnapshot: vi.fn(() => undefined),
      setSnapshot: vi.fn(),
      fetchMapInfo: vi.fn(async () => ({
        id: 9999,
        name: "Unified Overworld",
        width: 128,
        height: 128,
        isOverworld: 1,
        tileMinX: 0,
        tileMinY: 0,
        tileMaxX: 127,
        tileMaxY: 127,
      })),
      fetchActors: vi.fn(async () => []),
      fetchWarps: vi.fn(async () => []),
      fetchTilesInBounds,
      fetchTileImages: vi.fn(async () => []),
      getOverworldTileChunk: vi.fn(() => undefined),
      setOverworldTileChunk: vi.fn(),
      invalidateOverworldTileChunkAt: vi.fn(),
    },
    tileManager: { loadTileImages: vi.fn(async () => undefined) },
    actorManager: { preloadActorSprites: vi.fn(async () => undefined) },
    mapRenderer: {
      clearLocalActorPositionOverrides: vi.fn(),
      clear: vi.fn(),
      renderMap: vi.fn(() => ({ centerX: 1024, centerY: 1024 })),
      upsertTileChunk: vi.fn(),
      unloadTileChunk: vi.fn(),
    },
    cameraController: {
      setViewMode: vi.fn(),
      centerOnMap: vi.fn(),
      setZoom: vi.fn(),
      getZoom: vi.fn(() => 1),
      clearOverworldCameraState: vi.fn(),
      restoreOverworldCameraState: vi.fn(() => false),
    },
    playerMovementController: {
      buildCollisionMap,
      setBlockingActors: vi.fn(),
      addCollisionTiles: vi.fn(),
      removeCollisionTiles: vi.fn(),
      getCurrentPosition: vi.fn(() => ({ x: 0, y: 0 })),
    },
    uiManager: {
      setLoadingText,
      setModeText: vi.fn(),
      hideLoadingText: vi.fn(),
    },
    getPlayerActor: () => null,
    prepareActorsForLoadedView: (actors: unknown[]) => actors,
    createMapLegend: vi.fn(),
    applyDestinationDirection: vi.fn(),
    updateCameraFollow: vi.fn(),
    getState: () => state,
    setState: (patch: Record<string, unknown>) => Object.assign(state, patch),
  });

  return {
    loader: raw as unknown as MapLoader,
    raw,
    scene,
    state,
    overviewClear,
    buildCollisionMap,
    setLoadingText,
    fetchTilesInBounds,
  };
}

describe("MapLoader async lifecycle", () => {
  it("does not let an obsolete load mutate the scene or clear a newer load flag", async () => {
    const { loader, raw, scene, compatibility, rendererClear } =
      createPendingLoader();
    const oldLoad = loader.loadOverworldData();
    await vi.waitFor(() => expect(scene.mapLoadInProgress).toBe(true));

    raw.mapLoadGeneration = 2;
    scene.mapLoadInProgress = true;
    compatibility.resolve();
    await oldLoad;

    expect(rendererClear).not.toHaveBeenCalled();
    expect(scene.mapLoadInProgress).toBe(true);
  });

  it("invalidates pending work and releases stream resources on cleanup", () => {
    const { loader, raw, scene, clear } = createPendingLoader();
    const stop = vi.fn();
    raw.mapLoadGeneration = 7;
    raw.overworldChunkStream = { stop };
    scene.mapLoadInProgress = true;

    loader.cleanup();

    expect(raw.mapLoadGeneration).toBe(8);
    expect(stop).toHaveBeenCalledOnce();
    expect(clear).toHaveBeenCalledOnce();
    expect(scene.mapLoadInProgress).toBe(false);
  });

  it("tears down a failed initial chunk stream and keeps world input frozen", async () => {
    const {
      loader,
      raw,
      scene,
      state,
      overviewClear,
      buildCollisionMap,
      setLoadingText,
      fetchTilesInBounds,
    } = createFailedOverworldLoader();

    await loader.loadOverworldData();

    expect(fetchTilesInBounds).toHaveBeenCalled();
    expect(raw.overworldChunkStream).toBeNull();
    expect(overviewClear).toHaveBeenCalled();
    expect(buildCollisionMap).toHaveBeenLastCalledWith([]);
    expect(state.tiles).toEqual([]);
    expect(scene.mapLoadInProgress).toBe(true);
    expect(setLoadingText).toHaveBeenLastCalledWith(
      expect.stringContaining("required exact chunk timed out"),
    );
  });

  it("suppresses camera stream updates until the active map load is ready", () => {
    const raw = Object.create(MapLoader.prototype) as Record<string, unknown>;
    const update = vi.fn();
    const scene = {
      mapLoadInProgress: true,
      time: { now: 1_000 },
      cameras: {
        main: {
          width: 800,
          height: 600,
          getWorldPoint: (x: number, y: number) => ({ x, y }),
        },
      },
    };
    Object.assign(raw, {
      scene,
      overworldChunkStream: { update },
      lastOverworldStreamUpdateAt: 0,
      preferOverworldOverviewForZoom: false,
      cameraController: { getZoom: () => 1 },
      getState: () => ({ mapInfo: { id: 9999 } }),
    });
    const loader = raw as unknown as MapLoader;

    loader.updateOverworldStreaming();
    expect(update).not.toHaveBeenCalled();

    scene.mapLoadInProgress = false;
    loader.updateOverworldStreaming();
    expect(update).toHaveBeenCalledOnce();
  });
});
