import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();

function absolute(relativePath: string) {
  return path.join(root, relativePath);
}

function expectFile(relativePath: string, minimumBytes = 1) {
  const filePath = absolute(relativePath);
  expect(fs.existsSync(filePath), `${relativePath} should exist`).toBe(true);
  expect(
    fs.statSync(filePath).size,
    `${relativePath} should be non-empty`,
  ).toBeGreaterThanOrEqual(minimumBytes);
}

function fileCount(relativePath: string, extension: string) {
  return fs
    .readdirSync(absolute(relativePath))
    .filter((entry) => entry.endsWith(extension)).length;
}

test("generated asset bundle and extractor submodule are bootstrapped", async () => {
  expectFile("public/phaser/pokemon.db", 1_000_000);
  expectFile("public/phaser/sprites/red.png");
  expectFile("public/phaser/sprites/red_bike.png");
  expectFile("public/phaser/sprites/red_surf.png");
  expectFile("public/assets/trainers/player_back.png");
  expectFile("public/fonts/Pokemon GB.ttf");
  expectFile("src/constants/audio_manifest.json");
  expectFile("tools/pokemon-gameboy-extractor-tool/generate.sh");
  expectFile("tools/pokemon-gameboy-extractor-tool/package.json");

  expect(fileCount("public/phaser/tile_images", ".png")).toBeGreaterThanOrEqual(800);
  expect(fileCount("public/assets/pokemon/front", ".png")).toBe(151);
  expect(fileCount("public/assets/pokemon/back", ".png")).toBe(151);
});
