import { TILE_SIZE } from "../constants";

/** Exact overworld data is fetched and rendered on a stable global grid. */
export const OVERWORLD_CHUNK_SIZE_TILES = 64;

/** Keep close gameplay useful even when the camera viewport is very small. */
export const MINIMUM_LIVE_TILE_SPAN = 50;

/** Three aligned chunks are 192 tiles, the largest exact span below 200. */
export const MAXIMUM_LIVE_CHUNKS_PER_AXIS = 3;
export const MAXIMUM_LIVE_TILE_SPAN =
  OVERWORLD_CHUNK_SIZE_TILES * MAXIMUM_LIVE_CHUNKS_PER_AXIS;

export interface InclusiveTileBounds {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

export interface CameraWorldView {
  /** Left edge in world pixels. */
  x: number;
  /** Top edge in world pixels. */
  y: number;
  /** Visible width in world pixels. */
  width: number;
  /** Visible height in world pixels. */
  height: number;
}

export interface OverworldChunk {
  /** Stable key derived from the chunk's global grid coordinates. */
  key: string;
  chunkX: number;
  chunkY: number;
  /** The complete 64x64 global-grid bounds, including space outside the map. */
  canonicalBounds: InclusiveTileBounds;
  /** Request bounds clipped to the actual overworld. */
  bounds: InclusiveTileBounds;
}

interface ChunkRange {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

interface OverworldChunkPlanBase {
  /** Camera footprint after it has been clamped to the overworld. */
  visibleTileBounds: InclusiveTileBounds;
}

export interface ExactOverworldChunkPlan extends OverworldChunkPlanBase {
  mode: "exact";
  /** Camera footprint expanded to the 50x50 minimum intent where possible. */
  requiredTileBounds: InclusiveTileBounds;
  /** Chunks that must be present before the exact view is considered ready. */
  requiredChunks: OverworldChunk[];
  /** One-chunk, per-axis prefetch margin when it remains within the 3x3 cap. */
  preloadChunks: OverworldChunk[];
  /** Required and preload chunks, in deterministic row-major order. */
  desiredChunks: OverworldChunk[];
  /** Inclusive union of the desired chunk request bounds. */
  desiredTileBounds: InclusiveTileBounds;
  /**
   * One additional ring of keys that an LRU may retain if already loaded.
   * These keys are not fetch requests and do not count against the live cap.
   */
  retentionChunkKeys: string[];
}

export interface OverviewOverworldChunkPlan extends OverworldChunkPlanBase {
  mode: "overview";
  reason: "visible-area-exceeds-exact-limit";
  requiredChunks: [];
  preloadChunks: [];
  desiredChunks: [];
  retentionChunkKeys: [];
}

export type OverworldChunkPlan =
  ExactOverworldChunkPlan | OverviewOverworldChunkPlan;

export interface PlanOverworldChunksInput {
  camera: CameraWorldView;
  /** Inclusive tile-coordinate bounds of the complete overworld. */
  mapBounds: InclusiveTileBounds;
}

function assertFinite(value: number, name: string): void {
  if (!Number.isFinite(value)) {
    throw new TypeError(`${name} must be finite`);
  }
}

function validateInput(input: PlanOverworldChunksInput): void {
  const { camera, mapBounds } = input;
  assertFinite(camera.x, "camera.x");
  assertFinite(camera.y, "camera.y");
  assertFinite(camera.width, "camera.width");
  assertFinite(camera.height, "camera.height");
  if (camera.width <= 0 || camera.height <= 0) {
    throw new RangeError("camera width and height must be greater than zero");
  }

  for (const [name, value] of Object.entries(mapBounds)) {
    assertFinite(value, `mapBounds.${name}`);
    if (!Number.isInteger(value)) {
      throw new TypeError(
        `mapBounds.${name} must be an integer tile coordinate`,
      );
    }
  }
  if (mapBounds.minX > mapBounds.maxX || mapBounds.minY > mapBounds.maxY) {
    throw new RangeError("mapBounds must describe a non-empty inclusive area");
  }
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, value));
}

/**
 * Converts a Phaser-style world view into the inclusive set of intersected
 * tiles. Right and bottom are treated as exclusive pixel edges, so a camera
 * ending exactly on a tile edge does not load the next tile.
 */
