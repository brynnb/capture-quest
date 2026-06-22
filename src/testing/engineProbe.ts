import type {
  PhaserActor,
  PhaserTile,
  PhaserWarp,
} from "@/net/generated/world_api";

export type EngineProbeDirection = "UP" | "DOWN" | "LEFT" | "RIGHT";
export type EngineProbePressKey = EngineProbeDirection | "SPACE";
export type EngineProbeDialogueChoice = "yes" | "no";
export type EngineProbeBattleAction =
  | "fight"
  | "item"
  | "switch"
  | "run"
  | "safari-ball"
  | "bait"
  | "rock";

export const ENGINE_PROBE_EVENT = {
  actorInteracted: "actorInteracted",
  battleActionSelected: "battleActionSelected",
  battleMoveSelected: "battleMoveSelected",
  boulderMoved: "boulderMoved",
  dialogueChoiceSelected: "dialogueChoiceSelected",
  directionChanged: "directionChanged",
  engineProbeCommand: "engineProbeCommand",
  fieldMoveUsed: "fieldMoveUsed",
  itemUsed: "itemUsed",
  movementBlocked: "movementBlocked",
  movementCompleted: "movementCompleted",
  movementStarted: "movementStarted",
  pathFound: "pathFound",
  pathNotFound: "pathNotFound",
  scenarioJumped: "scenarioJumped",
  spacePressed: "spacePressed",
  waitForIdle: "waitForIdle",
  warpActivated: "warpActivated",
} as const;

export interface EngineProbeSnapshotOptions {
  bounds?: {
    mode?: "radius";
    radius?: number;
    center?: "player" | { x: number; y: number };
  };
}

export type EngineProbeCommand =
  | { press: EngineProbePressKey }
  | { clickTile: { x: number; y: number } }
  | { face: EngineProbeDirection }
  | { scenario: { name: string } }
  | { dialogueChoice: { choice: EngineProbeDialogueChoice } }
  | { battleAction: { action: EngineProbeBattleAction } }
  | { battleMove: { index: number } }
  | { useItem: { itemId?: number; itemName?: string; testId?: string } }
  | { interactActor: { actorId?: number; name?: string; text?: string } }
  | { activateWarp: { x: number; y: number } }
  | {
      useFieldMove: {
        moveName: string;
        targetX?: number;
        targetY?: number;
      };
    }
  | { warpTo: { mapId: number; x: number; y: number; direction?: string } }
  | { waitForIdle: true };

export interface EngineProbeEvent {
  type: string;
  at: number;
  detail?: unknown;
}

export interface EngineProbeActorSnapshot {
  id: number;
  internalId: number | null;
  name: string | null;
  text: string | null;
  type: string;
  spriteName: string | null;
  mapId: number | null;
  x: number | null;
  y: number | null;
  direction: string | null;
  blocksMovement: boolean;
}

export interface EngineProbeWarpSnapshot {
  id: number;
  x: number;
  y: number;
  sourceMapId: number;
  destinationMapId: number | null;
  destinationMap: string | null;
  destinationX: number | null;
  destinationY: number | null;
  warpType: string;
  warpDirection: string | null;
}

export interface EngineProbeTileSnapshot {
  x: number;
  y: number;
  tileImageId: number;
  collisionType: number;
  rawFootTileId: number | null;
  talkOverTile: boolean;
  walkable: boolean;
  isWater: boolean;
  isCutTree: boolean;
  isWarp: boolean;
  occupiedByActorId: number | null;
}

export interface EngineProbeWorldInputSnapshot {
  frozen: boolean;
  reason: string | null;
}

export interface EngineProbeDialogueSnapshot {
  isOpen: boolean;
  text: string;
  isTyping: boolean;
  speakerName: string | null;
  actorId: number | null;
  isChoicePending: boolean;
  choicePrompt: string | null;
  selectedChoice: EngineProbeDialogueChoice | null;
}

export interface EngineProbeBattleSnapshot {
  isOpen: boolean;
  phase: string;
  battleType: string | null;
  turnNumber: number;
  allowedActions: string[];
  guaranteedCatch: boolean;
  enemyPokemonId: number | null;
  enemyPokemonName: string | null;
  selectedAction: string | null;
  selectedMoveIndex: number | null;
  selectedItemIndex: number | null;
  selectedPokemonIndex: number | null;
}

