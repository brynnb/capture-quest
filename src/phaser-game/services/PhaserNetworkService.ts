import type { CQPartyItemUseResponse, CQMerchantOpenResponse, CQMerchantBuyResponse, CQMerchantSellResponse, RepelUseResponse, PokemonPartyReorderResponse, InventoryCommandError } from "@/net/generated/world_api";
import { openShopForActor, buyShopItem, sellShopItem } from "./ShopCommandService";
import type { BattleCommandResponse, SafariBattleActionResponse, BattleCommandError } from "@/net/generated/world_api";
import type { GameplayStateRequest, GameplayStateResponse } from "@/net/generated/world_api";
import type { TrainerEncounterNotifyPayload } from "@/net/generated/protocol";
import type { CutsceneEndRequest, CutsceneEndResponse, OwnedPlayerPositionRequest, OwnedPlayerPositionResponse, ServerPlayerMovementNotify, PlayerFacingRequest, PlayerFacingResponse, PlayerStepRequest, PlayerStepResponse, PlayerStepCompleteRequest, PlayerStepCompleteResponse, PlayerStepError, PhaserMapInfoRequest, PhaserMapInfoResponse, PhaserMapLoadRequest, PhaserMapLoadResponse, PhaserMapRequestError, PhaserWarpActivateRequest, PhaserWarpActivateResponse, PhaserInstantWarpRequest, PhaserInstantWarpResponse } from "@/net/generated/protocol";
import type { GameCornerSlotPlayRequest } from "@/net/generated/world_api";
import type { PhaserMapScriptsRequest } from "@/net/generated/protocol";
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
  PhaserTile,
  PhaserTilesRequest,
  PhaserTilesResponse,
  PhaserActor,
} from "@/net/generated/world_api";

/**
 * Check if WebTransport connection is established
 */
export function isConnected(): boolean {
  return WorldSocket.isConnected;
}

/** Read-only metadata; arrivals use the separate map-load command. */
export function requestMapInfo(request: PhaserMapInfoRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PhaserMapInfoRequest);
}

export function requestMapLoad(request: PhaserMapLoadRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PhaserMapLoadRequest);
}

export function completeCutscene(request: CutsceneEndRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.CutsceneEndRequest);
}
export function requestGameplayState(request: GameplayStateRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.GameplayStateRequest);
}

export function requestOwnedPlayerPosition(request: OwnedPlayerPositionRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.OwnedPlayerPositionRequest);
}

export function requestPlayerFacing(request: PlayerFacingRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PlayerFacingRequest);
}

export function requestPlayerStep(request: PlayerStepRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PlayerStepRequest);
}
export function completePlayerStep(request: PlayerStepCompleteRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PlayerStepCompleteRequest);
}

export function requestInstantWarp(request: PhaserInstantWarpRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PhaserInstantWarpRequest);
}

export function requestWarpActivation(request: PhaserWarpActivateRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PhaserWarpActivateRequest);
}

/**
 * Notify the server that a map is rendered and ready for map-script cutscenes.
 */
export function requestMapScripts(mapName: string): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request map scripts");
    return;
  }
  NetworkBridge.send({ mapName } satisfies PhaserMapScriptsRequest, OpCodes.PhaserMapScriptsRequest);
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

export function sendTrainerBattleStart(trainerActorId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ trainerActorId }, OpCodes.TrainerBattleStartRequest);
}

/**
 * Request tiles for a specific map ID
 */
export function requestTiles(request: PhaserTilesRequest): void {
  if (!WorldSocket.isConnected) {
    console.warn("[PhaserNetwork] Not connected - cannot request tiles");
    return;
  }
  NetworkBridge.send(request, OpCodes.PhaserTilesRequest);
}

/**
 * Request all overworld maps
 */


/**
 * Request actors for a specific map ID
 */
export function requestActors(request: import("@/net/generated/world_api").PhaserActorsRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PhaserActorsRequest);
}

/**
 * Request warps for a specific map ID
 */
