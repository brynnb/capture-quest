/**
 * CutsceneService — receives cutscene action sequences from the server
 * and plays them client-side using temporary sprites and dialogue.
 *
 * The server sends a CutsceneStartNotify with a JSON array of actions.
 * The client plays them sequentially, then sends CutsceneEndRequest.
 *
 * Action types:
 *   lockInput    — disable player input
 *   unlockInput  — re-enable player input
 *   dialogue     — show dialogue lines via PokemonDialogueStore
 *   dialogueText — fetch dialogue by TEXT_ constant, then show it
 *   choice       — show a YES/NO prompt; YES continues, NO aborts unless continueOnNo is set
 *   playSFX      — play an original source SFX constant
 *   playCry      — play a species-specific Pokémon cry
 *   playMusic    — play an original source music constant
 *   move         — animate a temporary actor along a movement path
 *   parallel     — execute a nested action list concurrently
 *   hideActor    — hide/remove a temporary actor sprite
 *   showActor    — spawn a temporary actor sprite at a position
 *   delay        — wait N milliseconds
 *   facePlayer   — change a temporary actor's facing direction
 *   setFlag      — (handled server-side on CutsceneEnd, no-op client-side)
 *   resetFlag    — (handled server-side on CutsceneEnd, no-op client-side)
 *   toggleFlag   — (handled server-side on CutsceneEnd, no-op client-side)
 *   giveItem     — (handled server-side on CutsceneEnd, no-op client-side)
 *   takeItem     — (handled server-side on CutsceneEnd, no-op client-side)
 *   takeMoney    — (handled server-side on CutsceneEnd, no-op client-side)
 *   gameCornerPrizeVendor — (handled server-side on CutsceneEnd, no-op client-side)
 *   startSafariSession — (handled server-side on CutsceneEnd, no-op client-side)
 *   endSafariSession — (handled server-side on CutsceneEnd, no-op client-side)
 *   startTrainerBattle — (handled server-side on CutsceneEnd, no-op client-side)
 *   startWildBattle — (handled server-side on CutsceneEnd, no-op client-side)
 */

import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import { completeCutscene, readOwnedPlayerPosition } from "./PlayerMovementService";
import { CorrelatedResponseError } from "./CorrelatedRequest";
import type { CutsceneStartNotify, OwnedPlayerPositionResponse, PlayerStepError } from "@/net/generated/protocol";
export type CutsceneOwnedPosition = Pick<OwnedPlayerPositionResponse, "mapId" | "x" | "y" | "direction" | "serverMovementPending">;
import AudioManager from "@/services/audio/AudioManager";
import {
  cryPathForPokemon,
  musicTrackForConstant,
  sfxPathForConstant,
} from "@/services/audio/pokemonMusic";
import { fetchDialogue, parseDialogueText } from "./DialogueService";
import {
  resolveDialoguePlaceholders,
  resolveDialogueSpeaker,
} from "@/utils/dialoguePlaceholders";


// --- Types ---

export type CutsceneAction = import("@/net/generated/scriptedactions").Action;
export type CutsceneStartPayload = CutsceneStartNotify;

// --- State ---

let isPlaying = false;
let currentScriptLabel: string | null = null;
let currentCompletionToken: string | null = null;
let lastStartedScriptLabel: string | null = null;
let lastCompletedScriptLabel: string | null = null;
let activeRunId = 0;
let activeRunAbort: AbortController | null = null;
const queuedCutscenes: CutsceneStartPayload[] = [];

type CancelListener = {
  runId: number;
  callback: () => void;
};
const cancelListeners: Set<CancelListener> = new Set();

// Callbacks for the Phaser scene to hook into for sprite operations
type MoveCallback = (
  actor: string,
  movements: string[],
) => Promise<void>;
type ShowActorCallback = (
  actor: string,
  x: number,
  y: number,
  sprite: string,
) => void;
type HideActorCallback = (actor: string) => void;
type HideObjectCallback = (actorId: number) => void;
type FaceCallback = (actor: string, direction: string) => void;
type InputLockCallback = (locked: boolean) => void;

let onMove: MoveCallback | null = null;
let onShowActor: ShowActorCallback | null = null;
let onHideActor: HideActorCallback | null = null;
let onHideObject: HideObjectCallback | null = null;
let onFace: FaceCallback | null = null;
let onInputLock: InputLockCallback | null = null;
let onReconcile: ((position: CutsceneOwnedPosition) => Promise<void>) | null = null;
let onCancelPlayback: (() => void) | null = null;

// --- Public API ---

