import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("./GameplayRecoveryService", () => ({ readCurrentGameplayState: vi.fn(), applyGameplaySnapshot: vi.fn() }));
import * as recovery from "./GameplayRecoveryService";
afterEach(() => vi.clearAllMocks());

import { MapLoader } from "./MapLoader";
import useGameStatusStore from "@/stores/GameStatusStore";
import * as PhaserNet from "./PhaserNetworkService";
afterEach(() => useGameStatusStore.getState().setIsMapLoading(false));

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
  raw.overworldOverviewLayer = { clear };
  raw.scene = scene;
  raw.mapDataService = {
    isReady: () => true,
    beginOwnedTileView: vi.fn(),
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
      events: {
        on: vi.fn((_event: string, listener: () => void) => queueMicrotask(listener)),
        off: vi.fn(),
      },
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
    overworldOverviewLayer: {
      clear: overviewClear,
      sync: vi.fn(async () => undefined),
      setVisible: vi.fn(),
      invalidate: vi.fn(),
    },
    mapDataService: {
      isReady: () => true,
      beginOwnedTileView: vi.fn(),
      ensureRuntimeTileCatalogCurrent: vi.fn(async () => undefined),
      getSnapshot: vi.fn(() => undefined),
      setSnapshot: vi.fn(),
      prepareMapLoad: vi.fn(async () => undefined),
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
    captureActorReadView: () => (actors: unknown[]) => actors,
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

  it("aborts requests owned by superseded loads and by scene cleanup", async () => {
    const { loader, raw, compatibility, rendererClear } = createPendingLoader();
    const first = loader.loadOverworldData();
    const firstSignal = (raw.mapRequestAbort as AbortController).signal;
    const second = loader.loadOverworldData();
    const secondSignal = (raw.mapRequestAbort as AbortController).signal;
    expect(firstSignal.aborted).toBe(true);
    expect(secondSignal.aborted).toBe(false);
    loader.cleanup();
    expect(secondSignal.aborted).toBe(true);
    compatibility.resolve();
    await Promise.all([first, second]);
    expect(rendererClear).not.toHaveBeenCalled();
  });

  it("invalidates pending work and releases stream resources on cleanup", () => {
    const { loader, raw, scene, clear } = createPendingLoader();
    const stop = vi.fn();
    raw.mapLoadGeneration = 7;
    raw.overworldChunkStream = { stop };
    scene.mapLoadInProgress = true;

    const requestAbort = new AbortController();
    raw.mapRequestAbort = requestAbort;
    loader.cleanup();

    expect(requestAbort.signal.aborted).toBe(true);
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
      "The world could not be loaded. Please try again.",
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


it("final map recovery projects missed movement before resuming plans", async () => {
  const order: string[] = [];
  const raw = Object.create(MapLoader.prototype) as any;
  raw.mapLoadGeneration = 7;
  raw.playerMovementController = {
    getPositionGeneration: () => 3,
    projectOwnedPosition: vi.fn(() => order.push("position")),
  };
  const snapshot = { position: { mapId: 200, x: 2, y: 9, direction: "LEFT" } } as any;
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce(snapshot);
  vi.mocked(recovery.applyGameplaySnapshot).mockImplementationOnce(() => order.push("plans"));
  await raw.recoverLoadedOwnership(7, new AbortController().signal);
  expect(raw.playerMovementController.projectOwnedPosition).toHaveBeenCalledWith(snapshot.position);
  expect(order).toEqual(["position", "plans"]);
});

it("retired map loads cannot project a late current position", async () => {
  const raw = Object.create(MapLoader.prototype) as any;
  raw.mapLoadGeneration = 7;
  raw.playerMovementController = { getPositionGeneration: () => 3, projectOwnedPosition: vi.fn() };
  let resolve!: (value: any) => void;
  vi.mocked(recovery.readCurrentGameplayState).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const pending = raw.recoverLoadedOwnership(7, new AbortController().signal);
  raw.mapLoadGeneration++;
  resolve({ position: { mapId: 200, x: 2, y: 9, direction: "LEFT" } });
  await pending;
  expect(raw.playerMovementController.projectOwnedPosition).not.toHaveBeenCalled();
  expect(recovery.applyGameplaySnapshot).not.toHaveBeenCalled();
});

it("tile presentation lease rejects another map and retires with load generation",()=>{
 const loader=Object.create(MapLoader.prototype) as MapLoader;
 const raw=loader as unknown as {mapLoadGeneration:number;getState:()=>unknown};raw.mapLoadGeneration=3;
 raw.getState=()=>({mapInfo:{id:50,isOverworld:0}});
 const current=loader.captureTilePresentationView(50);expect(current()).toBe(true);
 expect(loader.captureTilePresentationView(51)()).toBe(false);
 raw.mapLoadGeneration++;expect(current()).toBe(false);
 raw.getState=()=>({mapInfo:{id:0,isOverworld:1}});expect(loader.captureTilePresentationView(9999)()).toBe(true);
});

it("initial tile presentation rereads changes during image preparation before returning a current view",async()=>{
 const loader=Object.create(MapLoader.prototype) as MapLoader;
 const raw=loader as unknown as {mapDataService:unknown;tileManager:unknown;prepareTilePresentation:(mapId:number,tiles:unknown[],signal:AbortSignal)=>Promise<unknown[]>};
 const old:unknown[]=[],first:unknown[]=[],current:unknown[]=[];let accepted:unknown[]=old;
 const fetch=vi.fn().mockResolvedValueOnce(first).mockResolvedValueOnce(current);
 raw.mapDataService={isTileReadCurrent:(tiles:unknown[])=>tiles===accepted&&tiles!==old,fetchTiles:fetch,fetchTileImages:vi.fn(async()=>[])};
 const images=vi.fn(async()=>{accepted=fetch.mock.calls.length===1?old:current});raw.tileManager={loadTileImages:images};
 const result=await raw.prepareTilePresentation(50,old,new AbortController().signal);
 expect(result).toBe(current);expect(fetch).toHaveBeenCalledTimes(2);expect(images).toHaveBeenCalledTimes(2);
});

function createInteriorLoader() {
  const harness = createFailedOverworldLoader();
  const raw = harness.raw as Record<string, any>;
  const listeners = new Set<() => void>();
  const camera = { ...harness.scene.cameras.main, fadeEffect: { isRunning: true } };
  Object.assign(harness.scene.cameras, { main: camera });
  Object.assign(harness.scene.game, { events: {
    on: (_event: string, listener: () => void) => listeners.add(listener),
    off: (_event: string, listener: () => void) => listeners.delete(listener),
  } });
  Object.assign(raw.mapDataService, {
    fetchMapInfo: vi.fn(async () => ({ id: 50, name: "Test room", width: 10, height: 10, isOverworld: 0 })),
    fetchTiles: vi.fn(async () => []),
    fetchItems: vi.fn(async () => []),
    isTileReadCurrent: () => true,
  });
  Object.assign(raw.playerMovementController, {
    capturePositionView: () => () => true,
    projectOwnedPosition: vi.fn(),
  });
  raw.removeMapLegend = vi.fn();
  vi.spyOn(PhaserNet, "requestMapScripts").mockImplementation(() => undefined);
  return { ...harness, raw, camera, render: () => { for (const listener of [...listeners]) listener(); }, listeners };
}

it("keeps the opaque loading owner through terrain, recovery and the first unfaded frame", async () => {
  const { loader, raw, scene, camera, render, listeners } = createInteriorLoader();
  const terrain = deferred();
  raw.tileManager.loadTileImages.mockImplementationOnce(() => terrain.promise);
  let recover!: (value: any) => void;
  vi.mocked(recovery.readCurrentGameplayState).mockImplementationOnce(() => new Promise(resolve => { recover = resolve; }));
  const pending = loader.loadMapData(50);
  await vi.waitFor(() => expect(raw.tileManager.loadTileImages).toHaveBeenCalled());
  expect(useGameStatusStore.getState()).toMatchObject({ isMapLoading: true, mapLoading: { message: "Loading terrain...", failed: false } });
  expect(scene.mapLoadInProgress).toBe(true);
  expect(raw.mapRenderer.renderMap).not.toHaveBeenCalled();
  terrain.resolve();
  await vi.waitFor(() => expect(recovery.readCurrentGameplayState).toHaveBeenCalled());
  expect(useGameStatusStore.getState().isMapLoading).toBe(true);
  recover({ position: { mapId: 50, x: 2, y: 3, direction: "DOWN" } });
  await vi.waitFor(() => expect(listeners.size).toBe(1));
  render();
  expect(useGameStatusStore.getState().isMapLoading).toBe(true);
  camera.fadeEffect.isRunning = false;
  render();
  await pending;
  expect(scene.mapLoadInProgress).toBe(false);
  expect(useGameStatusStore.getState()).toMatchObject({ isMapLoading: false, mapLoading: null });
  expect(listeners.size).toBe(0);
});

it.each(["terrain", "characters"])("keeps an interior locked after missing %s and lets Retry start a new owned load", async resource => {
  const { loader, raw, scene } = createInteriorLoader();
  const failingRead = resource === "terrain" ? raw.tileManager.loadTileImages : raw.mapDataService.fetchActors;
  failingRead.mockRejectedValueOnce(new Error(`missing ${resource}`));
  await loader.loadMapData(50);
  expect(scene.mapLoadInProgress).toBe(true);
  expect(useGameStatusStore.getState()).toMatchObject({ isMapLoading: true, mapLoading: { failed: true } });
  expect(raw.mapRenderer.renderMap).not.toHaveBeenCalled();
  const owner = useGameStatusStore.getState().mapLoading;
  const nextTerrain = deferred();
  raw.tileManager.loadTileImages.mockImplementationOnce(() => nextTerrain.promise);
  owner!.retry();
  expect(useGameStatusStore.getState().mapLoading?.owner).not.toBe(owner!.owner);
  expect(useGameStatusStore.getState().mapLoading?.failed).toBe(false);
  loader.cleanup();
  nextTerrain.resolve();
});

it("a retired scene cannot dismiss another scene's loading screen", async () => {
  const first = createPendingLoader();
  const next = createPendingLoader();
  const firstLoad = first.loader.loadMapData(50);
  const nextLoad = next.loader.loadMapData(51);
  const nextOwner = useGameStatusStore.getState().mapLoading?.owner;
  first.loader.cleanup();
  expect(useGameStatusStore.getState().isMapLoading).toBe(true);
  expect(useGameStatusStore.getState().mapLoading?.owner).toBe(nextOwner);
  next.loader.cleanup();
  first.compatibility.resolve();
  next.compatibility.resolve();
  await Promise.all([firstLoad, nextLoad]);
  expect(useGameStatusStore.getState().isMapLoading).toBe(false);
});

it("retires a pending render-frame wait during scene cleanup", async () => {
  const { loader, raw, listeners } = createInteriorLoader();
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({ position: { mapId: 50 } } as any);
  const pending = loader.loadMapData(50);
  await vi.waitFor(() => expect(listeners.size).toBe(1));
  loader.cleanup();
  await pending;
  expect(listeners.size).toBe(0);
  expect(raw.uiManager.hideLoadingText).not.toHaveBeenCalled();
  expect(useGameStatusStore.getState().isMapLoading).toBe(false);
});

it("plans initial overview readiness from the newly rendered camera instead of its previous indoor transform", async () => {
  const { loader, raw, scene, camera, render, listeners } = createInteriorLoader();
  raw.mapDataService.fetchMapInfo.mockResolvedValue({
    id: 9999, name: "Unified Overworld", width: 1024, height: 1024,
    isOverworld: 1, tileMinX: 0, tileMinY: 0, tileMaxX: 1023, tileMaxY: 1023,
  });
  let renderedScale = 1;
  camera.getWorldPoint = vi.fn((x: number, y: number) => ({ x: x * renderedScale, y: y * renderedScale }));
  const visibleTerrain = deferred();
  raw.overworldOverviewLayer.sync.mockImplementationOnce(() => visibleTerrain.promise);
  const pending = loader.loadOverworldData({ viewOnly: true });
  await vi.waitFor(() => expect(listeners.size).toBe(1));
  expect(camera.getWorldPoint).not.toHaveBeenCalled();
  expect(raw.mapDataService.fetchTilesInBounds).not.toHaveBeenCalled();
  expect(raw.overworldOverviewLayer.sync).not.toHaveBeenCalled();
  // Phaser updates its camera matrix at preRender, before this frame event.
  renderedScale = 16;
  render();
  await vi.waitFor(() => expect(raw.overworldOverviewLayer.sync).toHaveBeenCalledOnce());
  const requiredKeys = raw.overworldOverviewLayer.sync.mock.calls[0][3] as ReadonlySet<string>;
  expect(requiredKeys.size).toBeGreaterThan(1);
  expect(loader.getOverworldChunkStatus()?.mode).toBe("overview");
  expect(raw.mapDataService.fetchTilesInBounds).not.toHaveBeenCalled();
  expect(useGameStatusStore.getState().isMapLoading).toBe(true);
  expect(scene.mapLoadInProgress).toBe(true);
  visibleTerrain.resolve();
  await vi.waitFor(() => expect(listeners.size).toBe(1));
  camera.fadeEffect.isRunning = false;
  render();
  await pending;
  expect(useGameStatusStore.getState().isMapLoading).toBe(false);
  loader.cleanup();
});