export function requestWarps(request: import("@/net/generated/world_api").PhaserWarpsRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.PhaserWarpsRequest);
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
export function playSlotMachine(bet: number, machineX: number, machineY: number): void {
  if (!WorldSocket.isConnected) return;
  const request: GameCornerSlotPlayRequest = { bet, machineX, machineY };
  NetworkBridge.send(request, OpCodes.GameCornerSlotPlayRequest);
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
export type PhaserMapInfoHandler = (data: PhaserMapInfoResponse | PhaserMapRequestError) => void;
export type PhaserMapLoadHandler = (data: PhaserMapLoadResponse | PhaserMapRequestError) => void;
export type PhaserTilesHandler = (data: PhaserTilesResponse | PhaserTile[]) => void;
export type PhaserActorsHandler = (data: import("@/net/generated/world_api").PhaserActorsResponse | PlayerStepError) => void;
export type PhaserWarpsHandler = (data: import("@/net/generated/world_api").PhaserWarpsResponse | PlayerStepError) => void;
export type PhaserActorUpdateHandler = (data: PhaserActor) => void;
export type PhaserActorDespawnHandler = (data: { id: number }) => void;
export type TrainerEncounterHandler = (
  data: TrainerEncounterNotifyPayload,
) => void;

type BattleCommandHandler = (data: BattleCommandResponse | SafariBattleActionResponse | BattleCommandError) => void;
const battleCommandHandlers = new Map<number, Set<BattleCommandHandler>>([
  OpCodes.SafariBattleActionResponse, OpCodes.PokeBattleActionResponse, OpCodes.PokeBattleSwitchResponse,
  OpCodes.CQBattleItemUseResponse, OpCodes.PokeMoveLearnResponse, OpCodes.PokeBattleCloseResponse,
].map(opcode => [opcode, new Set<BattleCommandHandler>()]));

export function onBattleCommand(opcode: number, receive: BattleCommandHandler): () => void {
  const listeners = battleCommandHandlers.get(opcode);
  if (!listeners) throw new Error("Unsupported battle response opcode");
  listeners.add(receive); return () => listeners.delete(receive);
}

type InventoryReply = import("@/net/generated/world_api").PokemonPCResponse | CQPartyItemUseResponse | CQMerchantOpenResponse | CQMerchantBuyResponse | CQMerchantSellResponse | RepelUseResponse | PokemonPartyReorderResponse | InventoryCommandError;
const inventoryCommandHandlers = new Map<number, Set<(reply: InventoryReply) => void>>([
 [OpCodes.CQItemUseResponse,new Set()], [OpCodes.CQMerchantOpenResponse,new Set()], [OpCodes.CQMerchantBuyResponse,new Set()], [OpCodes.CQMerchantSellResponse,new Set()],
 [OpCodes.RepelUseResponse,new Set()],
 [OpCodes.PokemonPartyReorderResponse,new Set()],
 ...[OpCodes.PokemonPCOpenResponse,OpCodes.PokemonPCDepositResponse,OpCodes.PokemonPCWithdrawResponse,OpCodes.PokemonPCReleaseResponse,OpCodes.PokemonPCSwitchBoxResponse].map(opcode => [opcode,new Set<(reply: InventoryReply) => void>()] as const),
]);
export function onInventoryCommand<T extends InventoryReply>(opcode: number, receive: (reply: T) => void): () => void {
 const listeners = inventoryCommandHandlers.get(opcode);
 if (!listeners) throw new Error("Unsupported inventory response opcode");
 const listener = (reply: InventoryReply) => receive(reply as T);
 listeners.add(listener); return () => listeners.delete(listener);
}

const handlers = {
  escapeRope: new Set<(data: import("@/net/generated/world_api").EscapeRopeUseResponse | PlayerStepError) => void>(),
  staticContent: new Set<(data: import("@/net/generated/world_api").StaticDataResponse | PlayerStepError) => void>(),
  preferences: new Set<(data: import("@/net/generated/world_api").PreferenceResponse | PlayerStepError) => void>(),
  bicycleState: new Set<(data: import("@/net/generated/world_api").BicycleStateResponse | PlayerStepError) => void>(),
  gameplayState: new Set<(data: GameplayStateResponse | PlayerStepError) => void>(),
  cutsceneEnd: new Set<(data: CutsceneEndResponse | PlayerStepError) => void>(),
  ownedPlayerPosition: new Set<(data: OwnedPlayerPositionResponse | PlayerStepError) => void>(),
  serverPlayerMovement: new Set<(data: ServerPlayerMovementNotify) => void>(),
  playerFacing: new Set<(data: PlayerFacingResponse | PlayerStepError) => void>(),
  playerStep: new Set<(data: PlayerStepResponse | PlayerStepError) => void>(),
  playerStepComplete: new Set<(data: PlayerStepCompleteResponse | PlayerStepError) => void>(),
  mapInfo: new Set<PhaserMapInfoHandler>(),
  mapLoad: new Set<PhaserMapLoadHandler>(),
  instantWarp: new Set<(data: PhaserInstantWarpResponse | PhaserMapRequestError) => void>(),
  warpActivation: new Set<(data: PhaserWarpActivateResponse | PhaserMapRequestError) => void>(),
  tiles: new Set<PhaserTilesHandler>(),
  actors: new Set<PhaserActorsHandler>(),
  warps: new Set<PhaserWarpsHandler>(),
  actorUpdate: new Set<PhaserActorUpdateHandler>(),
  actorDespawn: new Set<PhaserActorDespawnHandler>(),
  trainerEncounter: new Set<TrainerEncounterHandler>(),
};

export function onEscapeRope(handler: (data: import("@/net/generated/world_api").EscapeRopeUseResponse | PlayerStepError) => void): () => void {
  handlers.escapeRope.add(handler);
  return () => handlers.escapeRope.delete(handler);
}
export function requestEscapeRope(request: import("@/net/generated/world_api").EscapeRopeUseRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.EscapeRopeUseRequest);
}
export function onStaticContent(handler: (data: import("@/net/generated/world_api").StaticDataResponse | PlayerStepError) => void): () => void {
  handlers.staticContent.add(handler); return () => handlers.staticContent.delete(handler);
}
export function requestStaticContent(requestId: string, creation: boolean): Promise<void> {
  return NetworkBridge.send({requestId}, creation ? OpCodes.CharCreateDataRequest : OpCodes.StaticDataRequest);
}
export function onPreferences(handler: (data: import("@/net/generated/world_api").PreferenceResponse | PlayerStepError) => void): () => void {
  handlers.preferences.add(handler); return () => handlers.preferences.delete(handler);
}
export function requestPreferences(request: import("@/net/generated/world_api").PreferenceRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.SetOption);
}
export function onBicycleState(handler: (data: import("@/net/generated/world_api").BicycleStateResponse | PlayerStepError) => void): () => void {
  handlers.bicycleState.add(handler);
  return () => handlers.bicycleState.delete(handler);
}
export function requestBicycleState(request: import("@/net/generated/world_api").BicycleStateRequest): Promise<void> {
  return NetworkBridge.send(request, OpCodes.BicycleStateRequest);
}
export function onGameplayState(handler: (data: GameplayStateResponse | PlayerStepError) => void): () => void {
  handlers.gameplayState.add(handler); return () => handlers.gameplayState.delete(handler);
}

