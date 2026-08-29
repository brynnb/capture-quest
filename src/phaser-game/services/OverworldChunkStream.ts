import type { PhaserTile } from "@/net/generated/world_api";

import type { PlayerMovementController } from "../controllers/PlayerMovementController";
import type { MapRenderer } from "../renderers/MapRenderer";
import type { OverworldOverviewLayer } from "../renderers/OverworldOverviewLayer";
import type { TileManager } from "../managers";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import type { MapDataService } from "./MapDataService";
import {
  OVERWORLD_CHUNK_SIZE_TILES,
  overworldChunkKey,
  overworldChunksCoveringBounds,
  planOverworldChunks,
  type CameraWorldView,
  type InclusiveTileBounds,
  type OverworldChunk,
  type OverworldChunkPlan,
} from "./OverworldChunkPlanner";

const CHUNK_LOAD_CONCURRENCY = 2;
const INITIAL_PLAN_RETRY_DELAY_MS = 500;
const MAXIMUM_PLAN_RETRY_DELAY_MS = 8_000;

export interface OverworldChunkStreamStatus {
  mode: "exact" | "overview";
  loadedExactChunkKeys: string[];
  requestedChunkKeys: string[];
  overviewChunkKeys: string[];
}

export interface CommittedOverworldTileUpdate {
  x: number;
  y: number;
  tileImageId: number;
  collisionType: number;
  rawFootTileId?: number;
  talkOverTile?: boolean;
  erased?: boolean;
}

interface OverworldChunkStreamOptions {
  mapBounds: InclusiveTileBounds;
  viewOnly: boolean;
  mapDataService: MapDataService;
  tileManager: TileManager;
  mapRenderer: MapRenderer;
  movementController: PlayerMovementController;
  overviewLayer: OverworldOverviewLayer;
  onTilesChanged: (tiles: PhaserTile[]) => void;
}

interface QueuedExactFetch {
  chunk: OverworldChunk;
  resolve: (result: FetchedExactChunk) => void;
  reject: (error: unknown) => void;
}

interface FetchedExactChunk {
  tiles: PhaserTile[];
  renderTiles: PhaserTile[];
  revision: number;
}

interface StagedExactChunk extends FetchedExactChunk {
  chunk: OverworldChunk;
}

export class OverworldChunkStream {
  private readonly loadedExactChunks = new Map<string, PhaserTile[]>();
  private readonly pendingExactChunks = new Map<
    string,
    Promise<FetchedExactChunk>
  >();
  private readonly chunkRevisionByKey = new Map<string, number>();
  private readonly exactFetchQueue: QueuedExactFetch[] = [];
  private readonly desiredExactChunkKeys = new Set<string>();
  private latestAdHocExactChunkKey: string | null = null;
  private adHocRequestSequence = 0;
  private activeExactFetches = 0;
  private requestGeneration = 0;
  private planSignature = "";
  private lastCamera: CameraWorldView | null = null;
  private lastPlan: OverworldChunkPlan | null = null;
  private lastMode: "exact" | "overview" = "exact";
  private stopped = false;
  private overviewChunkKeys: string[] = [];
  private queuedCameraUpdate: CameraWorldView | null = null;
  private updateDrainRunning = false;
  private failedPlanSignature = "";
  private planRetryNotBefore = 0;
  private planRetryDelayMs = INITIAL_PLAN_RETRY_DELAY_MS;

  constructor(private readonly options: OverworldChunkStreamOptions) {}

  async initialize(camera: CameraWorldView): Promise<void> {
    this.stopped = false;
    await this.applyCamera(camera, true);
  }

  update(camera: CameraWorldView): void {
    if (this.stopped) return;
    // Keep only the newest camera target. WebTransport tile requests are not
    // individually abortable, so serializing plan application is what keeps a
    // rapid drag from spawning a fresh two-request pool every 100ms.
    this.queuedCameraUpdate = { ...camera };
    this.startUpdateDrain();
  }

