import { OpCodes, WorldSocket } from "@/net";
import type { CQMerchantOpenResponse, CQMerchantBuyRequest, CQMerchantSellRequest, CQMerchantBuyResponse, CQMerchantSellResponse } from "@/net/generated/world_api";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import { correlatedRequest } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { readCurrentGameplayState } from "./GameplayRecoveryService";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

let scene: symbol | null = null;
let active: AbortController | null = null;
type ShopCommand = Omit<CQMerchantBuyRequest,"requestId"|"shop"|"actorId"> | Omit<CQMerchantSellRequest,"requestId"|"shop"|"actorId">;

export function bindShopScene(): () => void {
  active?.abort(); active = null;
  useCQInventoryStore.getState().closeShop();
  useCQInventoryStore.setState({ shopCommandPending: false });
  const owner = Symbol("shop scene"); scene = owner;
  return () => {
    if (scene !== owner) return;
    scene = null; active?.abort(); active = null;
    useCQInventoryStore.getState().closeShop();
    useCQInventoryStore.setState({ shopCommandPending: false });
  };
}

async function sendShopCommand(opcode: number, responseOpcode: number, command: ShopCommand): Promise<void> {
  if (active) return;
  const owner = scene;
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  const initialBag = useCQInventoryStore.getState();
  const initialProfile = usePlayerCharacterStore.getState().characterProfile;
  const revision = initialBag.shopRevision;
  const actorId = initialBag.shopActorId;
  if (!initialBag.shopOpen || !Number.isSafeInteger(actorId) || !actorId || actorId <= 0 || !owner || !characterId || !Number.isSafeInteger(revision) || revision < 0) return;
  const controller = new AbortController(); active = controller;
  const current = () => !controller.signal.aborted && scene === owner && usePlayerCharacterStore.getState().characterProfile.id === characterId;
  const stopProfile = usePlayerCharacterStore.subscribe(state => { if (state.characterProfile.id !== characterId) controller.abort(); });
  const stopShop = useCQInventoryStore.subscribe(state => { if (!state.shopOpen) controller.abort(); });
  const unsubscribe = () => { stopProfile(); stopShop(); };
  controller.signal.addEventListener("abort", unsubscribe, {once:true});
  useCQInventoryStore.setState({ shopCommandPending:true, shopCommandError:null });
  try {
    const reply = await correlatedRequest<CQMerchantBuyResponse | CQMerchantSellResponse>(
      receive => PhaserNet.onShopCommand(responseOpcode,receive),
      requestId => WorldSocket.sendStreamJsonMessage(opcode,{...command,requestId,actorId,shop:{characterId,revision}}),
      controller.signal,
    );
    if (!current()) return;
    const bag = useCQInventoryStore.getState();
    if (bag.items !== initialBag.items || bag.money !== initialBag.money || bag.shopRevision !== revision || usePlayerCharacterStore.getState().characterProfile !== initialProfile) throw new Error("Shop reply overtaken by newer state");
    if (!reply.inventory || !Array.isArray(reply.inventory.items) || reply.inventory.shopRevision !== revision+1
      || !Number.isSafeInteger(reply.inventory.money) || reply.inventory.money < 0 || reply.inventory.money > 0xffffffff) throw new Error("Invalid committed shop response");
    useCQInventoryStore.getState().setInventory(reply.inventory.items,reply.inventory.money,reply.inventory.shopRevision);
    usePlayerCharacterStore.getState().handleCharacterWalletData({characterId,pokedollars:reply.inventory.money});
    const sound = sfxPathForConstant("SFX_PURCHASE"); if (sound) void AudioManager.playSFX(sound,0.8);
  } catch {
    if (!current()) return;
    // Rejection/timeout does not prove rollback. Read current authority without
    // resending the mutation or applying a delayed historical acknowledgement.
    try {
      const snapshot = await readCurrentGameplayState(controller.signal);
      if (!current()) return;
      useCQInventoryStore.getState().setInventory(snapshot.inventory,snapshot.wallet.pokedollars,snapshot.shopRevision);
      usePlayerCharacterStore.getState().handleCharacterWalletData(snapshot.wallet);
      useChatStore.getState().addMessage("Shop state refreshed. Check your bag and balance before another purchase.",MessageType.SYSTEM);
    } catch {
      if (!current()) return;
      const message = "Could not restore shop state. Please reconnect.";
      useCQInventoryStore.setState({shopCommandError:message});
      useChatStore.getState().addMessage(message,MessageType.SYSTEM);
    }
  } finally {
    controller.signal.removeEventListener("abort",unsubscribe); unsubscribe();
    if (active === controller) { active = null; useCQInventoryStore.setState({shopCommandPending:false}); }
  }
}