// Subscribe to response events
export function onMapInfo(handler: PhaserMapInfoHandler): () => void {
  handlers.mapInfo.add(handler);
  return () => handlers.mapInfo.delete(handler);
}

export function onCutsceneEnd(handler: (data: CutsceneEndResponse | PlayerStepError) => void): () => void {
  handlers.cutsceneEnd.add(handler);
  return () => { handlers.cutsceneEnd.delete(handler); };
}
export function onOwnedPlayerPosition(handler: (data: OwnedPlayerPositionResponse | PlayerStepError) => void): () => void {
  handlers.ownedPlayerPosition.add(handler);
  return () => { handlers.ownedPlayerPosition.delete(handler); };
}

export function onServerPlayerMovement(handler: (data: ServerPlayerMovementNotify) => void): () => void {
  handlers.serverPlayerMovement.add(handler);
  return () => { handlers.serverPlayerMovement.delete(handler); };
}

export function onPlayerFacing(handler: (data: PlayerFacingResponse | PlayerStepError) => void): () => void {
  handlers.playerFacing.add(handler);
  return () => { handlers.playerFacing.delete(handler); };
}

export function onPlayerStep(handler: (data: PlayerStepResponse | PlayerStepError) => void): () => void {
  handlers.playerStep.add(handler);
  return () => { handlers.playerStep.delete(handler); };
}
export function onPlayerStepComplete(handler: (data: PlayerStepCompleteResponse | PlayerStepError) => void): () => void {
  handlers.playerStepComplete.add(handler);
  return () => { handlers.playerStepComplete.delete(handler); };
}