  async ensureTileAvailable(x: number, y: number): Promise<boolean> {
    if (
      x < this.options.mapBounds.minX ||
      x > this.options.mapBounds.maxX ||
      y < this.options.mapBounds.minY ||
      y > this.options.mapBounds.maxY
    ) {
      return false;
    }
    const chunk = overworldChunksCoveringBounds(
      { minX: x, minY: y, maxX: x, maxY: y },
      this.options.mapBounds,
    )[0];
    if (!chunk) return false;
    const requestSequence = ++this.adHocRequestSequence;
    this.latestAdHocExactChunkKey = chunk.key;
    this.purgeObsoleteQueuedExactFetches();
    try {
      const { tiles } = await this.fetchChunk(chunk, true);
      if (requestSequence !== this.adHocRequestSequence) return false;
      return tiles.some((tile) => tile.x === x && tile.y === y);
    } finally {
      if (requestSequence === this.adHocRequestSequence) {
        this.latestAdHocExactChunkKey = null;
        this.purgeObsoleteQueuedExactFetches();
      }
    }
  }

  invalidateAt(x: number, y: number): void {
    this.invalidateCoordinates([{ x, y }]);
    if (this.lastCamera && this.lastMode === "overview") {
      this.planSignature = "";
      this.update(this.lastCamera);
    }
  }

  /**
   * Keep the resident exact-tile aggregate coherent with a committed editor
   * broadcast. The renderer and collision controller apply the same broadcast
   * in TileViewer; this copy prevents a later chunk callback from resurrecting
   * an erased tile or dropping a newly painted one from lookup state.
   */
  applyCommittedTileUpdates(
    updates: readonly CommittedOverworldTileUpdate[],
  ): void {
    if (updates.length === 0) return;
    this.invalidateCoordinates(updates);

    let exactTilesChanged = false;
    for (const update of updates) {
      const key = overworldChunkKey(
        Math.floor(update.x / OVERWORLD_CHUNK_SIZE_TILES),
        Math.floor(update.y / OVERWORLD_CHUNK_SIZE_TILES),
      );
      const tiles = this.loadedExactChunks.get(key);
      if (!tiles) continue;

      const index = tiles.findIndex(
        (tile) => tile.x === update.x && tile.y === update.y,
      );
      if (update.erased || update.tileImageId === 0) {
        if (index >= 0) {
          tiles.splice(index, 1);
          exactTilesChanged = true;
        }
        continue;
      }

      const existing = index >= 0 ? tiles[index] : undefined;
      const next: PhaserTile = {
        id: existing?.id ?? -1,
        x: update.x,
        y: update.y,
        tileImageId: update.tileImageId,
        mapId: UNIFIED_OVERWORLD_MAP_ID,
        collisionType: update.collisionType,
        rawFootTileId: update.rawFootTileId,
        talkOverTile: update.talkOverTile === true,
        isNativeGameData: existing?.isNativeGameData ?? false,
        coordinateOrigin: existing?.coordinateOrigin ?? "user",
        contentOrigin: "user",
        ...(existing?.localX === undefined ? {} : { localX: existing.localX }),
        ...(existing?.localY === undefined ? {} : { localY: existing.localY }),
        ...(existing?.sourceMapId === undefined
          ? {}
          : { sourceMapId: existing.sourceMapId }),
        ...(existing?.sourceMapName === undefined
          ? {}
          : { sourceMapName: existing.sourceMapName }),
      };
      if (index >= 0) tiles[index] = next;
      else tiles.push(next);
      exactTilesChanged = true;
    }

    if (exactTilesChanged) this.emitTiles();
    if (this.lastCamera && this.lastMode === "overview") {
      this.planSignature = "";
      this.update(this.lastCamera);
    }
  }

