import { OpCodes, WorldSocket } from "@/net";
import type { GameCornerSlotResultResponse, GameCornerPrizeBuyResponse, GameCornerPrizeListResponse } from "@/net/generated/world_api";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import { WHEEL_SIZE } from "@/game/SlotMachineEngine";
import { readForCurrentCharacter } from "./CorrelatedRequest";
import { applyGameplayResourceSnapshot } from "./GameplayRecoveryService";
import { runInventoryRequest, watchInteractionPosition } from "./InventoryCommandService";

type Source = { mapId: number; fromX: number; fromY: number };
function currentSource(): Source | undefined {
  const source = useGameStatusStore.getState().playerTileContext;
  if (!source || ![source.mapId, source.x, source.y].every(Number.isSafeInteger)) return;
  return { mapId: source.mapId, fromX: source.x, fromY: source.y };
}
function sameSource(source: Source): boolean {
  const current = currentSource();
  return current?.mapId === source.mapId && current.fromX === source.fromX && current.fromY === source.fromY;
}
function watchDemand(retire: () => void, signal?: AbortSignal): () => void {
  const stop = watchInteractionPosition(retire);
  signal?.addEventListener("abort", retire, {once:true});
  if (signal?.aborted) retire();
  return () => { stop(); signal?.removeEventListener("abort", retire); };
}

// One wager owns the existing inventory admission slot. The durable reply is
// only a roll receipt: money/coins always come from an awaited current snapshot.
export async function playSlotCommand(bet: number, machineX: number, machineY: number, present: (result: GameCornerSlotResultResponse) => void): Promise<void> {
  const slot = useSlotMachineStore.getState();
  const source = currentSource();
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  if (!source || !slot.isOpen || !slot.coinsReady || useCQInventoryStore.getState().inventoryCommandPending
      || slot.machineX !== machineX || slot.machineY !== machineY || !Number.isInteger(bet) || bet < 1 || bet > 3) return;
  const view = slot.openVersion;
  const transport = WorldSocket.sessionGeneration;
  let presentationCurrent = true;
  const current = () => presentationCurrent && WorldSocket.sessionGeneration === transport && useGameScreenStore.getState().currentScreen === "game" && useSlotMachineStore.getState().isOpen && useSlotMachineStore.getState().openVersion === view
    && usePlayerCharacterStore.getState().characterProfile.id === characterId && sameSource(source);
  let settled = false;
  try {
    await runInventoryRequest<GameCornerSlotResultResponse>({
      mutation:true, currentResources:true,
      opcode:OpCodes.GameCornerSlotPlayRequest, responseOpcode:OpCodes.GameCornerSlotResultResponse,
      payload:{...source,bet,machineX,machineY},
      watchPresentation: retire => {
        const retireView = () => { presentationCurrent = false; retire(); };
        const stopPosition = watchInteractionPosition(retireView);
        const stopView = useSlotMachineStore.subscribe(state => { if (!state.isOpen || state.openVersion !== view) retireView(); });
        return () => { stopPosition(); stopView(); };
      },
      validate: reply => {
        if (reply.version !== 1 || reply.kind !== "slot" || reply.characterId !== characterId
            || reply.machineX !== machineX || reply.machineY !== machineY || reply.bet !== bet
            || !Array.isArray(reply.reelPositions) || reply.reelPositions.length !== 3
            || reply.reelPositions.some(position => !Number.isInteger(position) || position < 0 || position >= WHEEL_SIZE)
            || !Number.isSafeInteger(reply.payout) || reply.payout < 0 || typeof reply.matchLine !== "string" || typeof reply.isLucky !== "boolean") throw new Error("Invalid slot result receipt");
      },
      present: reply => {
        if (!current()) return;
        settled = true;
        useSlotMachineStore.getState().setCoins(useCQInventoryStore.getState().coins);
        present(reply);
      },
      recovered: snapshot => {
        if (!current()) return;
        settled = true;
        useSlotMachineStore.setState({coins:snapshot.coins,coinsReady:true,isSpinning:false,payout:0,message:"Balance refreshed. The spin will not be repeated."});
      },
      recoveryMessage:"Coin balance refreshed. The spin was not repeated.",
    });
  } finally {
    if (!settled && current()) useSlotMachineStore.setState({isSpinning:false,coinsReady:false,coinsLoading:false,payout:0,message:"Could not confirm the spin. Select RETRY to check your coins."});
  }
}