export interface EngineProbeInventorySnapshot {
  money: number;
  shopOpen: boolean;
  shopName: string;
  items: Array<{
    instanceId: number;
    itemId: number;
    name: string;
    shortName: string;
    quantity: number;
  }>;
}

export interface EngineProbePokemonSnapshot {
  id?: number;
  speciesId?: number;
  name?: string;
  level?: number;
  hp?: number;
  maxHp?: number;
}

export interface EngineProbePCSnapshot {
  isOpen: boolean;
  currentBox: number;
  boxPokemon: unknown[];
}

export interface EngineProbeMessageSnapshot {
  text: string;
  type: string;
}

export interface EngineProbeSnapshot {
  map: {
    id: number | null;
    name: string | null;
    isLoading: boolean;
  };
  player: {
    id: number | null;
    internalId: number | null;
    name: string | null;
    x: number | null;
    y: number | null;
    direction: string | null;
    isMoving: boolean;
    isSurfing: boolean;
    isCycling: boolean;
  };
  worldInput: EngineProbeWorldInputSnapshot;
  bounds: {
    mode: "radius";
    radius: number;
    center: { x: number; y: number };
    minX: number;
    maxX: number;
    minY: number;
    maxY: number;
  };
  tiles: EngineProbeTileSnapshot[];
  actors: EngineProbeActorSnapshot[];
  warps: EngineProbeWarpSnapshot[];
  dialogue: EngineProbeDialogueSnapshot;
  battle: EngineProbeBattleSnapshot;
  inventory: EngineProbeInventorySnapshot;
  party: unknown[];
  pc: EngineProbePCSnapshot;
  messages: EngineProbeMessageSnapshot[];
  lastEvents: EngineProbeEvent[];
}

export interface EngineProbeResult {
  ok: boolean;
  command: EngineProbeCommand;
  before: EngineProbeSnapshot;
  after: EngineProbeSnapshot;
  events: EngineProbeEvent[];
  error?: string;
}

export interface EngineProbeSnapshotSource {
  map: EngineProbeSnapshot["map"];
  player: EngineProbeSnapshot["player"];
  tiles: PhaserTile[];
  actors: PhaserActor[];
  warps: PhaserWarp[];
  worldInput?: Partial<EngineProbeWorldInputSnapshot>;
  dialogue?: Partial<EngineProbeDialogueSnapshot>;
  battle?: Partial<EngineProbeBattleSnapshot>;
  inventory?: Partial<EngineProbeInventorySnapshot>;
  party?: unknown[];
  pc?: Partial<EngineProbePCSnapshot>;
  messages?: EngineProbeMessageSnapshot[];
  lastEvents?: EngineProbeEvent[];
}

export interface EngineProbeAdapter {
  getSnapshot(options?: EngineProbeSnapshotOptions): EngineProbeSnapshot;
  sendCommand(
    command: EngineProbeCommand,
    options?: EngineProbeSnapshotOptions,
  ): Promise<EngineProbeResult>;
  waitForIdle(options?: { timeoutMs?: number }): Promise<void>;
}

const DEFAULT_RADIUS = 8;
const MAX_RADIUS = 20;
const COLLISION_LAND = 1;
const COLLISION_WATER = 2;
const CUT_TREE_RAW_FOOT_TILE_ID = 0x3d;
const NON_SURF_WARP_MAT_RAW_FOOT_TILE_IDS = new Set([0x04]);

type Bounds = EngineProbeSnapshot["bounds"];

function clampRadius(radius: number | undefined): number {
  if (!Number.isFinite(radius)) return DEFAULT_RADIUS;
  return Math.max(0, Math.min(MAX_RADIUS, Math.floor(radius as number)));
}

function tileKey(x: number, y: number): string {
  return `${x},${y}`;
}

function normalizeDirection(direction: string | null | undefined): EngineProbeDirection | null {
  const normalized = direction?.trim().toUpperCase();
  if (
    normalized === "UP" ||
    normalized === "DOWN" ||
    normalized === "LEFT" ||
    normalized === "RIGHT"
  ) {
    return normalized;
  }
  return null;
}