  invalidateOverviewCatalog(): void {
    for (const key of this.overviewChunkKeys) {
      this.chunkRevisionByKey.set(
        key,
        (this.chunkRevisionByKey.get(key) ?? 0) + 1,
      );
    }
    this.options.overviewLayer.clear();
    if (this.lastCamera && this.lastMode === "overview") {
      this.planSignature = "";
      this.update(this.lastCamera);
    }
  }

  getStatus(): OverworldChunkStreamStatus {
    return {
      mode: this.lastMode,
      loadedExactChunkKeys: [...this.loadedExactChunks.keys()].sort(),
      requestedChunkKeys:
        this.lastMode === "exact" && this.lastPlan?.mode === "exact"
          ? this.lastPlan.desiredChunks.map((chunk) => chunk.key)
          : [],
      overviewChunkKeys: [...this.overviewChunkKeys],
    };
  }

  stop(): void {
    this.stopped = true;
    this.requestGeneration += 1;
    this.queuedCameraUpdate = null;
    for (const queued of this.exactFetchQueue.splice(0)) {
      queued.resolve({
        tiles: [],
        renderTiles: [],
        revision: this.chunkRevisionByKey.get(queued.chunk.key) ?? 0,
      });
    }
    this.desiredExactChunkKeys.clear();
    this.latestAdHocExactChunkKey = null;
    this.adHocRequestSequence += 1;
    for (const key of [...this.loadedExactChunks.keys()]) {
      this.unloadExactChunk(key);
    }
    this.pendingExactChunks.clear();
    this.options.overviewLayer.clear();
    this.overviewChunkKeys = [];
    this.lastPlan = null;
    this.lastMode = "exact";
    this.planSignature = "";
    this.failedPlanSignature = "";
    this.planRetryNotBefore = 0;
    this.planRetryDelayMs = INITIAL_PLAN_RETRY_DELAY_MS;
    this.emitTiles();
  }

  private startUpdateDrain(): void {
    if (this.updateDrainRunning || this.stopped) return;
    this.updateDrainRunning = true;
    void (async () => {
      while (!this.stopped && this.queuedCameraUpdate) {
        const next = this.queuedCameraUpdate;
        this.queuedCameraUpdate = null;
        await this.applyCamera(next, false);
      }
    })()
      .catch((error) => {
        console.error("[OverworldChunks] Camera update failed:", error);
      })
      .finally(() => {
        this.updateDrainRunning = false;
        if (this.queuedCameraUpdate && !this.stopped) {
          this.startUpdateDrain();
        }
      });
  }

  private async applyCamera(
    camera: CameraWorldView,
    throwOnFailure: boolean,
  ): Promise<void> {
    this.lastCamera = { ...camera };
    const plan = planOverworldChunks({
      camera,
      mapBounds: this.options.mapBounds,
    });
    // LOD follows the actual camera footprint. A normal gameplay camera that
    // fits within the exact 3x3-chunk budget must never be downgraded merely
    // because its zoom number crosses an unrelated threshold.
    const useOverview = plan.mode === "overview";
    const exactPlan = plan.mode === "exact" ? plan : null;
    const gameplayPlan = useOverview ? this.planGameplayFootprint() : null;
    const desiredExactChunks = useOverview
      ? (gameplayPlan?.desiredChunks ?? [])
      : (exactPlan?.desiredChunks ?? []);
    const overviewChunks = useOverview
      ? overworldChunksCoveringBounds(
          plan.visibleTileBounds,
          this.options.mapBounds,
          1,
        )
      : [];
    const signature =
      !useOverview && exactPlan
        ? `exact:required=${exactPlan.requiredChunks
            .map((chunk) => chunk.key)
            .join(",")};desired=${exactPlan.desiredChunks
            .map((chunk) => chunk.key)
            .join(",")}`
        : `overview:${overviewChunks
            .map((chunk) => chunk.key)
            .join(",")};gameplay=${desiredExactChunks
            .map((chunk) => chunk.key)
            .join(",")}`;
    if (signature === this.planSignature) return;
    if (
      signature === this.failedPlanSignature &&
      Date.now() < this.planRetryNotBefore
    ) {
      return;
    }

    this.planSignature = signature;
    this.lastPlan = plan;
    this.lastMode = useOverview ? "overview" : "exact";
    const generation = ++this.requestGeneration;
    this.replaceDesiredExactChunks(desiredExactChunks);

    try {
      if (useOverview) {
        await this.applyOverviewPlan(
          overviewChunks,
          camera,
          gameplayPlan,
          generation,
          signature,
        );
      } else {
        if (!exactPlan) {
          throw new Error("Exact overworld mode requires an exact chunk plan");
        }
        await this.applyExactPlan(exactPlan, generation, signature);
      }
      if (this.isCurrent(generation) && this.planSignature === signature) {
        this.failedPlanSignature = "";
        this.planRetryNotBefore = 0;
        this.planRetryDelayMs = INITIAL_PLAN_RETRY_DELAY_MS;
      }
    } catch (error) {
      this.markPlanForRetry(signature, generation);
      if (throwOnFailure) throw error;
      console.error("[OverworldChunks] Failed to apply chunk plan:", error);
    }
  }

