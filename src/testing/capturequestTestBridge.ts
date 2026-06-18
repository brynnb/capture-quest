import { WorldSocket } from "@/net";
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
    lastGeneratedSFXName: string | null;
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
  tileToViewport?: (x: number, y: number) => TileViewportPoint | null;
}

export interface CaptureQuestTestBridge {
  getState: () => CaptureQuestTestState;
  waitForEvent: (type: string, timeoutMs?: number) => Promise<unknown>;
  tileToViewport: (x: number, y: number) => TileViewportPoint | null;
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
      lastGeneratedSFXName: AudioManager.getLastGeneratedSFXName(),
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
    let timeout: number;
    const handler = (event: Event) => {
      window.clearTimeout(timeout);
      resolve((event as CustomEvent).detail);
    };

    timeout = window.setTimeout(() => {
      window.removeEventListener(type, handler);
      reject(new Error(`Timed out waiting for ${type}`));
    }, timeoutMs);

    window.addEventListener(type, handler, { once: true });
  });
}

function tileToViewport(x: number, y: number): TileViewportPoint | null {
  return tileViewerDiagnostics?.tileToViewport?.(x, y) ?? null;
}

export function installCaptureQuestTestBridge(): void {
  if (!IS_TEST_MODE || typeof window === "undefined" || installed) return;
  installed = true;

  window.__capturequestTest = {
    getState,
    waitForEvent,
    tileToViewport,
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
