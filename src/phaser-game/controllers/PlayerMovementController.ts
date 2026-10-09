import {recoverBattlePublication} from "../services/BattleCommandService";
import type { OwnedPlayerPositionResponse, PlayerStepResponse, PlayerStepError } from "@/net/generated/protocol";
import { requestFishing, requestEscapeRope, requestBicycleState, requestPlayerFacing, requestPlayerStep, completePlayerStep, readOwnedPlayerPosition } from "../services/PlayerMovementService";
import { CorrelatedResponseError } from "../services/CorrelatedRequest";
import { Scene } from "phaser";
import { PhaserActor, PhaserTile, PhaserWarp } from "@/net/generated/world_api";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import * as PhaserNet from "../services/PhaserNetworkService";
import { isWorldInputFrozen } from "../utils/worldInputGuard";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import { readCurrentGameplayState, applyGameplayResourceSnapshot } from "../services/GameplayRecoveryService";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import { emitCaptureQuestTestEvent } from "@/testing/capturequestTestBridge";
import type { MapRenderer } from "../renderers/MapRenderer";

type MovementDirection = "UP" | "DOWN" | "LEFT" | "RIGHT";

interface InteractionTarget {
  x: number;
  y: number;
}

interface MovementPathStep {
  x: number;
  y: number;
  ledgeJump?: boolean;
}

interface MovementPathNode extends MovementPathStep {
  g: number;
  h: number;
  f: number;
  parent: MovementPathNode | null;
}

const COLLISION_LAND = 1;
const COLLISION_WATER = 2;
const SURF_MOVE_ID = 57;
const CUT_TREE_RAW_FOOT_TILE_ID = 0x3d;
const NON_SURF_WARP_MAT_RAW_FOOT_TILE_IDS = new Set([0x04]);
const SURF_PLAYER_SPRITE = "SPRITE_RED_SURF";
const WATER_SEARCH_LIMIT = 2000;
const DEBUG_PLAYER_MOVEMENT = import.meta.env.VITE_DEBUG_MOVEMENT === "true";
const WARP_ACTIVATION_REQUEST_COOLDOWN_MS = 120;

function debugPlayerMovement(message: string): void {
  if (DEBUG_PLAYER_MOVEMENT) {
    console.debug(message);
  }
}

interface LedgeRule {
  direction: MovementDirection;
  standingRawFootTileId: number;
  frontRawFootTileId: number;
}

const LEDGE_RULES: LedgeRule[] = [
  { direction: "DOWN", standingRawFootTileId: 0x2c, frontRawFootTileId: 0x37 },
  { direction: "DOWN", standingRawFootTileId: 0x39, frontRawFootTileId: 0x36 },
  { direction: "DOWN", standingRawFootTileId: 0x39, frontRawFootTileId: 0x37 },
  { direction: "LEFT", standingRawFootTileId: 0x2c, frontRawFootTileId: 0x27 },
  { direction: "LEFT", standingRawFootTileId: 0x39, frontRawFootTileId: 0x27 },
  { direction: "RIGHT", standingRawFootTileId: 0x2c, frontRawFootTileId: 0x0d },
  { direction: "RIGHT", standingRawFootTileId: 0x2c, frontRawFootTileId: 0x1d },
  { direction: "RIGHT", standingRawFootTileId: 0x39, frontRawFootTileId: 0x0d },
];

function canJumpLedge(
  direction: MovementDirection,
  standingRawFootTileId: number | undefined,
  frontRawFootTileId: number | undefined,
): boolean {
  if (standingRawFootTileId == null || frontRawFootTileId == null) return false;
  return LEDGE_RULES.some(
    (rule) =>
      rule.direction === direction &&
      rule.standingRawFootTileId === standingRawFootTileId &&
      rule.frontRawFootTileId === frontRawFootTileId,
  );
}

/**
 * PlayerMovementController handles click-to-walk movement for the player character.
 *
 * This follows the ARCHITECTURE.md Manager Pattern - separating player movement
 * logic from the main scene and rendering concerns.
 *
 * Movement flow:
 * 1. Player clicks on a walkable tile
 * 2. Controller calculates path using A* pathfinding
 * 3. Controller moves sprite one tile at a time
 * 4. Each step sends position update to server via PhaserNetworkService
 * 5. Server broadcasts position to other players
 */
export class PlayerMovementController {
  private scene: Scene;
  private playerId: number | null = null;
  private mapRenderer: MapRenderer | null = null;

  // Collision map: key = "x,y", value = collision type (0 blocked, 1 land, 2 water)
  private collisionMap: Map<string, number> = new Map();
  private rawFootTileMap: Map<string, number> = new Map();
  private talkOverTileMap: Map<string, boolean> = new Map();
  private sourceMapByTile: Map<string, { id: number; name: string | null }> =
    new Map();
  private actorBlockers: Map<number, { x: number; y: number }> = new Map();
  private isSurfing: boolean = false;
  private preSurfSpriteName: string | null = null;

  // Current path (array of tile coordinates)
  private currentPath: MovementPathStep[] = [];
  private serverMovementInProgress = false;
  private serverPathFinished = true;
  private facingAbort: AbortController | null = null;
  private facingRequestKey: string | null = null;
  private stepAbort: AbortController | null = null;
  private issuedStep: PlayerStepResponse | null = null;
  private actorReconciler: (signal: AbortSignal) => Promise<void> = async () => {};
  setActorReconciler(reconcile: (signal: AbortSignal) => Promise<void>): void { this.actorReconciler = reconcile; }
  reconcileActors(signal: AbortSignal): Promise<void> { return this.actorReconciler(signal); }
  private fieldCommandAbort: AbortController | null = null;
  private fieldCommandsRetired = false;

  retireFieldCommands(): void {
    this.fieldCommandsRetired = true;
    this.fieldCommandAbort?.abort();
  }

  private async runOwnedFieldCommand(operation: (characterId: number, signal: AbortSignal, current: () => boolean) => Promise<void>): Promise<void> {
    if (this.fieldCommandAbort || this.fieldCommandsRetired || this.movementRecoveryRequired || this.playerId === null) return;
    const characterId = usePlayerCharacterStore.getState().characterProfile.id;
    if (!characterId) return;
    const abort = new AbortController();
    this.fieldCommandAbort = abort;
    const stopProfile = usePlayerCharacterStore.subscribe(state => {
      if (state.characterProfile.id !== characterId) abort.abort();
    });
    const current = () => !abort.signal.aborted && !this.fieldCommandsRetired
      && usePlayerCharacterStore.getState().characterProfile.id === characterId;
    try { await operation(characterId, abort.signal, current); }
    finally { stopProfile(); if (this.fieldCommandAbort === abort) this.fieldCommandAbort = null; }
  }

  async fish(instanceId:number):Promise<void>{
    if(this.isMoving || this.facingAbort || this.issuedStep || this.serverMovementInProgress)return;
    return this.runOwnedFieldCommand(async(characterId,signal,current)=>{
      const source={mapId:this.currentMapId,x:this.currentTileX,y:this.currentTileY,direction:this.currentDirection};
      const reconcile=()=>recoverBattlePublication({success:true},"ordinary-start",signal);
      try{
        const reply=await requestFishing({...source,characterId,instanceId,itemId:0},signal);
        if(!current())return;
        if(reply.characterId!==characterId || reply.instanceId!==instanceId || typeof reply.hooked!=="boolean" || typeof reply.message!=="string")throw new Error("Invalid fishing response identity");
        useChatStore.getState().addMessage(reply.message,MessageType.SYSTEM);
        if(reply.hooked && !(await reconcile()))this.movementRecoveryRequired=true;
      }catch(error){
        if(!current())return;
        try{if(!(await reconcile()))this.movementRecoveryRequired=true;if(current() && error instanceof CorrelatedResponseError)useChatStore.getState().addMessage(error.message,MessageType.SYSTEM_ERROR);}
        catch{if(current()){this.movementRecoveryRequired=true;useChatStore.getState().addMessage("Fishing state could not be recovered. Reconnect before moving.",MessageType.SYSTEM_ERROR);}}
      }
    });
  }

  async changeBicyclePreference(instanceId: number): Promise<void> {
    return this.runOwnedFieldCommand(async (characterId, signal, current) => {
      const apply = (reply: import("@/net/generated/world_api").BicycleStateResponse) => {
        if (reply.characterId !== characterId || !Number.isSafeInteger(reply.bicycle?.revision)
          || reply.bicycle.revision < 0 || typeof reply.bicycle.wantsRiding !== "boolean"
          || typeof reply.bicycle.activeRiding !== "boolean"
          || typeof reply.bicycle.forcedRiding !== "boolean") {
          throw new Error("Invalid Bicycle state");
        }
        useAudioActivityStore.getState().setBicycleState(reply.bicycle);
      };
      try {
        const before = await requestBicycleState(characterId, signal);
        if (!current()) return;
        apply(before);
        const result = await requestBicycleState(characterId, signal, {
          instanceId, wantsRiding: !before.bicycle.wantsRiding, revision: before.bicycle.revision,
        });
        if (current()) {
          apply(result);
          const bike = result.bicycle;
          const message = bike.forcedRiding ? "You can't get off here."
            : bike.wantsRiding ? (bike.activeRiding ? "You got on the Bicycle!"
              : "You'll get on the Bicycle when you go outside.") : "You got off the Bicycle.";
          useChatStore.getState().addMessage(message, MessageType.SYSTEM);
        }
      } catch {
        if (!current()) return;
        // An uncertain setter is reconciled by reading; never toggle or retry it.
        try {
          const owned = await requestBicycleState(characterId, signal);
          if (current()) apply(owned);
        } catch {
          if (current()) console.warn("[Movement] Bicycle state unavailable; reconnect before trying again.");
        }
      }
    });
  }
  async useEscapeRope(instanceId: number): Promise<void> {
    if (this.getIsMoving()) return;
    return this.runOwnedFieldCommand(async (characterId, signal, current) => {
      const source = { mapId: this.currentMapId, x: this.currentTileX, y: this.currentTileY };
      let accepted = false;
      let rejection: string | undefined;
      try {
        const reply = await requestEscapeRope({ ...source, instanceId,
          command: { characterId, revision: useCQInventoryStore.getState().commandRevision } }, signal);
        if (reply.characterId !== characterId) throw new Error("Escape Rope owner mismatch");
        accepted = true;
      } catch (error) {
        if (!current()) return;
        if (error instanceof CorrelatedResponseError) rejection = error.message;
        // The mutation may have committed. Recover once; never resend it.
      }
      try {
        const snapshot = await readCurrentGameplayState(signal, () => this.movementGeneration);
        if (!current()) return;
        applyGameplayResourceSnapshot(snapshot);
        const position = snapshot.position;
        if (accepted) useChatStore.getState().addMessage("You escaped from the dungeon.", MessageType.SYSTEM);
        else if (rejection) useChatStore.getState().addMessage(rejection, MessageType.SYSTEM);
        this.projectOwnedPosition(position);
      } catch {
        if (current()) {
          this.movementRecoveryRequired = true;
          console.warn("[Movement] Escape Rope recovery unavailable; reconnect before moving.");
        }
      }
    });
  }
  getPositionGeneration(): number { return this.movementGeneration; }

