/**
 * Character Service - Static data, character creation, and character state mapping
 */
import { WorldSocket, OpCodes } from "@/net";

// Static data types
export interface FactionData {
  id: number;
  name: string;
  shortName?: string;
  lore?: string;
  isPlayable?: boolean;
  isStarting?: boolean;
}

export interface ClassData {
  id: number;
  name: string;
  classType?: string;
  lore?: string;
}

export interface MapData {
  id: number;
  name: string;
  width: number;
  height: number;
  tilesetId: number | null;
  isOverworld: boolean;
  northConnection: number | null;
  southConnection: number | null;
  westConnection: number | null;
  eastConnection: number | null;
}

export interface HomeTownData {
  id: number;
  mapId: number;
  name: string;
  spawnX: number;
  spawnY: number;
  description: string;
  sortOrder: number;
}

interface StaticDataResponse {
  success: boolean;
  error?: string;
  maps?: Array<MapData & {
    tileset_id?: number | null;
    north_connection?: number | null;
    south_connection?: number | null;
    west_connection?: number | null;
    east_connection?: number | null;
  }>;
  classes?: ClassData[];
  factions?: FactionData[];
  startCities?: HomeTownData[];
}

interface CharacterCreationResponse {
  success: boolean;
  error?: string;
  factions?: FactionData[];
  classes?: ClassData[];
  startCities?: HomeTownData[];
}

/**
 * Get static game data.
 */
export async function getStaticData(): Promise<{
  classes: ClassData[];
  maps: MapData[];
  factions: FactionData[];
  startCities: HomeTownData[];
}> {
  if (!WorldSocket.isConnected) {
    throw new Error("WorldSocket not connected");
  }

  const response = (await WorldSocket.sendJsonRequest(
    OpCodes.StaticDataRequest,
    OpCodes.StaticDataResponse,
    {},
  )) as StaticDataResponse;

  if (!response.success) {
    throw new Error(response.error || "Failed to load static data");
  }

  return {
    maps: (response.maps || []).map((m) => ({
      ...m,
      tilesetId: m.tilesetId ?? m.tileset_id ?? null,
      isOverworld: !!m.isOverworld,
      northConnection: m.northConnection ?? m.north_connection ?? null,
      southConnection: m.southConnection ?? m.south_connection ?? null,
      westConnection: m.westConnection ?? m.west_connection ?? null,
      eastConnection: m.eastConnection ?? m.east_connection ?? null,
    })),
    classes: (response.classes || []).map((c) => ({
      ...c,
    })),
    factions: (response.factions || []).map((f) => ({
      ...f,
      isPlayable: !!f.isPlayable,
      isStarting: !!f.isStarting,
    })),
    startCities: (response.startCities || []).map((sc) => ({ ...sc })),
  };
}

/**
 * Get character creation data
 */
export async function getCharCreateData(): Promise<{
  factions: FactionData[];
  classes: ClassData[];
  homeTowns: HomeTownData[];
}> {
  if (!WorldSocket.isConnected) {
    throw new Error("WorldSocket not connected");
  }

  const response = (await WorldSocket.sendJsonRequest(
    OpCodes.CharCreateDataRequest,
    OpCodes.CharCreateDataResponse,
    {},
  )) as CharacterCreationResponse;

  if (!response.success) {
    throw new Error(response.error || "Failed to load character creation data");
  }

  return {
    factions: (response.factions || []).map((f) => ({
      ...f,
      isPlayable: !!f.isPlayable,
      isStarting: !!f.isStarting,
    })),
    classes: (response.classes || []).map((c) => ({ ...c })),
    homeTowns: (response.startCities || []).map((sc) => ({ ...sc })),
  };
}
