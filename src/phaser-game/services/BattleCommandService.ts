import {acceptOwnedSafariVisit,claimSafariExit,isCurrentSafariExit,acceptSafariVisitNotice} from "./SafariVisitService";
import { OpCodes, WorldSocket } from "@/net";
import type { BattleCommandResponse, SafariBattleActionResponse, SafariBattleActionRequest, BattleEndOutcome, PokeBattleActionRequest, PokeBattleSwitchRequest, PokeMoveLearnRequest } from "@/net/generated/world_api";
import type { OwnedPlayerPositionResponse } from "@/net/generated/protocol";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import type {BattleEvent} from "@/net/generated/battle_events";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import useChatStore, {MessageType} from "@/stores/ChatStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import AudioManager from "@/services/audio/AudioManager";
import { victoryMusicTrackForState, sfxPathForConstant, cryPathForPokemon } from "@/services/audio/pokemonMusic";
import { correlatedRequest, readForCurrentCharacter } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { readCurrentGameplayState, applyGameplaySnapshot, applyGameplayResourceSnapshot } from "./GameplayRecoveryService";

type TurnCommand = Omit<PokeBattleActionRequest, "battle" | "requestId">;
type SwitchCommand = Omit<PokeBattleSwitchRequest, "battle" | "requestId">;
type LearnCommand = Omit<PokeMoveLearnRequest, "battle" | "requestId">;
type Projection = (position: OwnedPlayerPositionResponse) => Promise<void>;
let sceneProjection: Projection | null = null;
let scenePositionView: (()=>unknown)|undefined;
let active: AbortController | null = null;
let publicationRead: AbortController | null = null;

// The scene owns both projection and retirement. An older cleanup must never
// clear a newer binding or let a late command reply revive a retired panel.
export function bindBattleScene(reconcile: Projection, capturePositionView?:()=>unknown): () => void {
  active?.abort();
  publicationRead?.abort();publicationRead=null;
  // Scene retirement releases admission immediately. The old projection may
  // still be settling; its identity-guarded finally cannot retire a newer slot.
  active = null;
  sceneProjection = reconcile;
  scenePositionView = capturePositionView;
  return () => {
    if (sceneProjection !== reconcile) return;
    sceneProjection = null;
    scenePositionView = undefined;
    publicationRead?.abort();publicationRead=null;
    active?.abort();
    active = null;
  };
}