  // Retiring a route is only one change to ownership. Reads also need to notice
  // ordinary predicted movement/facing and a newly issued step before snapping.
  capturePositionView(): string {
    return `${this.movementGeneration}:${this.currentMapId}:${this.currentTileX}:${this.currentTileY}:${this.currentDirection}:${this.isMoving}:${this.issuedStep?.stepToken ?? ""}:${!!this.facingAbort}:${!!this.fieldCommandAbort}`;
  }

  // Call only after an owned, validated current read. Visual/server notifications
  // can be missed while a scene binds; resource and plan publication stay outside.
  projectOwnedPosition(position: OwnedPlayerPositionResponse): void {
    if (this.fieldCommandsRetired) return;
    this.movementRecoveryRequired = false;
    this.stopMovement(true);
    if (position.mapId === this.currentMapId) {
      this.syncPosition(position.x, position.y);
      this.syncDirection(position.direction);
      if (this.playerId !== null) this.mapRenderer?.snapActorPosition(this.playerId, position.x, position.y, position.direction);
      if (position.serverMovementPending) this.beginServerMovement(false);
    } else {
      window.dispatchEvent(new CustomEvent("warpTileTeleport", { detail: { ...position, serverCommitted: true } }));
    }
  }

  private movementGeneration = 0;
  private isMoving: boolean = false;
  private activeMoveDestination: {
    x: number;
    y: number;
    mapId: number;
    activateWarpId?: number;
  } | null = null;

  // Current player tile position
  private currentTileX: number = 0;
  private currentTileY: number = 0;
  private currentMapId: number = 0;
  private currentDirection: string = "DOWN";

  // Callback fired when player arrives at a destination (used for warp pathing)
  private arrivalCallback: ((x: number, y: number) => boolean) | null = null;
  private inputFreezeProvider: () => boolean = () => isWorldInputFrozen();
  private movementRecoveryRequired = false;
  private inputFrozenChecker = (): boolean => this.movementRecoveryRequired || this.fieldCommandAbort !== null || this.inputFreezeProvider();
  private warpTileChecker: (x: number, y: number) => boolean = () => false;
  private warpAtProvider: (x: number, y: number) => PhaserWarp | null =
    () => null;
  private warpActivator: (
    warp: PhaserWarp,
    direction?: MovementDirection,
  ) => void = () => {};
  private heldKeyboardDirectionProvider: () => MovementDirection | null =
    () => null;
  private lastWarpActivationRequestKey: string = "";
  private lastWarpActivationRequestAt: number = 0;

  constructor(scene: Scene) {
    this.scene = scene;
    scene.events?.once?.("shutdown", () => {
      this.retireFieldCommands();
      this.stopMovement(true);
    });
  }

  setInputFrozenChecker(checker: () => boolean): void {
    this.inputFreezeProvider = checker;
  }

  setWarpTileChecker(checker: (x: number, y: number) => boolean): void {
    this.warpTileChecker = checker;
  }

  setWarpAtProvider(provider: (x: number, y: number) => PhaserWarp | null): void {
    this.warpAtProvider = provider;
  }

  setWarpActivator(
    activator: (warp: PhaserWarp, direction?: MovementDirection) => void,
  ): void {
    this.warpActivator = activator;
  }

  setHeldKeyboardDirectionProvider(
    provider: () => MovementDirection | null,
  ): void {
    this.heldKeyboardDirectionProvider = provider;
  }

  private emitPlayerPositionChanged(): void {
    useGameStatusStore.getState().setPlayerTileContext({
      x: this.currentTileX,
      y: this.currentTileY,
      mapId: this.currentMapId,
      direction: this.currentDirection,
    });
    emitCaptureQuestTestEvent("cq:playerPositionChanged", {
      x: this.currentTileX,
      y: this.currentTileY,
      mapId: this.currentMapId,
      direction: this.currentDirection,
      isMoving: this.isMoving,
      isSurfing: this.isSurfing,
    });
  }

  /**
   * Set the player info and map renderer
   */
  setPlayer(
    playerId: number,
    startX: number,
    startY: number,
    mapId: number,
    mapRenderer: MapRenderer,
  ): void {
    // console.log(`[PlayerMovement] setPlayer: ID=${playerId}, pos=(${startX}, ${startY}), map=${mapId}`);
    this.playerId = playerId;

    // Preserve current position if new values are null/undefined
    if (startX !== null && startX !== undefined) this.currentTileX = startX;
    if (startY !== null && startY !== undefined) this.currentTileY = startY;

    if (this.currentMapId !== mapId) {
      this.activeMoveDestination = null;
    }
    this.currentMapId = mapId;
    this.mapRenderer = mapRenderer;
    this.updateTravelMapForTile(this.currentTileX, this.currentTileY);
    this.emitPlayerPositionChanged();
  }

  private queueClientMove(
    destX: number,
    destY: number,
    mapId: number,
    inputSource: "click" | "keyboard",
    activateWarpId?: number,
  ): void {
    const path = this.findPath(this.currentTileX, this.currentTileY, destX, destY);
    if (path.length > 0) {
      this.queuePredictedPathMove(
        destX,
        destY,
        mapId,
        inputSource,
        path,
        undefined,
        activateWarpId,
      );
      return;
    }

    debugPlayerMovement(
      `[PlayerMovement] No client path from (${this.currentTileX}, ${this.currentTileY}) to (${destX}, ${destY}) on map ${mapId}`,
    );
  }

  private queuePredictedKeyboardMove(
    destX: number,
    destY: number,
    mapId: number,
    direction: MovementDirection,
    activateWarpId?: number,
  ): void {
    this.queuePredictedPathMove(
      destX,
      destY,
      mapId,
      "keyboard",
      [{ x: destX, y: destY }],
      direction,
      activateWarpId,
    );
  }

  private queuePredictedPathMove(
    destX: number,
    destY: number,
    mapId: number,
    inputSource: "click" | "keyboard",
    path: MovementPathStep[],
    initialDirection?: MovementDirection,
    activateWarpId?: number,
  ): void {
    const wasMoving = this.isMoving;

    this.lastInputSource = inputSource;
    this.isMoving = true;
    this.currentPath = path.slice();
    this.activeMoveDestination = { x: destX, y: destY, mapId, activateWarpId };
    if (initialDirection) {
      this.currentDirection = initialDirection;
    }

    if (!wasMoving) {
      this.moveToNextTile();
    }
  }

  requestMoveTo(
    destX: number,
    destY: number,
    mapId: number = this.currentMapId,
    inputSource: "click" | "keyboard" = "click",
    activateWarpId?: number,
  ): boolean {
    if (this.inputFrozenChecker() || this.facingAbort || this.serverMovementInProgress) return false;
    if (this.playerId === null || !this.mapRenderer) return false;

    const path = this.findPath(this.currentTileX, this.currentTileY, destX, destY);
    if (path.length > 0) {
      this.queuePredictedPathMove(
        destX,
        destY,
        mapId,
        inputSource,
        path,
        undefined,
        activateWarpId,
      );
      return true;
    }

    this.queueClientMove(destX, destY, mapId, inputSource, activateWarpId);
    return true;
  }

  requestPathToTile(
    destX: number,
    destY: number,
    onReach: () => void,
    inputSource: "click" | "keyboard" = "click",
  ): boolean {
    if (this.inputFrozenChecker() || this.facingAbort || this.serverMovementInProgress) return false;
    if (this.playerId === null || !this.mapRenderer) return false;
    if (!this.isWalkable(destX, destY)) return false;

    if (this.currentTileX === destX && this.currentTileY === destY) {
      onReach();
      return true;
    }

    const path = this.findPath(this.currentTileX, this.currentTileY, destX, destY);
    if (path.length === 0) return false;

    this.queuePredictedPathMove(
      destX,
      destY,
      this.currentMapId,
      inputSource,
      path,
    );
    this.setArrivalCallback((arrivedX, arrivedY) => {
      if (arrivedX !== destX || arrivedY !== destY) return false;
      if (!this.inputFrozenChecker()) {
        onReach();
      }
      return true;
    });
    return true;
  }

  private canActivateWarpWithDirection(
    warp: PhaserWarp,
    direction: MovementDirection,
  ): boolean {
    const warpDirection = warp.warpDirection?.trim().toUpperCase();
    return !warpDirection || warpDirection === direction;
  }

  private isCurrentTileDirectionalWarp(warp: PhaserWarp): boolean {
    const warpType = warp.warpType?.trim().toLowerCase();
    return warpType === "carpet" || warpType === "directional";
  }

