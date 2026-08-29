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
  PhaserMapInfo,
  PhaserTile,
  PhaserTilesRequest,
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
}

/**
 * Create a promise that rejects after a timeout
 */
function createTimeoutPromise<T>(ms: number, errorMessage: string): Promise<T> {
  return new Promise((_, reject) => {
    setTimeout(() => reject(new Error(errorMessage)), ms);
  });
}

export class MapDataService {
  private static readonly MAX_CACHED_OVERWORLD_CHUNKS = 18;
  // Cache of known tile image IDs from tiles
  private knownTileImageIds: Set<number> = new Set();
  private snapshots = new MapSnapshotCache<MapDataSnapshot>(
    3,
    new Set([UNIFIED_OVERWORLD_MAP_ID]),
  );
  private tileRequestSequence = 0;
  private overworldTileChunks = new Map<string, CachedTileChunk>();

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
  ): void {
    this.overworldTileChunks.delete(key);
    this.overworldTileChunks.set(key, { bounds, tiles });
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
        x >= chunk.bounds.minX &&
        x <= chunk.bounds.maxX &&
        y >= chunk.bounds.minY &&
        y <= chunk.bounds.maxY
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
  async fetchMapInfo(mapId: number, destX?: number, destY?: number): Promise<PhaserMapInfo> {
    if (!PhaserNet.isConnected()) {
      throw new Error("Not connected to server - please log in first");
    }

    const dataPromise = new Promise<PhaserMapInfo>((resolve) => {
      const unsubscribe = PhaserNet.onMapInfo((data) => {
        if (data.id === mapId) {
          unsubscribe();
          resolve(data);
        }
      });
      PhaserNet.requestMapInfo(mapId, destX, destY);
    });

    return Promise.race([
      dataPromise,
      createTimeoutPromise<PhaserMapInfo>(REQUEST_TIMEOUT_MS, `Timeout fetching map info for map ${mapId}`)
    ]);
  }

  /**
   * Fetch tiles for a specific map
   */
  async fetchTiles(mapId: number): Promise<PhaserTile[]> {
    return (await this.requestTileBatch(mapId, {})).tiles;
  }

  async fetchTilesInBounds(mapId: number, bounds: TileBoundsRequest): Promise<PhaserTile[]> {
    return (await this.requestTileBatch(mapId, bounds)).tiles;
  }

  async fetchTilePage(mapId: number, afterId: number, limit: number): Promise<TilePage> {
    return this.requestTileBatch(mapId, { afterId, limit });
  }

  private async requestTileBatch(
    mapId: number,
    options: Omit<PhaserTilesRequest, "mapId" | "requestId">,
  ): Promise<TilePage> {
    await this.ensureRuntimeTileCatalogCurrent();
    if (!PhaserNet.isConnected()) {
      throw new Error("Not connected to server - please log in first");
    }

    const requestId = `tiles-${++this.tileRequestSequence}`;
    const timeoutMs = mapId === UNIFIED_OVERWORLD_MAP_ID
      ? OVERWORLD_TILE_REQUEST_TIMEOUT_MS
      : REQUEST_TIMEOUT_MS;

    return new Promise<TilePage>((resolve, reject) => {
      const unsubscribe = PhaserNet.onTiles((data) => {
        // A raw array is the backward-compatible response from a server that
        // predates request correlation and paging.
        if (Array.isArray(data)) {
          cleanup();
          cacheTileImageIds(data);
          resolve({ tiles: data, nextAfterId: 0, hasMore: false });
          return;
        }
        if (data.requestId !== requestId) {
          return;
        }
        cleanup();
        if (data.error) {
          reject(new Error(data.error));
          return;
        }
        cacheTileImageIds(data.tiles);
        resolve({
          tiles: data.tiles,
          nextAfterId: data.nextAfterId,
          hasMore: data.hasMore,
        });
      });
      const cacheTileImageIds = (tiles: PhaserTile[]) => {
        for (const tile of tiles) {
          this.knownTileImageIds.add(tile.tileImageId);
        }
      };
      const cleanup = () => {
        unsubscribe();
        clearTimeout(timeoutId);
      };

      const timeoutId = setTimeout(() => {
        cleanup();
        reject(new Error(`Timeout fetching tiles for map ${mapId}`));
      }, timeoutMs);
      PhaserNet.requestTiles({ mapId, requestId, ...options });
    });
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
   * Fetch all overworld maps
   */
  async fetchOverworldMaps(): Promise<PhaserMapInfo[]> {
    if (!PhaserNet.isConnected()) {
      throw new Error("Not connected to server - please log in first");
    }

    const dataPromise = new Promise<PhaserMapInfo[]>((resolve) => {
      const unsubscribe = PhaserNet.onOverworldMaps((data) => {
        unsubscribe();
        resolve(data || []);
      });
      PhaserNet.requestOverworldMaps();
    });

    return Promise.race([
      dataPromise,
      createTimeoutPromise<PhaserMapInfo[]>(REQUEST_TIMEOUT_MS, "Timeout fetching overworld maps")
    ]);
  }

  /**
   * Fetch actors for a specific map (or all maps if mapId is omitted)
   */
  async fetchActors(mapId?: number): Promise<PhaserActor[]> {
    if (!PhaserNet.isConnected()) {
      throw new Error("Not connected to server - please log in first");
    }

    // If no mapId provided, return empty - caller should use mapId
    if (mapId === undefined) {
      // console.warn("fetchActors called without mapId - returning empty array");
      return [];
    }

    const dataPromise = new Promise<PhaserActor[]>((resolve) => {
      const unsubscribe = PhaserNet.onActors((data) => {
        unsubscribe();
        resolve(data || []);
      });
      PhaserNet.requestActors(mapId);
    });

    return Promise.race([
      dataPromise,
      createTimeoutPromise<PhaserActor[]>(REQUEST_TIMEOUT_MS, `Timeout fetching actors for map ${mapId}`)
    ]);
  }

  /**
   * Fetch warps for a specific map (or empty if mapId is omitted)
   */
  async fetchWarps(mapId?: number): Promise<PhaserWarp[]> {
    if (!PhaserNet.isConnected()) {
      throw new Error("Not connected to server - please log in first");
    }

    // If no mapId provided, return empty - caller should use mapId
    if (mapId === undefined) {
      console.warn("fetchWarps called without mapId - returning empty array");
      return [];
    }

    const dataPromise = new Promise<PhaserWarp[]>((resolve) => {
      const unsubscribe = PhaserNet.onWarps((data) => {
        unsubscribe();
        resolve(data || []);
      });
      PhaserNet.requestWarps(mapId);
    });

    return Promise.race([
      dataPromise,
      createTimeoutPromise<PhaserWarp[]>(REQUEST_TIMEOUT_MS, `Timeout fetching warps for map ${mapId}`)
    ]);
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
