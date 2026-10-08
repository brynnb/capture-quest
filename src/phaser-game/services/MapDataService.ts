import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import { correlatedRequest } from "./CorrelatedRequest";
import type { PhaserMapInfo, PhaserMapInfoResponse, PhaserMapLoadResponse, PhaserWarpActivateResponse, PhaserInstantWarpResponse } from "@/net/generated/protocol";
/**
 * MapDataService - Phaser map data fetching via WebTransport
 *
 * Uses PhaserNetworkService for WebTransport communication instead of REST API.
 * Returns Promises that resolve when data is received from the server.
 *
 * IMPORTANT: Requires WebTransport connection to be established first (via login).
 */

import * as PhaserNet from "./PhaserNetworkService";
import { getTileImageUrl } from "../api/tileService";
import type {
  PhaserTile,
  PhaserTilesRequest,
  PhaserTilesResponse,
  PhaserActor,
  PhaserWarp
} from "@/net/generated/world_api";
import { UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import type { MapItem } from "../renderers/MapRenderer";
import { MapSnapshotCache } from "./MapSnapshotCache";
import { ensureRuntimeTileCatalogCurrent } from "./RuntimeAssetCompatibility";

// Default timeout for network requests (10 seconds)
const REQUEST_TIMEOUT_MS = 10000;
// The unified overworld currently contains more than 43,000 tiles in one
// reliable-stream response. Slow mobile or distant connections need a larger
// transfer budget than compact interior maps.
const OVERWORLD_TILE_REQUEST_TIMEOUT_MS = 30000;

// Tile image data format (for TileManager compatibility)
export interface TileImageData {
  id: number;
  image_path: string;
}

export interface MapDataSnapshot {
  mapInfo: PhaserMapInfo;
  tiles: PhaserTile[];
  warps: PhaserWarp[];
  actors: PhaserActor[];
}

export interface TileBoundsRequest {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

export interface TilePage {
  tiles: PhaserTile[];
  nextAfterId: number;
  hasMore: boolean;
}

export interface CachedTileChunk {
  bounds: TileBoundsRequest;
  tiles: PhaserTile[];
  renderBounds: TileBoundsRequest;
  renderTiles: PhaserTile[];
}

function normalizeCorrelatedTiles(data: PhaserTilesResponse): PhaserTile[] {
  const tiles = (data as { tiles?: unknown }).tiles;
  if (!Array.isArray(tiles)) {
    throw new Error("Invalid tile response: tiles must be an array");
  }
  return tiles as PhaserTile[];
}

export class MapDataService {
  private static readonly MAX_CACHED_OVERWORLD_CHUNKS = 18;
  // Cache of known tile image IDs from tiles
  private tileReadRevision = 0;
  private tileReadViews = new WeakMap<PhaserTile[],number>();
  private knownTileImageIds: Set<number> = new Set();
  private snapshots = new MapSnapshotCache<MapDataSnapshot>(
    3,
    new Set([UNIFIED_OVERWORLD_MAP_ID]),
  );
  private overworldTileChunks = new Map<string, CachedTileChunk>();

  isTileReadCurrent(tiles: PhaserTile[]):boolean { return this.tileReadViews.get(tiles)===this.tileReadRevision; }

  recordCommittedTileUpdate(): void {
    this.tileReadRevision++;
    // At most three snapshots are retained. None may carry older character
    // tile projection through a streamed change or a later map return.
    this.snapshots.clear();
  }

  getSnapshot(mapId: number): MapDataSnapshot | undefined {
    return this.snapshots.get(mapId);
  }

  setSnapshot(mapId: number, snapshot: MapDataSnapshot): void {
    this.snapshots.set(mapId, snapshot);
  }

  getOverworldTileChunk(key: string): CachedTileChunk | undefined {
    const cached = this.overworldTileChunks.get(key);
    if (!cached) return undefined;
    this.overworldTileChunks.delete(key);
    this.overworldTileChunks.set(key, cached);
    return cached;
  }

  setOverworldTileChunk(
    key: string,
    bounds: TileBoundsRequest,
    tiles: PhaserTile[],
    renderBounds: TileBoundsRequest = bounds,
    renderTiles: PhaserTile[] = tiles,
  ): void {
    this.overworldTileChunks.delete(key);
    this.overworldTileChunks.set(key, {
      bounds,
      tiles,
      renderBounds,
      renderTiles,
    });
    while (
      this.overworldTileChunks.size >
      MapDataService.MAX_CACHED_OVERWORLD_CHUNKS
    ) {
      const oldestKey = this.overworldTileChunks.keys().next().value;
      if (oldestKey === undefined) break;
      this.overworldTileChunks.delete(oldestKey);
    }
  }

  invalidateOverworldTileChunkAt(x: number, y: number): void {
    for (const [key, chunk] of this.overworldTileChunks) {
      if (
        x >= chunk.renderBounds.minX &&
        x <= chunk.renderBounds.maxX &&
        y >= chunk.renderBounds.minY &&
        y <= chunk.renderBounds.maxY
      ) {
        this.overworldTileChunks.delete(key);
      }
    }
  }

  /**
   * Check if a map ID is part of the overworld
   */
  isOverworld(mapId: number): boolean {
    // UNIFIED_OVERWORLD_MAP_ID is the explicit unified overworld ID
    if (mapId === UNIFIED_OVERWORLD_MAP_ID) return true;

    // For now, we can legacy-check map IDs 0-33 as overworld
    // (though the server should be sending 9999)
    return mapId >= 0 && mapId <= 33;
  }

  /**
   * Check if connection is ready for Phaser data
   */
  isReady(): boolean {
    return PhaserNet.isConnected();
  }

  ensureRuntimeTileCatalogCurrent(force = false): Promise<void> {
    return ensureRuntimeTileCatalogCurrent(force);
  }

  /**
   * Fetch map info by ID - returns a Promise that resolves when data arrives
   */
  async fetchMapInfo(mapId: number, signal?: AbortSignal): Promise<PhaserMapInfo> {
    const response = await correlatedRequest<PhaserMapInfoResponse>(
      (receive) => PhaserNet.onMapInfo(receive),
      (requestId) => PhaserNet.requestMapInfo({ mapId, requestId }),
      signal,
    );
    if (response.id !== mapId) throw new Error("Map response identity disagrees with the request");
    return response;
  }

  async prepareMapLoad(mapId: number, signal?: AbortSignal): Promise<void> {
    await correlatedRequest<PhaserMapLoadResponse>(
      (receive) => PhaserNet.onMapLoad(receive),
      (requestId) => PhaserNet.requestMapLoad({ mapId, requestId }),
      signal,
    );
  }

  async instantWarp(mapId: number, x: number, y: number, direction: string, signal?: AbortSignal): Promise<PhaserInstantWarpResponse> {
    return correlatedRequest<PhaserInstantWarpResponse>(
      (receive) => PhaserNet.onInstantWarp(receive),
      (requestId) => PhaserNet.requestInstantWarp({ mapId, x, y, direction, requestId }),
      signal,
    );
  }

  async activateWarp(warpId: number, direction: string, inputSource: "click" | "keyboard", signal?: AbortSignal): Promise<PhaserWarpActivateResponse> {
    return correlatedRequest<PhaserWarpActivateResponse>(
      (receive) => PhaserNet.onWarpActivation(receive),
      (requestId) => PhaserNet.requestWarpActivation({ warpId, direction, inputSource, requestId }),
      signal,
    );
  }

  /**
   * Fetch tiles for a specific map
   */
  async fetchTiles(mapId: number, signal?: AbortSignal): Promise<PhaserTile[]> {
    return (await this.requestTileBatch(mapId, {}, signal)).tiles;
  }

  async fetchTilesInBounds(mapId: number, bounds: TileBoundsRequest, signal?: AbortSignal): Promise<PhaserTile[]> {
    return (await this.requestTileBatch(mapId, bounds, signal)).tiles;
  }

  async fetchTilePage(mapId: number, afterId: number, limit: number, signal?: AbortSignal): Promise<TilePage> {
    return this.requestTileBatch(mapId, { afterId, limit }, signal);
  }

  private async requestTileBatch(mapId:number,options:Omit<PhaserTilesRequest,"mapId"|"requestId">,signal?:AbortSignal):Promise<TilePage>{
    await this.ensureRuntimeTileCatalogCurrent();
    const characterId=usePlayerCharacterStore.getState().characterProfile.id;
    for(let attempt=0;attempt<2;attempt++){
      const revision=this.tileReadRevision;
      const response=await correlatedRequest<PhaserTilesResponse>(PhaserNet.onTiles,requestId=>PhaserNet.requestTiles({mapId,requestId,...options}),signal,mapId===UNIFIED_OVERWORLD_MAP_ID?OVERWORLD_TILE_REQUEST_TIMEOUT_MS:REQUEST_TIMEOUT_MS);
    if(signal?.aborted || usePlayerCharacterStore.getState().characterProfile.id!==characterId) throw new DOMException("Tile view retired","AbortError");
    if(response.mapId!==mapId || response.characterId!==characterId || !Number.isSafeInteger(response.nextAfterId) || response.nextAfterId<0 || typeof response.hasMore!=="boolean") throw new Error("Invalid owned tile response");
    if(revision!==this.tileReadRevision)continue;
    const tiles=normalizeCorrelatedTiles(response);
    this.tileReadViews.set(tiles,revision);
    for(const tile of tiles)this.knownTileImageIds.add(tile.tileImageId);
    return {tiles,nextAfterId:response.nextAfterId,hasMore:response.hasMore};
    }
    throw new Error("Tiles changed during read; another owned read is required");
  }

  /**
   * Generate tile image data from known tile IDs.
   * Called after fetchTiles() to get tile image URLs for loading.
   * This is a local operation - tile images are static files.
   */
  async fetchTileImages(): Promise<TileImageData[]> {
    // Generate tile image data for all known tile image IDs
    const tileImages: TileImageData[] = [];
    for (const id of this.knownTileImageIds) {
      tileImages.push({
        id,
        image_path: getTileImageUrl(id)
      });
    }
    return tileImages;
  }

  /**
   * Fetch actors for a specific map (or all maps if mapId is omitted)
   */
  async fetchActors(mapId?: number, signal?: AbortSignal): Promise<PhaserActor[]> {
    if (mapId === undefined) return [];
    const characterId = usePlayerCharacterStore.getState().characterProfile.id;
    if (!characterId) throw new Error("Actor view requires a selected character");
    const response = await correlatedRequest<import("@/net/generated/world_api").PhaserActorsResponse>(
      PhaserNet.onActors, requestId => PhaserNet.requestActors({ mapId, characterId, requestId }), signal,
    );
    if (signal?.aborted || usePlayerCharacterStore.getState().characterProfile.id !== characterId)
      throw new DOMException("Actor owner retired", "AbortError");
    if (response.characterId !== characterId || response.mapId !== mapId || !Array.isArray(response.actors))
      throw new Error("Actor view ownership mismatch");
    return response.actors;
  }

  /**
   * Fetch warps for a specific map (or empty if mapId is omitted)
   */
  async fetchWarps(mapId: number, signal?: AbortSignal): Promise<PhaserWarp[]> {
    const characterId=usePlayerCharacterStore.getState().characterProfile.id;
    const response=await correlatedRequest<import("@/net/generated/world_api").PhaserWarpsResponse>(PhaserNet.onWarps,id=>PhaserNet.requestWarps({mapId,requestId:id}),signal);
    if (signal?.aborted || usePlayerCharacterStore.getState().characterProfile.id!==characterId) throw new DOMException("Warp view retired","AbortError");
    if(response.mapId!==mapId || response.characterId!==characterId || !Array.isArray(response.warps)) throw new Error("Invalid owned warp view");
    return response.warps;
  }

  /**
   * Fetch items - Pokemon items come from objects with type 'item'
   * Items are fetched as part of actors/objects for a map.
   * For backwards compat, returns empty array - use fetchActors with mapId instead.
   */
  async fetchItems(): Promise<MapItem[]> {
    // console.warn("fetchItems is deprecated - items come from fetchActors() objects");
    return [];
  }

  /**
   * Subscribe to real-time actor position updates
   * Returns unsubscribe function
   */
  onActorUpdate(callback: (actor: PhaserActor) => void): () => void {
    return PhaserNet.onActorUpdate(callback);
  }

  /**
   * Clear the tile image cache
   */
  clearCache(): void {
    this.knownTileImageIds.clear();
    this.snapshots.clear();
    this.overworldTileChunks.clear();
  }
}
