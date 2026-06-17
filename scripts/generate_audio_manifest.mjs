import fs from "fs";
import path from "path";

const SOUND_DIR = path.resolve("public/sound");
const MANIFEST_PATH = path.resolve("src/constants/audio_manifest.json");
const AUDIO_EXTENSIONS = new Set([".mp3", ".ogg", ".wav"]);

function listLocalSounds() {
  if (!fs.existsSync(SOUND_DIR)) {
    return [];
  }

  const files = [];
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const absolutePath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(absolutePath);
        continue;
      }
      if (!entry.isFile()) continue;
      if (!AUDIO_EXTENSIONS.has(path.extname(entry.name).toLowerCase())) continue;
      const relativePath = path.relative(SOUND_DIR, absolutePath).split(path.sep).join("/");
      files.push(`/sound/${relativePath}`);
    }
  };

  walk(SOUND_DIR);
  return files.sort((a, b) => a.localeCompare(b));
}

function buildManifest() {
  const localSounds = listLocalSounds();
  return {
    global: localSounds,
    zones: {},
    combat: {},
    library: localSounds,
    metadata: {},
  };
}

fs.mkdirSync(path.dirname(MANIFEST_PATH), { recursive: true });
fs.writeFileSync(MANIFEST_PATH, `${JSON.stringify(buildManifest(), null, 2)}\n`);
console.log(`Generated ${path.relative(process.cwd(), MANIFEST_PATH)} from public/sound`);