  private isPairedWarpTileStep(
    targetX: number,
    targetY: number,
    direction: MovementDirection,
  ): boolean {
    if (
      Math.abs(targetX - this.currentTileX) +
        Math.abs(targetY - this.currentTileY) !==
      1
    ) {
      return false;
    }

    const currentWarp = this.warpAtProvider(
      this.currentTileX,
      this.currentTileY,
    );
    const targetWarp = this.warpAtProvider(targetX, targetY);
    if (!currentWarp || !targetWarp) return false;

    const currentDirection = currentWarp.warpDirection?.trim().toUpperCase();
    const targetDirection = targetWarp.warpDirection?.trim().toUpperCase();
    if (
      (currentDirection && direction === currentDirection) ||
      (targetDirection && direction === targetDirection)
    ) {
      return false;
    }
    if (currentWarp.sourceMapId !== targetWarp.sourceMapId) {
      return false;
    }
    if (currentWarp.destinationMapId !== targetWarp.destinationMapId) {
      return false;
    }
    if (
      currentWarp.destinationX == null ||
      currentWarp.destinationY == null ||
      targetWarp.destinationX == null ||
      targetWarp.destinationY == null
    ) {
      return false;
    }

    const sourceDx = targetX - this.currentTileX;
    const sourceDy = targetY - this.currentTileY;
    const destinationDx = targetWarp.destinationX - currentWarp.destinationX;
    const destinationDy = targetWarp.destinationY - currentWarp.destinationY;
    const sameDestination = destinationDx === 0 && destinationDy === 0;
    const parallelDestination =
      destinationDx === sourceDx && destinationDy === sourceDy;
    return sameDestination || parallelDestination;
  }

  private requestWarpActivationFromCurrentTile(
    warp: PhaserWarp,
    direction: MovementDirection,
  ): boolean {
    if (this.playerId === null || !this.mapRenderer) return false;

    this.currentDirection = direction;
    const movementController = this.mapRenderer.getMovementController();
    if (movementController) {
      movementController.handleDirectionUpdate(this.playerId, direction);
    }

    const now = Date.now();
    const requestKey = [
      this.currentMapId,
      this.currentTileX,
      this.currentTileY,
      direction,
      warp.id,
    ].join(":");
    if (
      requestKey === this.lastWarpActivationRequestKey &&
      now - this.lastWarpActivationRequestAt < WARP_ACTIVATION_REQUEST_COOLDOWN_MS
    ) {
      return true;
    }
    this.lastWarpActivationRequestKey = requestKey;
    this.lastWarpActivationRequestAt = now;

    this.warpActivator(warp, direction);
    return true;
  }

  /**
   * Build collision map from tiles data
   */
  buildCollisionMap(tiles: PhaserTile[]): void {
    debugPlayerMovement(
      `[PlayerMovement] Building collision map with ${tiles.length} tiles...`,
    );
    this.collisionMap.clear();
    this.rawFootTileMap.clear();
    this.talkOverTileMap.clear();
    this.sourceMapByTile.clear();
    let walkableCount = 0;
    for (const tile of tiles) {
      this.upsertCollisionTile(tile);
      if (this.isCollisionWalkable(tile.collisionType)) walkableCount++;
    }
    debugPlayerMovement(
      `[PlayerMovement] Built collision map. Total: ${this.collisionMap.size}, Walkable: ${walkableCount}`,
    );
    this.updateSurfingStateForTile(this.currentTileX, this.currentTileY);
    this.updateTravelMapForTile(this.currentTileX, this.currentTileY);
  }

  addCollisionTiles(tiles: readonly PhaserTile[]): void {
    for (const tile of tiles) {
      this.upsertCollisionTile(tile);
    }
  }

  removeCollisionTiles(tiles: readonly Pick<PhaserTile, "x" | "y">[]): void {
    for (const tile of tiles) {
      this.removeCollisionTile(tile.x, tile.y);
    }
  }

  private removeCollisionTile(x: number, y: number): void {
    const key = `${x},${y}`;
    this.collisionMap.delete(key);
    this.rawFootTileMap.delete(key);
    this.talkOverTileMap.delete(key);
    this.sourceMapByTile.delete(key);
  }

  private upsertCollisionTile(tile: PhaserTile): void {
    const key = `${tile.x},${tile.y}`;
    this.collisionMap.set(key, tile.collisionType);
    const sourceMapId = tile.sourceMapId ?? tile.mapId;
    if (Number.isFinite(sourceMapId)) {
      this.sourceMapByTile.set(key, {
        id: sourceMapId,
        name: tile.sourceMapName ?? null,
      });
    }
    if (tile.rawFootTileId != null) {
      this.rawFootTileMap.set(key, tile.rawFootTileId);
    } else {
      this.rawFootTileMap.delete(key);
    }
    if (tile.talkOverTile) {
      this.talkOverTileMap.set(key, true);
    } else {
      this.talkOverTileMap.delete(key);
    }
  }

  /**
   * Update a single tile's walkability in the collision map.
   * Called when the tile editor places, erases, or modifies tiles.
   * collisionType follows the same convention as PhaserTile: >0 = walkable, 0 = blocked.
   * Pass remove=true to delete the entry entirely (tile erased).
   */
  updateCollisionTile(
    x: number,
    y: number,
    collisionType: number,
    remove: boolean = false,
    rawFootTileId?: number,
    talkOverTile: boolean = false,
  ): void {
    const key = `${x},${y}`;
    if (remove) {
      this.removeCollisionTile(x, y);
    } else {
      this.collisionMap.set(key, collisionType);
      if (rawFootTileId != null) {
        this.rawFootTileMap.set(key, rawFootTileId);
      } else {
        this.rawFootTileMap.delete(key);
      }
      if (talkOverTile) {
        this.talkOverTileMap.set(key, true);
      } else {
        this.talkOverTileMap.delete(key);
      }
    }
  }

  /**
   * Check if a tile is walkable
   */
  isWalkable(x: number, y: number): boolean {
    return (
      this.isCollisionWalkable(this.collisionMap.get(`${x},${y}`)) &&
      !this.isActorBlocked(x, y)
    );
  }

  setBlockingActors(actors: PhaserActor[]): void {
    this.actorBlockers.clear();
    for (const actor of actors) {
      this.updateBlockingActor(actor);
    }
  }

  updateBlockingActor(actor: PhaserActor): void {
    if (actor.objectType === "npc" && actor.x != null && actor.y != null) {
      this.actorBlockers.set(actor.id, { x: actor.x, y: actor.y });
      return;
    }
    this.actorBlockers.delete(actor.id);
  }

  removeBlockingActor(actorId: number): void {
    this.actorBlockers.delete(actorId);
  }

  private isActorBlocked(x: number, y: number): boolean {
    for (const blocker of this.actorBlockers.values()) {
      if (blocker.x === x && blocker.y === y) {
        return true;
      }
    }
    return false;
  }

  private isCollisionWalkable(collisionType: number | undefined): boolean {
    return (
      collisionType === COLLISION_LAND ||
      (this.isSurfing && collisionType === COLLISION_WATER)
    );
  }

  private isWaterTile(x: number, y: number): boolean {
    return (
      this.collisionMap.get(`${x},${y}`) === COLLISION_WATER &&
      this.warpAtProvider(x, y) === null &&
      !NON_SURF_WARP_MAT_RAW_FOOT_TILE_IDS.has(
        this.rawFootTileMap.get(`${x},${y}`) ?? -1,
      )
    );
  }

  private isCuttableTile(x: number, y: number): boolean {
    return this.rawFootTileMap.get(`${x},${y}`) === CUT_TREE_RAW_FOOT_TILE_ID;
  }

  private updateSurfingStateForTile(x: number, y: number): void {
    if (this.isWaterTile(x, y)) {
      this.setSurfingActive(true);
      return;
    }
    const collisionType = this.collisionMap.get(`${x},${y}`);
    if (
      collisionType === COLLISION_LAND ||
      (collisionType === COLLISION_WATER && !this.isWaterTile(x, y))
    ) {
      this.setSurfingActive(false);
    }
  }

  private updateTravelMapForTile(x: number, y: number): void {
    const key = `${x},${y}`;
    const sourceMap = this.sourceMapByTile.get(key);
    if (sourceMap) {
      useAudioActivityStore
        .getState()
        .setTravelMap(sourceMap.id, sourceMap.name);
      return;
    }
    if (this.currentMapId > 0) {
      useAudioActivityStore.getState().setTravelMap(this.currentMapId, null);
    }
  }

  private setSurfingActive(active: boolean): void {
    const changed = this.isSurfing !== active;
    if (active) {
      if (!this.isSurfing && this.playerId !== null && this.mapRenderer) {
        const currentSprite = this.mapRenderer.getActorSpriteName?.(this.playerId);
        if (currentSprite && currentSprite !== SURF_PLAYER_SPRITE) {
          this.preSurfSpriteName = currentSprite;
        }
      }
      this.isSurfing = true;
      if (changed) {
        useAudioActivityStore.getState().setSurfing(true);
      }
      this.updatePlayerSprite(SURF_PLAYER_SPRITE);
      return;
    }

    if (!this.isSurfing && this.preSurfSpriteName === null) {
      return;
    }
    this.isSurfing = false;
    if (changed) {
      useAudioActivityStore.getState().setSurfing(false);
    }
    const restoreSprite = this.preSurfSpriteName || this.defaultPlayerSpriteName();
    this.preSurfSpriteName = null;
    this.updatePlayerSprite(restoreSprite);
  }

  private updatePlayerSprite(spriteName: string): void {
    if (this.playerId === null || !this.mapRenderer) return;
    this.mapRenderer.updateActorSpriteName?.(
      this.playerId,
      spriteName,
      this.currentDirection,
    );
  }

