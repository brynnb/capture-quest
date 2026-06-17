import type { PhaserTile } from "@/net/generated/world_api";
import { UNIFIED_OVERWORLD_MAP_ID } from "../constants";

export const PROCEDURAL_OVERWORLD_GENERATE_EVENT =
  "capturequest:procedural-overworld-generate";
export const PROCEDURAL_OVERWORLD_GENERATED_EVENT =
  "capturequest:procedural-overworld-generated";

export interface ProceduralOverworldGenerateDetail {
  seed?: number;
}

export interface ProceduralOverworldGeneratedDetail {
  seed: number;
  sourceTileCount: number;
  generatedTileCount: number;
  renderedTileCount: number;
  skippedTileCount: number;
  rockTileCount: number;
  grassPatchTileCount: number;
  waterTileCount: number;
  bounds?: TileBounds;
  error?: string;
}

export type ProceduralOverworldTile = PhaserTile & {
  generated: true;
  generatedSource: "procedural-overworld-preview";
};

export interface TileBounds {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

interface GenerateOptions {
  mapId?: number;
  seed?: number;
}

interface GenerationSummary {
  seed: number;
  sourceTileCount: number;
  generatedTileCount: number;
  rockTileCount: number;
  grassPatchTileCount: number;
  waterTileCount: number;
  bounds: TileBounds;
}

interface GenerateResult {
  tiles: ProceduralOverworldTile[];
  summary: GenerationSummary;
}

type BlockTileCatalog = Record<number, Partial<Record<number, number>>>;

type TerrainKind = "base" | "grassPatch" | "rock" | "water";

const DEFAULT_SEED = 0x51c0ffee;

// Extracted from the OVERWORLD tileset in public/phaser/pokemon.db.
// Values are tile_image.id, grouped by original block_index and block position.
const OVERWORLD_BLOCK_TILES: BlockTileCatalog = {
  1: { 0: 5, 1: 6 },
  4: { 0: 13 },
  6: { 0: 16, 3: 18 },
  11: { 0: 25 },
  29: { 0: 42, 1: 43, 2: 44 },
  30: { 1: 45, 3: 46 },
  36: { 0: 56, 2: 57 },
  37: { 1: 58, 3: 59 },
  59: { 0: 76, 1: 77 },
  62: { 0: 84 },
};

const ROCK_TILE_IMAGE_IDS = new Set<number>(
  [6, 36, 37, 59, 62]
    .flatMap((blockIndex) => Object.values(OVERWORLD_BLOCK_TILES[blockIndex] ?? {}))
    .filter((tileImageId): tileImageId is number => typeof tileImageId === "number"),
);

const FLAT_ROCK_TILE_IMAGE_IDS = new Set<number>([16]);
const WATER_TILE_IMAGE_IDS = new Set<number>([13, 42, 43, 44, 45, 46, 65, 66, 113]);

const NEIGHBOR_OFFSETS = [
  [-1, -1],
  [0, -1],
  [1, -1],
  [-1, 0],
  [1, 0],
  [-1, 1],
  [0, 1],
  [1, 1],
] as const;

const ORTHOGONAL_OFFSETS = [
  [0, -1],
  [-1, 0],
  [1, 0],
  [0, 1],
] as const;

export const tileKey = (x: number, y: number) => `${x},${y}`;

export function generateProceduralOverworldTiles(
  sourceTiles: PhaserTile[],
  options: GenerateOptions = {},
): GenerateResult {
  const seed = options.seed ?? DEFAULT_SEED;
  const mapId = options.mapId ?? UNIFIED_OVERWORLD_MAP_ID;
  const bounds = calculateTileBounds(sourceTiles);
  const sourceKeys = new Set<string>();
  const sourceRockKeys = new Set<string>();
  const sourceRockTileImageIds = new Map<string, number>();
  const sourceFlatRockKeys = new Set<string>();
  const sourceWaterKeys = new Set<string>();

  for (const tile of sourceTiles) {
    const key = tileKey(tile.x, tile.y);
    sourceKeys.add(key);
    if (ROCK_TILE_IMAGE_IDS.has(tile.tileImageId)) {
      sourceRockKeys.add(key);
      sourceRockTileImageIds.set(key, tile.tileImageId);
    }
    if (FLAT_ROCK_TILE_IMAGE_IDS.has(tile.tileImageId)) {
      sourceFlatRockKeys.add(key);
    }
    if (tile.collisionType === 2 || WATER_TILE_IMAGE_IDS.has(tile.tileImageId)) {
      sourceWaterKeys.add(key);
    }
  }

  const generatedKinds = new Map<string, TerrainKind>();

  for (let y = bounds.minY; y <= bounds.maxY; y++) {
    for (let x = bounds.minX; x <= bounds.maxX; x++) {
      const key = tileKey(x, y);
      if (sourceKeys.has(key)) continue;
      generatedKinds.set(key, "base");
    }
  }

  addWaterBodies(generatedKinds, bounds, sourceWaterKeys, seed);
  addGrassPatches(generatedKinds, bounds, seed);
  smoothGeneratedWater(generatedKinds, sourceWaterKeys, seed);
  enforceMinimumEdgeRuns(generatedKinds, sourceWaterKeys, bounds, "water");
  removeThinWaterAreas(generatedKinds, sourceWaterKeys);
  smoothGeneratedWater(generatedKinds, sourceWaterKeys, seed);
  removeThinWaterAreas(generatedKinds, sourceWaterKeys);
  enforceMinimumEdgeRuns(generatedKinds, sourceWaterKeys, bounds, "water");
  addRareRockFormations(generatedKinds, bounds, seed);
  smoothGeneratedRock(generatedKinds, sourceRockKeys);
  continueSourceRockFormations(generatedKinds, sourceFlatRockKeys, bounds, seed);
  fillTerrainHoles(generatedKinds, sourceRockKeys, "rock", 6, 1);
  enforceMinimumEdgeRuns(generatedKinds, sourceRockKeys, bounds, "rock");
  sealSourceFlatRockEdges(generatedKinds, sourceFlatRockKeys, bounds);
  removeUnsupportedRockCorners(generatedKinds, sourceRockKeys, sourceRockTileImageIds);
  sealSourceFlatRockEdges(generatedKinds, sourceFlatRockKeys, bounds);
  removeSkinnyGeneratedWaterComponents(generatedKinds);

  const generatedTiles: ProceduralOverworldTile[] = [];
  let rockTileCount = 0;
  let grassPatchTileCount = 0;
  let waterTileCount = 0;

  let generatedId = -1;
  for (let y = bounds.minY; y <= bounds.maxY; y++) {
    for (let x = bounds.minX; x <= bounds.maxX; x++) {
      const key = tileKey(x, y);
      const kind = generatedKinds.get(key);
      if (!kind) continue;

      if (kind === "rock") rockTileCount++;
      if (kind === "grassPatch") grassPatchTileCount++;
      if (kind === "water") waterTileCount++;

      generatedTiles.push({
        id: generatedId--,
        x,
        y,
        localX: x,
        localY: y,
        mapId,
        tileImageId:
          kind === "rock"
            ? chooseRockTileImageId(x, y, generatedKinds, sourceRockKeys)
            : kind === "water"
              ? chooseWaterTileImageId(x, y, generatedKinds, sourceWaterKeys)
            : chooseGroundTileImageId(x, y, kind, seed),
        collisionType: kind === "rock" ? 0 : kind === "water" ? 2 : 1,
        talkOverTile: false,
        generated: true,
        generatedSource: "procedural-overworld-preview",
      });
    }
  }

  return {
    tiles: generatedTiles,
    summary: {
      seed,
      sourceTileCount: sourceTiles.length,
      generatedTileCount: generatedTiles.length,
      rockTileCount,
      grassPatchTileCount,
      waterTileCount,
      bounds,
    },
  };
}

function calculateTileBounds(tiles: PhaserTile[]): TileBounds {
  if (tiles.length === 0) {
    return { minX: 0, minY: 0, maxX: 0, maxY: 0 };
  }

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  for (const tile of tiles) {
    minX = Math.min(minX, tile.x);
    minY = Math.min(minY, tile.y);
    maxX = Math.max(maxX, tile.x);
    maxY = Math.max(maxY, tile.y);
  }

  return { minX, minY, maxX, maxY };
}

function addWaterBodies(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  sourceWaterKeys: Set<string>,
  seed: number,
): void {
  for (const key of sourceWaterKeys) {
    const [x, y] = parseTileKey(key);
    if (hashNoise(x, y, seed ^ 0x20a7) >= 0.0028) {
      continue;
    }

    const angle = hashNoise(x, y, seed ^ 0x20a8) * Math.PI * 2;
    const distance = 5 + Math.floor(hashNoise(x, y, seed ^ 0x20a9) * 10);
    const centerX = Math.round(x + Math.cos(angle) * distance);
    const centerY = Math.round(y + Math.sin(angle) * distance);
    const radiusX = 5 + Math.floor(hashNoise(x, y, seed ^ 0x20aa) * 8);
    const radiusY = 6 + Math.floor(hashNoise(x, y, seed ^ 0x20ab) * 10);
    addOrganicBlob(generatedKinds, bounds, centerX, centerY, radiusX, radiusY, seed ^ 0x20ac, "water");
  }

  const chunkWidth = 30;
  const chunkHeight = 24;
  for (let chunkY = bounds.minY; chunkY <= bounds.maxY; chunkY += chunkHeight) {
    for (let chunkX = bounds.minX; chunkX <= bounds.maxX; chunkX += chunkWidth) {
      if (hashNoise(chunkX, chunkY, seed ^ 0x20ba) >= 0.018) {
        continue;
      }

      const centerX = chunkX + 6 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x20bb) * 18);
      const centerY = chunkY + 5 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x20bc) * 14);
      const radiusX = 6 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x20bd) * 10);
      const radiusY = 7 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x20be) * 10);
      addOrganicBlob(generatedKinds, bounds, centerX, centerY, radiusX, radiusY, seed ^ 0x20bf, "water");
    }
  }

  fillTerrainHoles(generatedKinds, sourceWaterKeys, "water", 5, 2);
}

