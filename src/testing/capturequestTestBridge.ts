import { WorldSocket, OpCodes } from "@/net";
import AudioManager from "@/services/audio/AudioManager";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import useChatStore from "@/stores/ChatStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import useDebugSceneStore from "@/stores/DebugSceneStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import {
  getCurrentCutsceneScriptLabel,
  getLastCompletedCutsceneScriptLabel,
  getLastStartedCutsceneScriptLabel,
  isCutscenePlaying,
} from "@/phaser-game/services/CutsceneService";
import {
  buyCoins,
  buyPrize,
  playSlotMachine,
  requestCoinBalance,
  requestPrizeList,
  sendPokemonPartyRequest,
  sendPokemonPCOpen,
} from "@/phaser-game/services/PhaserNetworkService";
import type { PokemonDTO } from "@/net/generated/world_api";
import {
  ENGINE_PROBE_EVENT,
  buildEngineProbeSnapshot,
  createEngineProbeEvent,
  type EngineProbeCommand,
  type EngineProbeEvent,
  type EngineProbeResult,
  type EngineProbeSnapshot,
  type EngineProbeSnapshotOptions,
  type EngineProbeSnapshotSource,
} from "./engineProbe";

const IS_TEST_MODE = import.meta.env.VITE_TEST_MODE === "true";

export interface CaptureQuestTestActor {
  id: number;
  internalId: number | null;
  name: string | null;
  text: string | null;
  type: string;
  mapId: number | null;
  x: number | null;
  y: number | null;
  direction: string | null;
}

export interface CaptureQuestTestState {
  screen: string;
  connected: boolean;
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
    isSurfing: boolean;
    isCycling: boolean;
    isMoving: boolean;
  };
  visibleActors: CaptureQuestTestActor[];
  warps: Array<{
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
  }>;
  worldInput: {
    frozen: boolean;
    reason: string | null;
  };
  ui: {
    isInventoryOpen: boolean;
    isPokedexOpen: boolean;
    isTrainerCardOpen: boolean;
    isOptionsOpen: boolean;
    isHelpOpen: boolean;
    isWarpMode: boolean;
    isCameraFollowEnabled: boolean;
    isTileManagerOpen: boolean;
    isArtStudioOpen: boolean;
    isDebugSceneOpen: boolean;
  };
  inventory: {
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
  };
  pokemon: {
    party: PokemonDTO[];
    pc: {
      isOpen: boolean;
      currentBox: number;
      boxPokemon: PokemonDTO[];
    };
  };
  messages: Array<{
    text: string;
    type: string;
  }>;
  audio: {
    currentMusicTrack: string | null;
    requestedMusicTrack: string | null;
    lastSFXTrack: string | null;
    recentSFXTracks: string[];
    isSurfing: boolean;
    isBicycleActive: boolean;
    wantsBicycle: boolean;
    forcedBicycle: boolean;
    travelMapId: number | null;
    travelMapName: string | null;
  };
  debug: {
    lastAppliedScenario: {
      label: string | null;
      scenarioName: string | null;
      scriptLabel: string | null;
    } | null;
    currentCutsceneScriptLabel: string | null;
    lastStartedCutsceneScriptLabel: string | null;
    lastCompletedCutsceneScriptLabel: string | null;
    isCutscenePlaying: boolean;
  };
  dialogue: {
    isOpen: boolean;
    text: string;
    isTyping: boolean;
    speakerName: string | null;
    actorId: number | null;
    isChoicePending: boolean;
    choicePrompt: string | null;
    selectedChoice: "yes" | "no" | null;
  };
  battle: {
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
  };
}

interface TileViewportPoint {
  x: number;
  y: number;
}

export type CaptureQuestTestStatePatch = Partial<
  Omit<
    CaptureQuestTestState,
    | "map"
    | "player"
    | "worldInput"
    | "ui"
    | "inventory"
    | "audio"
    | "debug"
    | "dialogue"
    | "battle"
  >