  private defaultPlayerSpriteName(): string {
    const gender = Number(
      usePlayerCharacterStore.getState().characterProfile?.gender ?? 0,
    );
    if (gender === 1) return "SPRITE_BEAUTY";
    if (gender === 2) return "SPRITE_BLUENB";
    return "SPRITE_BLUE";
  }

  private partyKnowsSurf(): boolean | null {
    const { party, isLoaded } = usePokemonPartyStore.getState();
    if (!isLoaded) return null;
    return party.some((pokemon) =>
      pokemon.moves?.some(
        (move) =>
          move?.id === SURF_MOVE_ID ||
          move?.name?.trim().toUpperCase() === "SURF",
      ),
    );
  }

  private promptSurfToWater(
    waterX: number,
    waterY: number,
    direction: MovementDirection,
  ): boolean {
    if (this.inputFrozenChecker()) return false;
    const knowsSurf = this.partyKnowsSurf();
    if (knowsSurf === false) {
      usePokemonDialogueStore
        .getState()
        .openDialogue(["No POKEMON knows that move."]);
      return false;
    }

    usePokemonDialogueStore.getState().showChoice(
      "The water is calm. Want to SURF?",
      (yes) => {
        if (!yes) return;
        PhaserNet.requestSurf(waterX, waterY, this.currentMapId, direction);
      },
    );
    return true;
  }

  private findSurfEntryForWaterTarget(
    targetX: number,
    targetY: number,
  ): {
    shore: { x: number; y: number };
    water: { x: number; y: number };
    direction: MovementDirection;
    pathLength: number;
  } | null {
    if (!this.isWaterTile(targetX, targetY)) return null;

    const directions: Array<{
      direction: MovementDirection;
      dx: number;
      dy: number;
    }> = [
      { direction: "UP", dx: 0, dy: -1 },
      { direction: "DOWN", dx: 0, dy: 1 },
      { direction: "LEFT", dx: -1, dy: 0 },
      { direction: "RIGHT", dx: 1, dy: 0 },
    ];
    const opposite: Record<MovementDirection, MovementDirection> = {
      UP: "DOWN",
      DOWN: "UP",
      LEFT: "RIGHT",
      RIGHT: "LEFT",
    };

    const queue = [{ x: targetX, y: targetY }];
    const visited = new Set<string>([`${targetX},${targetY}`]);
    let best:
      | {
          shore: { x: number; y: number };
          water: { x: number; y: number };
          direction: MovementDirection;
          pathLength: number;
        }
      | null = null;

    for (let i = 0; i < queue.length && i < WATER_SEARCH_LIMIT; i++) {
      const water = queue[i];

      for (const dir of directions) {
        const shore = { x: water.x + dir.dx, y: water.y + dir.dy };
        if (this.collisionMap.get(`${shore.x},${shore.y}`) !== COLLISION_LAND) {
          continue;
        }

        let pathLength = 0;
        if (shore.x !== this.currentTileX || shore.y !== this.currentTileY) {
          const path = this.findPath(
            this.currentTileX,
            this.currentTileY,
            shore.x,
            shore.y,
          );
          if (path.length === 0) continue;
          pathLength = path.length;
        }

        if (!best || pathLength < best.pathLength) {
          best = {
            shore,
            water,
            direction: opposite[dir.direction],
            pathLength,
          };
        }
      }

      for (const dir of directions) {
        const next = { x: water.x + dir.dx, y: water.y + dir.dy };
        const key = `${next.x},${next.y}`;
        if (visited.has(key) || !this.isWaterTile(next.x, next.y)) {
          continue;
        }
        visited.add(key);
        queue.push(next);
      }
    }

    return best;
  }

  private requestSurfPathToWater(targetX: number, targetY: number): boolean {
    if (this.inputFrozenChecker()) return false;
    if (this.playerId === null || !this.mapRenderer) return false;

    const entry = this.findSurfEntryForWaterTarget(targetX, targetY);
    if (!entry) {
      return false;
    }

    if (
      entry.shore.x === this.currentTileX &&
      entry.shore.y === this.currentTileY
    ) {
      this.currentDirection = entry.direction;
      this.faceDirection(entry.direction);
      return this.promptSurfToWater(
        entry.water.x,
        entry.water.y,
        entry.direction,
      );
    }

    this.queueClientMove(
      entry.shore.x,
      entry.shore.y,
      this.currentMapId,
      "click",
    );
    this.setArrivalCallback((arrivedX, arrivedY) => {
      if (arrivedX !== entry.shore.x || arrivedY !== entry.shore.y) {
        return false;
      }
      if (!this.inputFrozenChecker()) {
        this.currentDirection = entry.direction;
        this.faceDirection(entry.direction);
        this.promptSurfToWater(entry.water.x, entry.water.y, entry.direction);
      }
      return true;
    });
    return true;
  }

  private requestCutAtTile(
    targetX: number,
    targetY: number,
    direction: MovementDirection,
  ): boolean {
    if (this.inputFrozenChecker()) return false;
    PhaserNet.requestFieldMoveUse(
      "CUT",
      targetX,
      targetY,
      this.currentMapId,
      direction,
    );
    return true;
  }

  private requestCutPathToTile(targetX: number, targetY: number): boolean {
    if (this.inputFrozenChecker()) return false;
    if (this.playerId === null || !this.mapRenderer) return false;
    if (!this.isCuttableTile(targetX, targetY)) return false;

    if (this.canInteractWithTile(targetX, targetY)) {
      const direction = this.directionToInteractionTile(targetX, targetY);
      if (!direction) return false;
      this.currentDirection = direction;
      this.faceDirection(direction);
      return this.requestCutAtTile(targetX, targetY, direction);
    }

    const walkTarget = this.findReachableInteractionTile(targetX, targetY);
    if (!walkTarget) {
      return false;
    }

    this.queueClientMove(walkTarget.x, walkTarget.y, this.currentMapId, "click");
    this.setArrivalCallback(() => {
      const direction = this.directionToInteractionTile(targetX, targetY);
      if (!direction) {
        return false;
      }
      if (!this.inputFrozenChecker()) {
        this.currentDirection = direction;
        this.faceDirection(direction);
        this.requestCutAtTile(targetX, targetY, direction);
      }
      return true;
    });
    return true;
  }

  handleFieldMoveInteractionInFront(): boolean {
    if (this.inputFrozenChecker() || this.facingAbort || this.serverMovementInProgress) return false;
    if (this.playerId === null || !this.mapRenderer) return false;
    if (this.isMoving) return false;

    const direction = this.normalizeDirection(this.currentDirection);
    if (!direction) return false;
    const delta = this.directionDelta(direction);
    const targetX = this.currentTileX + delta.dx;
    const targetY = this.currentTileY + delta.dy;

    if (!this.isSurfing && this.isWaterTile(targetX, targetY)) {
      this.faceDirection(direction);
      return this.promptSurfToWater(targetX, targetY, direction);
    }

    if (this.isCuttableTile(targetX, targetY)) {
      this.faceDirection(direction);
      return this.requestCutAtTile(targetX, targetY, direction);
    }

    return false;
  }

  private normalizeDirection(direction: string): MovementDirection | null {
    switch (direction.toUpperCase()) {
      case "UP":
      case "DOWN":
      case "LEFT":
      case "RIGHT":
        return direction.toUpperCase() as MovementDirection;
      default:
        return null;
    }
  }

  private requestFacing(direction: MovementDirection): void {
    if (this.playerId === null || this.stepAbort || this.serverMovementInProgress) return;
    const characterId = usePlayerCharacterStore.getState().characterProfile.id;
    const request = { mapId: this.currentMapId, fromX: this.currentTileX, fromY: this.currentTileY, direction };
    const key = [request.mapId, request.fromX, request.fromY, direction].join(":");
    if (this.facingAbort && key === this.facingRequestKey) return;
    this.facingAbort?.abort();
    const abort = new AbortController();
    this.facingAbort = abort;
    this.facingRequestKey = key;
    const current = () => !abort.signal.aborted && !this.fieldCommandsRetired
      && usePlayerCharacterStore.getState().characterProfile.id === characterId;
    void requestPlayerFacing(request, abort.signal).then((result) => {
      if (!current()) return;
      if (result.serverMovementPending && result.mapId === this.currentMapId && result.x === this.currentTileX && result.y === this.currentTileY) {
        // Boulder updates can arrive before the first committed path point.
        // Preserve that server-owned phase instead of issuing an ordinary step
        // into the now visually unoccupied tile during this interval.
        this.beginServerMovement(false);
      }
    }).catch(async (error: unknown) => {
      if (!current()) return;
      if (!(error instanceof CorrelatedResponseError)) {
        // A turn can commit a boulder push and its route. Unknown transport
        // outcomes require a current read, never a facing retry or error pose.
        this.movementRecoveryRequired = true;
        try {
          await this.actorReconciler(abort.signal);
          if (!current()) return;
          const snapshot = await readCurrentGameplayState(abort.signal, () => this.movementGeneration);
          if (!current()) return;
          applyGameplayResourceSnapshot(snapshot);
          this.projectOwnedPosition(snapshot.position);
        } catch {
          if (current()) console.warn("[PlayerMovement] Facing recovery unavailable; reconnect before moving.");
        }
        return;
      }
      // Explicit rejection does not confirm a movement commit. Keep its
      // presentation directional and source-matching.
      if (error instanceof CorrelatedResponseError && !this.isMoving) {
        const owned = error.response as PlayerStepError;
        if (owned.mapId === this.currentMapId && owned.x === this.currentTileX && owned.y === this.currentTileY) {
          this.syncDirection(owned.direction);
          if (this.playerId !== null) this.mapRenderer?.getMovementController()?.handleDirectionUpdate(this.playerId, owned.direction);
        }
      }
      console.warn("[PlayerMovement] Facing was not accepted:", error);
    }).finally(() => {
      if (this.facingAbort === abort) {
        this.facingAbort = null;
        this.facingRequestKey = null;
      }
    });
  }