function addGrassPatches(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  seed: number,
): void {
  const chunkWidth = 24;
  const chunkHeight = 18;

  for (let chunkY = bounds.minY; chunkY <= bounds.maxY; chunkY += chunkHeight) {
    for (let chunkX = bounds.minX; chunkX <= bounds.maxX; chunkX += chunkWidth) {
      if (hashNoise(chunkX, chunkY, seed ^ 0x91ef) >= 0.26) {
        continue;
      }

      const centerX = chunkX + 4 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x91f0) * 16);
      const centerY = chunkY + 4 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x91f1) * 11);
      const radiusX = 6 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x91f2) * 12);
      const radiusY = 4 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x91f3) * 8);
      addOrganicBlob(
        generatedKinds,
        bounds,
        centerX,
        centerY,
        radiusX,
        radiusY,
        seed ^ 0x91f4,
        "grassPatch",
      );
    }
  }

  smoothGrassPatches(generatedKinds);
}

function addOrganicBlob(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  centerX: number,
  centerY: number,
  radiusX: number,
  radiusY: number,
  seed: number,
  kind: Exclude<TerrainKind, "base">,
): void {
  const minX = Math.max(bounds.minX, centerX - radiusX - 2);
  const maxX = Math.min(bounds.maxX, centerX + radiusX + 2);
  const minY = Math.max(bounds.minY, centerY - radiusY - 2);
  const maxY = Math.min(bounds.maxY, centerY + radiusY + 2);

  for (let y = minY; y <= maxY; y++) {
    for (let x = minX; x <= maxX; x++) {
      const key = tileKey(x, y);
      const current = generatedKinds.get(key);
      if (!current || current === "rock" || (kind !== "water" && current === "water")) {
        continue;
      }
      if (kind === "water" && !hasOpenWaterRoom(generatedKinds, x, y)) {
        continue;
      }

      const dx = (x - centerX) / Math.max(1, radiusX);
      const dy = (y - centerY) / Math.max(1, radiusY);
      const distance = dx * dx + dy * dy;
      const edgeNoise =
        (hashNoise(Math.floor(x / 2), Math.floor(y / 2), seed) - 0.5) * 0.32;
      const threshold = 1 + edgeNoise;

      if (distance <= threshold) {
        generatedKinds.set(key, kind);
      }
    }
  }
}

