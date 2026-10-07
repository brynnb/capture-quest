import { OpCodes } from "@/net";
import type { PokemonPCResponse } from "@/net/generated/world_api";
import usePokemonPCStore, { validatePCSnapshot } from "@/stores/PokemonPCStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import { runInventoryRequest } from "./InventoryCommandService";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

const watchPC = (retire: () => void) => usePokemonPCStore.subscribe((state, previous) => {
  if (previous.isOpen && !state.isOpen) retire();
});

function requestPC(opcode: number, responseOpcode: number, sourceId: number, payload: Record<string, number>, mutation: boolean): Promise<void> {
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  if (!characterId || !Number.isSafeInteger(sourceId) || sourceId <= 0) return Promise.resolve();
  return runInventoryRequest<PokemonPCResponse>({
    opcode, responseOpcode, payload: {...payload,sourceId}, mutation, watchPresentation: watchPC,
    validate: reply => {
      validatePCSnapshot(reply.pc);
      if (reply.characterId !== characterId || reply.sourceId !== sourceId || !Array.isArray(reply.party)
        || (mutation && reply.pc.currentBox !== payload.box)
        || !reply.pc.sources.some(source => source.id === sourceId)
        || !Array.isArray(reply.inventory?.items) || !Number.isSafeInteger(reply.inventory.commandRevision) || reply.inventory.commandRevision < 0
        || !Number.isSafeInteger(reply.inventory.money) || reply.inventory.money < 0 || reply.inventory.money > 0xffffffff) throw new Error("Invalid PC result");
    },
    apply: reply => {
      if (!mutation) {
        useCQInventoryStore.getState().setInventory(reply.inventory.items,reply.inventory.money,reply.inventory.commandRevision);
        usePlayerCharacterStore.getState().handleCharacterWalletData({characterId,pokedollars:reply.inventory.money});
      }
      usePokemonPartyStore.getState().setParty(reply.party);
      usePokemonPCStore.getState().applySnapshot(reply.pc,reply.party);
    },
    present: reply => {
      if (!mutation) usePokemonPCStore.getState().openPC({...reply.pc,party:reply.party,sourceId});
      const sound=sfxPathForConstant(mutation?"SFX_PRESS_AB":"SFX_TURN_ON_PC");
      if(sound) void AudioManager.playSFX(sound,0.7);
    },
    readError: "Interact with an available PC terminal again.",
  });
}

export function openPokemonPC(sourceId: number): Promise<void> {
  return requestPC(OpCodes.PokemonPCOpenRequest,OpCodes.PokemonPCOpenResponse,sourceId,{},false);
}
function mutatePC(opcode: number,responseOpcode: number,box: number,pokemonRowId?: number): Promise<void> {
  const pc=usePokemonPCStore.getState();
  if(!pc.isOpen || pc.sourceId===null || (pokemonRowId!==undefined && (!Number.isSafeInteger(pokemonRowId) || pokemonRowId<=0))) return Promise.resolve();
  return requestPC(opcode,responseOpcode,pc.sourceId,{box,...(pokemonRowId===undefined?{}:{pokemonRowId})},true);
}
export const depositPokemon = (pokemonRowId: number,box: number) => mutatePC(OpCodes.PokemonPCDepositRequest,OpCodes.PokemonPCDepositResponse,box,pokemonRowId);
export const withdrawPokemon = (box: number,pokemonRowId: number) => mutatePC(OpCodes.PokemonPCWithdrawRequest,OpCodes.PokemonPCWithdrawResponse,box,pokemonRowId);
export const releasePokemon = (box: number,pokemonRowId: number) => mutatePC(OpCodes.PokemonPCReleaseRequest,OpCodes.PokemonPCReleaseResponse,box,pokemonRowId);
export const switchPokemonBox = (box: number) => mutatePC(OpCodes.PokemonPCSwitchBoxRequest,OpCodes.PokemonPCSwitchBoxResponse,box);