function directionDelta(direction: EngineProbeDirection): { dx: number; dy: number } {
  switch (direction) {
    case "UP":
      return { dx: 0, dy: -1 };
    case "DOWN":
      return { dx: 0, dy: 1 };
    case "LEFT":
      return { dx: -1, dy: 0 };
    case "RIGHT":
      return { dx: 1, dy: 0 };
  }
}

function snapshotBounds(
  source: EngineProbeSnapshotSource,
  options?: EngineProbeSnapshotOptions,
): Bounds {
  const radius = clampRadius(options?.bounds?.radius);
  const requestedCenter = options?.bounds?.center ?? "player";
  const center =
    requestedCenter === "player"
      ? {
          x: source.player.x ?? 0,
          y: source.player.y ?? 0,
        }
      : requestedCenter;

  return {
    mode: "radius",
    radius,
    center,
    minX: center.x - radius,
    maxX: center.x + radius,
    minY: center.y - radius,
    maxY: center.y + radius,
  };
}

function inBounds(
  bounds: Bounds,
  point: { x?: number | null; y?: number | null },
): boolean {
  return (
    point.x != null &&
    point.y != null &&
    point.x >= bounds.minX &&
    point.x <= bounds.maxX &&
    point.y >= bounds.minY &&
    point.y <= bounds.maxY
  );
}

function actorBlocksMovement(actor: PhaserActor): boolean {
  return actor.objectType === "npc" && actor.x != null && actor.y != null;
}

function toActorSnapshot(actor: PhaserActor): EngineProbeActorSnapshot {
  return {
    id: actor.id,
    internalId: actor.internalId ?? null,
    name: actor.name ?? null,
    text: actor.text ?? null,
    type: actor.objectType,
    spriteName: actor.spriteName ?? null,
    mapId: actor.mapId ?? null,
    x: actor.x ?? null,
    y: actor.y ?? null,
    direction: actor.actionDirection ?? null,
    blocksMovement: actorBlocksMovement(actor),
  };
}

function toWarpSnapshot(warp: PhaserWarp): EngineProbeWarpSnapshot {
  return {
    id: warp.id,
    x: warp.x,
    y: warp.y,
    sourceMapId: warp.sourceMapId,
    destinationMapId: warp.destinationMapId ?? null,
    destinationMap: warp.destinationMap ?? null,
    destinationX: warp.destinationX ?? null,
    destinationY: warp.destinationY ?? null,
    warpType: warp.warpType,
    warpDirection: warp.warpDirection ?? null,
  };
}

function movementActorOccupancy(actors: PhaserActor[]): Map<string, number> {
  const occupied = new Map<string, number>();
  for (const actor of actors) {
    if (!actorBlocksMovement(actor) || actor.x == null || actor.y == null) {
      continue;
    }
    occupied.set(tileKey(actor.x, actor.y), actor.id);
  }
  return occupied;
}

function isSurfableWaterTile(tile: PhaserTile, hasWarp: boolean): boolean {
  return (
    tile.collisionType === COLLISION_WATER &&
    !hasWarp &&
    !NON_SURF_WARP_MAT_RAW_FOOT_TILE_IDS.has(tile.rawFootTileId ?? -1)
  );
}

function tileWalkable(
  tile: PhaserTile,
  playerSurfing: boolean,
  hasWarp: boolean,
  occupiedByActorId: number | null,
): boolean {
  if (occupiedByActorId != null) return false;
  if (tile.collisionType === COLLISION_LAND) return true;
  return playerSurfing && isSurfableWaterTile(tile, hasWarp);
}

function toTileSnapshot(
  tile: PhaserTile,
  warpsByTile: Map<string, PhaserWarp>,
  occupiedByTile: Map<string, number>,
  playerSurfing: boolean,
): EngineProbeTileSnapshot {
  const key = tileKey(tile.x, tile.y);
  const hasWarp = warpsByTile.has(key);
  const occupiedByActorId = occupiedByTile.get(key) ?? null;
  return {
    x: tile.x,
    y: tile.y,
    tileImageId: tile.tileImageId,
    collisionType: tile.collisionType,
    rawFootTileId: tile.rawFootTileId ?? null,
    talkOverTile: tile.talkOverTile,
    walkable: tileWalkable(tile, playerSurfing, hasWarp, occupiedByActorId),
    isWater: isSurfableWaterTile(tile, hasWarp),
    isCutTree: tile.rawFootTileId === CUT_TREE_RAW_FOOT_TILE_ID,
    isWarp: hasWarp,
    occupiedByActorId,
  };
}

