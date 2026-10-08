import { OpCodes, WorldSocket } from "@/net";
import type { CQMerchantOpenResponse, CQMerchantBuyResponse, CQMerchantSellResponse, CQPartyItemUseResponse, RepelUseResponse, PokemonPartyReorderResponse } from "@/net/generated/world_api";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import { correlatedRequest, CorrelatedResponseError } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { readCurrentGameplayState, applyGameplayResourceSnapshot } from "./GameplayRecoveryService";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

let scene: symbol | null = null;
let active: AbortController | null = null;
let resourcesDirtyCharacter=0;
type Reply = import("@/net/generated/world_api").GameplayStateResponse | import("@/net/generated/world_api").PokemonPCResponse | CQMerchantOpenResponse | CQMerchantBuyResponse | CQMerchantSellResponse | CQPartyItemUseResponse | RepelUseResponse | PokemonPartyReorderResponse;

export function bindInventoryScene(): () => void {
  resourcesDirtyCharacter=0;
  active?.abort(); active = null;
  useCQInventoryStore.getState().closeShop();
  usePokemonPCStore.getState().closePC();
  useCQInventoryStore.setState({ inventoryCommandPending: false, pendingTMHM: null });
  const owner = Symbol("inventory scene"); scene = owner;
  return () => {
    if (scene !== owner) return;
    scene = null; active?.abort(); active = null;
    resourcesDirtyCharacter=0;
    useCQInventoryStore.getState().closeShop();
    usePokemonPCStore.getState().closePC();
    useCQInventoryStore.setState({ inventoryCommandPending: false, pendingTMHM: null });
  };
}

function views() {
  const bag = useCQInventoryStore.getState();
  const pc = usePokemonPCStore.getState();
  return [bag.items, bag.money, bag.commandRevision, usePlayerCharacterStore.getState().characterProfile, usePokemonPartyStore.getState().party, pc.boxPokemon, pc.currentBox, pc.sources];
}

// Source-bound menus are a view of the interaction at one owned location.
// Retiring presentation never cancels a sent mutation; the coordinator below
// still reconciles its commit. Shops and PCs share this location watcher.
export function watchInteractionPosition(retire: () => void, includeFacing = false): () => void {
  const origin = useGameStatusStore.getState().playerTileContext;
  let retired=false;
  return useGameStatusStore.subscribe(state => {
    const current = state.playerTileContext;
    if (!retired && (current?.mapId !== origin?.mapId || current?.x !== origin?.x || current?.y !== origin?.y || (includeFacing && current?.direction !== origin?.direction))) { retired=true; retire(); }
  });
}

