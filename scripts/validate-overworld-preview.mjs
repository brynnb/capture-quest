#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(new URL("..", import.meta.url).pathname);
const defaultInputPath = resolve(repoRoot, "tmp/procedural-overworld-preview.json");
const inputPath = resolve(repoRoot, process.argv[2] ?? defaultInputPath);
const maxPrintedIssues = Number.parseInt(
  process.env.CAPTUREQUEST_TERRAIN_MAX_ISSUES ?? "80",
  10,
);
const showWarnings = process.env.CAPTUREQUEST_TERRAIN_SHOW_WARNINGS === "1";

const payload = JSON.parse(readFileSync(inputPath, "utf8"));

const ROCK = {
  topLeft: 84,
  top: 76,
  topRight: 77,
  left: 56,
  center: 16,
  right: 58,
  bottomLeft: 57,
  bottom: 18,
  bottomRight: 59,
};
const ROCK_IDS = new Set(Object.values(ROCK));
const ROCK_CORNERS = new Set([
  ROCK.topLeft,
  ROCK.topRight,
  ROCK.bottomLeft,
  ROCK.bottomRight,
]);

const WATER = {
  topLeft: 42,
  top: 43,
  topRight: 45,
  left: 44,
  center: 13,
  right: 46,
};
const WATER_IDS = new Set([13, 42, 43, 44, 45, 46, 65, 66, 113]);

const issues = [];
const warnings = [];
const sourceByKey = new Map();
const combinedByKey = new Map();

for (const tile of payload.sourceTiles ?? []) {
  const key = tileKey(tile.x, tile.y);
  const normalized = normalizeTile(tile, "original");
  sourceByKey.set(key, normalized);
  combinedByKey.set(key, normalized);
}

for (const tile of payload.generatedTiles ?? []) {
  const key = tileKey(tile.x, tile.y);
  const normalized = normalizeTile(tile, "generated");
  if (sourceByKey.has(key)) {
    issue("generated_overlaps_original", tile, "Generated tile overlaps an original game tile.");
    continue;
  }
  combinedByKey.set(key, normalized);
}

for (const tile of combinedByKey.values()) {
  if (tile.source === "generated" && isRock(tile)) {
    validateGeneratedRockTile(tile);
  }

  if (tile.source === "generated" && isWater(tile)) {
    validateGeneratedWaterTile(tile);
  }

  if (tile.tileImageId === ROCK.center) {
    validateRockCenterIsNotExposed(tile);
  }

  if (tile.source === "generated" && ROCK_CORNERS.has(tile.tileImageId)) {
    validateRockCornerSupport(tile);
  }
}

validateGeneratedWaterComponents();

if (warnings.length > 0 && showWarnings) {
  console.warn(`Terrain preview warnings: ${warnings.length}`);
  for (const warning of warnings.slice(0, maxPrintedIssues)) {
    console.warn(formatIssue(warning));
  }
} else if (warnings.length > 0) {
  console.warn(
    `Terrain preview warnings: ${warnings.length} original-source issue(s) hidden; set CAPTUREQUEST_TERRAIN_SHOW_WARNINGS=1 to print them.`,
  );
}

if (issues.length > 0) {
  console.error(`Terrain preview validation failed: ${issues.length} issue(s)`);
  for (const validationIssue of issues.slice(0, maxPrintedIssues)) {
    console.error(formatIssue(validationIssue));
  }
  if (issues.length > maxPrintedIssues) {
    console.error(`... ${issues.length - maxPrintedIssues} more issue(s) omitted`);
  }
  process.exit(1);
}

console.log(
  [
    "Terrain preview validation passed",
    `source=${payload.sourceTiles?.length ?? 0}`,
    `generated=${payload.generatedTiles?.length ?? 0}`,
    `warnings=${warnings.length}`,
  ].join(" "),
);

function validateGeneratedRockTile(tile) {
  const expected = expectedRockTileId(tile.x, tile.y);
  if (tile.tileImageId !== expected) {
    issue(
      "generated_rock_role_mismatch",
      tile,
      `Generated rock tile role is ${tile.tileImageId}, expected ${expected}.`,
    );
  }
}

function validateGeneratedWaterTile(tile) {
  const expected = expectedWaterTileId(tile.x, tile.y);
  if (tile.tileImageId !== expected) {
    issue(
      "generated_water_role_mismatch",
      tile,
      `Generated water tile role is ${tile.tileImageId}, expected ${expected}.`,
    );
  }
}

function validateRockCenterIsNotExposed(tile) {
  for (const [dx, dy, name] of [
    [0, -1, "north"],
    [-1, 0, "west"],
    [1, 0, "east"],
    [0, 1, "south"],
  ]) {
    const neighbor = combinedByKey.get(tileKey(tile.x + dx, tile.y + dy));
    if (!neighbor) continue;
    if (isRock(neighbor)) continue;

    const record = tile.source === "generated" || neighbor.source === "generated"
      ? issue
      : warning;
    record(
      "rock_center_exposed",
      tile,
      `Rock center tile touches non-rock ${name} neighbor ${neighbor.tileImageId}.`,
    );
  }
}