function defaultWorldInput(): EngineProbeWorldInputSnapshot {
  return {
    frozen: false,
    reason: null,
  };
}

function defaultDialogue(): EngineProbeDialogueSnapshot {
  return {
    isOpen: false,
    text: "",
    isTyping: false,
    speakerName: null,
    actorId: null,
    isChoicePending: false,
    choicePrompt: null,
    selectedChoice: null,
  };
}

function defaultBattle(): EngineProbeBattleSnapshot {
  return {
    isOpen: false,
    phase: "none",
    battleType: null,
    turnNumber: 0,
    allowedActions: [],
    guaranteedCatch: false,
    enemyPokemonId: null,
    enemyPokemonName: null,
    selectedAction: null,
    selectedMoveIndex: null,
    selectedItemIndex: null,
    selectedPokemonIndex: null,
  };
}

function defaultInventory(): EngineProbeInventorySnapshot {
  return {
    money: 0,
    shopOpen: false,
    shopName: "",
    items: [],
  };
}

function defaultPC(): EngineProbePCSnapshot {
  return {
    isOpen: false,
    currentBox: 0,
    boxPokemon: [],
  };
}

export function createEngineProbeEvent(
  type: string,
  detail?: unknown,
  at = Date.now(),
): EngineProbeEvent {
  return { type, detail, at };
}

export function buildEngineProbeSnapshot(
  source: EngineProbeSnapshotSource,
  options?: EngineProbeSnapshotOptions,
): EngineProbeSnapshot {
  const bounds = snapshotBounds(source, options);
  const boundedActors = source.actors.filter((actor) => inBounds(bounds, actor));
  const boundedWarps = source.warps.filter((warp) => inBounds(bounds, warp));
  const warpsByTile = new Map(
    source.warps.map((warp) => [tileKey(warp.x, warp.y), warp] as const),
  );
  const occupiedByTile = movementActorOccupancy(source.actors);
  const boundedTiles = source.tiles
    .filter((tile) => inBounds(bounds, tile))
    .map((tile) =>
      toTileSnapshot(
        tile,
        warpsByTile,
        occupiedByTile,
        source.player.isSurfing,
      ),
    )
    .sort((a, b) => a.y - b.y || a.x - b.x);

  return {
    map: source.map,
    player: source.player,
    worldInput: {
      ...defaultWorldInput(),
      ...(source.worldInput ?? {}),
      reason: source.worldInput?.reason ?? null,
    },
    bounds,
    tiles: boundedTiles,
    actors: boundedActors.map(toActorSnapshot),
    warps: boundedWarps.map(toWarpSnapshot),
    dialogue: {
      ...defaultDialogue(),
      ...(source.dialogue ?? {}),
      speakerName: source.dialogue?.speakerName ?? null,
      actorId: source.dialogue?.actorId ?? null,
      choicePrompt: source.dialogue?.choicePrompt ?? null,
      selectedChoice: source.dialogue?.selectedChoice ?? null,
    },
    battle: {
      ...defaultBattle(),
      ...(source.battle ?? {}),
      battleType: source.battle?.battleType ?? null,
      enemyPokemonId: source.battle?.enemyPokemonId ?? null,
      enemyPokemonName: source.battle?.enemyPokemonName ?? null,
      selectedAction: source.battle?.selectedAction ?? null,
      selectedMoveIndex: source.battle?.selectedMoveIndex ?? null,
      selectedItemIndex: source.battle?.selectedItemIndex ?? null,
      selectedPokemonIndex: source.battle?.selectedPokemonIndex ?? null,
    },
    inventory: {
      ...defaultInventory(),
      ...(source.inventory ?? {}),
      items: source.inventory?.items ?? [],
    },
    party: source.party ?? [],
    pc: {
      ...defaultPC(),
      ...(source.pc ?? {}),
      boxPokemon: source.pc?.boxPokemon ?? [],
    },
    messages: source.messages ?? [],
    lastEvents: (source.lastEvents ?? []).slice(-50),
  };
}