  private async applyOverviewPlan(
    chunks: readonly OverworldChunk[],
    camera: CameraWorldView,
    gameplayPlan: Extract<OverworldChunkPlan, { mode: "exact" }> | null,
    generation: number,
    signature: string,
  ): Promise<void> {
    const prioritizedChunks = this.prioritizeOverviewChunks(chunks, camera);
    this.overviewChunkKeys = prioritizedChunks.map((chunk) => chunk.key);
    // A newly completed chunk inherits this state, so make the overview layer
    // visible before starting any network or texture-loader work.
    this.options.overviewLayer.setVisible(true);
    await this.options.overviewLayer.sync(
      prioritizedChunks.map((chunk) => ({
        key: chunk.key,
        column: chunk.chunkX,
        row: chunk.chunkY,
      })),
      (key) => this.chunkRevisionByKey.get(key) ?? 0,
      (error) => {
        if (this.markPlanForRetry(signature, generation)) {
          console.warn(
            "[OverworldChunks] Overview background load failed; scheduling retry:",
            error,
          );
        }
      },
    );
    if (!this.isCurrent(generation)) return;

    if (gameplayPlan) {
      // Overview pixels are presentation-only. Gameplay still needs a bounded
      // exact neighborhood so collision and tile semantics remain authoritative
      // when a following player zooms out.
      await this.loadChunks(gameplayPlan.desiredChunks, generation);
      if (!this.isCurrent(generation)) return;
    }
    this.evictNonDesiredExactChunks();
    this.emitTiles();
  }

  private planGameplayFootprint(): Extract<
    OverworldChunkPlan,
    { mode: "exact" }
  > | null {
    if (this.options.viewOnly) return null;
    const position = this.options.movementController.getCurrentPosition();
    if (!Number.isFinite(position.x) || !Number.isFinite(position.y))
      return null;
    const footprint = planOverworldChunks({
      camera: {
        x: Math.floor(position.x) * TILE_SIZE,
        y: Math.floor(position.y) * TILE_SIZE,
        width: TILE_SIZE,
        height: TILE_SIZE,
      },
      mapBounds: this.options.mapBounds,
    });
    return footprint.mode === "exact" ? footprint : null;
  }