export function buyShopItem(merchantId: number,itemId: number,quantity: number): Promise<void> {
  return sendShopCommand(OpCodes.CQMerchantBuyRequest,OpCodes.CQMerchantBuyResponse,{merchantId,itemId,quantity});
}
export function sellShopItem(instanceId: number): Promise<void> {
  return sendShopCommand(OpCodes.CQMerchantSellRequest,OpCodes.CQMerchantSellResponse,{instanceId});
}

// Opening shares scene admission with purchases. An unsolicited or late menu
// cannot reopen another scene, and failures preserve the existing wallet/bag.
export async function openShopForActor(actorId: number): Promise<void> {
  if (active || !scene) return;
  const owner = scene;
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  if (!characterId || !Number.isSafeInteger(actorId) || actorId <= 0) return;
  useCQInventoryStore.getState().closeShop();
  const profile = usePlayerCharacterStore.getState().characterProfile;
  const initialBag = useCQInventoryStore.getState();
  const controller = new AbortController(); active = controller;
  const current = () => !controller.signal.aborted && scene === owner && usePlayerCharacterStore.getState().characterProfile.id === characterId;
  const stopProfile = usePlayerCharacterStore.subscribe(state => { if (state.characterProfile.id !== characterId) { controller.abort(); if (scene === owner) useCQInventoryStore.getState().closeShop(); } });
  const stopShop = useCQInventoryStore.subscribe((state, previous) => { if (!state.shopOpen && (previous.shopOpen || previous.shopItems !== state.shopItems)) controller.abort(); });
  useCQInventoryStore.setState({ shopCommandPending: true, shopCommandError: null });
  try {
    const reply = await correlatedRequest<CQMerchantOpenResponse>(
      receive => PhaserNet.onShopCommand(OpCodes.CQMerchantOpenResponse, receive),
      requestId => WorldSocket.sendStreamJsonMessage(OpCodes.CQMerchantOpenRequest, { requestId, characterId, actorId }),
      controller.signal,
    );
    if (!current()) return;
    const bag = useCQInventoryStore.getState();
    if (reply.characterId !== characterId || !Number.isSafeInteger(reply.merchantId) || reply.merchantId <= 0
      || typeof reply.name !== "string" || !Array.isArray(reply.items)
      || !Number.isSafeInteger(reply.money) || reply.money < 0 || reply.money > 0xffffffff
      || bag.items !== initialBag.items || bag.money !== initialBag.money || bag.shopRevision !== initialBag.shopRevision
      || usePlayerCharacterStore.getState().characterProfile !== profile) throw new Error("Invalid or overtaken merchant menu");
    useCQInventoryStore.getState().openShop(reply.merchantId, reply.name, reply.items, reply.money, actorId);
    usePlayerCharacterStore.getState().handleCharacterWalletData({ characterId, pokedollars: reply.money });
  } catch {
    if (current()) {
      const message = "Could not open this shop. Please interact with the clerk again.";
      useCQInventoryStore.setState({ shopCommandError: message });
      useChatStore.getState().addMessage(message, MessageType.SYSTEM);
    }
  } finally {
    stopProfile(); stopShop();
    if (active === controller) { active = null; useCQInventoryStore.setState({ shopCommandPending: false }); }
  }
}