/** Register Phaser scene callbacks for sprite operations. */
export function registerCutsceneCallbacks(callbacks: {
  onMove: MoveCallback;
  onShowActor: ShowActorCallback;
  onHideActor: HideActorCallback;
  onHideObject?: HideObjectCallback;
  onFace: FaceCallback;
  onInputLock: InputLockCallback;
  onReconcile?: (position: CutsceneOwnedPosition) => Promise<void>;
  onCancelPlayback?: () => void;
}): void {
  onMove = callbacks.onMove;
  onShowActor = callbacks.onShowActor;
  onHideActor = callbacks.onHideActor;
  onHideObject = callbacks.onHideObject ?? null;
  onFace = callbacks.onFace;
  onInputLock = callbacks.onInputLock;
  onReconcile = callbacks.onReconcile ?? null;
  onCancelPlayback = callbacks.onCancelPlayback ?? null;
}

/** Unregister callbacks (e.g., when scene is destroyed). */
export function unregisterCutsceneCallbacks(): void {
  cancelActiveCutscene("scene retired");
  onReconcile = null;
  onCancelPlayback = null;
  onMove = null;
  onShowActor = null;
  onHideActor = null;
  onHideObject = null;
  onFace = null;
  onInputLock = null;
}

/** Whether a cutscene is currently playing. */
export function isCutscenePlaying(): boolean {
  return isPlaying;
}

export function getCurrentCutsceneScriptLabel(): string | null {
  return currentScriptLabel;
}

export function getLastStartedCutsceneScriptLabel(): string | null {
  return lastStartedScriptLabel;
}

export function getLastCompletedCutsceneScriptLabel(): string | null {
  return lastCompletedScriptLabel;
}

export function cancelActiveCutscene(reason = "cancelled"): void {
  const cancelledRunId = activeRunId;
  activeRunId++;
  activeRunAbort?.abort();
  activeRunAbort = null;
  queuedCutscenes.length = 0;
  onCancelPlayback?.();
  const hadActiveCutscene = isPlaying || currentScriptLabel !== null;

  isPlaying = false;
  currentScriptLabel = null;
  currentCompletionToken = null;
  onInputLock?.(false);
  usePokemonDialogueStore.getState().resetDialogueState();

  for (const listener of Array.from(cancelListeners)) {
    if (listener.runId === cancelledRunId) {
      cancelListeners.delete(listener);
      listener.callback();
    }
  }

  if (hadActiveCutscene) {
    console.log(`[CutsceneService] Cancelled active cutscene: ${reason}`);
  }
}

/** Handle a CutsceneStartNotify from the server. */
export async function handleCutsceneStart(
  payload: CutsceneStartPayload,
): Promise<void> {
  if (isPlaying) {
    // A committed script can publish the next script before its completion
    // result. Preserve that issued payload until current reconciliation ends.
    if (payload.completionToken !== currentCompletionToken && !queuedCutscenes.some((event) => event.completionToken === payload.completionToken)) queuedCutscenes.push(payload);
    return;
  }

  const runId = ++activeRunId;
  const abort = new AbortController();
  activeRunAbort = abort;
  isPlaying = true;
  onInputLock?.(true);
  currentScriptLabel = payload.scriptLabel;
  currentCompletionToken = payload.completionToken;
  lastStartedScriptLabel = payload.scriptLabel;
  console.log(
    `[CutsceneService] Starting cutscene: ${payload.scriptLabel} (${payload.actions.length} actions)`,
  );

  let shouldComplete = true;
  try {
    for (const action of payload.actions) {
      if (!isActiveRun(runId) || !(await executeAction(action, runId))) {
        shouldComplete = false;
        break;
      }
    }
  } catch (err) {
    console.error("[CutsceneService] Error during cutscene:", err);
    shouldComplete = false;
  }

  if (!isActiveRun(runId)) {
    shouldComplete = false;
  }

  if (!shouldComplete && isActiveRun(runId)) onCancelPlayback?.();

  // Animation does not acknowledge durable success. Keep the run/input lock
  // until the correlated commit result and owned-position projection settle.
  let reconciled = false;
  try {
    let position: CutsceneOwnedPosition;
    if (shouldComplete && isActiveRun(runId)) {
      try {
        const result = await completeCutscene(payload.scriptLabel, payload.completionToken, abort.signal);
        if (!isActiveRun(runId)) return;
        position = result;
        if (result.completed) lastCompletedScriptLabel = payload.scriptLabel;
      } catch (error) {
        if (!isActiveRun(runId) || abort.signal.aborted) return;
        console.warn("[CutsceneService] Completion was not confirmed:", error);
        // A correlated failure includes owned state. A timeout/send failure may
        // have committed; read after the command instead of assuming rollback.
        position = error instanceof CorrelatedResponseError
          ? error.response as PlayerStepError
          : await readOwnedPlayerPosition(abort.signal);
      }
    } else {
      if (!isActiveRun(runId)) return;
      // A declined choice/interrupted active playback retires its durable plan
      // before input unlock. Scene/session retirement leaves it for reconnect.
      position = await completeCutscene(payload.scriptLabel, payload.completionToken, abort.signal, true);
    }
    if (!isActiveRun(runId)) return;
    if (!onReconcile) throw new Error("Cutscene position projection unavailable");
    await onReconcile(position);
    if (!isActiveRun(runId)) return;
    reconciled = true;
  } catch (error) {
    if (isActiveRun(runId)) console.warn("[CutsceneService] Could not recover owned position; input remains locked until scene/session retirement:", error);
  } finally {
    removeCancelListenersForRun(runId);
    if (activeRunId === runId && reconciled) {
      activeRunAbort = null;
      isPlaying = false;
      currentScriptLabel = null;
      currentCompletionToken = null;
      onInputLock?.(false);
      const next = queuedCutscenes.shift();
      if (next) void handleCutsceneStart(next);
    }
  }
}