function smoothGeneratedWater(
  generatedKinds: Map<string, TerrainKind>,
  sourceWaterKeys: Set<string>,
  seed: number,
): void {
  for (let pass = 0; pass < 2; pass++) {
    const updates: [string, TerrainKind][] = [];

    for (const [key, kind] of generatedKinds.entries()) {
      const [x, y] = parseTileKey(key);
      const waterNeighbors = countWaterNeighbors(x, y, sourceWaterKeys, generatedKinds);

      if (kind === "water" && waterNeighbors <= 2) {
        updates.push([key, "base"]);
        continue;
      }

      if (
        kind !== "water" &&
        kind !== "rock" &&
        waterNeighbors >= 6 &&
        hashNoise(x, y, seed ^ (0x6a00 + pass)) < 0.72
      ) {
        updates.push([key, "water"]);
      }
    }

    for (const [key, kind] of updates) {
      generatedKinds.set(key, kind);
    }
  }
}

function removeThinWaterAreas(
  generatedKinds: Map<string, TerrainKind>,
  sourceWaterKeys: Set<string>,
): void {
  for (let pass = 0; pass < 3; pass++) {
    const updates: [string, TerrainKind][] = [];

    for (const [key, kind] of generatedKinds.entries()) {
      if (kind !== "water") continue;

      const [x, y] = parseTileKey(key);
      const vertical =
        isWaterAt(x, y - 1, sourceWaterKeys, generatedKinds) ||
        isWaterAt(x, y + 1, sourceWaterKeys, generatedKinds);
      const horizontal =
        isWaterAt(x - 1, y, sourceWaterKeys, generatedKinds) ||
        isWaterAt(x + 1, y, sourceWaterKeys, generatedKinds);

      if (!vertical || !horizontal || countWaterNeighbors(x, y, sourceWaterKeys, generatedKinds) <= 2) {
        updates.push([key, "base"]);
      }
    }

    if (updates.length === 0) {
      return;
    }

    for (const [key, nextKind] of updates) {
      generatedKinds.set(key, nextKind);
    }
  }
}

