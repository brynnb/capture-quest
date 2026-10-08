import type { GameplayStateResponse } from "@/net/generated/world_api";
import { OpCodes } from "@/net";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePokemonPCStore, { validatePCSnapshot } from "@/stores/PokemonPCStore";
import { correlatedRequest } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { handleCutsceneStart } from "./CutsceneService";

export async function readGameplayState(mapId: number, signal?: AbortSignal): Promise<GameplayStateResponse> {
  const snapshot = await correlatedRequest<GameplayStateResponse>(PhaserNet.onGameplayState, requestId => PhaserNet.requestGameplayState({ mapId, requestId }), signal);
  if (signal?.aborted) throw new DOMException("Scene retired", "AbortError");
  validateGameplaySnapshot(snapshot);
  if (snapshot.position.mapId !== mapId) throw new Error("Gameplay recovery map differs from loaded ownership");
  return snapshot;
}

// Mutation recovery cannot assume the pre-command map after a committed blackout.
// This uses the same locked snapshot without the position endpoint's independent
// pending-plan redelivery. The caller still owns application and cancellation.
export async function readCurrentGameplayState(signal?: AbortSignal): Promise<GameplayStateResponse> {
  for (let attempt = 0; attempt < 2; attempt++) {
    const before = captureGameplayViews();
    const snapshot = await correlatedRequest<GameplayStateResponse>(PhaserNet.onGameplayState, requestId => PhaserNet.requestGameplayState({ current: true, requestId }), signal);
    if (signal?.aborted) throw new DOMException("Scene retired", "AbortError");
    validateGameplaySnapshot(snapshot);
    if (gameplayViewsChanged(before)) continue;
    return snapshot;
  }
  throw new Error("Gameplay changed during recovery; a new recovery read is required");
}

export function applyGameplaySnapshot(snapshot: GameplayStateResponse): void {
  applyGameplayResourceSnapshot(snapshot);
  usePlayerCharacterStore.getState().setEventFlags(snapshot.eventFlags);
  usePokeBattleStore.getState().restoreGameplay(snapshot);
  window.dispatchEvent(new CustomEvent("safariZoneEnter", { detail: snapshot.safari?.active
    ? { success: true, ballsLeft: snapshot.safari.ballsLeft, stepsLeft: snapshot.safari.stepsLeft }
    : { success: false } }));
  if (snapshot.trainer) PhaserNet.dispatchPhaserResponse(OpCodes.TrainerEncounterNotify, snapshot.trainer);
  if (snapshot.cutscene) void handleCutsceneStart(snapshot.cutscene);
}

// One resource projection; callers retain battle/movement/plan presentation.
export function applyGameplayResourceSnapshot(snapshot: GameplayStateResponse): void {
  validateGameplaySnapshot(snapshot);
  useCQInventoryStore.getState().setInventory(snapshot.inventory, snapshot.wallet.pokedollars, snapshot.commandRevision);
  usePlayerCharacterStore.getState().handleCharacterWalletData(snapshot.wallet);
  usePokemonPartyStore.getState().setParty(snapshot.party);
  usePokemonPCStore.getState().applySnapshot(snapshot.pc, snapshot.party);
}

// Apply only a correlated, source-matching read owned by this scene. No handler
// applies arbitrary recovery packets globally; retirement aborts the listener.
export async function recoverGameplayState(mapId: number, signal?: AbortSignal): Promise<GameplayStateResponse> {
  for (let attempt = 0; attempt < 2; attempt++) {
    const before = captureGameplayViews();
    const snapshot = await readGameplayState(mapId, signal);
    // A newer gameplay notification can overtake a read while its reply is delayed.
    if (gameplayViewsChanged(before)) continue;
    applyGameplaySnapshot(snapshot);
    return snapshot;
  }
  throw new Error("Gameplay changed during recovery; a new recovery read is required");
}


// Any participating notification can overtake a delayed read, not only a battle
// turn. Compare immutable store states and retry a read, never a mutation.
function captureGameplayViews() {
  const inventory = useCQInventoryStore.getState();
  const pc = usePokemonPCStore.getState();
  return [usePokeBattleStore.getState(), inventory.items, inventory.money, inventory.commandRevision, usePlayerCharacterStore.getState().characterProfile, usePokemonPartyStore.getState().party, pc.boxPokemon, pc.currentBox, pc.sources];
}
function gameplayViewsChanged(before: ReturnType<typeof captureGameplayViews>) {
  return captureGameplayViews().some((view, index) => view !== before[index]);
}


function validateGameplaySnapshot(snapshot: GameplayStateResponse): void {
  validatePCSnapshot(snapshot.pc);
  if (snapshot.pc.sources.some(source => source.mapId !== snapshot.position.mapId)) throw new Error("PC source belongs to another map");
  if (!Array.isArray(snapshot.inventory) || !Array.isArray(snapshot.party) || !Array.isArray(snapshot.eventFlags)
    || snapshot.eventFlags.some(flag => typeof flag !== "string" || !flag)
    || !Number.isSafeInteger(snapshot.commandRevision) || snapshot.commandRevision < 0
    || !Number.isSafeInteger(snapshot.wallet?.characterId) || snapshot.wallet.characterId <= 0
    || !Number.isSafeInteger(snapshot.wallet.pokedollars) || snapshot.wallet.pokedollars < 0) {
    throw new Error("Incomplete owned gameplay snapshot; reconnect to a matching server");
  }
  const characterId = usePlayerCharacterStore.getState().characterProfile?.id;
  if (characterId !== undefined && characterId !== snapshot.wallet.characterId) throw new Error("Gameplay snapshot belongs to another character");
}