// --- Action Executor ---

async function executeAction(
  action: CutsceneAction,
  runId: number,
): Promise<boolean> {
  if (!isActiveRun(runId)) return false;

  switch (action.type) {
    case "lockInput":
      onInputLock?.(true);
      return true;

    case "unlockInput":
      // Unlock only after completion/reconciliation, including nested actions.
      return true;

    case "dialogue":
    case "dialogueText":
      return showDialogueAction(action, runId);

    case "choice":
      return showChoiceAction(action, runId);

    case "playSFX":
      playSFXAction(action);
      return true;

    case "playMusic":
      playMusicAction(action);
      return true;

    case "playCry":
      playCryAction(action);
      return true;

    case "move":
      if (action.actor && action.movements && onMove) {
        await onMove(action.actor, action.movements);
      }
      return isActiveRun(runId);

    case "parallel":
      if (action.actions && action.actions.length > 0) {
        const results = await Promise.all(
          action.actions.map((nestedAction) =>
            executeAction(nestedAction, runId),
          ),
        );
        return isActiveRun(runId) && results.every(Boolean);
      }
      return true;

    case "showActor":
      if (action.actor && onShowActor) {
        onShowActor(
          action.actor,
          action.x ?? 0,
          action.y ?? 0,
          action.sprite ?? "",
        );
      }
      return true;

    case "hideActor":
      if (action.actor && onHideActor) {
        onHideActor(action.actor);
      }
      return true;

    case "delay":
      return delay(action.ms ?? 500, runId);

    case "facePlayer":
      if (action.actor && action.direction && onFace) {
        onFace(action.actor, action.direction);
      }
      return true;

    case "movePlayer":
      if (action.movements && onMove) {
        await onMove("__PLAYER__", action.movements);
      }
      return isActiveRun(runId);

    case "screenFade":
      // Visual effect only — brief black screen transition
      return delay(action.ms ?? 500, runId);

    case "setFlag":
      // No-op client-side — flags are set server-side on CutsceneEnd
      return true;

    case "resetFlag":
      // No-op client-side — flags are reset server-side on CutsceneEnd
      return true;

    case "toggleFlag":
      // No-op client-side — flags are toggled server-side on CutsceneEnd
      return true;

    case "givePokemon":
      // No-op client-side — Pokemon rewards are persisted server-side on CutsceneEnd
      return true;

    case "healParty":
      // Healing and center policy commit with the durable CutsceneEnd command.
      return true;

    case "giveItem":
    case "takeItem":
      // No-op client-side — inventory is persisted server-side on CutsceneEnd
      return true;

    case "takeMoney":
      // No-op client-side — money is persisted server-side on CutsceneEnd
      return true;

    case "gameCornerPrizeVendor":
      // No-op client-side — the server opens the prize menu after CutsceneEndRequest
      return true;

    case "startSafariSession":
      // No-op client-side — the server starts the paid Safari session after CutsceneEndRequest
      return true;

    case "endSafariSession":
      // No-op client-side — the server ends the Safari session after CutsceneEndRequest
      return true;

    case "hideObject":
      if (action.actorId && onHideObject) {
        onHideObject(action.actorId);
      }
      // Persistent visibility is still applied server-side on CutsceneEnd
      return true;

    case "showObject":
      // No-op client-side — persistent object visibility is applied server-side on CutsceneEnd
      return true;

    case "startTrainerBattle":
    case "startWildBattle":
      // No-op client-side — the server starts battle after CutsceneEndRequest
      return true;

    default:
      console.warn(
        `[CutsceneService] Unknown action type: ${action.type}`,
      );
      return true;
  }
}