function fillTerrainHoles(
  generatedKinds: Map<string, TerrainKind>,
  sourceKindKeys: Set<string>,
  kind: "rock" | "water",
  neighborThreshold: number,
  passes: number,
): void {
  for (let pass = 0; pass < passes; pass++) {
    const updates: [string, TerrainKind][] = [];

    for (const [key, currentKind] of generatedKinds.entries()) {
      if (currentKind === kind || (currentKind === "rock" && kind === "water")) {
        continue;
      }

      const [x, y] = parseTileKey(key);
      if (kind === "water" && !hasOpenWaterRoom(generatedKinds, x, y)) {
        continue;
      }
      if (countKindNeighbors(x, y, sourceKindKeys, generatedKinds, kind) >= neighborThreshold) {
        updates.push([key, kind]);
      }
    }

    if (updates.length === 0) {
      return;
    }

    for (const [key, nextKind] of updates) {
      generatedKinds.set(key, nextKind);
    }
  }
}

function smoothGrassPatches(generatedKinds: Map<string, TerrainKind>): void {
  const emptySourceKeys = new Set<string>();

  for (let pass = 0; pass < 2; pass++) {
    const updates: [string, TerrainKind][] = [];

    for (const [key, kind] of generatedKinds.entries()) {
      const [x, y] = parseTileKey(key);
      const grassNeighbors = countKindNeighbors(x, y, emptySourceKeys, generatedKinds, "grassPatch");

      if (kind === "grassPatch" && grassNeighbors <= 1) {
        updates.push([key, "base"]);
        continue;
      }

      if (kind === "base" && grassNeighbors >= 5) {
        updates.push([key, "grassPatch"]);
      }
    }

    if (updates.length === 0) {
      return;
    }

    for (const [key, nextKind] of updates) {
      generatedKinds.set(key, nextKind);
    }
  }
}

function addRareRockFormations(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  seed: number,
): void {
  const chunkWidth = 18;
  const chunkHeight = 14;

  for (let chunkY = bounds.minY; chunkY <= bounds.maxY; chunkY += chunkHeight) {
    for (let chunkX = bounds.minX; chunkX <= bounds.maxX; chunkX += chunkWidth) {
      if (hashNoise(chunkX, chunkY, seed ^ 0x7137) >= 0.038) {
        continue;
      }

      const width = 5 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x7138) * 8);
      const height = 4 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x7139) * 6);
      const maxOffsetX = Math.max(1, chunkWidth - width - 1);
      const maxOffsetY = Math.max(1, chunkHeight - height - 1);
      const originX =
        chunkX + 1 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x7140) * maxOffsetX);
      const originY =
        chunkY + 1 + Math.floor(hashNoise(chunkX, chunkY, seed ^ 0x7141) * maxOffsetY);

      addRockFormation(generatedKinds, originX, originY, width, height);
    }
  }
}

function continueSourceRockFormations(
  generatedKinds: Map<string, TerrainKind>,
  sourceFlatRockKeys: Set<string>,
  bounds: TileBounds,
  seed: number,
): void {
  const rockKeysToAdd = new Set<string>();
  const boundaryRuns = new Map<string, Map<number, number[]>>();

  for (const sourceKey of sourceFlatRockKeys) {
    const [sourceX, sourceY] = parseTileKey(sourceKey);

    for (const [dx, dy] of ORTHOGONAL_OFFSETS) {
      const firstX = sourceX + dx;
      const firstY = sourceY + dy;
      const firstKind = generatedKinds.get(tileKey(firstX, firstY));
      if (!firstKind || firstKind === "water") {
        continue;
      }

      const directionKey = `${dx},${dy}`;
      const lineCoord = dx === 0 ? sourceY : sourceX;
      const runCoord = dx === 0 ? sourceX : sourceY;
      const runsForDirection = boundaryRuns.get(directionKey) ?? new Map<number, number[]>();
      const coords = runsForDirection.get(lineCoord) ?? [];
      coords.push(runCoord);
      runsForDirection.set(lineCoord, coords);
      boundaryRuns.set(directionKey, runsForDirection);
    }
  }

  for (const [directionKey, runsByLine] of boundaryRuns.entries()) {
    const [dx, dy] = parseTileKey(directionKey);

    for (const [lineCoord, runCoords] of runsByLine.entries()) {
      const sortedCoords = [...new Set(runCoords)].sort((a, b) => a - b);
      let runStart = sortedCoords[0];
      let previous = sortedCoords[0];

      for (let index = 1; index <= sortedCoords.length; index++) {
        const current = sortedCoords[index];
        if (current === previous + 1) {
          previous = current;
          continue;
        }

        addSourceRockBoundaryRun(
          generatedKinds,
          bounds,
          rockKeysToAdd,
          lineCoord,
          runStart,
          previous,
          dx,
          dy,
          seed,
        );
        runStart = current;
        previous = current;
      }
    }
  }

  for (const key of rockKeysToAdd) {
    generatedKinds.set(key, "rock");
  }
}