export interface NodeEngineProbeState extends EngineProbeSnapshotSource {
  map: EngineProbeSnapshot["map"];
  player: EngineProbeSnapshot["player"];
  tiles: PhaserTile[];
  actors: PhaserActor[];
  warps: PhaserWarp[];
}

function cloneState(state: NodeEngineProbeState): NodeEngineProbeState {
  return {
    ...state,
    map: { ...state.map },
    player: { ...state.player },
    tiles: state.tiles.map((tile) => ({ ...tile })),
    actors: state.actors.map((actor) => ({ ...actor })),
    warps: state.warps.map((warp) => ({ ...warp })),
    lastEvents: [...(state.lastEvents ?? [])],
    messages: [...(state.messages ?? [])],
    party: [...(state.party ?? [])],
  };
}

function sourceTileMap(state: NodeEngineProbeState): Map<string, PhaserTile> {
  return new Map(state.tiles.map((tile) => [tileKey(tile.x, tile.y), tile]));
}

function sourceWarpMap(state: NodeEngineProbeState): Map<string, PhaserWarp> {
  return new Map(state.warps.map((warp) => [tileKey(warp.x, warp.y), warp]));
}

function nodeTileWalkable(state: NodeEngineProbeState, x: number, y: number): boolean {
  const tile = sourceTileMap(state).get(tileKey(x, y));
  if (!tile) return false;
  const hasWarp = sourceWarpMap(state).has(tileKey(x, y));
  const occupied = movementActorOccupancy(state.actors).get(tileKey(x, y)) ?? null;
  return tileWalkable(tile, state.player.isSurfing, hasWarp, occupied);
}

function findNodePath(
  state: NodeEngineProbeState,
  startX: number,
  startY: number,
  endX: number,
  endY: number,
): Array<{ x: number; y: number }> {
  const queue = [{ x: startX, y: startY, path: [] as Array<{ x: number; y: number }> }];
  const seen = new Set([tileKey(startX, startY)]);
  const directions: EngineProbeDirection[] = ["UP", "DOWN", "LEFT", "RIGHT"];

  while (queue.length > 0 && seen.size < 2000) {
    const current = queue.shift();
    if (!current) break;
    if (current.x === endX && current.y === endY) {
      return current.path;
    }

    for (const direction of directions) {
      const delta = directionDelta(direction);
      const next = { x: current.x + delta.dx, y: current.y + delta.dy };
      const key = tileKey(next.x, next.y);
      if (seen.has(key) || !nodeTileWalkable(state, next.x, next.y)) continue;
      seen.add(key);
      queue.push({ ...next, path: [...current.path, next] });
    }
  }

  return [];
}

function nodeEvent(
  state: NodeEngineProbeState,
  type: string,
  detail?: unknown,
): EngineProbeEvent {
  const event = createEngineProbeEvent(type, detail);
  state.lastEvents = [...(state.lastEvents ?? []), event].slice(-50);
  return event;
}

function applyNodeMovement(
  state: NodeEngineProbeState,
  x: number,
  y: number,
  direction: EngineProbeDirection,
): EngineProbeEvent[] {
  const events = [
    nodeEvent(state, ENGINE_PROBE_EVENT.movementStarted, {
      x: state.player.x,
      y: state.player.y,
      direction,
    }),
  ];
  state.player.x = x;
  state.player.y = y;
  state.player.direction = direction;
  state.player.isMoving = false;
  events.push(nodeEvent(state, ENGINE_PROBE_EVENT.movementCompleted, { x, y, direction }));
  return events;
}

function boulderAt(
  state: NodeEngineProbeState,
  x: number,
  y: number,
): PhaserActor | null {
  return (
    state.actors.find(
      (actor) =>
        actor.x === x &&
        actor.y === y &&
        actor.objectType === "npc" &&
        actor.spriteName === "SPRITE_BOULDER",
    ) ?? null
  );
}

