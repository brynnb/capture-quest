import { OpCodes, WorldSocket } from "@/net";
import type { CQMerchantBuyRequest, CQMerchantSellRequest, CQMerchantBuyResponse, CQMerchantSellResponse } from "@/net/generated/world_api";
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
type ShopCommand = Omit<CQMerchantBuyRequest,"requestId"|"shop"> | Omit<CQMerchantSellRequest,"requestId"|"shop">;

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
  if (!initialBag.shopOpen || !owner || !characterId || !Number.isSafeInteger(revision) || revision < 0) return;
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
      requestId => WorldSocket.sendStreamJsonMessage(opcode,{...command,requestId,shop:{characterId,revision}}),
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