function addSourceRockBoundaryRun(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  rockKeysToAdd: Set<string>,
  lineCoord: number,
  runStart: number,
  runEnd: number,
  dx: number,
  dy: number,
  seed: number,
): void {
  if (!Number.isFinite(runStart) || !Number.isFinite(runEnd)) {
    return;
  }

  const runLength = runEnd - runStart + 1;
  const minimumRunLength = 4;
  const runPadding = runLength < minimumRunLength ? minimumRunLength - runLength : 1;
  const start = runStart - Math.ceil(runPadding / 2);
  const end = runEnd + Math.floor(runPadding / 2);
  const depth = 4 + Math.floor(hashNoise(lineCoord, runStart, seed ^ 0xa350) * 4);

  for (let step = 1; step <= depth; step++) {
    const shelfLine = lineCoord + (dx !== 0 ? dx * step : dy * step);

    for (let runCoord = start; runCoord <= end; runCoord++) {
      const x = dx === 0 ? runCoord : shelfLine;
      const y = dx === 0 ? shelfLine : runCoord;
      if (!canPlaceSourceRockContinuation(generatedKinds, bounds, x, y)) {
        continue;
      }
      rockKeysToAdd.add(tileKey(x, y));
    }
  }
}

function sealSourceFlatRockEdges(
  generatedKinds: Map<string, TerrainKind>,
  sourceFlatRockKeys: Set<string>,
  bounds: TileBounds,
): void {
  for (const sourceKey of sourceFlatRockKeys) {
    const [sourceX, sourceY] = parseTileKey(sourceKey);

    for (const [dx, dy] of ORTHOGONAL_OFFSETS) {
      const x = sourceX + dx;
      const y = sourceY + dy;
      if (x < bounds.minX || x > bounds.maxX || y < bounds.minY || y > bounds.maxY) {
        continue;
      }
      if (!generatedKinds.has(tileKey(x, y))) {
        continue;
      }
      generatedKinds.set(tileKey(x, y), "rock");
    }
  }
}

function removeUnsupportedRockCorners(
  generatedKinds: Map<string, TerrainKind>,
  sourceRockKeys: Set<string>,
  sourceRockTileImageIds: Map<string, number>,
): void {
  for (let pass = 0; pass < 256; pass++) {
    const rockKeysToRemove = new Set<string>();

    for (const [key, kind] of generatedKinds.entries()) {
      if (kind !== "rock") {
        continue;
      }

      const [x, y] = parseTileKey(key);
      const tileImageId = chooseRockTileImageId(x, y, generatedKinds, sourceRockKeys);
      const support = cornerSupportForTile(tileImageId);
      if (!support) {
        continue;
      }

      for (const [dx, dy, expectedTileImageId] of support) {
        const supportX = x + dx;
        const supportY = y + dy;
        const supportTileImageId = rockTileImageIdAt(
          supportX,
          supportY,
          generatedKinds,
          sourceRockKeys,
          sourceRockTileImageIds,
        );
        if (supportTileImageId === expectedTileImageId) {
          continue;
        }
        rockKeysToRemove.add(key);
        break;
      }
    }

    if (rockKeysToRemove.size === 0) {
      return;
    }

    for (const key of rockKeysToRemove) {
      generatedKinds.set(key, "base");
    }
  }
}

function cornerSupportForTile(tileImageId: number): [number, number, number][] | undefined {
  if (tileImageId === 84) {
    return [
      [1, 0, 76],
      [0, 1, 56],
    ];
  }
  if (tileImageId === 77) {
    return [
      [-1, 0, 76],
      [0, 1, 58],
    ];
  }
  if (tileImageId === 57) {
    return [
      [1, 0, 18],
      [0, -1, 56],
    ];
  }
  if (tileImageId === 59) {
    return [
      [-1, 0, 18],
      [0, -1, 58],
    ];
  }
  return undefined;
}

function removeSkinnyGeneratedWaterComponents(
  generatedKinds: Map<string, TerrainKind>,
): void {
  const waterKeys = new Set<string>();
  for (const [key, kind] of generatedKinds.entries()) {
    if (kind === "water") {
      waterKeys.add(key);
    }
  }

  const visited = new Set<string>();

  for (const key of waterKeys) {
    if (visited.has(key)) {
      continue;
    }

    const component: string[] = [];
    const queue = [key];
    visited.add(key);

    while (queue.length > 0) {
      const currentKey = queue.shift();
      if (!currentKey) {
        continue;
      }
      component.push(currentKey);
      const [x, y] = parseTileKey(currentKey);

      for (const [dx, dy] of ORTHOGONAL_OFFSETS) {
        const nextKey = tileKey(x + dx, y + dy);
        if (!waterKeys.has(nextKey) || visited.has(nextKey)) {
          continue;
        }
        visited.add(nextKey);
        queue.push(nextKey);
      }
    }

    if (component.length < 5) {
      continue;
    }

    const xs = component.map((componentKey) => parseTileKey(componentKey)[0]);
    const ys = component.map((componentKey) => parseTileKey(componentKey)[1]);
    const width = Math.max(...xs) - Math.min(...xs) + 1;
    const height = Math.max(...ys) - Math.min(...ys) + 1;
    if (width >= 3 && height >= 3) {
      continue;
    }

    for (const componentKey of component) {
      generatedKinds.set(componentKey, "base");
    }
  }
}