function applyNodePress(
  state: NodeEngineProbeState,
  key: EngineProbePressKey,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  if (key === "SPACE") {
    return {
      ok: true,
      events: [nodeEvent(state, ENGINE_PROBE_EVENT.spacePressed)],
    };
  }

  const delta = directionDelta(key);
  const playerX = state.player.x ?? 0;
  const playerY = state.player.y ?? 0;
  const targetX = playerX + delta.dx;
  const targetY = playerY + delta.dy;
  state.player.direction = key;

  if (nodeTileWalkable(state, targetX, targetY)) {
    return { ok: true, events: applyNodeMovement(state, targetX, targetY, key) };
  }

  const boulder = boulderAt(state, targetX, targetY);
  const boulderTargetX = targetX + delta.dx;
  const boulderTargetY = targetY + delta.dy;
  if (boulder && nodeTileWalkable(state, boulderTargetX, boulderTargetY)) {
    boulder.x = boulderTargetX;
    boulder.y = boulderTargetY;
    const events = [
      nodeEvent(state, ENGINE_PROBE_EVENT.boulderMoved, {
        actorId: boulder.id,
        fromX: targetX,
        fromY: targetY,
        toX: boulderTargetX,
        toY: boulderTargetY,
      }),
      ...applyNodeMovement(state, targetX, targetY, key),
    ];
    return { ok: true, events };
  }

  return {
    ok: false,
    error: "Blocked",
    events: [
      nodeEvent(state, ENGINE_PROBE_EVENT.movementBlocked, {
        x: targetX,
        y: targetY,
        direction: key,
      }),
    ],
  };
}

function applyNodeClickTile(
  state: NodeEngineProbeState,
  x: number,
  y: number,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  if (state.player.x == null || state.player.y == null) {
    return { ok: false, events: [], error: "Player position is unavailable" };
  }
  const path = findNodePath(state, state.player.x, state.player.y, x, y);
  if (path.length === 0) {
    return {
      ok: false,
      error: "No path",
      events: [nodeEvent(state, ENGINE_PROBE_EVENT.pathNotFound, { x, y })],
    };
  }
  const last = path[path.length - 1];
  const prev = path.length > 1 ? path[path.length - 2] : { x: state.player.x, y: state.player.y };
  const direction =
    last.x > prev.x
      ? "RIGHT"
      : last.x < prev.x
        ? "LEFT"
        : last.y > prev.y
          ? "DOWN"
          : "UP";
  const events = [
    nodeEvent(state, ENGINE_PROBE_EVENT.pathFound, { x, y, length: path.length }),
    ...applyNodeMovement(state, last.x, last.y, direction),
  ];
  return { ok: true, events };
}

function applyNodeFieldMove(
  state: NodeEngineProbeState,
  moveName: string,
  targetX?: number,
  targetY?: number,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  const normalizedMove = moveName.trim().toUpperCase();
  if (targetX == null || targetY == null) {
    return {
      ok: false,
      events: [],
      error: "Field move target is required",
    };
  }
  const tileMap = sourceTileMap(state);
  const tile = tileMap.get(tileKey(targetX, targetY));
  if (!tile) {
    return { ok: false, events: [], error: "Target tile does not exist" };
  }

  if (normalizedMove === "CUT") {
    if (tile.rawFootTileId !== CUT_TREE_RAW_FOOT_TILE_ID) {
      return { ok: false, events: [], error: "There is nothing to cut" };
    }
    tile.rawFootTileId = undefined;
    tile.collisionType = COLLISION_LAND;
    const event = nodeEvent(state, ENGINE_PROBE_EVENT.fieldMoveUsed, {
      moveName: "CUT",
      targetX,
      targetY,
    });
    return { ok: true, events: [event] };
  }

  if (normalizedMove === "SURF") {
    const hasWarp = sourceWarpMap(state).has(tileKey(targetX, targetY));
    if (!isSurfableWaterTile(tile, hasWarp)) {
      return { ok: false, events: [], error: "Target is not surfable water" };
    }
    const playerX = state.player.x ?? targetX;
    const playerY = state.player.y ?? targetY;
    if (Math.abs(targetX - playerX) + Math.abs(targetY - playerY) !== 1) {
      return { ok: false, events: [], error: "Water must be adjacent" };
    }
    state.player.isSurfing = true;
    const direction =
      targetX > playerX
        ? "RIGHT"
        : targetX < playerX
          ? "LEFT"
          : targetY > playerY
            ? "DOWN"
            : "UP";
    const events = [
      nodeEvent(state, ENGINE_PROBE_EVENT.fieldMoveUsed, {
        moveName: "SURF",
        targetX,
        targetY,
      }),
      ...applyNodeMovement(state, targetX, targetY, direction),
    ];
    return { ok: true, events };
  }

  return {
    ok: false,
    events: [],
    error: `${normalizedMove} is not implemented by the Node probe`,
  };
}

