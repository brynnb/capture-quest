import { OpCodes, WorldSocket } from "@/net";
import type { BattleCommandResponse, BattleEndOutcome, PokeBattleActionRequest, PokeBattleSwitchRequest, PokeMoveLearnRequest } from "@/net/generated/world_api";
import type { OwnedPlayerPositionResponse } from "@/net/generated/protocol";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import AudioManager from "@/services/audio/AudioManager";
import { victoryMusicTrackForState, sfxPathForConstant } from "@/services/audio/pokemonMusic";
import { correlatedRequest } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { readCurrentGameplayState, applyGameplaySnapshot } from "./GameplayRecoveryService";

type TurnCommand = Omit<PokeBattleActionRequest, "battle" | "requestId">;
type SwitchCommand = Omit<PokeBattleSwitchRequest, "battle" | "requestId">;
type LearnCommand = Omit<PokeMoveLearnRequest, "battle" | "requestId">;
type Projection = (position: OwnedPlayerPositionResponse) => Promise<void>;
let sceneProjection: Projection | null = null;
let active: AbortController | null = null;

// The scene owns both projection and retirement. An older cleanup must never
// clear a newer binding or let a late command reply revive a retired panel.
export function bindBattleScene(reconcile: Projection): () => void {
  active?.abort();
  sceneProjection = reconcile;
  return () => {
    if (sceneProjection !== reconcile) return;
    sceneProjection = null;
    active?.abort();
  };
}

export function presentBattleEnd(end: BattleEndOutcome): void {
  const state = usePokeBattleStore.getState();
  if (end.playerWon) {
    const track = victoryMusicTrackForState(state.battleType, state.trainerClass);
    useAudioActivityStore.getState().setBattleVictoryTrack(track);
    if (track) AudioManager.playMusic(track);
  }
  const blackout = !end.playerWon && end.blackoutMapId > 0
    ? { mapId: end.blackoutMapId, x: end.blackoutX, y: end.blackoutY }
    : undefined;
  state.endBattle(end.playerWon, blackout, end.sentToPC, end.pcBox, end.lossMessage);
}

async function sendBattleCommand(opcode: number, responseOpcode: number, command: TurnCommand | SwitchCommand | LearnCommand | Record<string, never>): Promise<void> {
  if (active) return;
  const initial = usePokeBattleStore.getState();
  const { battleId, revision, presentationGeneration } = initial;
  const project = sceneProjection;
  if (!initial.isInBattle || initial.isSafari || !battleId || !Number.isSafeInteger(revision) || revision < 1 || !project) {
    usePokeBattleStore.setState({ commandError: "Could not identify the current battle. Please reconnect." });
    return;
  }
  const controller = new AbortController();
  active = controller;
  let applying = false;
  const current = () => !controller.signal.aborted && sceneProjection === project && usePokeBattleStore.getState().presentationGeneration === presentationGeneration;
  const unsubscribe = usePokeBattleStore.subscribe(state => {
    if (!applying && (state.presentationGeneration !== presentationGeneration || !state.isInBattle || state.battleId !== battleId)) controller.abort();
  });
  usePokeBattleStore.setState({ battleCommandPending: true, commandError: null, phase: "animating", eventQueue: [], currentEventIndex: 0 });
  try {
    const response = await correlatedRequest<BattleCommandResponse>(
      receive => PhaserNet.onBattleCommand(responseOpcode, receive),
      requestId => WorldSocket.sendStreamJsonMessage(opcode, { ...command, battle: { battleId, revision }, requestId }),
      controller.signal,
    );
    if (!current()) return;
    if (opcode === OpCodes.PokeBattleCloseRequest) {
      if (response.battle !== null) throw new Error("Battle close returned an active battle");
      applying = true;
      usePokeBattleStore.getState().retireBattle();
      await project(response.position);
      return;
    }
    const battle = response.battle;
    if (!battle || battle.battleId !== battleId || battle.revision < revision || battle.revision > revision + 1) {
      throw new Error("Battle reply differs from the requested durable identity");
    }
    let events = response.events;
    if (response.learning) {
      const path = sfxPathForConstant("SFX_GET_ITEM_1");
      if (path) void AudioManager.playSFX(path, 0.75);
      events = [{ type: "message", message: response.learning.message, targetHp: 0, targetMaxHp: 0 }, ...(response.learning.postEvents ?? [])];
    }
    usePokeBattleStore.getState().updateBattleState({ ...battle, events });
    if (response.end) presentBattleEnd(response.end);
  } catch (error) {
    if (!current()) return;
    // A timeout, rejection or failed send does not prove rollback. Read current
    // authority after the command; never resend a mutation with a fresh revision.
    try {
      const snapshot = await readCurrentGameplayState(controller.signal);
      if (!current()) return;
      applying = true;
      applyGameplaySnapshot(snapshot);
      await project(snapshot.position);
    } catch (recoveryError) {
      if (controller.signal.aborted || sceneProjection !== project) return;
      console.warn("[BattleCommand] Current-state recovery failed:", recoveryError);
      usePokeBattleStore.setState({ commandError: "Could not restore your battle. Please reconnect." });
    }
  } finally {
    unsubscribe();
    if (active === controller) {
      active = null;
      usePokeBattleStore.setState({ battleCommandPending: false });
    }
  }
}

export const sendBattleAction = (command: TurnCommand): Promise<void> => sendBattleCommand(OpCodes.PokeBattleActionRequest, OpCodes.PokeBattleActionResponse, command);
export const sendBattleSwitch = (command: SwitchCommand): Promise<void> => sendBattleCommand(OpCodes.PokeBattleSwitchRequest, OpCodes.PokeBattleSwitchResponse, command);
export const sendMoveLearningChoice = (command: LearnCommand): Promise<void> => sendBattleCommand(OpCodes.PokeMoveLearnRequest, OpCodes.PokeMoveLearnResponse, command);
export const closeOrdinaryBattle = (): Promise<void> => sendBattleCommand(OpCodes.PokeBattleCloseRequest, OpCodes.PokeBattleCloseResponse, {});
