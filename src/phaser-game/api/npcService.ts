/**
 * @deprecated NPCs are now fetched via WebTransport through MapDataService.
 * This REST function is kept for backwards compatibility but should not be used.
 */

import { API_BASE_URL } from "./constants";

/**
 * @deprecated Use MapDataService.fetchNPCs() instead
 */
export async function fetchNPCs(): Promise<any[]> {
  console.warn("fetchNPCs is deprecated - use MapDataService.fetchNPCs() via WebTransport");
  const response = await fetch(`${API_BASE_URL}/npcs`);
  return await response.json();
}