function applyNodeScenario(
  state: NodeEngineProbeState,
  name: string,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  return {
    ok: true,
    events: [nodeEvent(state, ENGINE_PROBE_EVENT.scenarioJumped, { name })],
  };
}

function applyNodeDialogueChoice(
  state: NodeEngineProbeState,
  choice: EngineProbeDialogueChoice,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  state.dialogue = {
    ...defaultDialogue(),
    ...(state.dialogue ?? {}),
    selectedChoice: choice,
    isChoicePending: false,
    isOpen: false,
  };
  return {
    ok: true,
    events: [nodeEvent(state, ENGINE_PROBE_EVENT.dialogueChoiceSelected, { choice })],
  };
}

function applyNodeBattleAction(
  state: NodeEngineProbeState,
  action: EngineProbeBattleAction,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  state.battle = {
    ...defaultBattle(),
    ...(state.battle ?? {}),
    selectedAction: action,
    phase: action === "fight" ? "move_select" : "action_select",
  };
  return {
    ok: true,
    events: [nodeEvent(state, ENGINE_PROBE_EVENT.battleActionSelected, { action })],
  };
}

function applyNodeBattleMove(
  state: NodeEngineProbeState,
  index: number,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  state.battle = {
    ...defaultBattle(),
    ...(state.battle ?? {}),
    selectedMoveIndex: index,
    phase: "animating",
    turnNumber: (state.battle?.turnNumber ?? 0) + 1,
  };
  return {
    ok: true,
    events: [nodeEvent(state, ENGINE_PROBE_EVENT.battleMoveSelected, { index })],
  };
}

function applyNodeUseItem(
  state: NodeEngineProbeState,
  command: Extract<EngineProbeCommand, { useItem: unknown }>["useItem"],
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  const inventory = { ...defaultInventory(), ...(state.inventory ?? {}) };
  const item = inventory.items.find((candidate) => {
    if (command.itemId != null) return candidate.itemId === command.itemId;
    if (command.itemName) {
      return (
        candidate.name.toLowerCase() === command.itemName.toLowerCase() ||
        candidate.shortName.toLowerCase() === command.itemName.toLowerCase()
      );
    }
    return false;
  });

  if (!item) {
    return {
      ok: false,
      error: "Item not found",
      events: [nodeEvent(state, ENGINE_PROBE_EVENT.itemUsed, { ...command, found: false })],
    };
  }

  return {
    ok: true,
    events: [
      nodeEvent(state, ENGINE_PROBE_EVENT.itemUsed, {
        itemId: item.itemId,
        itemName: item.name,
        found: true,
      }),
    ],
  };
}

function applyNodeInteractActor(
  state: NodeEngineProbeState,
  command: Extract<EngineProbeCommand, { interactActor: unknown }>["interactActor"],
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  const actor = state.actors.find((candidate) => {
    if (command.actorId != null) return candidate.id === command.actorId;
    if (command.name) return candidate.name === command.name;
    if (command.text) return candidate.text === command.text;
    return false;
  });

  if (!actor) {
    return {
      ok: false,
      error: "Actor not found",
      events: [nodeEvent(state, ENGINE_PROBE_EVENT.actorInteracted, { ...command, found: false })],
    };
  }

  state.dialogue = {
    ...defaultDialogue(),
    isOpen: true,
    text: actor.text ?? actor.name ?? "",
    speakerName: actor.name ?? null,
    actorId: actor.id,
  };
  return {
    ok: true,
    events: [
      nodeEvent(state, ENGINE_PROBE_EVENT.actorInteracted, {
        actorId: actor.id,
        name: actor.name,
        text: actor.text,
        found: true,
      }),
    ],
  };
}