> & {
  map?: Partial<CaptureQuestTestState["map"]>;
  player?: Partial<CaptureQuestTestState["player"]>;
  worldInput?: Partial<CaptureQuestTestState["worldInput"]>;
  ui?: Partial<CaptureQuestTestState["ui"]>;
  inventory?: Partial<CaptureQuestTestState["inventory"]>;
  pokemon?: Partial<CaptureQuestTestState["pokemon"]>;
  audio?: Partial<CaptureQuestTestState["audio"]>;
  debug?: Partial<CaptureQuestTestState["debug"]>;
  dialogue?: Partial<CaptureQuestTestState["dialogue"]>;
  battle?: Partial<CaptureQuestTestState["battle"]>;
};

export interface CaptureQuestTileViewerDiagnostics {
  getState: () => CaptureQuestTestStatePatch;
  getEngineProbeSource?: () => Pick<
    EngineProbeSnapshotSource,
    "tiles" | "actors" | "warps"
  >;
  sendCommand?: (
    command: EngineProbeCommand,
  ) =>
    | Promise<{ ok: boolean; events?: EngineProbeEvent[]; error?: string } | undefined>
    | { ok: boolean; events?: EngineProbeEvent[]; error?: string }
    | undefined;
  tileToViewport?: (x: number, y: number) => TileViewportPoint | null;
  centerTileInView?: (x: number, y: number) => void;
}

export interface CaptureQuestWarpProbeCase {
  id: number;
  sourceMapId: number;
  sourceClientMapId: number;
  sourceMapName: string;
  sourceIsOverworld: boolean;
  x: number;
  y: number;
  destinationMapId: number;
  destinationClientMapId: number;
  destinationMapName: string;
  destinationIsOverworld: boolean;
  destinationX: number;
  destinationY: number;
  expectedX: number;
  expectedY: number;
  warpType: string;
  warpDirection: string;
  keyboardSetupX: number;
  keyboardSetupY: number;
  keyboardDirection: string;
  clickSetupX: number;
  clickSetupY: number;
  postWarpMoveX: number;
  postWarpMoveY: number;
  postWarpMoveDirection: string;
}

export interface CaptureQuestWarpProbeSkippedCase {
  id: number;
  sourceMapId: number;
  sourceMapName: string;
  x: number;
  y: number;
  reason: string;
}

export interface CaptureQuestWarpProbeCasesResponse {
  success: boolean;
  totalCaseCount: number;
  cases: CaptureQuestWarpProbeCase[];
  skippedCases: CaptureQuestWarpProbeSkippedCase[];
  error?: string;
}

export interface CaptureQuestTestBridge {
  getState: () => CaptureQuestTestState;
  getSnapshot: (options?: EngineProbeSnapshotOptions) => EngineProbeSnapshot;
  sendCommand: (
    command: EngineProbeCommand,
    options?: EngineProbeSnapshotOptions,
  ) => Promise<EngineProbeResult>;
  waitForIdle: (options?: { timeoutMs?: number }) => Promise<void>;
  waitForEvent: (type: string, timeoutMs?: number) => Promise<unknown>;
  tileToViewport: (x: number, y: number) => TileViewportPoint | null;
  centerTileInView: (x: number, y: number) => void;
  warpToMap: (mapId: number, x: number, y: number, direction?: string) => void;
  requestWarpProbeCases: (options?: {
    limit?: number;
    offset?: number;
    sourceMapId?: number;
  }) => Promise<CaptureQuestWarpProbeCasesResponse>;
  requestGameCornerCoinBalance: () => void;
  buyGameCornerCoins: () => void;
  playGameCornerSlot: (bet: number, isLucky?: boolean) => void;
  requestGameCornerPrizeList: () => void;
  buyGameCornerPrize: (prizeId: number) => void;
  requestPokemonParty: () => void;
  requestPokemonPC: () => void;
}

