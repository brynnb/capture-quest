/**
 * Tile Service - Static tile image URL generation
 *
 * Tile images are served as static files from the Go backend.
 * Tiles are named tile_0.png through tile_825.png (0-indexed).
 */

import { API_BASE_URL, PHASER_TILES_PATH } from "./constants";

export interface TileImageCacheEntry {
  key: string;
  path: string;
}

/**
 * Get the URL for a tile image by its ID
 * Note: tile_image_id in the database is 1-indexed, but files are 0-indexed
 */
export const getTileImageUrl = (tileImageId: number): string => {
  // Convert 1-indexed tile ID to 0-indexed filename
  const fileIndex = tileImageId - 1;
  return `${API_BASE_URL}${PHASER_TILES_PATH}/tile_${fileIndex}.png`;
};