  private faceDirection(direction: MovementDirection): void {
    const movementController = this.mapRenderer?.getMovementController();
    if (movementController && this.playerId !== null) {
      movementController.handleDirectionUpdate(this.playerId, direction);
    }
    this.requestFacing(direction);
    this.emitPlayerPositionChanged();
  }

  private directionToAdjacentTile(
    targetX: number,
    targetY: number,
  ): MovementDirection | null {
    const dx = targetX - this.currentTileX;
    const dy = targetY - this.currentTileY;
    if (dx === 0 && dy === -1) return "UP";
    if (dx === 0 && dy === 1) return "DOWN";
    if (dx === -1 && dy === 0) return "LEFT";
    if (dx === 1 && dy === 0) return "RIGHT";
    return null;
  }

  private directionDelta(direction: MovementDirection): { dx: number; dy: number } {
    if (direction === "UP") return { dx: 0, dy: -1 };
    if (direction === "DOWN") return { dx: 0, dy: 1 };
    if (direction === "LEFT") return { dx: -1, dy: 0 };
    return { dx: 1, dy: 0 };
  }

  private ledgeLandingFromCurrent(
    direction: MovementDirection,
    ledgeX: number,
    ledgeY: number,
  ): { x: number; y: number } | null {
    return this.ledgeLandingFromTile(
      this.currentTileX,
      this.currentTileY,
      direction,
      ledgeX,
      ledgeY,
    );
  }

  private ledgeLandingFromTile(
    standingX: number,
    standingY: number,
    direction: MovementDirection,
    ledgeX: number,
    ledgeY: number,
  ): { x: number; y: number } | null {
    if (this.currentMapId !== UNIFIED_OVERWORLD_MAP_ID) {
      return null;
    }

    const { dx, dy } = this.directionDelta(direction);
    if (ledgeX !== standingX + dx || ledgeY !== standingY + dy) {
      return null;
    }
    if (
      !canJumpLedge(
        direction,
        this.rawFootTileMap.get(`${standingX},${standingY}`),
        this.rawFootTileMap.get(`${ledgeX},${ledgeY}`),
      )
    ) {
      return null;
    }

    const landingX = standingX + 2 * dx;
    const landingY = standingY + 2 * dy;
    if (!this.isWalkable(landingX, landingY)) {
      return null;
    }
    return { x: landingX, y: landingY };
  }

  private queueLedgeJump(
    landing: { x: number; y: number },
    direction: MovementDirection,
    inputSource: "click" | "keyboard",
  ): void {
    this.queuePredictedPathMove(
      landing.x,
      landing.y,
      this.currentMapId,
      inputSource,
      [{ ...landing, ledgeJump: true }],
      direction,
    );
  }

  isAdjacentToTile(targetX: number, targetY: number): boolean {
    return (
      Math.abs(targetX - this.currentTileX) +
        Math.abs(targetY - this.currentTileY) ===
      1
    );
  }

  isTalkOverTile(x: number, y: number): boolean {
    return this.talkOverTileMap.get(`${x},${y}`) === true;
  }

  canInteractWithTile(targetX: number, targetY: number): boolean {
    return this.directionToInteractionTile(targetX, targetY) !== null;
  }

  faceInteractionTarget(targetX: number, targetY: number): boolean {
    const direction = this.directionToInteractionTile(targetX, targetY);
    if (!direction) return false;

    this.currentDirection = direction;
    this.faceDirection(direction);
    return true;
  }

  faceTile(targetX: number, targetY: number): boolean {
    const direction = this.directionToAdjacentTile(targetX, targetY);
    if (!direction) return false;

    this.currentDirection = direction;
    this.faceDirection(direction);
    return true;
  }

  private directionToInteractionTile(
    targetX: number,
    targetY: number,
  ): MovementDirection | null {
    const adjacentDirection = this.directionToAdjacentTile(targetX, targetY);
    if (adjacentDirection) return adjacentDirection;

    const dx = targetX - this.currentTileX;
    const dy = targetY - this.currentTileY;
    let direction: MovementDirection | null = null;
    if (dx === 0 && dy === -2) direction = "UP";
    if (dx === 0 && dy === 2) direction = "DOWN";
    if (dx === -2 && dy === 0) direction = "LEFT";
    if (dx === 2 && dy === 0) direction = "RIGHT";
    if (!direction) return null;

    const step = this.directionDelta(direction);
    const middleX = this.currentTileX + step.dx;
    const middleY = this.currentTileY + step.dy;
    return this.isTalkOverTile(middleX, middleY) ? direction : null;
  }

  private findReachableInteractionTile(
    targetX: number,
    targetY: number,
  ): { x: number; y: number; pathLength: number } | null {
    const directions = [
      { dx: 0, dy: 1 },
      { dx: 0, dy: -1 },
      { dx: -1, dy: 0 },
      { dx: 1, dy: 0 },
    ];
    const candidates: { x: number; y: number }[] = [];

    for (const dir of directions) {
      candidates.push({ x: targetX + dir.dx, y: targetY + dir.dy });

      const counterX = targetX + dir.dx;
      const counterY = targetY + dir.dy;
      if (this.isTalkOverTile(counterX, counterY)) {
        candidates.push({
          x: targetX + 2 * dir.dx,
          y: targetY + 2 * dir.dy,
        });
      }
    }

    let best: { x: number; y: number; pathLength: number } | null = null;
    for (const candidate of candidates) {
      if (!this.isWalkable(candidate.x, candidate.y)) continue;

      if (
        candidate.x === this.currentTileX &&
        candidate.y === this.currentTileY
      ) {
        return { ...candidate, pathLength: 0 };
      }

      const path = this.findPath(
        this.currentTileX,
        this.currentTileY,
        candidate.x,
        candidate.y,
      );
      if (path.length === 0) continue;
      if (!best || path.length < best.pathLength) {
        best = { ...candidate, pathLength: path.length };
      }
    }
    return best;
  }

  private queuePathToInteractionTile(
    walkTarget: { x: number; y: number },
  ): boolean {
    if (
      walkTarget.x === this.currentTileX &&
      walkTarget.y === this.currentTileY
    ) {
      return false;
    }

    const path = this.findPath(
      this.currentTileX,
      this.currentTileY,
      walkTarget.x,
      walkTarget.y,
    );
    if (path.length === 0) {
      return false;
    }

    this.queuePredictedPathMove(
      walkTarget.x,
      walkTarget.y,
      this.currentMapId,
      "click",
      path,
    );
    return true;
  }

  requestInteractionPath(
    targetX: number,
    targetY: number,
    onReach: () => void,
  ): boolean {
    if (this.inputFrozenChecker()) return false;
    if (this.playerId === null || !this.mapRenderer) return false;

    if (this.canInteractWithTile(targetX, targetY)) {
      this.faceInteractionTarget(targetX, targetY);
      onReach();
      return true;
    }

    const walkTarget = this.findReachableInteractionTile(targetX, targetY);
    if (!walkTarget) {
      console.warn(
        `[PlayerMovement] No reachable interaction tile near (${targetX}, ${targetY})`,
      );
      return false;
    }
    if (!this.queuePathToInteractionTile(walkTarget)) {
      console.warn(
        `[PlayerMovement] No path to interaction tile near (${targetX}, ${targetY})`,
      );
      return false;
    }

    this.setArrivalCallback(() => {
      if (
        !this.canInteractWithTile(targetX, targetY)
      ) {
        return false;
      }
      if (!this.inputFrozenChecker()) {
        this.faceInteractionTarget(targetX, targetY);
        onReach();
      }
      return true;
    });
    return true;
  }

  requestInteractionPathToMovingTarget(
    getTarget: () => InteractionTarget | null,
    onReach: () => void,
    attempt = 0,
  ): boolean {
    if (this.inputFrozenChecker()) return false;
    if (this.playerId === null || !this.mapRenderer) return false;

    const target = getTarget();
    if (!target) return false;

    if (this.canInteractWithTile(target.x, target.y)) {
      this.faceInteractionTarget(target.x, target.y);
      onReach();
      return true;
    }

    const walkTarget = this.findReachableInteractionTile(target.x, target.y);
    if (!walkTarget) {
      console.warn(
        `[PlayerMovement] No reachable interaction tile near moving target (${target.x}, ${target.y})`,
      );
      return false;
    }
    if (!this.queuePathToInteractionTile(walkTarget)) {
      console.warn(
        `[PlayerMovement] No path to interaction tile near moving target (${target.x}, ${target.y})`,
      );
      return false;
    }

    this.setArrivalCallback(() => {
      const latestTarget = getTarget();
      if (!latestTarget) {
        return true;
      }

      if (
        this.canInteractWithTile(latestTarget.x, latestTarget.y)
      ) {
        if (!this.inputFrozenChecker()) {
          this.faceInteractionTarget(latestTarget.x, latestTarget.y);
          onReach();
        }
        return true;
      }

      const nextWalkTarget = this.findReachableInteractionTile(
        latestTarget.x,
        latestTarget.y,
      );
      const canRetry =
        attempt < 2 &&
        nextWalkTarget !== null &&
        (nextWalkTarget.x !== this.currentTileX ||
          nextWalkTarget.y !== this.currentTileY);

      if (canRetry && !this.inputFrozenChecker()) {
        window.setTimeout(() => {
          if (!this.isMoving) {
            this.requestInteractionPathToMovingTarget(
              getTarget,
              onReach,
              attempt + 1,
            );
          }
        }, 0);
      }
      return true;
    });
    return true;
  }