  private prioritizeOverviewChunks(
    chunks: readonly OverworldChunk[],
    camera: CameraWorldView,
  ): OverworldChunk[] {
    const centerX = (camera.x + camera.width / 2) / TILE_SIZE;
    const centerY = (camera.y + camera.height / 2) / TILE_SIZE;
    const distanceSquared = (chunk: OverworldChunk) => {
      const dx =
        centerX < chunk.bounds.minX
          ? chunk.bounds.minX - centerX
          : centerX > chunk.bounds.maxX
            ? centerX - chunk.bounds.maxX
            : 0;
      const dy =
        centerY < chunk.bounds.minY
          ? chunk.bounds.minY - centerY
          : centerY > chunk.bounds.maxY
            ? centerY - chunk.bounds.maxY
            : 0;
      return dx * dx + dy * dy;
    };

    return [...chunks].sort((left, right) => {
      const distanceDifference = distanceSquared(left) - distanceSquared(right);
      if (distanceDifference !== 0) return distanceDifference;
      if (left.chunkY !== right.chunkY) return left.chunkY - right.chunkY;
      return left.chunkX - right.chunkX;
    });
  }

  private async applyExactPlan(
    plan: Extract<OverworldChunkPlan, { mode: "exact" }>,
    generation: number,
    signature: string,
  ): Promise<void> {
    await this.loadChunks(plan.requiredChunks, generation);
    if (!this.isCurrent(generation)) return;

    this.evictNonDesiredExactChunks();
    // Required exact data is now committed, so cancel all background overview
    // work and release its textures instead of downloading a hidden old plan.
    this.options.overviewLayer.clear();
    this.overviewChunkKeys = [];
    this.emitTiles();

    void this.loadChunks(plan.preloadChunks, generation)
      .then(() => {
        if (this.isCurrent(generation)) this.emitTiles();
      })
      .catch((error) => {
        if (this.isCurrent(generation)) {
          console.warn("[OverworldChunks] Prefetch failed:", error);
          this.markPlanForRetry(signature, generation);
        }
      });
  }

  private async loadChunks(
    chunks: readonly OverworldChunk[],
    generation: number,
  ): Promise<void> {
    const missingChunks = chunks.filter(
      (chunk) => !this.loadedExactChunks.has(chunk.key),
    );
    if (missingChunks.length === 0) return;

    for (;;) {
      const staged: StagedExactChunk[] = [];
      for (
        let index = 0;
        index < missingChunks.length;
        index += CHUNK_LOAD_CONCURRENCY
      ) {
        const batch = missingChunks.slice(
          index,
          index + CHUNK_LOAD_CONCURRENCY,
        );
        const loaded = await Promise.all(
          batch.map(async (chunk) => ({
            chunk,
            ...(await this.fetchChunk(chunk)),
          })),
        );
        if (!this.isCurrent(generation)) return;
        staged.push(...loaded);
      }

      await this.options.tileManager.loadTileImages(
        await this.options.mapDataService.fetchTileImages(),
      );
      if (!this.isCurrent(generation)) return;

      // A committed paint/erase can arrive while Phaser loads the tile image.
      // No renderer, resident lookup, or collision state is touched until every
      // staged chunk still matches the revision that produced its tile array.
      if (
        staged.some(
          ({ chunk, revision }) =>
            (this.chunkRevisionByKey.get(chunk.key) ?? 0) !== revision,
        )
      ) {
        continue;
      }

      // This synchronous section cannot interleave with an editor broadcast.
      // Evict the previous plan before allocating replacements so GPU and
      // collision residency never exceeds the plan's <=3x3 desired set.
      this.evictNonDesiredExactChunks();
      for (const { chunk, tiles, renderTiles } of staged) {
        if (this.loadedExactChunks.has(chunk.key)) continue;
        this.options.mapRenderer.upsertTileChunk(
          chunk.key,
          chunk.chunkX,
          chunk.chunkY,
          tiles,
          renderTiles,
        );
        this.loadedExactChunks.set(chunk.key, tiles);
        if (!this.options.viewOnly) {
          this.options.movementController.addCollisionTiles(tiles);
        }
      }
      return;
    }
  }

