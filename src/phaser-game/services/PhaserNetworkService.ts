/**
 * Phaser Network Service
 *
 * Uses the existing WorldSocket/NetworkBridge WebTransport infrastructure
 * instead of REST API calls for Phaser game data.
 */

import { WorldSocket } from "@/net/index";
import { NetworkBridge } from "@/net/NetworkBridge";
import * as OpCodes from "@/net/generated/opcodes";
import type {
  PhaserMapInfo,
  PhaserTile,
  PhaserActor,
  PhaserWarp,
  TrainerEncounterNotifyPayload,
} from "@/net/generated/world_api";

/**
 * Check if WebTransport connection is established
 */
export function isConnected(): boolean {
  return WorldSocket.isConnected;
}

/**
 * Request map information for a specific map ID.
 * Optional destX/destY atomically update the player's position on the server
 * when warping to a new map, avoiding race conditions with fetchActors.
 */
export function requestMapInfo(mapId: number, destX?: number, destY?: number): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request map info");
    return;
  }
  const payload: Record<string, number> = { mapId: mapId };
  if (destX !== undefined && destY !== undefined) {
    payload.destX = destX;
    payload.destY = destY;
  }
  NetworkBridge.send(payload, OpCodes.PhaserMapInfoRequest);
}

/**
 * Notify the server that a map is rendered and ready for map-script cutscenes.
 */
export function requestMapScripts(mapName: string): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request map scripts");
    return;
  }
  NetworkBridge.send({ mapName }, OpCodes.PhaserMapScriptsRequest);
}

/**
 * Ask the server whether a clicked actor/object starts a scripted event.
 */
export async function tryScriptedEventInteraction(
  actorId: number,
): Promise<boolean> {
  if (!WorldSocket.isConnected) {
    return false;
  }

  try {
    const response = await WorldSocket.sendJsonRequest<{
      success: boolean;
      started: boolean;
      scriptLabel?: string;
      error?: string;
    }>(
      OpCodes.ScriptedEventInteractRequest,
      OpCodes.ScriptedEventInteractResponse,
      { actorId },
      2500,
    );
    if (!response.success && response.error) {
      console.warn("[PhaserNetwork] Scripted event interaction failed:", response.error);
    }
    return Boolean(response.success && response.started);
  } catch (err) {
    console.warn("[PhaserNetwork] Scripted event interaction request failed:", err);
    return false;
  }
}

export interface TrainerInteractResult {
  success: boolean;
  error?: string;
  trainerActorId?: number;
  trainerName?: string;
  trainerClass?: string;
  dialogue?: string;
  shouldBattle?: boolean;
  defeated?: boolean;
}

export async function requestTrainerInteraction(
  actorId: number,
): Promise<TrainerInteractResult | null> {
  if (!WorldSocket.isConnected) {
    return null;
  }

  try {
    const response = await WorldSocket.sendJsonRequest<TrainerInteractResult>(
      OpCodes.TrainerInteractRequest,
      OpCodes.TrainerInteractResponse,
      { actorId },
      2500,
    );
    if (!response.success && response.error) {
      console.warn("[PhaserNetwork] Trainer interaction failed:", response.error);
    }
    return response;
  } catch (err) {
    console.warn("[PhaserNetwork] Trainer interaction request failed:", err);
    return null;
  }
}

export interface MapMusicResult {
  success: boolean;
  mapId?: number;
  musicConstant?: string;
  error?: string;
}

export async function requestMapMusic(
  mapId: number,
): Promise<MapMusicResult | null> {
  if (!WorldSocket.isConnected) {
    return null;
  }

  try {
    return await WorldSocket.sendJsonRequest<MapMusicResult>(
      OpCodes.PhaserMapMusicRequest,
      OpCodes.PhaserMapMusicResponse,
      { mapId },
      2500,
    );
  } catch (err) {
    console.warn("[PhaserNetwork] Map music request failed:", err);
    return null;
  }
}

export function sendTrainerBattleStart(trainerActorId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ trainerActorId }, OpCodes.TrainerBattleStartRequest);
}

/**
 * Request tiles for a specific map ID
 */
