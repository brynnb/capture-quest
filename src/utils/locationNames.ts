import { UNIFIED_OVERWORLD_MAP_ID } from "@/phaser-game/constants";
import type { MapData } from "@/services/characterService";

const LEGACY_OVERWORLD_MIN_ID = 0;
const LEGACY_OVERWORLD_MAX_ID = 33;

export function isLegacyOverworldMapId(mapId: number): boolean {
  return mapId >= LEGACY_OVERWORLD_MIN_ID && mapId <= LEGACY_OVERWORLD_MAX_ID;
}

export function formatLocationName(rawName?: string | null): string | undefined {
  const trimmed = rawName?.trim();
  if (!trimmed) return undefined;

  const normalized = trimmed.toUpperCase().replace(/[^A-Z0-9]+/g, "_");
  if (
    normalized === "UNIFIED_OVERWORLD" ||
    normalized === "OVERWORLD" ||
    normalized === "UNIFIED_OVERWORLD_MAP"
  ) {
    return "Kanto";
  }

  if (
    normalized === "SS_ANNE" ||
    normalized.startsWith("SS_ANNE_") ||
    normalized === "S_S_ANNE" ||
    normalized.startsWith("S_S_ANNE_")
  ) {
    const suffix = normalized
      .replace(/^(?:SS_ANNE|S_S_ANNE)_?/, "")
      .split("_")
      .filter(Boolean)
      .map(formatLocationToken)
      .join(" ");
    return suffix ? `S.S. Anne ${suffix}` : "S.S. Anne";
  }

  const spaced = trimmed
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z0-9])/g, "$1 $2")
    .replace(/([A-Z])([A-Z][a-z])/g, "$1 $2")
    .replace(/([0-9])\s+([FB])\b/gi, "$1$2")
    .replace(/\s+/g, " ")
    .trim();

  return spaced
    .split(" ")
    .filter(Boolean)
    .map(formatLocationToken)
    .join(" ");
}

export function displayLocationNameForMap(
  mapId: number,
  maps: MapData[],
): string {
  const map = maps.find((m) => m.id === mapId);
  const formatted = formatLocationName(map?.name);
  if (formatted) return formatted;

  if (mapId === UNIFIED_OVERWORLD_MAP_ID || isLegacyOverworldMapId(mapId)) {
    return "Kanto";
  }

  return `Map ${mapId}`;
}

function formatLocationToken(token: string): string {
  const upper = token.toUpperCase();
  if (/^(?:B?\d+F|\d+B)$/.test(upper)) return upper;
  if (/^\d+$/.test(upper)) return upper;

  switch (upper) {
    case "MT":
      return "Mt.";
    case "MR":
      return "Mr.";
    case "MRS":
      return "Mrs.";
    case "TM":
    case "HM":
    case "PC":
      return upper;
    case "POKEMON":
      return "Pokemon";
    default:
      return upper.charAt(0) + upper.slice(1).toLowerCase();
  }
}