export function onInstantWarp(handler: (data: PhaserInstantWarpResponse | PhaserMapRequestError) => void): () => void {
  handlers.instantWarp.add(handler);
  return () => { handlers.instantWarp.delete(handler); };
}

export function onWarpActivation(handler: (data: PhaserWarpActivateResponse | PhaserMapRequestError) => void): () => void {
  handlers.warpActivation.add(handler);
  return () => { handlers.warpActivation.delete(handler); };
}

export function onMapLoad(handler: PhaserMapLoadHandler): () => void {
  handlers.mapLoad.add(handler);
  return () => handlers.mapLoad.delete(handler);
}

export function onTiles(handler: PhaserTilesHandler): () => void {
  handlers.tiles.add(handler);
  return () => handlers.tiles.delete(handler);
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



/**
 * Tell the server the local trainer approach animation has finished and battle can start.
 */
export function sendTrainerEncounterReady(trainerActorId: number, encounterToken: string): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ trainerActorId, encounterToken }, OpCodes.TrainerEncounterReady);
}

// Both read intents share the current locked resource projection and scene owner.
export { refreshOwnedGameplayResources as sendPokemonPartyRequest, refreshOwnedGameplayResources as sendCQInventoryRequest } from "./InventoryCommandService";

export const sendCQMerchantOpen = openShopForActor;

/**
 * Buy an item from a merchant.
 */
export const sendCQMerchantBuy = buyShopItem;
export const sendCQMerchantSell = sellShopItem;

/**
 * Clear all registered handlers
 * Used during game destruction to prevent late network messages from calling stale handlers
 */
export function clearAllHandlers(): void {
  for (const listeners of Object.values(handlers)) listeners.clear();
  for (const listeners of battleCommandHandlers.values()) listeners.clear();
  for (const listeners of inventoryCommandHandlers.values()) listeners.clear();
}


export function normalizePhaserArrayPayload<T>(
  data: unknown,
  responseName: string,
): T[] {
  if (Array.isArray(data)) return data as T[];

  const serverError =
    data !== null &&
    typeof data === "object" &&
    "error" in data &&
    typeof data.error === "string"
      ? `: ${data.error}`
      : "";
  console.error(
    `[PhaserNetwork] Invalid ${responseName} payload; expected an array${serverError}`,
    data,
  );
  return [];
}

