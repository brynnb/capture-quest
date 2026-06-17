/**
 * @deprecated Map data is now fetched via WebTransport through MapDataService.
 * These REST functions are kept for backwards compatibility but should not be used.
 */

import { API_BASE_URL } from "./constants";

/**
 * @deprecated Use MapDataService.fetchMapInfo() instead
 */
export const fetchMapInfo = async (mapId: number): Promise<any> => {
  console.warn("fetchMapInfo is deprecated - use MapDataService.fetchMapInfo() via WebTransport");
  const response = await fetch(`${API_BASE_URL}/map-info/${mapId}`);
  return await response.json();
};

/**
 * @deprecated Use MapDataService.fetchOverworldMaps() instead
 */
export const fetchOverworldMaps = async (): Promise<any[]> => {
  console.warn("fetchOverworldMaps is deprecated - use MapDataService.fetchOverworldMaps() via WebTransport");
  const response = await fetch(`${API_BASE_URL}/overworld-maps`);
  return await response.json();
};