export function cameraWorldViewToTileBounds(
  camera: CameraWorldView,
  mapBounds: InclusiveTileBounds,
): InclusiveTileBounds {
  const rawMinX = Math.floor(camera.x / TILE_SIZE);
  const rawMinY = Math.floor(camera.y / TILE_SIZE);
  const rawMaxX = Math.ceil((camera.x + camera.width) / TILE_SIZE) - 1;
  const rawMaxY = Math.ceil((camera.y + camera.height) / TILE_SIZE) - 1;

  // Clamp both edges independently. A camera beyond a map edge therefore
  // resolves to the nearest edge tile and can still produce a useful plan.
  return {
    minX: clamp(rawMinX, mapBounds.minX, mapBounds.maxX),
    minY: clamp(rawMinY, mapBounds.minY, mapBounds.maxY),
    maxX: clamp(rawMaxX, mapBounds.minX, mapBounds.maxX),
    maxY: clamp(rawMaxY, mapBounds.minY, mapBounds.maxY),
  };
}

function inclusiveSpan(minimum: number, maximum: number): number {
  return maximum - minimum + 1;
}

function expandAxisToMinimum(
  minimum: number,
  maximum: number,
  mapMinimum: number,
  mapMaximum: number,
): [number, number] {
  const mapSpan = inclusiveSpan(mapMinimum, mapMaximum);
  const targetSpan = Math.min(MINIMUM_LIVE_TILE_SPAN, mapSpan);
  const currentSpan = inclusiveSpan(minimum, maximum);
  if (currentSpan >= targetSpan) return [minimum, maximum];

  const missing = targetSpan - currentSpan;
  let nextMinimum = minimum - Math.floor(missing / 2);
  let nextMaximum = maximum + Math.ceil(missing / 2);

  if (nextMinimum < mapMinimum) {
    nextMaximum += mapMinimum - nextMinimum;
    nextMinimum = mapMinimum;
  }
  if (nextMaximum > mapMaximum) {
    nextMinimum -= nextMaximum - mapMaximum;
    nextMaximum = mapMaximum;
  }

  return [Math.max(mapMinimum, nextMinimum), Math.min(mapMaximum, nextMaximum)];
}

function expandToMinimumLiveBounds(
  visible: InclusiveTileBounds,
  mapBounds: InclusiveTileBounds,
): InclusiveTileBounds {
  const [minX, maxX] = expandAxisToMinimum(
    visible.minX,
    visible.maxX,
    mapBounds.minX,
    mapBounds.maxX,
  );
  const [minY, maxY] = expandAxisToMinimum(
    visible.minY,
    visible.maxY,
    mapBounds.minY,
    mapBounds.maxY,
  );
  return { minX, minY, maxX, maxY };
}

function chunkCoordinate(tileCoordinate: number): number {
  // Math.floor is intentional: truncation would put -1 in chunk 0 instead of
  // the canonical -1 chunk spanning tiles -64 through -1.
  return Math.floor(tileCoordinate / OVERWORLD_CHUNK_SIZE_TILES);
}

function chunkRangeForBounds(bounds: InclusiveTileBounds): ChunkRange {
  return {
    minX: chunkCoordinate(bounds.minX),
    minY: chunkCoordinate(bounds.minY),
    maxX: chunkCoordinate(bounds.maxX),
    maxY: chunkCoordinate(bounds.maxY),
  };
}

function chunkRangeSpan(minimum: number, maximum: number): number {
  return maximum - minimum + 1;
}

export function overworldChunkKey(chunkX: number, chunkY: number): string {
  return `overworld:${chunkX}:${chunkY}`;
}

export function overworldChunkAt(
  chunkX: number,
  chunkY: number,
  mapBounds: InclusiveTileBounds,
): OverworldChunk {
  const canonicalBounds = {
    minX: chunkX * OVERWORLD_CHUNK_SIZE_TILES,
    minY: chunkY * OVERWORLD_CHUNK_SIZE_TILES,
    maxX: (chunkX + 1) * OVERWORLD_CHUNK_SIZE_TILES - 1,
    maxY: (chunkY + 1) * OVERWORLD_CHUNK_SIZE_TILES - 1,
  };
  return {
    key: overworldChunkKey(chunkX, chunkY),
    chunkX,
    chunkY,
    canonicalBounds,
    bounds: {
      minX: Math.max(canonicalBounds.minX, mapBounds.minX),
      minY: Math.max(canonicalBounds.minY, mapBounds.minY),
      maxX: Math.min(canonicalBounds.maxX, mapBounds.maxX),
      maxY: Math.min(canonicalBounds.maxY, mapBounds.maxY),
    },
  };
}

function chunksForRange(
  range: ChunkRange,
  mapBounds: InclusiveTileBounds,
): OverworldChunk[] {
  const chunks: OverworldChunk[] = [];
  for (let chunkY = range.minY; chunkY <= range.maxY; chunkY += 1) {
    for (let chunkX = range.minX; chunkX <= range.maxX; chunkX += 1) {
      chunks.push(overworldChunkAt(chunkX, chunkY, mapBounds));
    }
  }
  return chunks;
}

