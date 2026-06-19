import { Scene } from "phaser";
import { PhaserActor, PhaserTile, PhaserWarp } from "@/net/generated/world_api";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import * as PhaserNet from "../services/PhaserNetworkService";
import { isWorldInputFrozen } from "../utils/worldInputGuard";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import { emitCaptureQuestTestEvent } from "@/testing/capturequestTestBridge";

type MovementDirection = "UP" | "DOWN" | "LEFT" | "RIGHT";

interface InteractionTarget {
  x: number;
  y: number;
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
  private mapRenderer: any = null;

  // Collision map: key = "x,y", value = collision type (0 blocked, 1 land, 2 water)
  private collisionMap: Map<string, number> = new Map();
  private rawFootTileMap: Map<string, number> = new Map();
  private talkOverTileMap: Map<string, boolean> = new Map();
  private sourceMapByTile: Map<string, { id: number; name: string | null }> =
    new Map();
  private actorBlockers: Map<number, { x: number; y: number }> = new Map();
  private isSurfing: boolean = false;
  private preSurfSpriteName: string | null = null;

  // Current path (array of {x, y} tile coordinates)
  private currentPath: { x: number; y: number }[] = [];
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
  private inputFrozenChecker: () => boolean = () => isWorldInputFrozen();
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
  }

  setInputFrozenChecker(checker: () => boolean): void {
    this.inputFrozenChecker = checker;
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
    mapRenderer: any,
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
    path: { x: number; y: number }[],
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
    if (this.inputFrozenChecker()) return false;
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
    if (this.inputFrozenChecker()) return false;
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
      }
      if (tile.talkOverTile) {
        this.talkOverTileMap.set(key, true);
      }
      if (this.isCollisionWalkable(tile.collisionType)) walkableCount++;
    }
    debugPlayerMovement(
      `[PlayerMovement] Built collision map. Total: ${this.collisionMap.size}, Walkable: ${walkableCount}`,
    );
    this.updateSurfingStateForTile(this.currentTileX, this.currentTileY);
    this.updateTravelMapForTile(this.currentTileX, this.currentTileY);
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
      this.collisionMap.delete(key);
      this.rawFootTileMap.delete(key);
      this.talkOverTileMap.delete(key);
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
    this.setArrivalCallback((_arrivedX, _arrivedY) => {
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
    if (this.inputFrozenChecker()) return false;
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

  private faceDirection(direction: MovementDirection): void {
    const movementController = this.mapRenderer?.getMovementController();
    if (movementController && this.playerId !== null) {
      movementController.handleDirectionUpdate(this.playerId, direction);
    }
    PhaserNet.sendDirectionUpdate(
      this.currentTileX,
      this.currentTileY,
      this.currentMapId,
      direction,
    );
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
    if (this.currentMapId !== UNIFIED_OVERWORLD_MAP_ID) {
      return null;
    }

    const { dx, dy } = this.directionDelta(direction);
    if (ledgeX !== this.currentTileX + dx || ledgeY !== this.currentTileY + dy) {
      return null;
    }
    if (
      !canJumpLedge(
        direction,
        this.rawFootTileMap.get(`${this.currentTileX},${this.currentTileY}`),
        this.rawFootTileMap.get(`${ledgeX},${ledgeY}`),
      )
    ) {
      return null;
    }

    const landingX = this.currentTileX + 2 * dx;
    const landingY = this.currentTileY + 2 * dy;
    if (!this.isWalkable(landingX, landingY)) {
      return null;
    }
    return { x: landingX, y: landingY };
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
    const movementController = this.mapRenderer?.getMovementController();
    if (movementController && this.playerId !== null) {
      movementController.handleDirectionUpdate(this.playerId, direction);
    }
    PhaserNet.sendDirectionUpdate(
      this.currentTileX,
      this.currentTileY,
      this.currentMapId,
      direction,
    );
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

    this.setArrivalCallback((_arrivedX, _arrivedY) => {
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

    this.setArrivalCallback((_arrivedX, _arrivedY) => {
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
    if (this.inputFrozenChecker()) {
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
        this.queueClientMove(
          ledgeLanding.x,
          ledgeLanding.y,
          this.currentMapId,
          "click",
        );
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
        const movementController = this.mapRenderer.getMovementController();
        if (movementController && this.playerId !== null) {
          movementController.handleDirectionUpdate(this.playerId, direction);
        }
        PhaserNet.sendDirectionUpdate(
          this.currentTileX,
          this.currentTileY,
          this.currentMapId,
          direction,
        );
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
  onStepComplete(actorId: number, x: number, y: number): void {
    if (this.playerId === actorId) {
      this.currentTileX = x;
      this.currentTileY = y;
      this.updateSurfingStateForTile(x, y);
      this.updateTravelMapForTile(x, y);
      this.emitPlayerPositionChanged();
      PhaserNet.sendPlayerPosition(
        x,
        y,
        this.currentMapId,
        this.currentDirection,
      );
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

    this.currentDirection = direction;

    // Trigger local movement update (so we see ourselves move immediately)
    // This adds to the ActorMovementController queue via MapRenderer
    this.mapRenderer.updateActorPosition(
      this.playerId!,
      this.currentTileX,
      this.currentTileY,
      nextTile.x,
      nextTile.y,
      direction,
    );
  }

  /**
   * A* pathfinding algorithm
   */
  private findPath(
    startX: number,
    startY: number,
    endX: number,
    endY: number,
  ): { x: number; y: number }[] {
    const openSet: {
      x: number;
      y: number;
      g: number;
      h: number;
      f: number;
      parent: any;
    }[] = [];
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

    const directions = [
      { dx: 0, dy: -1 }, // UP
      { dx: 0, dy: 1 }, // DOWN
      { dx: -1, dy: 0 }, // LEFT
      { dx: 1, dy: 0 }, // RIGHT
    ];

    let iterations = 0;
    const maxIterations = 2000;

    while (openSet.length > 0 && iterations < maxIterations) {
      iterations++;

      openSet.sort((a, b) => a.f - b.f);
      const current = openSet.shift()!;

      if (current.x === endX && current.y === endY) {
        const path: { x: number; y: number }[] = [];
        let node = current;
        while (node.parent) {
          path.unshift({ x: node.x, y: node.y });
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
        if (!this.isWalkable(nx, ny)) continue;

        const g = current.g + 1;
        const h = heuristic(nx, ny);
        const f = g + h;

        const existing = openSet.find((n) => n.x === nx && n.y === ny);
        if (existing) {
          if (g < existing.g) {
            existing.g = g;
            existing.f = f;
            existing.parent = current;
          }
        } else {
          openSet.push({ x: nx, y: ny, g, h, f, parent: current });
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
    if (this.inputFrozenChecker()) return false;
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
        this.queueClientMove(
          ledgeLanding.x,
          ledgeLanding.y,
          this.currentMapId,
          "keyboard",
        );
        return true;
      }

      // Visually turn the sprite to face the blocked direction
      const movementController = this.mapRenderer.getMovementController();
      if (movementController && this.playerId !== null) {
        movementController.handleDirectionUpdate(this.playerId, direction);
      }
      // Notify server so other players see the turn
      PhaserNet.sendDirectionUpdate(
        this.currentTileX,
        this.currentTileY,
        this.currentMapId,
        direction,
      );
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

  stopMovement(): void {
    this.currentPath = [];
    this.isMoving = false;
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

    this.currentPath = [];
    this.activeMoveDestination = { x, y, mapId };
    this.currentMapId = mapId;
    this.setSurfingActive(true);
    this.updateTravelMapForTile(x, y);

    if (this.currentTileX === x && this.currentTileY === y) {
      this.isMoving = false;
      this.activeMoveDestination = null;
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
      );
      return;
    }

    this.isMoving = false;
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
   * Animate a visual-only step toward a target tile, bypassing collision checks
   * and server updates. Used by WarpManager to show the player stepping into
   * a door before the warp transition.
   * Returns a promise that resolves when the step animation completes.
   */
  animateStepToward(targetX: number, targetY: number, overrideDirection?: string): Promise<void> {
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
          `[PlayerMovement] animateStepToward: already at target, resolving`,
        );
        resolve();
        return;
      }

      // Get the player sprite directly from the map renderer
      const sprite = this.mapRenderer.getActorSprite(this.playerId);
      if (!sprite) {
        debugPlayerMovement(
          `[PlayerMovement] animateStepToward: no sprite found, resolving`,
        );
        resolve();
        return;
      }

      debugPlayerMovement(
        `[PlayerMovement] animateStepToward: tweening from (${this.currentTileX},${this.currentTileY}) to (${stepX},${stepY}), direction=${direction}`,
      );

      // Stop any existing movement
      this.stopMovement();

      // Safety timeout — if the tween doesn't complete in 1s, resolve anyway
      let resolved = false;
      const timeout = setTimeout(() => {
        if (!resolved) {
          console.warn(
            `[PlayerMovement] animateStepToward: tween timed out, resolving`,
          );
          resolved = true;
          this.currentTileX = stepX;
          this.currentTileY = stepY;
          this.emitPlayerPositionChanged();
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
        );
        void this.mapRenderer.waitForActorIdle(this.playerId).then(() => {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            debugPlayerMovement(
              `[PlayerMovement] animateStepToward: tween complete at (${stepX},${stepY})`,
            );
            this.currentTileX = stepX;
            this.currentTileY = stepY;
            this.emitPlayerPositionChanged();
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
              `[PlayerMovement] animateStepToward: fallback tween complete at (${stepX},${stepY})`,
            );
            this.currentTileX = stepX;
            this.currentTileY = stepY;
            this.emitPlayerPositionChanged();
            resolve();
          }
        },
      });
    });
  }

  clear(): void {
    this.stopMovement();
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