declare global {
  interface Window {
    __capturequestTest?: CaptureQuestTestBridge;
  }
}

let tileViewerDiagnostics: CaptureQuestTileViewerDiagnostics | null = null;
let installed = false;
let engineProbeEvents: EngineProbeEvent[] = [];

function selectedChoiceFromDOM(): "yes" | "no" | null {
  if (typeof document === "undefined") return null;
  const selected = document.querySelector<HTMLElement>(
    "[data-testid^='dialogue-choice-'][data-selected='true']",
  );
  const testId = selected?.dataset.testid;
  if (testId === "dialogue-choice-yes") return "yes";
  if (testId === "dialogue-choice-no") return "no";
  return null;
}

function selectedBattleTestId(prefix: string): string | null {
  if (typeof document === "undefined") return null;
  const selected = document.querySelector<HTMLElement>(
    `[data-testid^='${prefix}'][data-selected='true']`,
  );
  return selected?.dataset.testid ?? null;
}

function selectedBattleIndex(prefix: string): number | null {
  const testId = selectedBattleTestId(prefix);
  if (!testId) return null;
  const raw = testId.slice(prefix.length);
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? parsed : null;
}

function emitCaptureQuestTestEvent(type: string, detail?: unknown): void {
  if (!IS_TEST_MODE || typeof window === "undefined") return;
  if (type !== "cq:stateChanged") {
    engineProbeEvents = [
      ...engineProbeEvents,
      createEngineProbeEvent(type, detail),
    ].slice(-50);
  }
  window.dispatchEvent(new CustomEvent(type, { detail }));
  window.dispatchEvent(new CustomEvent("cq:stateChanged", { detail: { type } }));
}