// One owner for admission, correlation, cancellation, stale replies and recovery.
// Domain adapters validate/apply their projection and present effects; only this
// layer applies the bag. Projection application outlives a closed presentation.
export async function runInventoryRequest<T extends Reply>(options: {
  mutation: boolean;
  watchPresentation?: (retire: () => void) => () => void;
  validate?: (reply: T) => void;
  apply?: (reply: T) => void;
  present: (reply: T) => void;
  readError?: string;
  recoveryMessage?: string;
} & ({read: (signal: AbortSignal) => Promise<T>; mutation: false; opcode?: never; responseOpcode?: never; payload?: never} | {read?: never; opcode: number; responseOpcode: number; payload: Record<string,unknown>})): Promise<void> {
  if (active || !scene) return;
  const owner = scene;
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  const revision = useCQInventoryStore.getState().commandRevision;
  if (!characterId || !Number.isSafeInteger(revision) || revision < 0) return;
  const initial = views();
  const controller = new AbortController(); active = controller;
  const current = () => !controller.signal.aborted && scene === owner && usePlayerCharacterStore.getState().characterProfile.id === characterId;
  const stopProfile = usePlayerCharacterStore.subscribe(state => {
    if (state.characterProfile.id !== characterId) { resourcesDirtyCharacter=0; controller.abort(); useCQInventoryStore.getState().closeShop(); }
  });
  let presentationCurrent = true;
  const stopPresentation = options.watchPresentation?.(() => {
    presentationCurrent = false;
    // Closing a view cannot undo a sent mutation. Keep its reply/recovery and
    // admission slot alive; only character/scene retirement cancels ownership.
    if (!options.mutation) controller.abort();
  });
  useCQInventoryStore.setState({ inventoryCommandPending: true, inventoryCommandError: null });
  try {
    const reply = options.read ? await options.read(controller.signal) : await correlatedRequest<T>(
      receive => PhaserNet.onInventoryCommand(options.responseOpcode, value => receive(value as T)),
      requestId => WorldSocket.sendStreamJsonMessage(options.opcode, {
        ...options.payload, requestId,
        ...(options.mutation ? {command: {characterId, revision}} : {characterId}),
      }), controller.signal,
    );
    if (!current()) return;
    if (views().some((view, i) => view !== initial[i])) throw new Error("Inventory reply overtaken by newer state");
    options.validate?.(reply);
    if (options.mutation) {
      if (!("inventory" in reply) || Array.isArray(reply.inventory)) throw new Error("Invalid mutation response kind");
      if (!Array.isArray(reply.inventory?.items)
        || reply.inventory.commandRevision !== revision + 1 || !Number.isSafeInteger(reply.inventory.money)
        || reply.inventory.money < 0 || reply.inventory.money > 0xffffffff) throw new Error("Invalid committed inventory response");
      useCQInventoryStore.getState().setInventory(reply.inventory.items, reply.inventory.money, reply.inventory.commandRevision);
      usePlayerCharacterStore.getState().handleCharacterWalletData({characterId, pokedollars: reply.inventory.money});
    }
    options.apply?.(reply);
    if (presentationCurrent) options.present(reply);
  } catch (error) {
    if (!current()) return;
    if (!options.mutation) {
      reportError(options.readError ?? "Could not read inventory state. Please try again.");
      return;
    }
    // Even rejection cannot prove rollback after a lost earlier reply. Read
    // current authority once; never resend the mutation or apply its late reply.
    try {
      const snapshot = await readCurrentGameplayState(controller.signal);
      if (!current()) return;
      applyGameplayResourceSnapshot(snapshot);
      useCQInventoryStore.getState().setPendingTMHM(null);
      useChatStore.getState().addMessage(error instanceof CorrelatedResponseError ? error.message : (options.recoveryMessage ?? "Inventory state refreshed. Check your bag and party before trying again."), MessageType.SYSTEM);
    } catch {
      if (current()) reportError("Could not restore inventory state. Please reconnect.");
    }
  } finally {
    stopProfile(); stopPresentation?.();
    if (active === controller) { active = null; useCQInventoryStore.setState({inventoryCommandPending: false}); }
    flushResourceChanges();
  }
}

// Explicit resource reads use the same scene/admission/cancellation owner and
// locked current-state endpoint as mutation recovery, without replaying plans.
export function refreshOwnedGameplayResources(): Promise<void> {
 return runInventoryRequest<import("@/net/generated/world_api").GameplayStateResponse>({
  read: readCurrentGameplayState, mutation:false,
  apply:applyGameplayResourceSnapshot, present:()=>{},
  readError:"Could not refresh current party and inventory. Please try again.",
 });
}

// Notices never apply their payload. Burst notices coalesce behind the same
// admission slot; scene/character retirement discards the old owner's work.
export function acceptResourceChangeNotification(value: unknown): void {
  if (!value || typeof value!=="object") return;
  const notice=value as {success?:boolean;resourcesChanged?:boolean;characterId?:number};
  if (!scene || notice.success!==true || notice.resourcesChanged!==true
    || notice.characterId!==usePlayerCharacterStore.getState().characterProfile.id) return;
  resourcesDirtyCharacter=notice.characterId!; flushResourceChanges();
}
function flushResourceChanges(): void {
  if (!resourcesDirtyCharacter || active || !scene) return;
  if (resourcesDirtyCharacter!==usePlayerCharacterStore.getState().characterProfile.id) {resourcesDirtyCharacter=0;return;}
  resourcesDirtyCharacter=0;
  void refreshOwnedGameplayResources();
}