  /**
   * Find a walkable tile adjacent to the given position.
   * Checks the 4 cardinal directions first, then diagonals.
   * Returns the walkable tile closest to the player, or null if none found.
   */
  findWalkableNearTile(
    targetX: number,
    targetY: number,
  ): { x: number; y: number } | null {
    // If the target itself is walkable, return it directly
    if (this.isWalkable(targetX, targetY)) {
      return { x: targetX, y: targetY };
    }

    // Check cardinal directions first (preferred), then diagonals
    const directions = [
      { dx: 0, dy: 1 }, // below (most common for doors)
      { dx: 0, dy: -1 }, // above
      { dx: -1, dy: 0 }, // left
      { dx: 1, dy: 0 }, // right
      { dx: -1, dy: 1 }, // below-left
      { dx: 1, dy: 1 }, // below-right
      { dx: -1, dy: -1 }, // above-left
      { dx: 1, dy: -1 }, // above-right
    ];

    // Find all walkable adjacent tiles
    const walkable: { x: number; y: number; dist: number }[] = [];
    for (const dir of directions) {
      const nx = targetX + dir.dx;
      const ny = targetY + dir.dy;
      if (this.isWalkable(nx, ny)) {
        // Manhattan distance from player to this candidate
        const dist =
          Math.abs(nx - this.currentTileX) + Math.abs(ny - this.currentTileY);
        walkable.push({ x: nx, y: ny, dist });
      }
    }

    if (walkable.length === 0) return null;

    // Return the closest walkable adjacent tile to the player
    walkable.sort((a, b) => a.dist - b.dist);
    return { x: walkable[0].x, y: walkable[0].y };
  }

  /**
   * Handle click on a tile using client-side pathing.
   */
  handleTileClick(worldX: number, worldY: number): void {
    // A new click cannot replace a server-issued step while its animation or
    // completion is outstanding. Scene snaps retire it explicitly instead.
    if (this.stepAbort || this.facingAbort || this.serverMovementInProgress || this.inputFrozenChecker()) {
      return;
    }

    debugPlayerMovement(
      `[PlayerMovement] handleTileClick at world (${worldX.toFixed(1)}, ${worldY.toFixed(1)})`,
    );
    if (this.playerId === null || !this.mapRenderer) {
      console.warn(
        `[PlayerMovement] Cannot move: playerId=${this.playerId}, hasRenderer=${!!this.mapRenderer}`,
      );
      return;
    }

    // Convert world coordinates to tile coordinates
    const targetTileX = Math.floor(worldX / TILE_SIZE);
    const targetTileY = Math.floor(worldY / TILE_SIZE);

    const key = `${targetTileX},${targetTileY}`;
    const collisionType = this.collisionMap.get(key);
    const walkable = this.isCollisionWalkable(collisionType);
    debugPlayerMovement(
      `[PlayerMovement] Target tile: (${targetTileX}, ${targetTileY}), key: "${key}", collision value: ${collisionType}, walkable: ${walkable}, exists: ${this.collisionMap.has(key)}`,
    );
    debugPlayerMovement(
      `[PlayerMovement] Current player pos: (${this.currentTileX}, ${this.currentTileY})`,
    );

    if (!this.isSurfing && this.isWaterTile(targetTileX, targetTileY)) {
      if (this.requestSurfPathToWater(targetTileX, targetTileY)) {
        return;
      }
    }

    if (this.isCuttableTile(targetTileX, targetTileY)) {
      if (this.requestCutPathToTile(targetTileX, targetTileY)) {
        return;
      }
    }

    const targetWarp = this.warpAtProvider(targetTileX, targetTileY);
    if (
      targetWarp &&
      targetTileX === this.currentTileX &&
      targetTileY === this.currentTileY
    ) {
      const direction =
        this.normalizeDirection(targetWarp.warpDirection ?? "") ??
        this.normalizeDirection(this.currentDirection) ??
        "DOWN";
      this.requestWarpActivationFromCurrentTile(targetWarp, direction);
      return;
    }

    // Check if target is walkable
    if (!walkable) {
      debugPlayerMovement(
        `[PlayerMovement] Target tile is NOT walkable. (Map size: ${this.collisionMap.size})`,
      );
      const direction = this.directionToAdjacentTile(targetTileX, targetTileY);
      const ledgeLanding =
        direction != null
          ? this.ledgeLandingFromCurrent(direction, targetTileX, targetTileY)
          : null;
      if (direction && ledgeLanding) {
        this.currentDirection = direction;
        debugPlayerMovement(
          `[PlayerMovement] Moving over ledge to (${ledgeLanding.x}, ${ledgeLanding.y})`,
        );
        this.queueLedgeJump(ledgeLanding, direction, "click");
        return;
      }
      const currentWarp = this.warpAtProvider(
        this.currentTileX,
        this.currentTileY,
      );
      if (
        direction &&
        currentWarp &&
        (this.isCurrentTileDirectionalWarp(currentWarp) ||
          !this.collisionMap.has(key)) &&
        this.canActivateWarpWithDirection(currentWarp, direction)
      ) {
        this.requestWarpActivationFromCurrentTile(currentWarp, direction);
        return;
      }
      if (
        direction &&
        targetWarp &&
        this.canActivateWarpWithDirection(targetWarp, direction)
      ) {
        this.requestWarpActivationFromCurrentTile(targetWarp, direction);
        return;
      }
      if (direction) {
        this.currentDirection = direction;
        this.faceDirection(direction);
        this.scene.events.emit(
          "playerFacedDirection",
          direction,
          this.currentTileX,
          this.currentTileY,
        );
      }
      return;
    }

    debugPlayerMovement(
      `[PlayerMovement] Starting client path on map ${this.currentMapId}: destination (${targetTileX}, ${targetTileY})`,
    );
    const path = this.findPath(
      this.currentTileX,
      this.currentTileY,
      targetTileX,
      targetTileY,
    );
    if (path.length === 0) {
      this.queueClientMove(
        targetTileX,
        targetTileY,
        this.currentMapId,
        "click",
        targetWarp?.id,
      );
      return;
    }
    this.queuePredictedPathMove(
      targetTileX,
      targetTileY,
      this.currentMapId,
      "click",
      path,
      undefined,
      targetWarp?.id,
    );
  }

  /**
   * Callback fired by ActorMovementController when a step completes
   */
  onStepComplete(
    actorId: number,
    x: number,
    y: number,
    completedDirection: string,
    kind: "step" | "snap" | "serverStep" = "step",
  ): void {
    void this.finishVisualStep(actorId, x, y, completedDirection, kind).catch((error: unknown) => this.handleStepFailure(error));
  }

  private async finishVisualStep(actorId: number, x: number, y: number, completedDirection: string, kind: "step" | "snap" | "serverStep"): Promise<void> {
    if (this.playerId === actorId) {
      // ActorMovementController derives visual facing from the completed tile
      // delta; the cache can still contain the spawn default.
      this.currentDirection = completedDirection;
      this.currentTileX = x;
      this.currentTileY = y;
      this.updateSurfingStateForTile(x, y);
      this.updateTravelMapForTile(x, y);
      this.emitPlayerPositionChanged();
      // A snap projects a server position; it is not a new completed move.
      // Keep local context fresh without echoing a write or activating a warp.
      if (kind === "snap") {
        this.stopMovement(true);
        return;
      }
      if (kind === "serverStep") {
        // The server has already committed this path point. Projection never
        // acknowledges through the legacy coordinate writer or local warp path.
        if (this.serverPathFinished) this.stopMovement(true);
        return;
      }
      const issued = this.issuedStep;
      if (issued) {
        const abort = this.stepAbort;
        const generation = this.movementGeneration;
        if (!abort || issued.x !== x || issued.y !== y || issued.mapId !== this.currentMapId) {
          throw new Error("Movement animation disagrees with the issued step");
        }
        const result = await completePlayerStep(issued.stepToken, abort.signal);
        if (generation !== this.movementGeneration || abort.signal.aborted) return;
        if (result.replayed) {
          // A receipt is historical. Read the current owned location before
          // projecting recovery, and discard future path/arrival callbacks.
          let owned;
          try { owned = await readOwnedPlayerPosition(abort.signal, issued.stepToken); }
          catch (error) {
            if (generation !== this.movementGeneration || abort.signal.aborted) return;
            this.movementRecoveryRequired = true;
            this.currentPath = [];
            this.arrivalCallback = null;
            throw error;
          }
          if (generation !== this.movementGeneration || abort.signal.aborted) return;
          this.stopMovement(true);
          if (owned.mapId === this.currentMapId) {
            this.syncPosition(owned.x, owned.y);
            this.syncDirection(owned.direction);
            this.mapRenderer?.snapActorPosition(actorId, owned.x, owned.y, owned.direction);
            if (owned.serverMovementPending) this.beginServerMovement(false);
          } else {
            window.dispatchEvent(new CustomEvent("warpTileTeleport", { detail: { ...owned, serverCommitted: true, sfxAlreadyPlayed: true } }));
          }
          return;
        }
        this.issuedStep = null;
        this.stepAbort = null;
      } else {
        // Only an issued step may trigger acknowledgement and local arrival
        // effects. Server animations must explicitly use serverStep.
        throw new Error("Movement animation completed without an issued step");
      }

      // Finish the current visual step, then discard any queued user path as
      // soon as a panel, dialogue, battle, shop, or modal takes input focus.
      // Cutscenes are excluded because their scripted movement must continue.
      if (isWorldInputFrozen({ includeCutscene: false })) {
        this.stopMovement();
        return;
      }

      const requestedWarpId = this.activeMoveDestination?.activateWarpId;
      const reachedMoveDestination =
        this.activeMoveDestination !== null &&
        this.activeMoveDestination.x === x &&
        this.activeMoveDestination.y === y &&
        this.activeMoveDestination.mapId === this.currentMapId;

      if (reachedMoveDestination && requestedWarpId != null) {
        const requestedWarp = this.warpAtProvider(x, y);
        if (requestedWarp?.id === requestedWarpId) {
          const warpDirection =
            this.normalizeDirection(requestedWarp.warpDirection ?? "") ??
            this.normalizeDirection(this.currentDirection) ??
            "DOWN";
          this.currentPath = [];
          this.isMoving = false;
          this.activeMoveDestination = null;
          this.requestWarpActivationFromCurrentTile(
            requestedWarp,
            warpDirection,
          );
          return;
        }
      }

      // Emit step event so WarpManager can detect warp arrivals. Click pathing
      // gets the final-destination flag so intermediate exit tiles are ignored.
      this.scene.events.emit(
        "playerSteppedOnTile",
        x,
        y,
        this.lastInputSource,
        this.currentDirection,
        reachedMoveDestination,
      );

      // Check arrival callback after the visual tween completes
      if (this.arrivalCallback) {
        const arrived = this.arrivalCallback(x, y);
        if (arrived) {
          this.arrivalCallback = null;
        }
      }

      if (reachedMoveDestination) {
        this.activeMoveDestination = null;
      }

      if (this.isMoving) {
        this.moveToNextTile();
      }

      if (!this.isMoving) {
        this.continueHeldKeyboardMove();
      }
    }
  }