function validateRockCornerSupport(tile) {
  const checks = {
    [ROCK.topLeft]: [
      [1, 0, ROCK.top, "top edge to the east"],
      [0, 1, ROCK.left, "left edge to the south"],
    ],
    [ROCK.topRight]: [
      [-1, 0, ROCK.top, "top edge to the west"],
      [0, 1, ROCK.right, "right edge to the south"],
    ],
    [ROCK.bottomLeft]: [
      [1, 0, ROCK.bottom, "bottom edge to the east"],
      [0, -1, ROCK.left, "left edge to the north"],
    ],
    [ROCK.bottomRight]: [
      [-1, 0, ROCK.bottom, "bottom edge to the west"],
      [0, -1, ROCK.right, "right edge to the north"],
    ],
  }[tile.tileImageId];

  for (const [dx, dy, expectedTileImageId, label] of checks) {
    const neighbor = combinedByKey.get(tileKey(tile.x + dx, tile.y + dy));
    if (!neighbor || neighbor.tileImageId !== expectedTileImageId) {
      issue(
        "rock_corner_missing_edge_support",
        tile,
        `Rock corner ${tile.tileImageId} needs ${label}; found ${neighbor?.tileImageId ?? "missing"}.`,
      );
    }
  }
}

function validateGeneratedWaterComponents() {
  const generatedWaterKeys = new Set(
    (payload.generatedTiles ?? [])
      .filter((tile) => isWater(tile))
      .map((tile) => tileKey(tile.x, tile.y)),
  );
  const visited = new Set();

  for (const key of generatedWaterKeys) {
    if (visited.has(key)) continue;

    const component = [];
    const queue = [key];
    visited.add(key);

    while (queue.length > 0) {
      const currentKey = queue.shift();
      component.push(currentKey);
      const [x, y] = parseTileKey(currentKey);

      for (const [dx, dy] of [
        [0, -1],
        [-1, 0],
        [1, 0],
        [0, 1],
      ]) {
        const nextKey = tileKey(x + dx, y + dy);
        if (!generatedWaterKeys.has(nextKey) || visited.has(nextKey)) continue;
        visited.add(nextKey);
        queue.push(nextKey);
      }
    }

    if (component.length < 5) continue;

    const xs = component.map((componentKey) => parseTileKey(componentKey)[0]);
    const ys = component.map((componentKey) => parseTileKey(componentKey)[1]);
    const width = Math.max(...xs) - Math.min(...xs) + 1;
    const height = Math.max(...ys) - Math.min(...ys) + 1;

    if (width < 3 || height < 3) {
      const [x, y] = parseTileKey(component[0]);
      issue(
        "generated_water_too_thin",
        { x, y, tileImageId: WATER.center, source: "generated" },
        `Generated water component is too thin (${width}x${height}, ${component.length} tiles).`,
      );
    }
  }
}

function expectedRockTileId(x, y) {
  const north = isRockAt(x, y - 1);
  const south = isRockAt(x, y + 1);
  const west = isRockAt(x - 1, y);
  const east = isRockAt(x + 1, y);

  if (!north && !west) return ROCK.topLeft;
  if (!north && !east) return ROCK.topRight;
  if (!south && !west) return ROCK.bottomLeft;
  if (!south && !east) return ROCK.bottomRight;
  if (!north) return ROCK.top;
  if (!south) return ROCK.bottom;
  if (!west) return ROCK.left;
  if (!east) return ROCK.right;
  return ROCK.center;
}

function expectedWaterTileId(x, y) {
  const north = isWaterAt(x, y - 1);
  const west = isWaterAt(x - 1, y);
  const east = isWaterAt(x + 1, y);

  if (!north && !west) return WATER.topLeft;
  if (!north && !east) return WATER.topRight;
  if (!north) return WATER.top;
  if (!west) return WATER.left;
  if (!east) return WATER.right;
  return WATER.center;
}

function normalizeTile(tile, fallbackSource) {
  return {
    source: tile.source ?? fallbackSource,
    id: tile.id,
    x: tile.x,
    y: tile.y,
    mapId: tile.mapId,
    tileImageId: tile.tileImageId,
    collisionType: tile.collisionType,
  };
}

function isRock(tile) {
  return ROCK_IDS.has(tile.tileImageId);
}

function isWater(tile) {
  return WATER_IDS.has(tile.tileImageId) || tile.collisionType === 2;
}

function isRockAt(x, y) {
  const tile = combinedByKey.get(tileKey(x, y));
  return tile ? isRock(tile) : false;
}

function isWaterAt(x, y) {
  const tile = combinedByKey.get(tileKey(x, y));
  return tile ? isWater(tile) : false;
}

function issue(code, tile, message) {
  issues.push({ severity: "error", code, tile, message });
}

function warning(code, tile, message) {
  warnings.push({ severity: "warning", code, tile, message });
}

function formatIssue(validationIssue) {
  const { code, tile, message } = validationIssue;
  return `${validationIssue.severity.toUpperCase()} ${code} at (${tile.x},${tile.y}) tile=${tile.tileImageId} source=${tile.source}: ${message}`;
}

function tileKey(x, y) {
  return `${x},${y}`;
}

function parseTileKey(key) {
  const [x, y] = key.split(",").map((part) => Number.parseInt(part, 10));
  return [x, y];
}