  private fetchChunk(
    chunk: OverworldChunk,
    prioritize = false,
  ): Promise<FetchedExactChunk> {
    const cached = this.options.mapDataService.getOverworldTileChunk(chunk.key);
    if (cached && this.boundsEqual(cached.bounds, chunk.bounds)) {
      return Promise.resolve({
        tiles: cached.tiles,
        renderTiles: cached.renderTiles,
        revision: this.chunkRevisionByKey.get(chunk.key) ?? 0,
      });
    }
    const pending = this.pendingExactChunks.get(chunk.key);
    if (pending) return this.recoverSkippedFetch(chunk, pending);

    const request = new Promise<FetchedExactChunk>((resolve, reject) => {
      const queued = { chunk, resolve, reject };
      if (prioritize) this.exactFetchQueue.unshift(queued);
      else this.exactFetchQueue.push(queued);
      this.pumpExactFetchQueue();
    }).finally(() => this.pendingExactChunks.delete(chunk.key));
    this.pendingExactChunks.set(chunk.key, request);
    return this.recoverSkippedFetch(chunk, request);
  }

  private async recoverSkippedFetch(
    chunk: OverworldChunk,
    request: Promise<FetchedExactChunk>,
  ): Promise<FetchedExactChunk> {
    const result = await request;
    if (result.tiles.length > 0) return result;
    const cached = this.options.mapDataService.getOverworldTileChunk(chunk.key);
    if (cached && this.boundsEqual(cached.bounds, chunk.bounds)) {
      return {
        tiles: cached.tiles,
        renderTiles: cached.renderTiles,
        revision: this.chunkRevisionByKey.get(chunk.key) ?? 0,
      };
    }
    // A stale queued plan resolves without touching the network. If a newer
    // plan or an explicit tile verification now needs the same key, enqueue a
    // fresh bounded request instead of mistaking that cancellation for a
    // genuinely empty (and cached) chunk.
    if (this.shouldFetchQueuedChunk(chunk.key)) return this.fetchChunk(chunk);
    return result;
  }

  private pumpExactFetchQueue(): void {
    while (
      !this.stopped &&
      this.activeExactFetches < CHUNK_LOAD_CONCURRENCY &&
      this.exactFetchQueue.length > 0
    ) {
      const queued = this.exactFetchQueue.shift();
      if (!queued) return;
      if (!this.shouldFetchQueuedChunk(queued.chunk.key)) {
        queued.resolve({
          tiles: [],
          renderTiles: [],
          revision: this.chunkRevisionByKey.get(queued.chunk.key) ?? 0,
        });
        continue;
      }

      this.activeExactFetches += 1;
      void this.fetchChunkAtCurrentRevision(queued.chunk)
        .then(queued.resolve, queued.reject)
        .finally(() => {
          this.activeExactFetches -= 1;
          this.pumpExactFetchQueue();
        });
    }
  }

  private shouldFetchQueuedChunk(key: string): boolean {
    return (
      this.latestAdHocExactChunkKey === key ||
      this.desiredExactChunkKeys.has(key)
    );
  }

  private async fetchChunkAtCurrentRevision(
    chunk: OverworldChunk,
  ): Promise<FetchedExactChunk> {
    const streamGeneration = this.requestGeneration;
    for (;;) {
      const revision = this.chunkRevisionByKey.get(chunk.key) ?? 0;
      // Include one neighboring row and column as a render-only halo. The
      // adjacent chunk contains the same source tiles, so their RenderTextures
      // overlap cleanly instead of exposing transparent framebuffer edges.
      const renderBounds = {
        ...chunk.bounds,
        maxX: Math.min(this.options.mapBounds.maxX, chunk.bounds.maxX + 1),
        maxY: Math.min(this.options.mapBounds.maxY, chunk.bounds.maxY + 1),
      };
      const renderTiles = await this.options.mapDataService.fetchTilesInBounds(
        UNIFIED_OVERWORLD_MAP_ID,
        renderBounds,
      );
      const tiles = renderTiles.filter(
        (tile) =>
          tile.x <= chunk.bounds.maxX && tile.y <= chunk.bounds.maxY,
      );
      if (this.stopped || streamGeneration !== this.requestGeneration) {
        // A stopped or superseded stream must never repopulate the shared LRU
        // after cleanup (especially after a committed editor invalidation).
        return { tiles: [], renderTiles: [], revision };
      }
      if ((this.chunkRevisionByKey.get(chunk.key) ?? 0) !== revision) {
        // A committed paint/erase landed while this request was in flight.
        // Retry the bounded chunk instead of letting the older response
        // overwrite the broadcast's renderer and collision updates.
        continue;
      }
      this.options.mapDataService.setOverworldTileChunk(
        chunk.key,
        chunk.bounds,
        tiles,
        renderBounds,
        renderTiles,
      );
      return { tiles, renderTiles, revision };
    }
  }