export function overworldChunksCoveringBounds(
  bounds: InclusiveTileBounds,
  mapBounds: InclusiveTileBounds,
  marginChunks = 0,
): OverworldChunk[] {
  if (!Number.isInteger(marginChunks) || marginChunks < 0) {
    throw new RangeError("marginChunks must be a non-negative integer");
  }
  const mapRange = chunkRangeForBounds(mapBounds);
  const range = chunkRangeForBounds(bounds);
  return chunksForRange(
    {
      minX: Math.max(mapRange.minX, range.minX - marginChunks),
      minY: Math.max(mapRange.minY, range.minY - marginChunks),
      maxX: Math.min(mapRange.maxX, range.maxX + marginChunks),
      maxY: Math.min(mapRange.maxY, range.maxY + marginChunks),
    },
    mapBounds,
  );
}

function expandChunkAxisForPrefetch(
  minimum: number,
  maximum: number,
  mapMinimum: number,
  mapMaximum: number,
): [number, number] {
  const candidateMinimum = Math.max(mapMinimum, minimum - 1);
  const candidateMaximum = Math.min(mapMaximum, maximum + 1);
  if (
    chunkRangeSpan(candidateMinimum, candidateMaximum) <=
    MAXIMUM_LIVE_CHUNKS_PER_AXIS
  ) {
    return [candidateMinimum, candidateMaximum];
  }
  return [minimum, maximum];
}

function prefetchRange(required: ChunkRange, map: ChunkRange): ChunkRange {
  const [minX, maxX] = expandChunkAxisForPrefetch(
    required.minX,
    required.maxX,
    map.minX,
    map.maxX,
  );
  const [minY, maxY] = expandChunkAxisForPrefetch(
    required.minY,
    required.maxY,
    map.minY,
    map.maxY,
  );
  return { minX, minY, maxX, maxY };
}

function expandRangeByOne(range: ChunkRange, map: ChunkRange): ChunkRange {
  return {
    minX: Math.max(map.minX, range.minX - 1),
    minY: Math.max(map.minY, range.minY - 1),
    maxX: Math.min(map.maxX, range.maxX + 1),
    maxY: Math.min(map.maxY, range.maxY + 1),
  };
}

function unionBounds(chunks: readonly OverworldChunk[]): InclusiveTileBounds {
  const first = chunks[0];
  if (!first) {
    throw new RangeError("cannot calculate bounds for an empty chunk list");
  }
  return chunks.slice(1).reduce<InclusiveTileBounds>(
    (bounds, chunk) => ({
      minX: Math.min(bounds.minX, chunk.bounds.minX),
      minY: Math.min(bounds.minY, chunk.bounds.minY),
      maxX: Math.max(bounds.maxX, chunk.bounds.maxX),
      maxY: Math.max(bounds.maxY, chunk.bounds.maxY),
    }),
    { ...first.bounds },
  );
}

export function planOverworldChunks(
  input: PlanOverworldChunksInput,
): OverworldChunkPlan {
  validateInput(input);
  const { camera, mapBounds } = input;
  const visibleTileBounds = cameraWorldViewToTileBounds(camera, mapBounds);
  const requiredTileBounds = expandToMinimumLiveBounds(
    visibleTileBounds,
    mapBounds,
  );
  const requiredRange = chunkRangeForBounds(requiredTileBounds);

  if (
    chunkRangeSpan(requiredRange.minX, requiredRange.maxX) >
      MAXIMUM_LIVE_CHUNKS_PER_AXIS ||
    chunkRangeSpan(requiredRange.minY, requiredRange.maxY) >
      MAXIMUM_LIVE_CHUNKS_PER_AXIS
  ) {
    return {
      mode: "overview",
      reason: "visible-area-exceeds-exact-limit",
      visibleTileBounds,
      requiredChunks: [],
      preloadChunks: [],
      desiredChunks: [],
      retentionChunkKeys: [],
    };
  }

  const mapChunkRange = chunkRangeForBounds(mapBounds);
  const desiredRange = prefetchRange(requiredRange, mapChunkRange);
  const requiredChunks = chunksForRange(requiredRange, mapBounds);
  const requiredKeys = new Set(requiredChunks.map((chunk) => chunk.key));
  const desiredChunks = chunksForRange(desiredRange, mapBounds);
  const preloadChunks = desiredChunks.filter(
    (chunk) => !requiredKeys.has(chunk.key),
  );
  const retentionChunkKeys = chunksForRange(
    expandRangeByOne(desiredRange, mapChunkRange),
    mapBounds,
  ).map((chunk) => chunk.key);

  return {
    mode: "exact",
    visibleTileBounds,
    requiredTileBounds,
    requiredChunks,
    preloadChunks,
    desiredChunks,
    desiredTileBounds: unionBounds(desiredChunks),
    retentionChunkKeys,
  };
}
