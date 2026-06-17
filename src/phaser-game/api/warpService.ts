/**
 * @deprecated Warps are now fetched via WebTransport through MapDataService.
 * This REST function is kept for backwards compatibility but should not be used.
 */

import { API_BASE_URL } from "./constants";

/**
 * @deprecated Use MapDataService.fetchWarps() instead
 */
export async function fetchWarps(): Promise<any[]> {
  console.warn("fetchWarps is deprecated - use MapDataService.fetchWarps() via WebTransport");
  const response = await fetch(`${API_BASE_URL}/warps`);
  return await response.json();
}