function baseState(): CaptureQuestTestState {
  const screen = useGameScreenStore.getState();
  const status = useGameStatusStore.getState();
  const profile = usePlayerCharacterStore.getState().characterProfile;
  const dialogue = usePokemonDialogueStore.getState();
  const battle = usePokeBattleStore.getState();
  const audio = useAudioActivityStore.getState();
  const inventory = useCQInventoryStore.getState();
  const party = usePokemonPartyStore.getState();
  const pc = usePokemonPCStore.getState();
  const chat = useChatStore.getState();
  const debug = useDebugSceneStore.getState();
  const mapId = status.currentMap;
  const selectedActionTestId = selectedBattleTestId("battle-action-");

  return {
    screen: screen.currentScreen,
    connected: WorldSocket.isConnected,
    map: {
      id: mapId,
      name: mapId == null ? null : (status.getMapNameById(mapId) ?? null),
      isLoading: status.isMapLoading,
    },
    player: {
      id: profile?.id ?? null,
      internalId: profile?.id ?? null,
      name: profile?.name ?? null,
      x: profile?.x ?? null,
      y: profile?.y ?? null,
      direction: null,
      isSurfing: false,
      isCycling: audio.isBicycleActive,
      isMoving: false,
    },
    visibleActors: [],
    warps: [],
    worldInput: {
      frozen: false,
      reason: null,
    },
    ui: {
      isInventoryOpen: status.isInventoryOpen,
      isPokedexOpen: status.isPokedexOpen,
      isTrainerCardOpen: status.isTrainerCardOpen,
      isOptionsOpen: status.isOptionsOpen,
      isHelpOpen: status.isHelpOpen,
      isWarpMode: status.isWarpMode,
      isCameraFollowEnabled: status.isCameraFollowEnabled,
      isTileManagerOpen: status.isTileManagerOpen,
      isArtStudioOpen: status.isArtStudioOpen,
      isDebugSceneOpen: debug.isOpen,
    },
    inventory: {
      money: inventory.money,
      shopOpen: inventory.shopOpen,
      shopName: inventory.shopName,
      items: inventory.items.map((entry) => ({
        instanceId: entry.instance.id,
        itemId: entry.item.id,
        name: entry.item.name,
        shortName: entry.item.shortName,
        quantity: entry.instance.quantity,
      })),
    },
    pokemon: {
      party: party.party,
      pc: {
        isOpen: pc.isOpen,
        currentBox: pc.currentBox,
        boxPokemon: pc.boxPokemon,
      },
    },
    messages: chat.messages.slice(-10).map((message) => ({
      text: message.text,
      type: message.type,
    })),
    audio: {
      currentMusicTrack: AudioManager.getCurrentMusicTrack(),
      requestedMusicTrack: AudioManager.getRequestedMusicTrack(),
      lastSFXTrack: AudioManager.getLastSFXTrack(),
      recentSFXTracks: AudioManager.getRecentSFXTracks(),
      isSurfing: audio.isSurfing,
      isBicycleActive: audio.isBicycleActive,
      wantsBicycle: audio.wantsBicycle,
      forcedBicycle: audio.forcedBicycle,
      travelMapId: audio.travelMapId,
      travelMapName: audio.travelMapName,
    },
    debug: {
      lastAppliedScenario: debug.lastAppliedScenario,
      currentCutsceneScriptLabel: getCurrentCutsceneScriptLabel(),
      lastStartedCutsceneScriptLabel: getLastStartedCutsceneScriptLabel(),
      lastCompletedCutsceneScriptLabel: getLastCompletedCutsceneScriptLabel(),
      isCutscenePlaying: isCutscenePlaying(),
    },
    dialogue: {
      isOpen: dialogue.isOpen,
      text: dialogue.displayedText || dialogue.choicePrompt || "",
      isTyping: dialogue.isTyping,
      speakerName: dialogue.speakerName,
      actorId: dialogue.actorId,
      isChoicePending: dialogue.isChoicePending,
      choicePrompt: dialogue.choicePrompt,
      selectedChoice: selectedChoiceFromDOM(),
    },
    battle: {
      isOpen: battle.isInBattle,
      phase: battle.phase,
      battleType: battle.battleType,
      turnNumber: battle.turnNumber,
      allowedActions: battle.allowedActions,
      guaranteedCatch: battle.guaranteedCatch,
      enemyPokemonId: battle.enemyPokemon?.id ?? null,
      enemyPokemonName: battle.enemyPokemon?.name ?? null,
      selectedAction:
        selectedActionTestId?.replace("battle-action-", "") ?? null,
      selectedMoveIndex: selectedBattleIndex("battle-move-"),
      selectedItemIndex: selectedBattleIndex("battle-item-"),
      selectedPokemonIndex: selectedBattleIndex("battle-party-"),
    },
  };
}

function getState(): CaptureQuestTestState {
  const tileViewerState = tileViewerDiagnostics?.getState() ?? {};
  const state = baseState();

  return {
    ...state,
    ...tileViewerState,
    map: {
      ...state.map,
      ...(tileViewerState.map ?? {}),
    },
    player: {
      ...state.player,
      ...(tileViewerState.player ?? {}),
    },
    worldInput: {
      ...state.worldInput,
      ...(tileViewerState.worldInput ?? {}),
    },
    ui: {
      ...state.ui,
      ...(tileViewerState.ui ?? {}),
    },
    inventory: {
      ...state.inventory,
      ...(tileViewerState.inventory ?? {}),
    },
    pokemon: {
      ...state.pokemon,
      ...(tileViewerState.pokemon ?? {}),
      pc: {
        ...state.pokemon.pc,
        ...(tileViewerState.pokemon?.pc ?? {}),
      },
    },
    audio: {
      ...state.audio,
      ...(tileViewerState.audio ?? {}),
    },
    debug: {
      ...state.debug,
      ...(tileViewerState.debug ?? {}),
    },
    dialogue: {
      ...state.dialogue,
      ...(tileViewerState.dialogue ?? {}),
    },
    battle: {
      ...state.battle,
      ...(tileViewerState.battle ?? {}),
    },
  };
}