export function requestTiles(mapId: number): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request tiles");
    return;
  }
  NetworkBridge.send({ mapId: mapId }, OpCodes.PhaserTilesRequest);
}

/**
 * Request all overworld maps
 */
export function requestOverworldMaps(): void {
  if (!WorldSocket.isConnected) {
    console.warn(
      "[PhaserNetwork] Not connected - cannot request overworld maps",
    );
    return;
  }
  NetworkBridge.send({}, OpCodes.PhaserOverworldMapsRequest);
}

/**
 * Request actors for a specific map ID
 */
export function requestActors(mapId: number): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request actors");
    return;
  }
  NetworkBridge.send({ mapId: mapId }, OpCodes.PhaserActorsRequest);
}

/**
 * Request warps for a specific map ID
 */
export function requestWarps(mapId: number): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request warps");
    return;
  }
  NetworkBridge.send({ mapId: mapId }, OpCodes.PhaserWarpsRequest);
}

/**
 * Send player position update to server.
 */
export function sendPlayerPosition(
  x: number,
  y: number,
  mapId: number,
  direction?: string,
): void {
  if (!WorldSocket.isConnected) {
    return; // Silent fail for position updates
  }
  NetworkBridge.send({ x, y, mapId, direction }, OpCodes.PhaserPlayerPositionUpdate);
}

/**
 * Send a direction-only update (player turned but didn't move, e.g. facing a wall)
 */
export function sendDirectionUpdate(x: number, y: number, mapId: number, direction: string): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ x, y, mapId, direction }, OpCodes.PhaserPlayerPositionUpdate);
}

/**
 * Request to start surfing onto an adjacent water tile.
 */
export function requestSurf(
  targetX: number,
  targetY: number,
  mapId: number,
  direction: string,
): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request Surf");
    return;
  }
  NetworkBridge.send(
    { targetX, targetY, mapId, direction },
    OpCodes.PokeSurfingRequest,
  );
}

/**
 * Request a contextual field move against a target tile.
 */
export function requestFieldMoveUse(
  moveName: string,
  targetX: number,
  targetY: number,
  mapId: number,
  direction: string,
): void {
  if (!WorldSocket.isConnected) {
    console.warn(`[PhaserNetwork] Not connected - cannot request ${moveName}`);
    return;
  }
  NetworkBridge.send(
    { moveName, targetX, targetY, mapId, direction },
    OpCodes.FieldMoveUseRequest,
  );
}

/**
 * Request elevator floor list (player clicked elevator control panel)
 */
export function requestElevatorFloors(mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ mapId }, OpCodes.ElevatorFloorsRequest);
}

/**
 * Select an elevator floor (player chose a floor from the menu)
 */
export function selectElevatorFloor(floorMapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ floorMapId }, OpCodes.ElevatorSelectRequest);
}

/**
 * Request to enter Safari Zone (player at gate — creates new session)
 */
export function requestSafariZoneEnter(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.SafariZoneEnterRequest);
}

/**
 * Check for existing Safari Zone session (reconnect/warp — never creates new session)
 */
export function requestSafariZoneStatus(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ statusOnly: true }, OpCodes.SafariZoneEnterRequest);
}

/**
 * Send a Safari Zone battle action (ball, bait, rock, run)
 */
export function sendSafariAction(action: string): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ action }, OpCodes.SafariBattleActionRequest);
}

/**
 * Request current coin balance
 */
export function requestCoinBalance(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.GameCornerCoinBalanceRequest);
}

/**
 * Buy 50 coins for ₽1000
 */
export function buyCoins(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.GameCornerBuyCoinsRequest);
}

/**
 * Play slot machine (bet 1-3 coins)
 */
export function playSlotMachine(bet: number, isLucky: boolean): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ bet, isLucky }, OpCodes.GameCornerSlotPlayRequest);
}

/**
 * Request prize list
 */
export function requestPrizeList(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.GameCornerPrizeListRequest);
}

/**
 * Buy a prize with coins
 */
export function buyPrize(prizeId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ prizeId }, OpCodes.GameCornerPrizeBuyRequest);
}