export async function buyPrizeCommand(prizeId: number, source: Source, signal: AbortSignal): Promise<void> {
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  if (signal.aborted || !sameSource(source) || !Number.isSafeInteger(prizeId) || prizeId <= 0) return;
  await runInventoryRequest<GameCornerPrizeBuyResponse>({
    mutation:true,currentResources:true,
    opcode:OpCodes.GameCornerPrizeBuyRequest,responseOpcode:OpCodes.GameCornerPrizeBuyResponse,
    payload:{...source,prizeId},
    watchPresentation: retire => watchDemand(retire,signal),
    validate: reply => {
      if (reply.version !== 1 || reply.kind !== "prize" || reply.characterId !== characterId || reply.prizeId !== prizeId
          || reply.prize?.id !== prizeId || typeof reply.prizeName !== "string" || typeof reply.message !== "string"
          || typeof reply.addedToParty !== "boolean" || !Number.isInteger(reply.prizeLevel)
          || !Number.isInteger(reply.pcBox) || !Number.isInteger(reply.pcSlot)) throw new Error("Invalid prize result receipt");
    },
    present: reply => usePokemonDialogueStore.getState().openDialogue([reply.message]),
    recoveryMessage:"Prize state refreshed. Check your bag, party and PC before another purchase.",
  });
}

export async function openPrizeWindow(prizeWindow: number, show: (reply: GameCornerPrizeListResponse, signal: AbortSignal) => Promise<number | undefined>, signal: AbortSignal): Promise<void> {
  const source = currentSource();
  if (!source || signal.aborted || !Number.isInteger(prizeWindow) || prizeWindow < 1 || prizeWindow > 3) return;
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal.addEventListener("abort",abort,{once:true});
  const stopPosition = watchInteractionPosition(abort);
  try {
    await readForCurrentCharacter(async (characterId,lifetime) => {
      let menu: GameCornerPrizeListResponse | undefined;
      await runInventoryRequest<GameCornerPrizeListResponse>({
        mutation:false,opcode:OpCodes.GameCornerPrizeListRequest,responseOpcode:OpCodes.GameCornerPrizeListResponse,
        payload:{...source,prizeWindow},
        watchPresentation:retire=>watchDemand(retire,lifetime),
        validate:reply=>{
          if(reply.characterId!==characterId || reply.prizeWindow!==prizeWindow || !Array.isArray(reply.prizes)
              || reply.prizes.some(prize=>!Number.isSafeInteger(prize.id)||prize.id<=0||!Number.isSafeInteger(prize.coinCost)||prize.coinCost<0||typeof prize.name!=="string")
              || reply.snapshot?.position.mapId!==source.mapId || reply.snapshot.position.x!==source.fromX || reply.snapshot.position.y!==source.fromY) throw new Error("Invalid source-owned prize menu");
        },
        apply:reply=>applyGameplayResourceSnapshot(reply.snapshot),
        present:reply=>{menu=reply;},
        readError:"Could not open the prize window. Please interact with it again.",
      });
      if(!menu || lifetime.aborted || !sameSource(source))return;
      const prizeId=await show(menu,lifetime);
      if(prizeId===undefined || lifetime.aborted)return;
      if(!menu.prizes.some(prize=>prize.id===prizeId))return;
      await buyPrizeCommand(prizeId,source,lifetime);
    },controller.signal);
  } catch(error) {
    if(!(error instanceof DOMException && error.name==="AbortError"))throw error;
  } finally {stopPosition();signal.removeEventListener("abort",abort);}
}