function waitForEvent(type: string, timeoutMs = 10_000): Promise<unknown> {
  if (!IS_TEST_MODE || typeof window === "undefined") {
    return Promise.reject(new Error("CaptureQuest test bridge is not installed"));
  }

  return new Promise((resolve, reject) => {
    const handler = (event: Event) => {
      window.clearTimeout(timeout);
      resolve((event as CustomEvent).detail);
    };

    const timeout = window.setTimeout(() => {
      window.removeEventListener(type, handler);
      reject(new Error(`Timed out waiting for ${type}`));
    }, timeoutMs);

    window.addEventListener(type, handler, { once: true });
  });
}

function tileToViewport(x: number, y: number): TileViewportPoint | null {
  return tileViewerDiagnostics?.tileToViewport?.(x, y) ?? null;
}

function centerTileInView(x: number, y: number): void {
  tileViewerDiagnostics?.centerTileInView?.(x, y);
}

function getSnapshot(options?: EngineProbeSnapshotOptions): EngineProbeSnapshot {
  const state = getState();
  const source = tileViewerDiagnostics?.getEngineProbeSource?.() ?? {
    tiles: [],
    actors: [],
    warps: [],
  };
  return buildEngineProbeSnapshot(
    {
      ...source,
      map: state.map,
      player: state.player,
      worldInput: state.worldInput,
      dialogue: state.dialogue,
      battle: state.battle,
      inventory: state.inventory,
      party: state.pokemon.party,
      pc: state.pokemon.pc,
      messages: state.messages,
      lastEvents: engineProbeEvents,
    },
    options,
  );
}

function keyboardKeyForPress(press: string): string {
  switch (press) {
    case "UP":
      return "ArrowUp";
    case "DOWN":
      return "ArrowDown";
    case "LEFT":
      return "ArrowLeft";
    case "RIGHT":
      return "ArrowRight";
    case "SPACE":
      return " ";
    default:
      return press;
  }
}

function keyboardCodeForPress(press: string): string {
  switch (press) {
    case "UP":
      return "ArrowUp";
    case "DOWN":
      return "ArrowDown";
    case "LEFT":
      return "ArrowLeft";
    case "RIGHT":
      return "ArrowRight";
    case "SPACE":
      return "Space";
    default:
      return press;
  }
}

function dispatchKeyboardPress(press: string): void {
  const key = keyboardKeyForPress(press);
  const code = keyboardCodeForPress(press);
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur();
  }
  for (const eventType of ["keydown", "keyup"]) {
    const eventInit = { key, code, bubbles: true };
    window.dispatchEvent(new KeyboardEvent(eventType, eventInit));
    document.dispatchEvent(new KeyboardEvent(eventType, eventInit));
  }
}

function dispatchTileClick(x: number, y: number): void {
  const point = tileToViewport(x, y);
  const canvas = document.querySelector("canvas");
  if (!point || !canvas) return;
  for (const eventType of ["pointerdown", "pointerup", "click"]) {
    canvas.dispatchEvent(
      new MouseEvent(eventType, {
        clientX: point.x,
        clientY: point.y,
        bubbles: true,
        cancelable: true,
      }),
    );
  }
}

function inventoryItemTestId(shortName: string): string {
  return `inventory-item-${shortName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")}`;
}

async function waitUntil(
  predicate: () => boolean,
  description: string,
  timeoutMs = 15_000,
): Promise<void> {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (predicate()) return;
    await new Promise((resolve) => window.setTimeout(resolve, 50));
  }
  throw new Error(`Timed out waiting for ${description}`);
}

function sendFieldMoveCommand(command: Extract<EngineProbeCommand, { useFieldMove: unknown }>): void {
  WorldSocket.sendJsonMessage(OpCodes.FieldMoveUseRequest, {
    moveName: command.useFieldMove.moveName,
    targetX: command.useFieldMove.targetX,
    targetY: command.useFieldMove.targetY,
  });
}