// Unsolicited battle publication is a hint, not authority for a panel or warp.
// All producers use the same owned read; commands are never resent here.
export async function recoverBattlePublication(notice:Record<string,unknown>,kind:"ordinary-start"|"safari-start"|"standalone-end"|"safari-visit"|"safari-exit"="ordinary-start"):Promise<void>{
 if((kind==="ordinary-start" && notice.success!==true) || !sceneProjection || useGameScreenStore.getState().currentScreen!=="game")return;
 const initial=usePokeBattleStore.getState();
 if(initial.battleCommandPending)return;
 publicationRead?.abort();const controller=new AbortController();publicationRead=controller;
 const project=sceneProjection;
 const positionView=scenePositionView;
 let positionGeneration=positionView?.();
 const generation=WorldSocket.sessionGeneration;
 const characterId=usePlayerCharacterStore.getState().characterProfile.id;
 const presentation=initial.presentationGeneration;
 const current=(allowPositionChange=false)=>((allowPositionChange || kind!=="standalone-end" && kind!=="safari-exit") || positionView?.()===positionGeneration) && !controller.signal.aborted && publicationRead===controller && sceneProjection===project && WorldSocket.sessionGeneration===generation && usePlayerCharacterStore.getState().characterProfile.id===characterId && useGameScreenStore.getState().currentScreen==="game" && usePokeBattleStore.getState().presentationGeneration===presentation && !usePokeBattleStore.getState().battleCommandPending;
 try {
 let snapshot=await readForCurrentCharacter((_id,signal)=>readCurrentGameplayState(signal,(kind==="standalone-end" || kind==="safari-exit") ? positionView : undefined),controller.signal);
 if(kind==="safari-exit" && !current() && current(true)){
 // The final step acknowledgement can settle while expiry is read. Retry the
 // read once from the new movement view; never reuse the older pose snapshot.
 positionGeneration=positionView?.();
 snapshot=await readForCurrentCharacter((_id,signal)=>readCurrentGameplayState(signal,positionView),controller.signal);
 }
 if(!current())return;
 acceptOwnedSafariVisit(snapshot.safari);
 if(kind==="safari-visit")return;
 if(kind==="safari-exit"){
 const visit=snapshot.safari;
 if(snapshot.battle || !visit || !claimSafariExit(visit))return;
 applyGameplayResourceSnapshot(snapshot);
 const callbackCurrent=()=>isCurrentSafariExit(visit) && !controller.signal.aborted && sceneProjection===project && WorldSocket.sessionGeneration===generation && usePlayerCharacterStore.getState().characterProfile.id===characterId && useGameScreenStore.getState().currentScreen==="game" && positionView?.()===positionGeneration;
 // Retire local presentation without sending another Safari close mutation.
 usePokeBattleStore.getState().restoreGameplay(snapshot);
 usePokemonDialogueStore.getState().openDialogue([visit.exitMessage!],null,undefined,()=>{if(callbackCurrent())void project(snapshot.position);});
 return;
 }
 const state=usePokeBattleStore.getState();
 const owned=snapshot.battle;
 // Duplicate hints preserve current event queues. An end hint can still need
 // current resources/pose, even when battle presentation already matches.
 const duplicate=owned
 ? state.isInBattle && state.battleId===owned.battleId && state.revision>=owned.revision
 : !!snapshot.safari?.pokemon && state.isSafari && state.battleId===snapshot.safari.battleId && typeof snapshot.safari.revision==="number" && state.revision>=snapshot.safari.revision;
 if(!duplicate && (owned || snapshot.safari?.pokemon || state.isInBattle)){
 const events=kind==="ordinary-start" && owned && notice.battleId===owned.battleId && notice.revision===owned.revision && !owned.needsDismissal && Array.isArray(notice.events)
 ? notice.events as BattleEvent[] : [];
 state.restoreGameplay(snapshot,events);
 if(kind!=="standalone-end" && owned){const path=cryPathForPokemon(owned.enemyPokemon.name,owned.enemyPokemon.crySfx);if(path)void AudioManager.playSFX(path,0.8);}
 }
 if(kind==="standalone-end"){
 // No carrier destination is trusted. Synchronous publication precedes the
 // scene-owned projection; there is no later write from this retired binding.
 applyGameplayResourceSnapshot(snapshot);
 await project(snapshot.position);
 }
 }catch(error){
 if(current() && !(error instanceof DOMException && error.name==="AbortError"))useChatStore.getState().addMessage("Battle state could not be refreshed. Reconnect to recover it.",MessageType.SYSTEM_ERROR);
 }finally{if(publicationRead===controller)publicationRead=null;}
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

async function sendBattleCommand(opcode: number, responseOpcode: number, command: TurnCommand | SwitchCommand | LearnCommand | Omit<SafariBattleActionRequest, "battle" | "requestId"> | Record<string, never>): Promise<void> {
  if (active) return;
  const initial = usePokeBattleStore.getState();
  const { battleId, revision, presentationGeneration } = initial;
  const project = sceneProjection;
  const safariCommand = opcode === OpCodes.SafariBattleActionRequest;
  if (!initial.isInBattle || initial.isSafari !== safariCommand || !battleId || !Number.isSafeInteger(revision) || revision < 1 || !project) {
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
  // Do not retain a retired scene's store callback while its projection settles.
  controller.signal.addEventListener("abort", unsubscribe, { once: true });
  usePokeBattleStore.setState({ battleCommandPending: true, commandError: null, phase: "animating", eventQueue: [], currentEventIndex: 0 });
  try {
    const response = await correlatedRequest<BattleCommandResponse | SafariBattleActionResponse>(
      receive => PhaserNet.onBattleCommand(responseOpcode, receive),
      requestId => WorldSocket.sendStreamJsonMessage(opcode, { ...command, battle: { battleId, revision }, requestId }),
      controller.signal,
    );
    if (!current()) return;
    if (safariCommand && !("battleId" in response)) throw new Error("Missing Safari response identity");
    if ("battleId" in response) {
      if (!safariCommand || response.battleId !== battleId || response.revision !== revision + 1) throw new Error("Safari reply differs from requested identity");
      if (Boolean(response.closed) !== ("action" in command && command.action === "close")) throw new Error("Safari response differs from requested action");
      if (response.closed) {
        applying = true;
        usePokeBattleStore.getState().retireBattle();
      } else {
        usePokeBattleStore.setState({ revision: response.revision });
        if (response.playerParty) usePokemonPartyStore.getState().setParty(response.playerParty);
        usePokeBattleStore.getState().updateSafariState({ ...response, events: response.events.map(event => ({ ...event, targetHp: 0, targetMaxHp: 0 })) });
      }
      acceptSafariVisitNotice({...response,active:!response.safariOver});
      await project(response.position);
      if (response.closed && response.safariOver && response.exitMessage && !controller.signal.aborted && sceneProjection === project) {
        usePokemonDialogueStore.getState().openDialogue([response.exitMessage], null);
      }
      return;
    }
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
      // A failed dismissal must not turn recovery into an automatic retry loop.
      // Preserve the durable terminal battle for a later reconnect instead.
      if (opcode === OpCodes.PokeBattleCloseRequest && snapshot.battle?.needsDismissal) {
        throw new Error("Battle dismissal did not commit");
      }
      if (safariCommand && "action" in command && command.action === "close" && snapshot.safari?.pokemon) throw new Error("Safari dismissal did not commit");
      applying = true;
      applyGameplaySnapshot(snapshot);
      // Restoration resets presentation state. Keep this operation pending until
      // projection and coordinator retirement finish, so terminal auto-dismissal
      // cannot run while the preceding command still owns the single-flight slot.
      usePokeBattleStore.setState({ battleCommandPending: true });
      await project(snapshot.position);
      // Close may have committed even when its acknowledgement was lost. Keep
      // the server's previously recovered terminal message across store retirement;
      // opening it here follows confirmed absence and never initiates another warp.
      if (safariCommand && "action" in command && command.action === "close" && initial.safariExitMessage && !snapshot.battle && !snapshot.safari?.pokemon && !controller.signal.aborted && sceneProjection === project) {
        usePokemonDialogueStore.getState().openDialogue([initial.safariExitMessage], null);
      }
    } catch (recoveryError) {
      if (controller.signal.aborted || sceneProjection !== project) return;
      console.warn("[BattleCommand] Current-state recovery failed:", recoveryError);
      usePokeBattleStore.setState({ commandError: "Could not restore your battle. Please reconnect." });
    }
  } finally {
    controller.signal.removeEventListener("abort", unsubscribe);
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
export const sendSafariAction = (action: string): Promise<void> => sendBattleCommand(OpCodes.SafariBattleActionRequest, OpCodes.SafariBattleActionResponse, { action });
export const closeSafariBattle = (): Promise<void> => sendSafariAction("close");