  private replaceDesiredExactChunks(chunks: readonly OverworldChunk[]): void {
    this.desiredExactChunkKeys.clear();
    for (const chunk of chunks) this.desiredExactChunkKeys.add(chunk.key);
    this.purgeObsoleteQueuedExactFetches();
  }

  private purgeObsoleteQueuedExactFetches(): void {
    for (let index = this.exactFetchQueue.length - 1; index >= 0; index -= 1) {
      const queued = this.exactFetchQueue[index];
      if (this.shouldFetchQueuedChunk(queued.chunk.key)) continue;
      this.exactFetchQueue.splice(index, 1);
      queued.resolve({
        tiles: [],
        renderTiles: [],
        revision: this.chunkRevisionByKey.get(queued.chunk.key) ?? 0,
      });
    }
  }

  private evictNonDesiredExactChunks(): void {
    for (const key of [...this.loadedExactChunks.keys()]) {
      if (!this.desiredExactChunkKeys.has(key)) this.unloadExactChunk(key);
    }
  }

  private markPlanForRetry(signature: string, generation: number): boolean {
    if (!this.isCurrent(generation)) return false;
    if (this.planSignature === "" && this.failedPlanSignature === signature) {
      return false;
    }
    this.planSignature = "";
    this.failedPlanSignature = signature;
    this.planRetryNotBefore = Date.now() + this.planRetryDelayMs;
    this.planRetryDelayMs = Math.min(
      MAXIMUM_PLAN_RETRY_DELAY_MS,
      this.planRetryDelayMs * 2,
    );
    return true;
  }

  private boundsEqual(
    left: InclusiveTileBounds,
    right: InclusiveTileBounds,
  ): boolean {
    return (
      left.minX === right.minX &&
      left.minY === right.minY &&
      left.maxX === right.maxX &&
      left.maxY === right.maxY
    );
  }

  private invalidateCoordinates(
    coordinates: readonly { x: number; y: number }[],
  ): void {
    const affectedKeys = new Set<string>();
    for (const { x, y } of coordinates) {
      this.options.mapDataService.invalidateOverworldTileChunkAt(x, y);
      affectedKeys.add(
        overworldChunkKey(
          Math.floor(x / OVERWORLD_CHUNK_SIZE_TILES),
          Math.floor(y / OVERWORLD_CHUNK_SIZE_TILES),
        ),
      );
    }
    for (const key of affectedKeys) {
      this.chunkRevisionByKey.set(
        key,
        (this.chunkRevisionByKey.get(key) ?? 0) + 1,
      );
      this.options.overviewLayer.invalidate(key);
    }
  }

  private unloadExactChunk(key: string): void {
    const tiles = this.loadedExactChunks.get(key);
    if (!tiles) return;
    this.options.mapRenderer.unloadTileChunk(key);
    if (!this.options.viewOnly) {
      this.options.movementController.removeCollisionTiles(tiles);
    }
    this.loadedExactChunks.delete(key);
  }

  private emitTiles(): void {
    this.options.onTilesChanged([...this.loadedExactChunks.values()].flat());
  }

  private isCurrent(generation: number): boolean {
    return !this.stopped && generation === this.requestGeneration;
  }
}
