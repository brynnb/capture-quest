import { TILE_SIZE } from "../constants";

/** Must match the server's X-Overworld-Pixels-Per-Tile contract header. */
export const OVERWORLD_OVERVIEW_PIXELS_PER_TILE = 4;
const OVERWORLD_OVERVIEW_MAX_UPSCALE = 2;

/** Enter overview when its source pixels need no more than 2x enlargement. */
export const OVERWORLD_OVERVIEW_ENTER_ZOOM =
  (OVERWORLD_OVERVIEW_PIXELS_PER_TILE * OVERWORLD_OVERVIEW_MAX_UPSCALE) /
  TILE_SIZE;

/** A 25% hysteresis band prevents pinch zoom from thrashing both LODs. */
export const OVERWORLD_OVERVIEW_EXIT_ZOOM =
  OVERWORLD_OVERVIEW_ENTER_ZOOM * 1.25;

export function preferOverworldOverviewAtZoom(
  currentlyPreferred: boolean,
  zoom: number,
): boolean {
  if (!Number.isFinite(zoom) || zoom <= 0) return currentlyPreferred;
  return currentlyPreferred
    ? zoom <= OVERWORLD_OVERVIEW_EXIT_ZOOM
    : zoom <= OVERWORLD_OVERVIEW_ENTER_ZOOM;
}