  private continueHeldKeyboardMove(): void {
    if (this.lastInputSource !== "keyboard") return;
    if (this.inputFrozenChecker()) return;
    const heldDirection = this.heldKeyboardDirectionProvider();
    if (!heldDirection) return;
    const currentWarp = this.warpAtProvider(this.currentTileX, this.currentTileY);
    if (
      currentWarp &&
      this.isCurrentTileDirectionalWarp(currentWarp) &&
      this.canActivateWarpWithDirection(currentWarp, heldDirection)
    ) {
      return;
    }

    this.handleKeyboardMove(heldDirection);
  }

  /**
   * Propose next tile in current path
   */
  private moveToNextTile(): void {
    const mapRenderer = this.mapRenderer;
    const playerId = this.playerId;
    if (!mapRenderer || playerId === null) {
      this.isMoving = false;
      return;
    }
    if (this.currentPath.length === 0) {
      // console.log(`[PlayerMovement] Movement reached destination.`);
      this.isMoving = false;
      return;
    }

    const nextTile = this.currentPath.shift()!;
    debugPlayerMovement(
      `[PlayerMovement] Moving to next tile: (${nextTile.x}, ${nextTile.y})`,
    );

    // Determine direction
    let direction = this.currentDirection;
    if (nextTile.x > this.currentTileX) direction = "RIGHT";
    else if (nextTile.x < this.currentTileX) direction = "LEFT";
    else if (nextTile.y > this.currentTileY) direction = "DOWN";
    else if (nextTile.y < this.currentTileY) direction = "UP";

    const sourceX = this.currentTileX;
    const sourceY = this.currentTileY;
    const mapId = this.currentMapId;
    const generation = this.movementGeneration;
    const abort = new AbortController();
    this.stepAbort = abort;
    void requestPlayerStep({ mapId, fromX: sourceX, fromY: sourceY, direction }, abort.signal).then((step) => {
      if (abort.signal.aborted || generation !== this.movementGeneration) return;
      if (step.mapId !== mapId || step.x !== nextTile.x || step.y !== nextTile.y) {
        throw new Error("Movement destination disagrees with server collision data");
      }
      this.issuedStep = step;
      this.currentDirection = step.direction;
      mapRenderer.updateActorPosition(playerId, sourceX, sourceY, step.x, step.y, step.direction, undefined, step.ledgeJump ? { ledgeJump: true } : undefined);
    }).catch((error: unknown) => {
      if (generation === this.movementGeneration && !abort.signal.aborted) this.handleStepFailure(error);
    });
  }

  private handleStepFailure(error: unknown): void {
    if (error instanceof DOMException && error.name === "AbortError") return;
    if (this.movementRecoveryRequired) {
      console.error("[PlayerMovement] Could not recover movement; input remains locked until scene/session retirement", error);
      return;
    }
    this.stopMovement(true);
    if (error instanceof CorrelatedResponseError) {
      const owned = error.response as PlayerStepError;
      if (owned.mapId === this.currentMapId && this.playerId !== null) {
        this.syncPosition(owned.x, owned.y);
        this.syncDirection(owned.direction);
        this.mapRenderer?.snapActorPosition(this.playerId, owned.x, owned.y, owned.direction);
        if (owned.serverMovementPending) this.beginServerMovement(false);
      }
    }
    // A timeout does not prove rollback. Stop the path instead of animating or
    // replaying an unconfirmed result; reconnect/reload recovers owned position.
    console.error("[PlayerMovement] Movement request failed", error);
  }

  /**
   * A* pathfinding algorithm
   */
  private findPath(
    startX: number,
    startY: number,
    endX: number,
    endY: number,
  ): MovementPathStep[] {
    const openSet: MovementPathNode[] = [];
    const closedSet: Set<string> = new Set();

    const heuristic = (x: number, y: number) =>
      Math.abs(x - endX) + Math.abs(y - endY);

    openSet.push({
      x: startX,
      y: startY,
      g: 0,
      h: heuristic(startX, startY),
      f: heuristic(startX, startY),
      parent: null,
    });

    const directions: Array<{
      name: MovementDirection;
      dx: number;
      dy: number;
    }> = [
      { name: "UP", dx: 0, dy: -1 },
      { name: "DOWN", dx: 0, dy: 1 },
      { name: "LEFT", dx: -1, dy: 0 },
      { name: "RIGHT", dx: 1, dy: 0 },
    ];

    let iterations = 0;
    const maxIterations = 2000;

    while (openSet.length > 0 && iterations < maxIterations) {
      iterations++;

      openSet.sort((a, b) => a.f - b.f);
      const current = openSet.shift()!;

      if (current.x === endX && current.y === endY) {
        const path: MovementPathStep[] = [];
        let node = current;
        while (node.parent) {
          path.unshift({
            x: node.x,
            y: node.y,
            ...(node.ledgeJump ? { ledgeJump: true } : {}),
          });
          node = node.parent;
        }
        return path;
      }

      closedSet.add(`${current.x},${current.y}`);

      for (const dir of directions) {
        const nx = current.x + dir.dx;
        const ny = current.y + dir.dy;
        const key = `${nx},${ny}`;

        if (closedSet.has(key)) continue;
        let next: MovementPathStep = { x: nx, y: ny };
        let nextKey = key;
        if (!this.isWalkable(nx, ny)) {
          const ledgeLanding = this.ledgeLandingFromTile(
            current.x,
            current.y,
            dir.name,
            nx,
            ny,
          );
          if (!ledgeLanding) continue;
          next = { ...ledgeLanding, ledgeJump: true };
          nextKey = `${next.x},${next.y}`;
          if (closedSet.has(nextKey)) continue;
        }

        const g = current.g + 1;
        const h = heuristic(next.x, next.y);
        const f = g + h;

        const existing = openSet.find((n) => n.x === next.x && n.y === next.y);
        if (existing) {
          if (g < existing.g) {
            existing.g = g;
            existing.f = f;
            existing.parent = current;
            existing.ledgeJump = next.ledgeJump;
          }
        } else {
          openSet.push({
            x: next.x,
            y: next.y,
            ledgeJump: next.ledgeJump,
            g,
            h,
            f,
            parent: current,
          });
        }
      }
    }

    return [];
  }

  // Track how the last movement was initiated (for carpet warp behavior)
  private lastInputSource: "click" | "keyboard" = "click";

  getLastInputSource(): "click" | "keyboard" {
    return this.lastInputSource;
  }

  /**
   * Handle keyboard movement (WASD / arrow keys).
   * Moves the player one tile in the given direction.
   * Returns true if the move was initiated, false if blocked.
   */
  handleKeyboardMove(direction: "UP" | "DOWN" | "LEFT" | "RIGHT"): boolean {
    if (this.inputFrozenChecker() || this.facingAbort || this.serverMovementInProgress) return false;
    if (this.playerId === null || !this.mapRenderer) return false;
    if (this.isMoving) return false;

    const dx = direction === "LEFT" ? -1 : direction === "RIGHT" ? 1 : 0;
    const dy = direction === "UP" ? -1 : direction === "DOWN" ? 1 : 0;
    const targetX = this.currentTileX + dx;
    const targetY = this.currentTileY + dy;
    const targetKey = `${targetX},${targetY}`;
    const targetWarp = this.warpAtProvider(targetX, targetY);

    // Update facing direction even if we can't move
    this.currentDirection = direction;

    const currentWarp = this.warpAtProvider(
      this.currentTileX,
      this.currentTileY,
    );
    if (
      currentWarp &&
      this.isCurrentTileDirectionalWarp(currentWarp) &&
      this.canActivateWarpWithDirection(currentWarp, direction)
    ) {
      return this.requestWarpActivationFromCurrentTile(currentWarp, direction);
    }

    if (
      targetWarp &&
      !this.isCurrentTileDirectionalWarp(targetWarp) &&
      !this.isPairedWarpTileStep(targetX, targetY, direction) &&
      this.canActivateWarpWithDirection(targetWarp, direction)
    ) {
      return this.requestWarpActivationFromCurrentTile(targetWarp, direction);
    }

    if (!this.isWalkable(targetX, targetY)) {
      if (
        currentWarp &&
        (this.isCurrentTileDirectionalWarp(currentWarp) ||
          !this.collisionMap.has(targetKey)) &&
        this.canActivateWarpWithDirection(currentWarp, direction)
      ) {
        return this.requestWarpActivationFromCurrentTile(currentWarp, direction);
      }

      if (
        targetWarp &&
        this.isCurrentTileDirectionalWarp(targetWarp) &&
        this.canActivateWarpWithDirection(targetWarp, direction)
      ) {
        this.queuePredictedKeyboardMove(
          targetX,
          targetY,
          this.currentMapId,
          direction,
          targetWarp.id,
        );
        return true;
      }

      if (
        targetWarp &&
        this.isCurrentTileDirectionalWarp(targetWarp) &&
        !this.canActivateWarpWithDirection(targetWarp, direction)
      ) {
        this.queuePredictedKeyboardMove(
          targetX,
          targetY,
          this.currentMapId,
          direction,
        );
        return true;
      }

      if (this.isPairedWarpTileStep(targetX, targetY, direction)) {
        this.queuePredictedKeyboardMove(
          targetX,
          targetY,
          this.currentMapId,
          direction,
        );
        return true;
      }

      if (this.warpTileChecker(targetX, targetY)) {
        this.faceDirection(direction);
        return false;
      }

      if (!this.isSurfing && this.isWaterTile(targetX, targetY)) {
        this.faceDirection(direction);
        return false;
      }

      const ledgeLanding = this.ledgeLandingFromCurrent(
        direction,
        targetX,
        targetY,
      );
      if (ledgeLanding) {
        this.queueLedgeJump(ledgeLanding, direction, "keyboard");
        return true;
      }

      // Publish the same facing context used by click and source interactions.
      this.faceDirection(direction);
      // Emit event so warp manager can check
      this.scene.events.emit(
        "playerFacedDirection",
        direction,
        this.currentTileX,
        this.currentTileY,
      );
      return false;
    }

    this.queuePredictedKeyboardMove(
      targetX,
      targetY,
      this.currentMapId,
      direction,
    );
    return true;
  }