async function sendScenarioCommand(
  command: Extract<EngineProbeCommand, { scenario: unknown }>,
): Promise<EngineProbeEvent[]> {
  const positionChanged = waitForEvent("cq:playerPositionChanged", 30_000).catch(
    () => null,
  );
  const mapChanged = waitForEvent("cq:mapChanged", 30_000).catch(() => null);
  WorldSocket.sendJsonMessage(OpCodes.DebugSceneJumpRequest, {
    scenarioName: command.scenario.name,
  });
  await waitUntil(
    () =>
      useDebugSceneStore.getState().lastAppliedScenario?.scenarioName ===
      command.scenario.name,
    `scenario ${command.scenario.name} to apply`,
    30_000,
  );
  await Promise.race([
    Promise.all([positionChanged, mapChanged]),
    new Promise((resolve) => window.setTimeout(resolve, 3_000)),
  ]);
  await waitUntil(
    () => {
      const state = getState();
      return !state.map.isLoading && state.player.x != null && state.player.y != null;
    },
    `scenario ${command.scenario.name} map state`,
    30_000,
  );
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.scenarioJumped, {
      name: command.scenario.name,
    }),
  ];
}

function sendDialogueChoiceCommand(
  command: Extract<EngineProbeCommand, { dialogueChoice: unknown }>,
): EngineProbeEvent[] {
  const choice = command.dialogueChoice.choice;
  const button = document.querySelector<HTMLButtonElement>(
    `[data-testid="dialogue-choice-${choice}"]`,
  );
  if (!button) {
    throw new Error(`Dialogue choice ${choice} is not available`);
  }
  button.click();
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.dialogueChoiceSelected, {
      choice,
    }),
  ];
}

function sendBattleActionCommand(
  command: Extract<EngineProbeCommand, { battleAction: unknown }>,
): EngineProbeEvent[] {
  const button = document.querySelector<HTMLButtonElement>(
    `[data-testid="battle-action-${command.battleAction.action}"]`,
  );
  if (!button) {
    throw new Error(`Battle action ${command.battleAction.action} is not available`);
  }
  button.click();
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.battleActionSelected, {
      action: command.battleAction.action,
    }),
  ];
}

function sendBattleMoveCommand(
  command: Extract<EngineProbeCommand, { battleMove: unknown }>,
): EngineProbeEvent[] {
  const button = document.querySelector<HTMLButtonElement>(
    `[data-testid="battle-move-${command.battleMove.index}"]`,
  );
  if (!button) {
    throw new Error(`Battle move ${command.battleMove.index} is not available`);
  }
  button.click();
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.battleMoveSelected, {
      index: command.battleMove.index,
    }),
  ];
}

async function sendUseItemCommand(
  command: Extract<EngineProbeCommand, { useItem: unknown }>,
): Promise<EngineProbeEvent[]> {
  let testId = command.useItem.testId;
  if (!testId) {
    const item = getState().inventory.items.find((candidate) => {
      if (command.useItem.itemId != null) return candidate.itemId === command.useItem.itemId;
      if (command.useItem.itemName) {
        const itemName = command.useItem.itemName.toLowerCase();
        return (
          candidate.name.toLowerCase() === itemName ||
          candidate.shortName.toLowerCase() === itemName
        );
      }
      return false;
    });
    if (!item) {
      throw new Error("Inventory item is not available");
    }
    testId = inventoryItemTestId(item.shortName);
  }

  if (!useGameStatusStore.getState().isInventoryOpen) {
    useGameStatusStore.getState().toggleInventory();
  }
  await waitUntil(
    () => Boolean(document.querySelector(`[data-testid="${testId}"]`)),
    `inventory item ${testId}`,
  );
  const element = document.querySelector<HTMLElement>(`[data-testid="${testId}"]`);
  if (!element) {
    throw new Error(`Inventory item ${testId} is not available`);
  }
  element.click();
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.itemUsed, {
      itemId: command.useItem.itemId ?? null,
      itemName: command.useItem.itemName ?? null,
      testId,
    }),
  ];
}

