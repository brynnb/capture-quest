import { OpCodes } from "@/net";
import type { CQMerchantOpenResponse, CQMerchantBuyResponse, CQMerchantSellResponse } from "@/net/generated/world_api";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import { runInventoryRequest, watchInteractionPosition } from "./InventoryCommandService";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

const watchShop = (retire: () => void) => {
  const stopPosition=watchInteractionPosition(()=>{retire();useCQInventoryStore.getState().closeShop();});
  const stopPanel=useCQInventoryStore.subscribe((state, previous) => {
    if (!state.shopOpen && (previous.shopOpen || previous.shopItems !== state.shopItems)) retire();
  });
  return ()=>{stopPosition();stopPanel();};
};

function sendShopCommand(opcode: number, responseOpcode: number, payload: Record<string, number>): Promise<void> {
  const {shopOpen, shopActorId: actorId} = useCQInventoryStore.getState();
  if (!shopOpen || !actorId || !Number.isSafeInteger(actorId) || actorId <= 0) return Promise.resolve();
  return runInventoryRequest<CQMerchantBuyResponse | CQMerchantSellResponse>({
    opcode, responseOpcode, payload: {...payload, actorId}, mutation: true, watchPresentation: watchShop,
    present: () => {
      const sound = sfxPathForConstant("SFX_PURCHASE"); if (sound) void AudioManager.playSFX(sound, 0.8);
    },
  });
}

export function buyShopItem(merchantId: number, itemId: number, quantity: number): Promise<void> {
  return sendShopCommand(OpCodes.CQMerchantBuyRequest, OpCodes.CQMerchantBuyResponse, {merchantId, itemId, quantity});
}
export function sellShopItem(instanceId: number): Promise<void> {
  return sendShopCommand(OpCodes.CQMerchantSellRequest, OpCodes.CQMerchantSellResponse, {instanceId});
}

export async function openShopForActor(actorId: number): Promise<void> {
  if (useCQInventoryStore.getState().inventoryCommandPending || !Number.isSafeInteger(actorId) || actorId <= 0) return;
  useCQInventoryStore.getState().closeShop();
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  if (!characterId) return;
  return runInventoryRequest<CQMerchantOpenResponse>({
    opcode: OpCodes.CQMerchantOpenRequest, responseOpcode: OpCodes.CQMerchantOpenResponse,
    payload: {actorId}, mutation: false, watchPresentation: watchShop,
    validate: reply => {
      if (reply.characterId !== characterId || !Number.isSafeInteger(reply.merchantId) || reply.merchantId <= 0
        || typeof reply.name !== "string" || !Array.isArray(reply.items)
        || !Number.isSafeInteger(reply.money) || reply.money < 0 || reply.money > 0xffffffff) throw new Error("Invalid merchant menu");
    },
    present: reply => {
      useCQInventoryStore.getState().openShop(reply.merchantId, reply.name, reply.items, reply.money, actorId);
      usePlayerCharacterStore.getState().handleCharacterWalletData({characterId, pokedollars: reply.money});
    },
    readError: "Could not open this shop. Please interact with the clerk again.",
  });
}