  beginServerMovement(pathFinished: boolean): void {
    this.stopMovement(true);
    this.serverMovementInProgress = true;
    this.serverPathFinished = pathFinished;
    this.isMoving = true;
  }

  stopMovement(retireIssued = false): void {
    if (retireIssued) {
      // Retiring prediction or receiving an actor snap does not prove current
      // command/resources recovery. A failed read keeps this owner locked; a
      // replacement scene creates a fresh owner after loading owned state.
      this.serverMovementInProgress = false;
      this.serverPathFinished = true;
      this.facingAbort?.abort();
      this.facingAbort = null;
      this.facingRequestKey = null;
    }
    // Input focus discards the future path but lets the current issued animation
    // finish and acknowledge through its issued token.
    // Scene retirement and authoritative snaps explicitly retire that animation.
    if (retireIssued || !this.issuedStep) {
      this.movementGeneration++;
      this.stepAbort?.abort();
      this.stepAbort = null;
      this.issuedStep = null;
    }
    this.currentPath = [];
    // Keep the current issued animation/completion exclusive until its response.
    this.isMoving = this.issuedStep !== null || this.serverMovementInProgress;
    this.arrivalCallback = null;
    this.activeMoveDestination = null;
  }

  getIsMoving(): boolean {
    return this.isMoving;
  }

  getIsSurfing(): boolean {
    return this.isSurfing;
  }

  getCurrentPosition(): { x: number; y: number } {
    return { x: this.currentTileX, y: this.currentTileY };
  }

  getCurrentMapId(): number {
    return this.currentMapId;
  }

  getCurrentDirection(): string {
    return this.currentDirection;
  }

  syncPosition(x: number, y: number): void {
    this.currentTileX = x;
    this.currentTileY = y;
    this.updateSurfingStateForTile(x, y);
    this.updateTravelMapForTile(x, y);
    this.emitPlayerPositionChanged();
  }

  syncMapId(mapId: number): void {
    if (this.currentMapId !== mapId) {
      this.activeMoveDestination = null;
      this.setSurfingActive(false);
    }
    this.currentMapId = mapId;
    this.updateTravelMapForTile(this.currentTileX, this.currentTileY);
    this.emitPlayerPositionChanged();
  }

  syncDirection(direction: string): void {
    this.currentDirection = direction;
    this.emitPlayerPositionChanged();
  }

  applySurfingSuccess(
    x: number,
    y: number,
    mapId: number,
    direction?: string,
  ): void {
    if (this.playerId === null || !this.mapRenderer) return;

    const normalizedDirection =
      direction != null ? this.normalizeDirection(direction) : null;
    if (normalizedDirection) {
      this.currentDirection = normalizedDirection;
    }

    // Surf success follows a committed MovePlayerTo result. Retire prediction
    // and project it through the same path as other server movement.
    this.beginServerMovement(true);
    this.currentMapId = mapId;
    this.setSurfingActive(true);
    this.updateTravelMapForTile(x, y);

    if (this.currentTileX === x && this.currentTileY === y) {
      this.stopMovement(true);
      this.emitPlayerPositionChanged();
      return;
    }

    const oldX = this.currentTileX;
    const oldY = this.currentTileY;
    const isAdjacent = Math.abs(x - oldX) + Math.abs(y - oldY) === 1;
    if (isAdjacent) {
      this.isMoving = true;
      this.mapRenderer.updateActorPosition(
        this.playerId,
        oldX,
        oldY,
        x,
        y,
        this.currentDirection,
        undefined,
        { serverControlled: true },
      );
      return;
    }

    this.stopMovement(true);
    this.currentTileX = x;
    this.currentTileY = y;
    this.mapRenderer.snapActorPosition(
      this.playerId,
      x,
      y,
      this.currentDirection,
    );
    this.updateSurfingStateForTile(x, y);
    this.updateTravelMapForTile(x, y);
    this.emitPlayerPositionChanged();
  }

  /**
   * Set a persistent callback checked on each visual step completion (tween finish).
   * The callback should return true when the player has arrived (to consume it),
   * or false to keep checking. Used by WarpManager for warp pathing.
   */
  setArrivalCallback(
    callback: ((x: number, y: number) => boolean) | null,
  ): void {
    this.arrivalCallback = callback;
  }

  /**
   * Project a step toward an already committed warp destination. The native
   * entrance tile is animation metadata, never a new position command.
   * Returns a promise that resolves when the step animation completes.
   */
  animateCommittedStepToward(targetX: number, targetY: number, overrideDirection?: string): Promise<void> {
    return new Promise<void>((resolve) => {
      if (this.playerId === null || !this.mapRenderer) {
        resolve();
        return;
      }

      // Use override direction if provided, otherwise calculate from position
      let direction = overrideDirection || this.currentDirection;
      if (!overrideDirection) {
        if (targetX > this.currentTileX) direction = "RIGHT";
        else if (targetX < this.currentTileX) direction = "LEFT";
        else if (targetY > this.currentTileY) direction = "DOWN";
        else if (targetY < this.currentTileY) direction = "UP";
      }

      this.currentDirection = direction;

      // Calculate one step toward the target
      let stepX = this.currentTileX;
      let stepY = this.currentTileY;
      if (targetX > this.currentTileX) stepX++;
      else if (targetX < this.currentTileX) stepX--;
      else if (targetY > this.currentTileY) stepY++;
      else if (targetY < this.currentTileY) stepY--;

      // If already on the target, resolve immediately
      if (stepX === this.currentTileX && stepY === this.currentTileY) {
        debugPlayerMovement(
          `[PlayerMovement] animateCommittedStepToward: already at target, resolving`,
        );
        resolve();
        return;
      }

      // Get the player sprite directly from the map renderer
      const sprite = this.mapRenderer.getActorSprite(this.playerId);
      if (!sprite) {
        debugPlayerMovement(
          `[PlayerMovement] animateCommittedStepToward: no sprite found, resolving`,
        );
        resolve();
        return;
      }

      debugPlayerMovement(
        `[PlayerMovement] animateCommittedStepToward: tweening from (${this.currentTileX},${this.currentTileY}) to (${stepX},${stepY}), direction=${direction}`,
      );

      this.beginServerMovement(true);
      const generation = this.movementGeneration;

      // Safety timeout — if the tween doesn't complete in 1s, resolve anyway
      let resolved = false;
      const timeout = setTimeout(() => {
        if (!resolved) {
          console.warn(
            `[PlayerMovement] animateCommittedStepToward: tween timed out, resolving`,
          );
          resolved = true;
          if (generation === this.movementGeneration) {
            this.syncPosition(stepX, stepY);
            this.mapRenderer?.snapActorPosition(this.playerId!, stepX, stepY, direction);
            this.stopMovement(true);
          }
          resolve();
        }
      }, 1000);

      const movementController = this.mapRenderer.getMovementController();
      if (movementController.getActorState(this.playerId)) {
        this.mapRenderer.updateActorPosition(
          this.playerId,
          this.currentTileX,
          this.currentTileY,
          stepX,
          stepY,
          direction,
          undefined,
          { serverControlled: true },
        );
        void this.mapRenderer.waitForActorIdle(this.playerId).then(() => {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            debugPlayerMovement(
              `[PlayerMovement] animateCommittedStepToward: tween complete at (${stepX},${stepY})`,
            );
            if (generation === this.movementGeneration) {
              this.syncPosition(stepX, stepY);
              this.stopMovement(true);
            }
            resolve();
          }
        });
        return;
      }

      const destPixelX = stepX * TILE_SIZE + TILE_SIZE / 2;
      const destPixelY = stepY * TILE_SIZE + TILE_SIZE / 2;

      this.scene.tweens.add({
        targets: sprite,
        x: destPixelX,
        y: destPixelY,
        duration: 300,
        ease: "Linear",
        onComplete: () => {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            debugPlayerMovement(
              `[PlayerMovement] animateCommittedStepToward: fallback tween complete at (${stepX},${stepY})`,
            );
            if (generation === this.movementGeneration) {
              this.syncPosition(stepX, stepY);
              this.stopMovement(true);
            }
            resolve();
          }
        },
      });
    });
  }

  clear(): void {
    this.retireFieldCommands();
    this.stopMovement(true);
    this.arrivalCallback = null;
    this.collisionMap.clear();
    this.rawFootTileMap.clear();
    this.talkOverTileMap.clear();
    this.sourceMapByTile.clear();
    this.actorBlockers.clear();
    this.isSurfing = false;
    useAudioActivityStore.getState().setSurfing(false);
    this.preSurfSpriteName = null;
    this.playerId = null;
    this.mapRenderer = null;
    useGameStatusStore.getState().clearPlayerTileContext();
  }
}
