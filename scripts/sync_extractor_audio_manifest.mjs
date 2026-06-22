import fs from "fs";
import path from "path";
import { execFileSync } from "child_process";

const DEFAULT_EXTRACTOR_MANIFEST = path.resolve(
  "tools",
  "pokemon-gameboy-extractor-tool",
  "audio_manifest.json",
);
const SOURCE_PATH = path.resolve(
  process.env.EXTRACTOR_AUDIO_MANIFEST || DEFAULT_EXTRACTOR_MANIFEST,
);
const DESTINATION_PATH = path.resolve(
  "src/constants/pokemon_audio_manifest.json",
);
const DEFAULT_PHASER_DB = path.resolve("public", "phaser", "pokemon.db");
const PHASER_DB_PATH = path.resolve(process.env.PHASER_DB_PATH || DEFAULT_PHASER_DB);

if (!fs.existsSync(SOURCE_PATH)) {
  console.error(`Extractor audio manifest not found: ${SOURCE_PATH}`);
  console.error("Run `npm run bootstrap:assets` first.");
  process.exit(1);
}

const manifest = JSON.parse(fs.readFileSync(SOURCE_PATH, "utf8"));
for (const key of ["music", "sfx", "pokemonCries", "mapMusic", "moveSounds"]) {
  if (!manifest[key]) {
    console.error(`Extractor audio manifest is missing required key: ${key}`);
    process.exit(1);
  }
}

if ((!Array.isArray(manifest.mapMusic) || manifest.mapMusic.length === 0) && fs.existsSync(PHASER_DB_PATH)) {
  const rowsJson = execFileSync(
    "sqlite3",
    [
      "-json",
      PHASER_DB_PATH,
      `
        SELECT map_constant, map_id, music_constant
        FROM map_music
        ORDER BY map_id, map_constant
      `,
    ],
    { encoding: "utf8" },
  );
  const rows = JSON.parse(rowsJson || "[]");
  manifest.mapMusic = rows.map((row) => ({
    ...row,
    path: manifest.music?.[row.music_constant]?.path,
  }));
}

fs.mkdirSync(path.dirname(DESTINATION_PATH), { recursive: true });
fs.writeFileSync(DESTINATION_PATH, `${JSON.stringify(manifest, null, 2)}\n`);
console.log(
  `Synced ${path.relative(process.cwd(), DESTINATION_PATH)} from ${SOURCE_PATH} (${manifest.mapMusic.length} map music rows)`,
);