function reportError(message: string) {
  useCQInventoryStore.setState({inventoryCommandError: message});
  useChatStore.getState().addMessage(message, MessageType.SYSTEM);
}

export function sendPartyReorderCommand(pokemonIds: readonly (number | undefined)[]): Promise<void> {
  if (pokemonIds.length < 1 || pokemonIds.length > 6
    || !pokemonIds.every((id): id is number => id !== undefined && Number.isSafeInteger(id) && id > 0)
    || new Set(pokemonIds).size !== pokemonIds.length) {
    reportError("Party identity is unavailable. Please reconnect before reordering.");
    return Promise.resolve();
  }
  const ids = [...pokemonIds];
  return runInventoryRequest<PokemonPartyReorderResponse>({
    opcode: OpCodes.PokemonPartyReorderRequest, responseOpcode: OpCodes.PokemonPartyReorderResponse,
    payload: {pokemonIds: ids}, mutation: true,
    validate: reply => {
      if (!Array.isArray(reply.party) || reply.party.length !== ids.length
        || reply.party.some((pokemon, i) => pokemon.rowId !== ids[i])) throw new Error("Invalid reordered party");
    },
    apply: reply => usePokemonPartyStore.getState().setParty(reply.party),
    present: () => {
      const sound = sfxPathForConstant("SFX_PRESS_AB");
      if (sound) void AudioManager.playSFX(sound, 0.55);
    },
  });
}

export function sendRepelItemCommand(instanceId: number): Promise<void> {
  return runInventoryRequest<RepelUseResponse>({
    opcode: OpCodes.RepelUseRequest, responseOpcode: OpCodes.RepelUseResponse,
    payload: {instanceId}, mutation: true,
    watchPresentation: retire => useGameStatusStore.subscribe((state, previous) => {
      if (previous.isInventoryOpen && !state.isInventoryOpen) retire();
    }),
    validate: reply => {
      if (reply.instanceId !== instanceId || typeof reply.message !== "string"
        || !Number.isSafeInteger(reply.stepsLeft) || reply.stepsLeft <= 0) throw new Error("Invalid repel result");
    },
    present: reply => {
      useChatStore.getState().addMessage(reply.message, MessageType.SYSTEM);
      const sound = sfxPathForConstant("SFX_PRESS_AB");
      if (sound) void AudioManager.playSFX(sound, 0.75);
    },
  });
}

export function sendPartyItemCommand(instanceId: number, partySlot: number, moveSlot = -1, pokemonRowId = usePokemonPartyStore.getState().party[partySlot]?.rowId ?? 0): Promise<void> {
  return runInventoryRequest<CQPartyItemUseResponse>({
    opcode: OpCodes.CQItemUseRequest, responseOpcode: OpCodes.CQItemUseResponse,
    payload: {instanceId, partySlot, moveSlot, pokemonRowId}, mutation: true,
    validate: reply => {
      if (!Array.isArray(reply.party) || !reply.outcome || reply.outcome.instanceId !== instanceId
        || reply.outcome.partySlot !== partySlot || typeof reply.outcome.message !== "string"
        || (partySlot >= 0 && reply.party[partySlot]?.rowId !== pokemonRowId)) throw new Error("Invalid party-item result");
    },
    apply: reply => usePokemonPartyStore.getState().setParty(reply.party),
    present: reply => {
      const outcome = reply.outcome;
      const bag = useCQInventoryStore.getState();
      bag.setPendingTMHM(outcome.needsMoveSlot ? {
        instanceId, partySlot, pokemonRowId,
        itemName: bag.items.find(item => item.instance.id === instanceId)?.item.name ?? "TM/HM",
        moveId: outcome.moveId, moveName: outcome.moveName, message: outcome.message,
      } : null);
      useChatStore.getState().addMessage(outcome.message, MessageType.SYSTEM);
      const sound = sfxPathForConstant(outcome.needsMoveSlot ? "SFX_PRESS_AB" : "SFX_HEAL_HP");
      if (sound) void AudioManager.playSFX(sound, 0.8);
    },
  });
}