function actorMatchesCommand(
  actor: CaptureQuestTestActor,
  command: Extract<EngineProbeCommand, { interactActor: unknown }>["interactActor"],
): boolean {
  if (command.actorId != null) return actor.id === command.actorId;
  if (command.name) return actor.name === command.name;
  if (command.text) return actor.text === command.text;
  return false;
}

function sendInteractActorCommand(
  command: Extract<EngineProbeCommand, { interactActor: unknown }>,
): EngineProbeEvent[] {
  const actor = getState().visibleActors.find((candidate) =>
    actorMatchesCommand(candidate, command.interactActor),
  );
  if (!actor || actor.x == null || actor.y == null) {
    throw new Error("Actor is not available for interaction");
  }
  dispatchTileClick(actor.x, actor.y);
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.actorInteracted, {
      actorId: actor.id,
      name: actor.name,
      text: actor.text,
    }),
  ];
}

function sendActivateWarpCommand(
  command: Extract<EngineProbeCommand, { activateWarp: unknown }>,
): EngineProbeEvent[] {
  dispatchTileClick(command.activateWarp.x, command.activateWarp.y);
  return [
    createEngineProbeEvent(ENGINE_PROBE_EVENT.warpActivated, {
      x: command.activateWarp.x,
      y: command.activateWarp.y,
    }),
  ];
}

async function waitForIdle(options: { timeoutMs?: number } = {}): Promise<void> {
  const timeoutMs = options.timeoutMs ?? 10_000;
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    const state = getState();
    if (
      !state.map.isLoading &&
      !state.player.isMoving &&
      !state.worldInput.frozen
    ) {
      return;
    }
    await new Promise((resolve) => window.setTimeout(resolve, 50));
  }

  throw new Error("Timed out waiting for engine probe idle state");
}

async function sendCommand(
  command: EngineProbeCommand,
  options?: EngineProbeSnapshotOptions,
): Promise<EngineProbeResult> {
  const before = getSnapshot(options);
  let ok = true;
  let error: string | undefined;
  let events: EngineProbeEvent[] = [];

  try {
    const liveResult =
      "waitForIdle" in command
        ? null
        : await tileViewerDiagnostics?.sendCommand?.(command);
    if (liveResult) {
      ok = liveResult.ok;
      error = liveResult.error;
      events = liveResult.events ?? [];
    } else if ("waitForIdle" in command) {
      await waitForIdle({ timeoutMs: 10_000 });
    } else if ("press" in command) {
      dispatchKeyboardPress(command.press);
    } else if ("clickTile" in command) {
      dispatchTileClick(command.clickTile.x, command.clickTile.y);
    } else if ("face" in command) {
      dispatchKeyboardPress(command.face);
    } else if ("scenario" in command) {
      events = await sendScenarioCommand(command);
    } else if ("dialogueChoice" in command) {
      events = sendDialogueChoiceCommand(command);
    } else if ("battleAction" in command) {
      events = sendBattleActionCommand(command);
    } else if ("battleMove" in command) {
      events = sendBattleMoveCommand(command);
    } else if ("useItem" in command) {
      events = await sendUseItemCommand(command);
    } else if ("interactActor" in command) {
      events = sendInteractActorCommand(command);
    } else if ("activateWarp" in command) {
      events = sendActivateWarpCommand(command);
    } else if ("useFieldMove" in command) {
      sendFieldMoveCommand(command);
    } else if ("warpTo" in command) {
      warpToMap(
        command.warpTo.mapId,
        command.warpTo.x,
        command.warpTo.y,
        command.warpTo.direction,
      );
    }

    if (!("waitForIdle" in command)) {
      await waitForIdle({ timeoutMs: 10_000 }).catch(() => undefined);
    }
  } catch (err) {
    ok = false;
    error = err instanceof Error ? err.message : String(err);
  }

  const commandEvent = createEngineProbeEvent(ENGINE_PROBE_EVENT.engineProbeCommand, {
    command,
    ok,
    error,
  });
  engineProbeEvents = [...engineProbeEvents, commandEvent, ...events].slice(-50);

  return {
    ok,
    command,
    before,
    after: getSnapshot(options),
    events: [commandEvent, ...events],
    error,
  };
}

