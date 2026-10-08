import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePokedexStore from "@/stores/PokedexStore";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import { WorldSocket } from "@/net/index";
import * as OpCodes from "@/net/generated/opcodes";
import { correlatedRequest } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";

type Success = Exclude<PhaserNet.PokedexReadReply, {success:false}>;
type Channel = "card" | "pokedex";
const active = new Map<Channel, AbortController>();

function retire() {
  for (const controller of active.values()) controller.abort();
  active.clear();
  // Species are shared catalog data; status and cards belong to one character.
  usePokedexStore.setState({statusMap:new Map(),trainerCard:null});
}
function refreshVisible() {
  const state=useGameStatusStore.getState();
  if(state.isTrainerCardOpen || state.isInventoryOpen) void refreshTrainerCard();
  if(state.isPokedexOpen) void refreshPokedex();
}
WorldSocket.subscribeSessionRetirement(()=>{
  retire();
  // A replacement transport may serve a new catalog. Revalidate species then.
  usePokedexStore.setState({species:[],isLoaded:false});
});
usePlayerCharacterStore.subscribe((state,before)=>{
  if(state.characterProfile.id!==before.characterProfile.id){retire();refreshVisible();}
});
useGameScreenStore.subscribe((state,before)=>{
  if(state.currentScreen!==before.currentScreen){retire();if(state.currentScreen==="game")refreshVisible();}
});
useGameStatusStore.subscribe(state=>{
  if(!state.isTrainerCardOpen && !state.isInventoryOpen)active.get("card")?.abort();
  if(!state.isPokedexOpen)active.get("pokedex")?.abort();
});

async function read(channel:Channel, request:OpCodes.OpCode,response:OpCodes.OpCode,signal?:AbortSignal):Promise<void> {
  const characterId=usePlayerCharacterStore.getState().characterProfile.id;
  if(!characterId || useGameScreenStore.getState().currentScreen!=="game" || signal?.aborted)return;
  active.get(channel)?.abort();
  const controller=new AbortController();active.set(channel,controller);
  const generation=WorldSocket.sessionGeneration;
  const abort=()=>controller.abort();
  signal?.addEventListener("abort",abort,{once:true});
  const current=()=>!controller.signal.aborted && active.get(channel)===controller && WorldSocket.sessionGeneration===generation && useGameScreenStore.getState().currentScreen==="game" && usePlayerCharacterStore.getState().characterProfile.id===characterId;
  try {
    const reply=await correlatedRequest<Success>(receive=>PhaserNet.onPokedexRead(response,receive),requestId=>PhaserNet.requestPokedexRead(request,requestId),controller.signal);
    if(!current())return;
    if(reply.characterId!==characterId)throw new Error("Pokedex response belongs to another character");
    if(request===OpCodes.TrainerCardRequest ? !("badges" in reply && Array.isArray(reply.badges)) : !("status" in reply && Array.isArray(reply.status)))throw new Error("Invalid trainer information response");
    if(request===OpCodes.PokedexListRequest && !("species" in reply && Array.isArray(reply.species)))throw new Error("Invalid Pokedex catalog response");
    usePokedexStore.getState().applyRead(reply);
  } catch(error) {
    if(current() && !(error instanceof DOMException && error.name==="AbortError"))useChatStore.getState().addMessage("Trainer information could not be refreshed. Close and reopen the view to retry.",MessageType.SYSTEM_ERROR);
  } finally {
    signal?.removeEventListener("abort",abort);
    if(active.get(channel)===controller)active.delete(channel);
  }
}
export function refreshTrainerCard(signal?:AbortSignal):Promise<void> {
  return read("card",OpCodes.TrainerCardRequest,OpCodes.TrainerCardResponse,signal);
}
export function refreshPokedex(signal?:AbortSignal):Promise<void> {
  return usePokedexStore.getState().isLoaded
    ? read("pokedex",OpCodes.PokedexStatusRequest,OpCodes.PokedexStatusResponse,signal)
    : read("pokedex",OpCodes.PokedexListRequest,OpCodes.PokedexListResponse,signal);
}
export function acceptPokedexResourceChange(data:unknown):void {
  const notice=data as {success?:boolean;resourcesChanged?:boolean;characterId?:number};
  if(notice?.success!==true || notice.resourcesChanged!==true || notice.characterId!==usePlayerCharacterStore.getState().characterProfile.id)return;
  if(useGameScreenStore.getState().currentScreen==="game")refreshVisible();
}
