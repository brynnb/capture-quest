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
  PhaserActor,
  PhaserWarp
} from "@/net/generated/world_api";
import { UNIFIED_OVERWORLD_MAP_ID } from "../constants";

// Default timeout for network requests (10 seconds)
const REQUEST_TIMEOUT_MS = 10000;

// Tile image data format (for TileManager compatibility)
export interface TileImageData {
  id: number;
  image_path: string;
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
  // Cache of known tile image IDs from tiles
  private knownTileImageIds: Set<number> = new Set();

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
    if (!PhaserNet.isConnected()) {
      throw new Error("Not connected to server - please log in first");
    }

    const dataPromise = new Promise<PhaserTile[]>((resolve, reject) => {
      const unsubscribe = PhaserNet.onTiles((data) => {
        if (!Array.isArray(data)) {
          unsubscribe();
          const errorMessage =
            data && typeof data === "object" && "error" in data
              ? String((data as { error?: unknown }).error)
              : `Invalid tile response for map ${mapId}`;
          reject(new Error(errorMessage));
          return;
        }
        // Cache tile image IDs from these tiles
        for (const tile of data) {
          this.knownTileImageIds.add(tile.tileImageId);
        }
        unsubscribe();
        resolve(data);
      });
      PhaserNet.requestTiles(mapId);
    });

    return Promise.race([
      dataPromise,
      createTimeoutPromise<PhaserTile[]>(REQUEST_TIMEOUT_MS, `Timeout fetching tiles for map ${mapId}`)
    ]);
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
  async fetchItems(): Promise<any[]> {
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
  }
}