function applyNodeActivateWarp(
  state: NodeEngineProbeState,
  x: number,
  y: number,
): { ok: boolean; events: EngineProbeEvent[]; error?: string } {
  const warp = sourceWarpMap(state).get(tileKey(x, y));
  if (!warp || warp.destinationMapId == null || warp.destinationX == null || warp.destinationY == null) {
    return {
      ok: false,
      error: "Warp destination is unavailable",
      events: [nodeEvent(state, ENGINE_PROBE_EVENT.warpActivated, { x, y, found: false })],
    };
  }

  state.map.id = warp.destinationMapId;
  state.player.x = warp.destinationX;
  state.player.y = warp.destinationY;
  state.player.direction = warp.warpDirection ?? state.player.direction;
  return {
    ok: true,
    events: [nodeEvent(state, ENGINE_PROBE_EVENT.warpActivated, { ...warp, found: true })],
  };
}

export class NodeEngineProbeAdapter implements EngineProbeAdapter {
  private state: NodeEngineProbeState;

  constructor(state: NodeEngineProbeState) {
    this.state = cloneState(state);
  }

  getSnapshot(options?: EngineProbeSnapshotOptions): EngineProbeSnapshot {
    return buildEngineProbeSnapshot(this.state, options);
  }

  async sendCommand(
    command: EngineProbeCommand,
    options?: EngineProbeSnapshotOptions,
  ): Promise<EngineProbeResult> {
    const before = this.getSnapshot(options);
    let result: { ok: boolean; events: EngineProbeEvent[]; error?: string };

    if ("press" in command) {
      result = applyNodePress(this.state, command.press);
    } else if ("clickTile" in command) {
      result = applyNodeClickTile(this.state, command.clickTile.x, command.clickTile.y);
    } else if ("face" in command) {
      this.state.player.direction = command.face;
      result = {
        ok: true,
        events: [
          nodeEvent(this.state, ENGINE_PROBE_EVENT.directionChanged, {
            direction: command.face,
          }),
        ],
      };
    } else if ("scenario" in command) {
      result = applyNodeScenario(this.state, command.scenario.name);
    } else if ("dialogueChoice" in command) {
      result = applyNodeDialogueChoice(this.state, command.dialogueChoice.choice);
    } else if ("battleAction" in command) {
      result = applyNodeBattleAction(this.state, command.battleAction.action);
    } else if ("battleMove" in command) {
      result = applyNodeBattleMove(this.state, command.battleMove.index);
    } else if ("useItem" in command) {
      result = applyNodeUseItem(this.state, command.useItem);
    } else if ("interactActor" in command) {
      result = applyNodeInteractActor(this.state, command.interactActor);
    } else if ("activateWarp" in command) {
      result = applyNodeActivateWarp(
        this.state,
        command.activateWarp.x,
        command.activateWarp.y,
      );
    } else if ("useFieldMove" in command) {
      result = applyNodeFieldMove(
        this.state,
        command.useFieldMove.moveName,
        command.useFieldMove.targetX,
        command.useFieldMove.targetY,
      );
    } else if ("warpTo" in command) {
      this.state.map.id = command.warpTo.mapId;
      this.state.player.x = command.warpTo.x;
      this.state.player.y = command.warpTo.y;
      this.state.player.direction = command.warpTo.direction ?? this.state.player.direction;
      result = {
        ok: true,
        events: [nodeEvent(this.state, ENGINE_PROBE_EVENT.warpActivated, command.warpTo)],
      };
    } else {
      result = {
        ok: true,
        events: [nodeEvent(this.state, ENGINE_PROBE_EVENT.waitForIdle)],
      };
    }

    return {
      ok: result.ok,
      command,
      before,
      after: this.getSnapshot(options),
      events: result.events,
      error: result.error,
    };
  }

  async waitForIdle(): Promise<void> {
    return Promise.resolve();
  }
}

export const engineProbeInternals = {
  CUT_TREE_RAW_FOOT_TILE_ID,
  DEFAULT_RADIUS,
  MAX_RADIUS,
  normalizeDirection,
};