function warpToMap(mapId: number, x: number, y: number, direction = "DOWN"): void {
  window.dispatchEvent(
    new CustomEvent("warpTileTeleport", {
      detail: { mapId, x, y, direction },
    }),
  );
}

function requestWarpProbeCases(options: {
  limit?: number;
  offset?: number;
  sourceMapId?: number;
} = {}): Promise<CaptureQuestWarpProbeCasesResponse> {
  return WorldSocket.sendJsonRequest<CaptureQuestWarpProbeCasesResponse>(
    OpCodes.DebugWarpProbeCasesRequest,
    OpCodes.DebugWarpProbeCasesResponse,
    options,
    30_000,
  );
}

export function installCaptureQuestTestBridge(): void {
  if (!IS_TEST_MODE || typeof window === "undefined" || installed) return;
  installed = true;

  window.__capturequestTest = {
    getState,
    getSnapshot,
    sendCommand,
    waitForIdle,
    waitForEvent,
    tileToViewport,
    centerTileInView,
    warpToMap,
    requestWarpProbeCases,
    requestGameCornerCoinBalance: requestCoinBalance,
    buyGameCornerCoins: buyCoins,
    playGameCornerSlot: (bet, isLucky = false) => playSlotMachine(bet, isLucky),
    requestGameCornerPrizeList: requestPrizeList,
    buyGameCornerPrize: buyPrize,
    requestPokemonParty: sendPokemonPartyRequest,
    requestPokemonPC: sendPokemonPCOpen,
  };

  let previousScreen = useGameScreenStore.getState().currentScreen;
  useGameScreenStore.subscribe((state) => {
    if (state.currentScreen !== previousScreen) {
      previousScreen = state.currentScreen;
      emitCaptureQuestTestEvent("cq:screenChanged", { screen: state.currentScreen });
    }
  });

  let previousDialogueOpen = usePokemonDialogueStore.getState().isOpen;
  usePokemonDialogueStore.subscribe((state) => {
    if (state.isOpen !== previousDialogueOpen) {
      previousDialogueOpen = state.isOpen;
      emitCaptureQuestTestEvent(
        state.isOpen ? "cq:dialogueOpened" : "cq:dialogueClosed",
        getState().dialogue,
      );
    }
  });

  let previousBattleOpen = usePokeBattleStore.getState().isInBattle;
  usePokeBattleStore.subscribe((state) => {
    if (state.isInBattle !== previousBattleOpen) {
      previousBattleOpen = state.isInBattle;
      emitCaptureQuestTestEvent(
        state.isInBattle ? "cq:battleOpened" : "cq:battleClosed",
        getState().battle,
      );
    }
  });

  emitCaptureQuestTestEvent("cq:testBridgeReady", getState());
}

export function setCaptureQuestTileViewerDiagnostics(
  diagnostics: CaptureQuestTileViewerDiagnostics,
): void {
  if (!IS_TEST_MODE) return;
  tileViewerDiagnostics = diagnostics;
  emitCaptureQuestTestEvent("cq:tileViewerReady", getState());
}

export function clearCaptureQuestTileViewerDiagnostics(): void {
  if (!IS_TEST_MODE) return;
  tileViewerDiagnostics = null;
  emitCaptureQuestTestEvent("cq:tileViewerClosed");
}

export { emitCaptureQuestTestEvent };