// Response handler registration
export type PhaserMapInfoHandler = (data: PhaserMapInfo) => void;
export type PhaserTilesHandler = (data: PhaserTile[]) => void;
export type PhaserOverworldMapsHandler = (data: PhaserMapInfo[]) => void;
export type PhaserActorsHandler = (data: PhaserActor[]) => void;
export type PhaserWarpsHandler = (data: PhaserWarp[]) => void;
export type PhaserActorUpdateHandler = (data: PhaserActor) => void;
export type PhaserActorDespawnHandler = (data: { id: number }) => void;
export type TrainerEncounterHandler = (
  data: TrainerEncounterNotifyPayload,
) => void;
export type PhaserMapMusicHandler = (data: MapMusicResult) => void;

const handlers = {
  mapInfo: new Set<PhaserMapInfoHandler>(),
  tiles: new Set<PhaserTilesHandler>(),
  overworldMaps: new Set<PhaserOverworldMapsHandler>(),
  actors: new Set<PhaserActorsHandler>(),
  warps: new Set<PhaserWarpsHandler>(),
  actorUpdate: new Set<PhaserActorUpdateHandler>(),
  actorDespawn: new Set<PhaserActorDespawnHandler>(),
  trainerEncounter: new Set<TrainerEncounterHandler>(),
  mapMusic: new Set<PhaserMapMusicHandler>(),
};

// Subscribe to response events
export function onMapInfo(handler: PhaserMapInfoHandler): () => void {
  handlers.mapInfo.add(handler);
  return () => handlers.mapInfo.delete(handler);
}

export function onTiles(handler: PhaserTilesHandler): () => void {
  handlers.tiles.add(handler);
  return () => handlers.tiles.delete(handler);
}

export function onOverworldMaps(
  handler: PhaserOverworldMapsHandler,
): () => void {
  handlers.overworldMaps.add(handler);
  return () => handlers.overworldMaps.delete(handler);
}

export function onActors(handler: PhaserActorsHandler): () => void {
  handlers.actors.add(handler);
  return () => handlers.actors.delete(handler);
}

export function onWarps(handler: PhaserWarpsHandler): () => void {
  handlers.warps.add(handler);
  return () => handlers.warps.delete(handler);
}

export function onActorUpdate(handler: PhaserActorUpdateHandler): () => void {
  handlers.actorUpdate.add(handler);
  return () => handlers.actorUpdate.delete(handler);
}

export function onActorDespawn(handler: PhaserActorDespawnHandler): () => void {
  handlers.actorDespawn.add(handler);
  return () => handlers.actorDespawn.delete(handler);
}

export function onTrainerEncounter(
  handler: TrainerEncounterHandler,
): () => void {
  handlers.trainerEncounter.add(handler);
  return () => handlers.trainerEncounter.delete(handler);
}

export function onMapMusic(handler: PhaserMapMusicHandler): () => void {
  handlers.mapMusic.add(handler);
  return () => handlers.mapMusic.delete(handler);
}

/**
 * Tell the server the local trainer approach animation has finished and battle can start.
 */
export function sendTrainerEncounterReady(trainerActorId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ trainerActorId }, OpCodes.TrainerEncounterReady);
}

/**
 * Send a Pokémon Center heal request to the server (triggered by clicking Nurse Joy).
 */
export function sendPokeCenterHeal(mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ mapId }, OpCodes.PokeCenterHealRequest);
}

/**
 * Request the player's current Pokémon party from the server.
 */
export function sendPokemonPartyRequest(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.PokemonPartyRequest);
}

/**
 * Request the player's CQ inventory from the server.
 */
export function sendCQInventoryRequest(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.CQInventoryRequest);
}

/**
 * Open a merchant shop by merchant ID or map ID.
 */
export function sendCQMerchantOpen(merchantId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ merchantId }, OpCodes.CQMerchantOpenRequest);
}

/**
 * Open a merchant shop by the map ID the clerk NPC is on.
 */
export function sendCQMerchantOpenByMap(mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ mapId }, OpCodes.CQMerchantOpenRequest);
}

/**
 * Buy an item from a merchant.
 */
export function sendCQMerchantBuy(
  merchantId: number,
  itemId: number,
  quantity: number,
): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send(
    { merchantId, itemId, quantity },
    OpCodes.CQMerchantBuyRequest,
  );
}

/**
 * Sell an item to a merchant.
 */