function rockTileImageIdAt(
  x: number,
  y: number,
  generatedKinds: Map<string, TerrainKind>,
  sourceRockKeys: Set<string>,
  sourceRockTileImageIds: Map<string, number>,
): number | undefined {
  const key = tileKey(x, y);
  if (sourceRockTileImageIds.has(key)) {
    return sourceRockTileImageIds.get(key);
  }
  if (generatedKinds.get(key) !== "rock") {
    return undefined;
  }
  return chooseRockTileImageId(x, y, generatedKinds, sourceRockKeys);
}

function addRockFormation(
  generatedKinds: Map<string, TerrainKind>,
  originX: number,
  originY: number,
  width: number,
  height: number,
): void {
  for (let dy = 0; dy < height; dy++) {
    for (let dx = 0; dx < width; dx++) {
      const x = originX + dx;
      const y = originY + dy;
      const key = tileKey(x, y);
      const existingKind = generatedKinds.get(key);

      if (!existingKind || existingKind === "water") {
        continue;
      }

      generatedKinds.set(key, "rock");
    }
  }
}

function smoothGeneratedRock(
  generatedKinds: Map<string, TerrainKind>,
  sourceRockKeys: Set<string>,
): void {
  for (let pass = 0; pass < 2; pass++) {
    const updates: [string, TerrainKind][] = [];

    for (const [key, kind] of generatedKinds.entries()) {
      const [x, y] = parseTileKey(key);
      const rockNeighbors = countRockNeighbors(x, y, sourceRockKeys, generatedKinds);

      if (kind === "rock" && rockNeighbors <= 1) {
        updates.push([key, "base"]);
        continue;
      }

      if (kind !== "rock" && kind !== "water" && rockNeighbors >= 7) {
        updates.push([key, "rock"]);
      }
    }

    for (const [key, kind] of updates) {
      generatedKinds.set(key, kind);
    }
  }
}

function enforceMinimumEdgeRuns(
  generatedKinds: Map<string, TerrainKind>,
  sourceKindKeys: Set<string>,
  bounds: TileBounds,
  kind: "rock" | "water",
): void {
  for (let pass = 0; pass < 2; pass++) {
    enforceHorizontalEdgeRuns(generatedKinds, sourceKindKeys, bounds, kind, -1);
    enforceHorizontalEdgeRuns(generatedKinds, sourceKindKeys, bounds, kind, 1);
    enforceVerticalEdgeRuns(generatedKinds, sourceKindKeys, bounds, kind, -1);
    enforceVerticalEdgeRuns(generatedKinds, sourceKindKeys, bounds, kind, 1);
  }
}

function enforceHorizontalEdgeRuns(
  generatedKinds: Map<string, TerrainKind>,
  sourceKindKeys: Set<string>,
  bounds: TileBounds,
  kind: "rock" | "water",
  neighborDy: -1 | 1,
): void {
  for (let y = bounds.minY; y <= bounds.maxY; y++) {
    let x = bounds.minX;
    while (x <= bounds.maxX) {
      if (!isGeneratedEdgeCell(generatedKinds, sourceKindKeys, x, y, kind, 0, neighborDy)) {
        x++;
        continue;
      }

      const runStart = x;
      while (
        x <= bounds.maxX &&
        isGeneratedEdgeCell(generatedKinds, sourceKindKeys, x, y, kind, 0, neighborDy)
      ) {
        x++;
      }

      ensureRunLength(generatedKinds, bounds, kind, runStart, y, x - 1, y);
    }
  }
}

function enforceVerticalEdgeRuns(
  generatedKinds: Map<string, TerrainKind>,
  sourceKindKeys: Set<string>,
  bounds: TileBounds,
  kind: "rock" | "water",
  neighborDx: -1 | 1,
): void {
  for (let x = bounds.minX; x <= bounds.maxX; x++) {
    let y = bounds.minY;
    while (y <= bounds.maxY) {
      if (!isGeneratedEdgeCell(generatedKinds, sourceKindKeys, x, y, kind, neighborDx, 0)) {
        y++;
        continue;
      }

      const runStart = y;
      while (
        y <= bounds.maxY &&
        isGeneratedEdgeCell(generatedKinds, sourceKindKeys, x, y, kind, neighborDx, 0)
      ) {
        y++;
      }

      ensureRunLength(generatedKinds, bounds, kind, x, runStart, x, y - 1);
    }
  }
}