// --- Helpers ---

function isActiveRun(runId: number): boolean {
  return isPlaying && activeRunId === runId;
}

function addCancelListener(runId: number, callback: () => void): () => void {
  if (!isActiveRun(runId)) {
    callback();
    return () => {};
  }

  const listener = { runId, callback };
  cancelListeners.add(listener);
  return () => {
    cancelListeners.delete(listener);
  };
}

function removeCancelListenersForRun(runId: number): void {
  for (const listener of Array.from(cancelListeners)) {
    if (listener.runId === runId) {
      cancelListeners.delete(listener);
    }
  }
}

function delay(ms: number, runId: number): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false;
    let offCancel = () => {};
    let timeout: ReturnType<typeof setTimeout> | null = null;
    const finish = (completed: boolean) => {
      if (settled) return;
      settled = true;
      if (timeout) {
        clearTimeout(timeout);
      }
      offCancel();
      resolve(completed && isActiveRun(runId));
    };
    timeout = setTimeout(() => finish(true), ms);
    offCancel = addCancelListener(runId, () => finish(false));
  });
}

function playSFXAction(action: CutsceneAction): void {
  const path = sfxPathForConstant(action.sfxConstant);
  if (path) {
    void AudioManager.playSFX(path, action.volume ?? 0.8);
  }
}

function playMusicAction(action: CutsceneAction): void {
  const path =
    action.musicPath || musicTrackForConstant(action.musicConstant, "");
  if (path) {
    AudioManager.playMusic(path, 0, action.loop ?? true);
  }
}

function playCryAction(action: CutsceneAction): void {
  const path = cryPathForPokemon(
    action.pokemonName ?? action.pokemonConstant,
    action.sfxConstant,
  );
  if (path) {
    void AudioManager.playSFX(path, action.volume ?? 0.8);
  }
}

function showDialogue(
  speaker: string | null,
  lines: string[],
  runId: number,
): Promise<boolean> {
  if (lines.length === 0) return Promise.resolve(isActiveRun(runId));
  const normalizedLines = lines.flatMap((line) => parseDialogueText(line));
  const normalizedSpeaker = resolveDialogueSpeaker(speaker);

  return new Promise<boolean>((resolve) => {
    let settled = false;
    let offCancel = () => {};
    const finish = (completed: boolean) => {
      if (settled) return;
      settled = true;
      offCancel();
      resolve(completed && isActiveRun(runId));
    };

    usePokemonDialogueStore
      .getState()
      .openDialogue(normalizedLines, normalizedSpeaker, undefined, () => {
        finish(true);
      });

    offCancel = addCancelListener(runId, () => finish(false));
  });
}

async function showDialogueAction(
  action: CutsceneAction,
  runId: number,
): Promise<boolean> {
  let lines = action.lines ?? [];
  if (action.textConstant) {
    const fetched = await fetchDialogue(action.textConstant);
    if (!isActiveRun(runId)) return false;
    if (fetched.length > 0) {
      lines = fetched;
    }
  }
  return showDialogue(action.speaker ?? null, lines, runId);
}

function showChoiceAction(
  action: CutsceneAction,
  runId: number,
): Promise<boolean> {
  const prompt =
    action.prompt ??
    action.lines?.join("\n") ??
    "Do you want this?";
  const resolvedPrompt = resolveDialoguePlaceholders(prompt);

  return new Promise<boolean>((resolve) => {
    let settled = false;
    let offCancel = () => {};
    const finish = (result: boolean) => {
      if (settled) return;
      settled = true;
      offCancel();
      resolve(result && isActiveRun(runId));
    };
    offCancel = addCancelListener(runId, () => finish(false));

    usePokemonDialogueStore.getState().showChoice(resolvedPrompt, (yes) => {
      if (!isActiveRun(runId)) {
        finish(false);
        return;
      }
      if (yes && action.yesLines && action.yesLines.length > 0) {
        showDialogue(action.speaker ?? null, action.yesLines, runId).then(
          (completed) => finish(completed && !action.stopOnYes),
        );
        return;
      }
      if (!yes && action.noLines && action.noLines.length > 0) {
        showDialogue(action.speaker ?? null, action.noLines, runId).then(
          (completed) => finish(completed && Boolean(action.continueOnNo)),
        );
        return;
      }
      finish(
        (yes && !action.stopOnYes) || (!yes && Boolean(action.continueOnNo)),
      );
    });
  });
}
