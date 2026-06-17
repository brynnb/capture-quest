/**
 * Sprite Service - Static sprite image URL generation
 *
 * Sprite images are served as static files from the Go backend.
 */

import { API_BASE_URL, PHASER_SPRITES_PATH } from "./constants";

/**
 * Get the URL for a sprite image by name
 * @param spriteName The name of the sprite file (e.g., "poke_ball.png")
 * @returns The full URL to the sprite image
 */
export const getSpriteUrl = (spriteName: string): string => {
  return `${API_BASE_URL}${PHASER_SPRITES_PATH}/${spriteName}`;
};