function isGeneratedEdgeCell(
  generatedKinds: Map<string, TerrainKind>,
  sourceKindKeys: Set<string>,
  x: number,
  y: number,
  kind: "rock" | "water",
  neighborDx: number,
  neighborDy: number,
): boolean {
  if (generatedKinds.get(tileKey(x, y)) !== kind) {
    return false;
  }

  return !isKindAt(x + neighborDx, y + neighborDy, sourceKindKeys, generatedKinds, kind);
}

function ensureRunLength(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  kind: "rock" | "water",
  startX: number,
  startY: number,
  endX: number,
  endY: number,
): void {
  const horizontal = startY === endY;
  const currentLength = horizontal ? endX - startX + 1 : endY - startY + 1;
  if (currentLength >= 3) {
    return;
  }

  let needed = 3 - currentLength;
  let beforeX = horizontal ? startX - 1 : startX;
  let beforeY = horizontal ? startY : startY - 1;
  let afterX = horizontal ? endX + 1 : endX;
  let afterY = horizontal ? endY : endY + 1;

  while (needed > 0) {
    if (canGrowTerrainInto(generatedKinds, bounds, kind, afterX, afterY)) {
      generatedKinds.set(tileKey(afterX, afterY), kind);
      if (horizontal) afterX++;
      else afterY++;
      needed--;
      continue;
    }

    if (canGrowTerrainInto(generatedKinds, bounds, kind, beforeX, beforeY)) {
      generatedKinds.set(tileKey(beforeX, beforeY), kind);
      if (horizontal) beforeX--;
      else beforeY--;
      needed--;
      continue;
    }

    break;
  }

  if (needed === 0) {
    return;
  }

  forEachRunTile(startX, startY, endX, endY, (x, y) => {
    generatedKinds.set(tileKey(x, y), "base");
  });
}

function canGrowTerrainInto(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  kind: "rock" | "water",
  x: number,
  y: number,
): boolean {
  if (x < bounds.minX || x > bounds.maxX || y < bounds.minY || y > bounds.maxY) {
    return false;
  }

  const current = generatedKinds.get(tileKey(x, y));
  if (!current || current === kind) {
    return false;
  }

  if (kind === "rock" && current === "water") {
    return false;
  }
  if (kind === "water" && current === "rock") {
    return false;
  }

  return true;
}

function canPlaceSourceRockContinuation(
  generatedKinds: Map<string, TerrainKind>,
  bounds: TileBounds,
  x: number,
  y: number,
): boolean {
  if (x < bounds.minX || x > bounds.maxX || y < bounds.minY || y > bounds.maxY) {
    return false;
  }

  const current = generatedKinds.get(tileKey(x, y));
  return current !== undefined;
}

function hasOpenWaterRoom(generatedKinds: Map<string, TerrainKind>, x: number, y: number): boolean {
  const verticalRoom =
    generatedKinds.has(tileKey(x, y - 2)) &&
    generatedKinds.has(tileKey(x, y - 1)) &&
    generatedKinds.has(tileKey(x, y + 1)) &&
    generatedKinds.has(tileKey(x, y + 2));
  const horizontalRoom =
    generatedKinds.has(tileKey(x - 2, y)) &&
    generatedKinds.has(tileKey(x - 1, y)) &&
    generatedKinds.has(tileKey(x + 1, y)) &&
    generatedKinds.has(tileKey(x + 2, y));

  return verticalRoom && horizontalRoom;
}

function forEachRunTile(
  startX: number,
  startY: number,
  endX: number,
  endY: number,
  callback: (x: number, y: number) => void,
): void {
  if (startY === endY) {
    for (let x = startX; x <= endX; x++) callback(x, startY);
    return;
  }

  for (let y = startY; y <= endY; y++) callback(startX, y);
}

function chooseGroundTileImageId(
  x: number,
  y: number,
  kind: Exclude<TerrainKind, "rock" | "water">,
  seed: number,
): number {
  if (kind === "grassPatch") {
    return tileImageIdForBlock(11, x, y);
  }

  if (hashNoise(x, y, seed ^ 0x41ad) < 0.08) {
    return 5;
  }

  return tileImageIdForBlock(1, x, y);
}

function chooseWaterTileImageId(
  x: number,
  y: number,
  generatedKinds: Map<string, TerrainKind>,
  sourceWaterKeys: Set<string>,
): number {
  const north = isWaterAt(x, y - 1, sourceWaterKeys, generatedKinds);
  const west = isWaterAt(x - 1, y, sourceWaterKeys, generatedKinds);
  const east = isWaterAt(x + 1, y, sourceWaterKeys, generatedKinds);

  if (!north && !west) {
    return 42;
  }
  if (!north && !east) {
    return 45;
  }
  if (!north) {
    return 43;
  }
  if (!west) {
    return 44;
  }
  if (!east) {
    return 46;
  }

  return tileImageIdForBlock(4, x, y);
}

