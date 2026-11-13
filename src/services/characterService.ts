/**
 * Character Service - Static data, character creation, and character state mapping
 */
import { correlatedRequest } from "@/phaser-game/services/CorrelatedRequest";
import * as PhaserNet from "@/phaser-game/services/PhaserNetworkService";

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

type StaticDataResponse = import("@/net/generated/world_api").StaticDataResponse;
function requireStaticLists(response: import("@/net/generated/world_api").StaticDataResponse): asserts response is import("@/net/generated/world_api").StaticDataResponse & { maps:MapData[] } {
 if (![response.maps,response.classes,response.factions,response.startCities].every(Array.isArray)) throw new Error("Incomplete static content response");
 for (const map of response.maps) {
  if (!map || typeof map.name!=="string" || ![map.id,map.width,map.height].every(Number.isSafeInteger) || typeof map.isOverworld!=="boolean"
   || (["tilesetId","northConnection","southConnection","westConnection","eastConnection"] as const).some(key=>map[key]!==null&&!Number.isSafeInteger(map[key]))) throw new Error(`Invalid static map ${map?.id}`);
 }
}

/**
 * Get static game data.
 */
export async function getStaticData(signal?: AbortSignal): Promise<{
  classes: ClassData[];
  maps: MapData[];
  factions: FactionData[];
  startCities: HomeTownData[];
}> {
  const response = await correlatedRequest<StaticDataResponse>(PhaserNet.onStaticContent, id => PhaserNet.requestStaticContent(id,false), signal);
  requireStaticLists(response);
  return { maps:response.maps, classes:response.classes, factions:response.factions, startCities:response.startCities };
}

/**
 * Get character creation data
 */
export async function getCharCreateData(signal?: AbortSignal): Promise<{
  factions: FactionData[];
  classes: ClassData[];
  homeTowns: HomeTownData[];
  maps: MapData[];
}> {
  const response = await correlatedRequest<StaticDataResponse>(PhaserNet.onStaticContent, id => PhaserNet.requestStaticContent(id,true), signal);
  requireStaticLists(response);
  return { maps:response.maps, factions:response.factions, classes:response.classes, homeTowns:response.startCities };
}
