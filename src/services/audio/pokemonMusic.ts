import audioManifest from "@/constants/audio_manifest.json";
import pokemonAudioManifest from "@/constants/pokemon_audio_manifest.json";

type AudioAsset = {
  path?: string;
  loop?: boolean;
};

type PokemonCryAsset = {
  path?: string;
  basePath?: string;
  baseCry?: string;
  pokemonName?: string;
};

type PokemonAudioManifest = {
  mapMusic?: Array<{
    map_id?: number;
    music_constant?: string;
    path?: string;
  }>;
  music: Record<string, AudioAsset>;
  sfx: Record<string, AudioAsset>;
  pokemonCries?: Record<string, PokemonCryAsset>;
};

export const DEFAULT_WORLD_MUSIC = "/sound/pokemon/music/pallet_town.ogg";
export const TITLE_MUSIC = "/sound/title.mp3";

const localAudioFiles = new Set<string>([
  ...((audioManifest.global as string[]) || []),
  ...((audioManifest.library as string[]) || []),
]);

const pokemonAudio = pokemonAudioManifest as PokemonAudioManifest;
const mapMusicById = new Map<number, string>();
for (const entry of pokemonAudio.mapMusic ?? []) {
  if (typeof entry.map_id === "number" && entry.music_constant) {
    mapMusicById.set(entry.map_id, entry.music_constant);
  }
}

export function isLocalAudioFileAvailable(path?: string | null): boolean {
  return !!path && localAudioFiles.has(path);
}

export function musicTrackForConstant(
  musicConstant?: string | null,
  fallback = DEFAULT_WORLD_MUSIC,
): string {
  const normalized = musicConstant?.trim().toUpperCase();
  if (!normalized) return fallback;
  const path = pokemonAudio.music[normalized]?.path;
  return isLocalAudioFileAvailable(path) ? path! : fallback;
}

export function musicTrackForMapConstant(
  musicConstant?: string | null,
): string {
  return musicTrackForConstant(musicConstant, DEFAULT_WORLD_MUSIC);
}

export function musicTrackForMapId(mapId?: number | null): string {
  if (mapId == null) return DEFAULT_WORLD_MUSIC;
  return musicTrackForMapConstant(mapMusicById.get(mapId));
}

const GYM_LEADER_CLASSES = new Set([
  "BROCK",
  "MISTY",
  "LT_SURGE",
  "ERIKA",
  "KOGA",
  "SABRINA",
  "BLAINE",
  "GIOVANNI",
  "LORELEI",
  "BRUNO",
  "AGATHA",
  "LANCE",
]);

export function battleMusicTrackForState(
  battleType?: string | null,
  trainerClass?: string | null,
): string {
  const normalizedClass = trainerClass?.trim().toUpperCase() ?? "";
  if (normalizedClass === "RIVAL3" || normalizedClass === "CHAMPION") {
    return musicTrackForConstant("MUSIC_FINAL_BATTLE", DEFAULT_WORLD_MUSIC);
  }
  if (battleType === "trainer") {
    const constant = GYM_LEADER_CLASSES.has(normalizedClass)
      ? "MUSIC_GYM_LEADER_BATTLE"
      : "MUSIC_TRAINER_BATTLE";
    return musicTrackForConstant(constant, DEFAULT_WORLD_MUSIC);
  }
  return musicTrackForConstant("MUSIC_WILD_BATTLE", DEFAULT_WORLD_MUSIC);
}

export function victoryMusicTrackForState(
  battleType?: string | null,
  trainerClass?: string | null,
): string {
  if (battleType === "trainer") {
    const normalizedClass = trainerClass?.trim().toUpperCase() ?? "";
    const constant = GYM_LEADER_CLASSES.has(normalizedClass)
      ? "MUSIC_DEFEATED_GYM_LEADER"
      : "MUSIC_DEFEATED_TRAINER";
    return musicTrackForConstant(constant, DEFAULT_WORLD_MUSIC);
  }
  return musicTrackForConstant("MUSIC_DEFEATED_WILD_MON", DEFAULT_WORLD_MUSIC);
}

export function bikeMusicTrack(): string {
  return musicTrackForConstant("MUSIC_BIKE_RIDING", DEFAULT_WORLD_MUSIC);
}

export function surfingMusicTrack(): string {
  return musicTrackForConstant("MUSIC_SURFING", DEFAULT_WORLD_MUSIC);
}

export function sfxPathForConstant(sfxConstant?: string | null): string | null {
  const normalized = sfxConstant?.trim().toUpperCase();
  if (!normalized) return null;
  const path = pokemonAudio.sfx[normalized]?.path;
  return isLocalAudioFileAvailable(path) ? path! : null;
}

function normalizePokemonCryKey(name?: string | null): string {
  let normalized = name?.trim().toUpperCase() ?? "";
  if (!normalized) return "";
  normalized = normalized
    .replace(/POKéMON/g, "POKEMON")
    .replace(/POKÉMON/g, "POKEMON")
    .replace(/MR\. MIME/g, "MR_MIME")
    .replace(/MR MIME/g, "MR_MIME")
    .replace(/FARFETCH'D/g, "FARFETCHD")
    .replace(/FARFETCH’D/g, "FARFETCHD")
    .replace(/NIDORAN♂/g, "NIDORAN_M")
    .replace(/NIDORAN♀/g, "NIDORAN_F")
    .replace(/NIDORAN_MALE/g, "NIDORAN_M")
    .replace(/NIDORAN_FEMALE/g, "NIDORAN_F")
    .replace(/NIDORAN M/g, "NIDORAN_M")
    .replace(/NIDORAN F/g, "NIDORAN_F");
  return normalized.replace(/[^A-Z0-9]+/g, "_").replace(/^_+|_+$/g, "");
}

export function cryPathForPokemon(
  pokemonName?: string | null,
  fallbackSfxConstant?: string | null,
): string | null {
  const key = normalizePokemonCryKey(pokemonName);
  const speciesPath = key ? pokemonAudio.pokemonCries?.[key]?.path : null;
  if (isLocalAudioFileAvailable(speciesPath)) {
    return speciesPath!;
  }
  return sfxPathForConstant(fallbackSfxConstant);
}

export function sfxPathOrFallback(
  sfxConstant: string,
  fallback: string,
): string {
  return sfxPathForConstant(sfxConstant) ?? fallback;
}
