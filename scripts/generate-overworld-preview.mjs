#!/usr/bin/env node
import { build } from "esbuild";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const defaultDbPath = resolve(repoRoot, "public/phaser/pokemon.db");
const defaultOutPath = resolve(repoRoot, "tmp/procedural-overworld-preview.json");
const defaultSeed = 0x51c0ffee;

const args = parseArgs(process.argv.slice(2));
const dbPath = resolve(repoRoot, args.db ?? defaultDbPath);
const outPath = resolve(repoRoot, args.out ?? defaultOutPath);
const seed = Number.parseInt(args.seed ?? `${defaultSeed}`, 10);
const pretty = args.pretty === "true" || args.pretty === "1";

if (!Number.isFinite(seed)) {
  throw new Error(`Invalid --seed value: ${args.seed}`);
}

const sourceTiles = readOverworldTiles(dbPath);
const generatorModule = await loadGeneratorModule();
const { tiles: generatedTiles, summary } =
  generatorModule.generateProceduralOverworldTiles(sourceTiles, { seed });

const payload = {
  format: "capturequest.proceduralOverworldPreview.v1",
  generatedAt: new Date().toISOString(),
  sourceDb: dbPath,
  seed,
  summary,
  rules: {
    originalTilesAreImmutable: true,
    generatedTilesOnlyFillMissingOriginalCells: true,
    rockCenterTileImageId: 16,
    rockEdgeTileImageIds: {
      topLeft: 84,
      top: 76,
      topRight: 77,
      left: 56,
      center: 16,
      right: 58,
      bottomLeft: 57,
      bottom: 18,
      bottomRight: 59,
    },
    waterEdgeTileImageIds: {
      topLeft: 42,
      top: 43,
      topRight: 45,
      left: 44,
      center: 13,
      right: 46,
    },
  },
  sourceTiles: sourceTiles.map((tile) => ({
    source: "original",
    id: tile.id,
    x: tile.x,
    y: tile.y,
    mapId: tile.mapId,
    tileImageId: tile.tileImageId,
    collisionType: tile.collisionType,
  })),
  generatedTiles: generatedTiles.map((tile) => ({
    source: "generated",
    id: tile.id,
    x: tile.x,
    y: tile.y,
    mapId: tile.mapId,
    tileImageId: tile.tileImageId,
    collisionType: tile.collisionType,
    generatedSource: tile.generatedSource,
  })),
};

mkdirSync(dirname(outPath), { recursive: true });
writeFileSync(outPath, JSON.stringify(payload, null, pretty ? 2 : 0));

console.log(
  [
    `Wrote ${outPath}`,
    `source=${sourceTiles.length}`,
    `generated=${generatedTiles.length}`,
    `rock=${summary.rockTileCount}`,
    `grass=${summary.grassPatchTileCount}`,
    `water=${summary.waterTileCount}`,
    `seed=${seed}`,
  ].join(" "),
);

async function loadGeneratorModule() {
  const entryPoint = resolve(
    repoRoot,
    "src/phaser-game/procedural/overworldProceduralTiles.ts",
  );
  const outFile = resolve(repoRoot, "tmp/procedural-overworld-generator.mjs");

  await build({
    entryPoints: [entryPoint],
    outfile: outFile,
    bundle: true,
    format: "esm",
    platform: "browser",
    sourcemap: false,
    logLevel: "silent",
  });

  return import(`${pathToFileURL(outFile).href}?t=${Date.now()}`);
}

function readOverworldTiles(dbPath) {
  const query = `
    SELECT
      t.id AS id,
      t.x AS x,
      t.y AS y,
      t.map_id AS mapId,
      t.tile_image_id AS tileImageId,
      t.collision_type AS collisionType,
      0 AS talkOverTile
    FROM tiles t
    JOIN maps m ON m.id = t.map_id
    WHERE t.is_overworld = 1 OR m.is_overworld = 1
    ORDER BY t.y, t.x, t.id;
  `;
  const stdout = execFileSync("sqlite3", ["-json", dbPath, query], {
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024,
  });
  return JSON.parse(stdout).map((row) => ({
    id: row.id,
    x: row.x,
    y: row.y,
    mapId: row.mapId,
    tileImageId: row.tileImageId,
    collisionType: row.collisionType,
    talkOverTile: false,
  }));
}

function parseArgs(argv) {
  const parsed = {};
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (!arg.startsWith("--")) continue;
    const [rawKey, inlineValue] = arg.slice(2).split("=", 2);
    parsed[rawKey] = inlineValue ?? argv[++i] ?? "true";
  }
  return parsed;
}