// Internal: dispatch incoming Phaser responses
export function dispatchPhaserResponse(opcode: number, data: unknown): void {
  switch (opcode) {
    case OpCodes.CQItemUseResponse:
    case OpCodes.CQMerchantOpenResponse:
    case OpCodes.CQMerchantBuyResponse:
    case OpCodes.CQMerchantSellResponse:
    case OpCodes.RepelUseResponse:
    case OpCodes.PokemonPCOpenResponse:
    case OpCodes.PokemonPCDepositResponse:
    case OpCodes.PokemonPCWithdrawResponse:
    case OpCodes.PokemonPCReleaseResponse:
    case OpCodes.PokemonPCSwitchBoxResponse:
    case OpCodes.PokemonPartyReorderResponse:
      inventoryCommandHandlers.get(opcode)?.forEach(receive => receive(data as InventoryReply));
      break;
    case OpCodes.SafariBattleActionResponse:
    case OpCodes.PokeBattleActionResponse:
    case OpCodes.PokeBattleSwitchResponse:
    case OpCodes.CQBattleItemUseResponse:
    case OpCodes.PokeMoveLearnResponse:
    case OpCodes.PokeBattleCloseResponse:
      battleCommandHandlers.get(opcode)?.forEach(receive => receive(data as BattleCommandResponse | SafariBattleActionResponse | BattleCommandError));
      break;

    case OpCodes.EscapeRopeUseResponse:
      handlers.escapeRope.forEach(handler => handler(data as import("@/net/generated/world_api").EscapeRopeUseResponse | PlayerStepError));
      break;
    case OpCodes.StaticDataResponse:
    case OpCodes.CharCreateDataResponse:
      handlers.staticContent.forEach(handler => handler(data as import("@/net/generated/world_api").StaticDataResponse | PlayerStepError));
      break;
    case OpCodes.SetOption:
      handlers.preferences.forEach(handler => handler(data as import("@/net/generated/world_api").PreferenceResponse | PlayerStepError));
      break;
    case OpCodes.BicycleStateResponse:
      handlers.bicycleState.forEach(handler => handler(data as import("@/net/generated/world_api").BicycleStateResponse | PlayerStepError));
      break;
    case OpCodes.GameplayStateResponse:
      handlers.gameplayState.forEach(h => h(data as GameplayStateResponse | PlayerStepError));
      break;
    case OpCodes.CutsceneEndResponse:
      handlers.cutsceneEnd.forEach((h) => h(data as CutsceneEndResponse | PlayerStepError));
      break;
    case OpCodes.OwnedPlayerPositionResponse:
      handlers.ownedPlayerPosition.forEach((h) => h(data as OwnedPlayerPositionResponse | PlayerStepError));
      break;
    case OpCodes.ServerPlayerMovementNotify:
      handlers.serverPlayerMovement.forEach((h) => h(data as ServerPlayerMovementNotify));
      break;
    case OpCodes.PlayerFacingResponse:
      handlers.playerFacing.forEach((h) => h(data as PlayerFacingResponse | PlayerStepError));
      break;
    case OpCodes.PlayerStepResponse:
      handlers.playerStep.forEach((h) => h(data as PlayerStepResponse | PlayerStepError));
      break;
    case OpCodes.PlayerStepCompleteResponse:
      handlers.playerStepComplete.forEach((h) => h(data as PlayerStepCompleteResponse | PlayerStepError));
      break;
    case OpCodes.PhaserMapInfoResponse:
      handlers.mapInfo.forEach((h) => h(data as PhaserMapInfoResponse | PhaserMapRequestError));
      break;
    case OpCodes.PhaserInstantWarpResponse:
      handlers.instantWarp.forEach((h) => h(data as PhaserInstantWarpResponse | PhaserMapRequestError));
      break;
    case OpCodes.PhaserWarpActivateResponse:
      handlers.warpActivation.forEach((h) => h(data as PhaserWarpActivateResponse | PhaserMapRequestError));
      break;
    case OpCodes.PhaserMapLoadResponse:
      handlers.mapLoad.forEach((h) => h(data as PhaserMapLoadResponse | PhaserMapRequestError));
      break;
    case OpCodes.PhaserTilesResponse:
      handlers.tiles.forEach((h) => h(data as PhaserTilesResponse | PhaserTile[]));
      break;

    case OpCodes.PhaserActorsResponse:
      handlers.actors.forEach((h) =>
        h(data as import("@/net/generated/world_api").PhaserActorsResponse | PlayerStepError),
      );
      break;

    case OpCodes.PhaserWarpsResponse:
      handlers.warps.forEach((h) =>
        h(data as import("@/net/generated/world_api").PhaserWarpsResponse | PlayerStepError),
      );
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

  }
}

/**
 * Send an item pickup request to the server (triggered by clicking an item ball).
 */
export function sendItemPickup(actorId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ actorId }, OpCodes.ItemPickupRequest);
}

export { openPokemonPC as sendPokemonPCOpen, depositPokemon as sendPokemonPCDeposit, withdrawPokemon as sendPokemonPCWithdraw, releasePokemon as sendPokemonPCRelease, switchPokemonBox as sendPokemonPCSwitchBox } from "./PCCommandService";

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