function chooseRockTileImageId(
  x: number,
  y: number,
  generatedKinds: Map<string, TerrainKind>,
  sourceRockKeys: Set<string>,
): number {
  const north = isRockAt(x, y - 1, sourceRockKeys, generatedKinds);
  const south = isRockAt(x, y + 1, sourceRockKeys, generatedKinds);
  const west = isRockAt(x - 1, y, sourceRockKeys, generatedKinds);
  const east = isRockAt(x + 1, y, sourceRockKeys, generatedKinds);

  if (!north && !west) {
    return 84;
  }
  if (!north && !east) {
    return 77;
  }
  if (!south && !west) {
    return 57;
  }
  if (!south && !east) {
    return 59;
  }
  if (!north) {
    return 76;
  }
  if (!south) {
    return 18;
  }
  if (!west) {
    return 56;
  }
  if (!east) {
    return 58;
  }

  return 16;
}

function tileImageIdForBlock(blockIndex: number, x: number, y: number): number {
  const entries = OVERWORLD_BLOCK_TILES[blockIndex];
  if (!entries) {
    return 19;
  }

  const position = tilePosition(x, y);
  if (entries[position] !== undefined) {
    return entries[position] as number;
  }

  const fallbackPosition =
    position === 1 ? 0 :
    position === 3 ? 2 :
    position === 2 ? 0 :
    0;

  if (entries[fallbackPosition] !== undefined) {
    return entries[fallbackPosition] as number;
  }

  const first = Object.values(entries).find(
    (tileImageId): tileImageId is number => typeof tileImageId === "number",
  );
  return first ?? 19;
}

function tilePosition(x: number, y: number): number {
  const parityX = positiveModulo(x, 2);
  const parityY = positiveModulo(y, 2);
  return parityY * 2 + parityX;
}

function positiveModulo(value: number, divisor: number): number {
  return ((value % divisor) + divisor) % divisor;
}

function countRockNeighbors(
  x: number,
  y: number,
  sourceRockKeys: Set<string>,
  generatedKinds?: Map<string, TerrainKind>,
): number {
  let count = 0;
  for (const [dx, dy] of NEIGHBOR_OFFSETS) {
    if (isRockAt(x + dx, y + dy, sourceRockKeys, generatedKinds)) {
      count++;
    }
  }
  return count;
}

function countWaterNeighbors(
  x: number,
  y: number,
  sourceWaterKeys: Set<string>,
  generatedKinds?: Map<string, TerrainKind>,
): number {
  let count = 0;
  for (const [dx, dy] of NEIGHBOR_OFFSETS) {
    if (isWaterAt(x + dx, y + dy, sourceWaterKeys, generatedKinds)) {
      count++;
    }
  }
  return count;
}

function countKindNeighbors(
  x: number,
  y: number,
  sourceKindKeys: Set<string>,
  generatedKinds: Map<string, TerrainKind>,
  kind: Exclude<TerrainKind, "base">,
): number {
  let count = 0;
  for (const [dx, dy] of NEIGHBOR_OFFSETS) {
    const key = tileKey(x + dx, y + dy);
    if (sourceKindKeys.has(key) || generatedKinds.get(key) === kind) {
      count++;
    }
  }
  return count;
}

function isRockAt(
  x: number,
  y: number,
  sourceRockKeys: Set<string>,
  generatedKinds?: Map<string, TerrainKind>,
): boolean {
  const key = tileKey(x, y);
  return sourceRockKeys.has(key) || generatedKinds?.get(key) === "rock";
}

function isWaterAt(
  x: number,
  y: number,
  sourceWaterKeys: Set<string>,
  generatedKinds?: Map<string, TerrainKind>,
): boolean {
  const key = tileKey(x, y);
  return sourceWaterKeys.has(key) || generatedKinds?.get(key) === "water";
}

function isKindAt(
  x: number,
  y: number,
  sourceKindKeys: Set<string>,
  generatedKinds: Map<string, TerrainKind>,
  kind: "rock" | "water",
): boolean {
  const key = tileKey(x, y);
  return sourceKindKeys.has(key) || generatedKinds.get(key) === kind;
}

function parseTileKey(key: string): [number, number] {
  const [x, y] = key.split(",").map((part) => Number.parseInt(part, 10));
  return [x, y];
}

function hashNoise(x: number, y: number, seed: number): number {
  let h = Math.imul(x, 374761393) ^ Math.imul(y, 668265263) ^ Math.imul(seed, 1442695041);
  h = (h ^ (h >>> 13)) >>> 0;
  h = Math.imul(h, 1274126177) >>> 0;
  return ((h ^ (h >>> 16)) >>> 0) / 0xffffffff;
}