export function sendCQMerchantSell(instanceId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ instanceId }, OpCodes.CQMerchantSellRequest);
}

/**
 * Use an item during battle (Poké Ball, Potion, etc.)
 */
export function sendCQBattleItemUse(itemId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ itemId }, OpCodes.CQBattleItemUseRequest);
}

/**
 * Clear all registered handlers
 * Used during game destruction to prevent late network messages from calling stale handlers
 */
export function clearAllHandlers(): void {
  handlers.mapInfo.clear();
  handlers.tiles.clear();
  handlers.overworldMaps.clear();
  handlers.actors.clear();
  handlers.warps.clear();
  handlers.actorUpdate.clear();
  handlers.actorDespawn.clear();
  handlers.trainerEncounter.clear();
  handlers.mapMusic.clear();
}

// Internal: dispatch incoming Phaser responses
export function dispatchPhaserResponse(opcode: number, data: unknown): void {
  switch (opcode) {
    case OpCodes.PhaserMapInfoResponse:
      handlers.mapInfo.forEach((h) => h(data as PhaserMapInfo));
      break;
    case OpCodes.PhaserTilesResponse:
      handlers.tiles.forEach((h) => h(data as PhaserTile[]));
      break;
    case OpCodes.PhaserOverworldMapsResponse:
      handlers.overworldMaps.forEach((h) => h(data as PhaserMapInfo[]));
      break;
    case OpCodes.PhaserActorsResponse:
      handlers.actors.forEach((h) => h(data as PhaserActor[]));
      break;

    case OpCodes.PhaserWarpsResponse:
      handlers.warps.forEach((h) => h(data as PhaserWarp[]));
      break;
    case OpCodes.PhaserActorPositionUpdate:
      handlers.actorUpdate.forEach((h) => h(data as PhaserActor));
      break;
    case OpCodes.PhaserActorDespawn:
      handlers.actorDespawn.forEach((h) => h(data as { id: number }));
      break;
    case OpCodes.TrainerEncounterNotify:
      handlers.trainerEncounter.forEach((h) =>
        h(data as TrainerEncounterNotifyPayload),
      );
      break;
    case OpCodes.PhaserMapMusicResponse:
      handlers.mapMusic.forEach((h) => h(data as MapMusicResult));
      break;
  }
}

// Get tile image URL (static file from public folder)
export function getTileImageUrl(tileImageId: number): string {
  // Tile images are served from Vite's public folder
  // Adjust to 0-indexed: tile_image_id 1 → tile_0.png
  return `/phaser/tile_images/tile_${tileImageId - 1}.png`;
}

/**
 * Send an item pickup request to the server (triggered by clicking an item ball).
 */
export function sendItemPickup(actorId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ actorId }, OpCodes.ItemPickupRequest);
}

/**
 * Open the Pokémon PC (triggered by clicking a PC object).
 */
export function sendPokemonPCOpen(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.PokemonPCOpenRequest);
}

/**
 * Deposit a party Pokémon into a PC box.
 */
export function sendPokemonPCDeposit(partySlot: number, box: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ partySlot, box }, OpCodes.PokemonPCDepositRequest);
}

/**
 * Withdraw a Pokémon from a PC box to the party.
 */
export function sendPokemonPCWithdraw(box: number, boxSlot: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ box, boxSlot }, OpCodes.PokemonPCWithdrawRequest);
}

/**
 * Release a Pokémon from PC storage permanently.
 */
export function sendPokemonPCRelease(box: number, boxSlot: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ box, boxSlot }, OpCodes.PokemonPCReleaseRequest);
}

/**
 * Switch to a different PC box.
 */
export function sendPokemonPCSwitchBox(box: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ box }, OpCodes.PokemonPCSwitchBoxRequest);
}

/**
 * Send a dialogue YES/NO choice response to the server.
 */
export function sendDialogueChoice(
  textConstant: string,
  choice: boolean,
  actorId: number,
): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send(
    { textConstant, choice, actorId },
    OpCodes.DialogueChoiceRequest,
  );
}

// Get sprite URL (static file from public folder)
export function getSpriteUrl(spriteName: string): string {
  return `/phaser/sprites/${spriteName}`;
}
