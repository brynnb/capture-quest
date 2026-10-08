import {afterEach,beforeEach,expect,test,vi} from "vitest";
const network=vi.hoisted(()=>({listeners:new Map<number,Set<(data:unknown)=>void>>(),retire:new Set<()=>void>(),generation:0,send:vi.fn()}));
vi.mock("@/net/index",()=>({WorldSocket:{get sessionGeneration(){return network.generation},subscribeSessionRetirement:(receive:()=>void)=>{network.retire.add(receive);return()=>network.retire.delete(receive)}}}));
vi.mock("./PhaserNetworkService",()=>({
 isConnected:()=>true,
 requestPokedexRead:network.send,
 onPokedexRead:(opcode:number,receive:(data:unknown)=>void)=>{const listeners=network.listeners.get(opcode)??new Set();network.listeners.set(opcode,listeners);listeners.add(receive);return()=>listeners.delete(receive)},
}));
import * as OpCodes from "@/net/generated/opcodes";
import useGameScreenStore from "@/stores/GameScreenStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import usePokedexStore from "@/stores/PokedexStore";
import {refreshTrainerCard,refreshPokedex,acceptPokedexResourceChange} from "./PokedexReadService";
const card=(requestId:string,money=100)=>({success:true,requestId,characterId:42,name:"Red",money,timePlayed:120,badges:[],badgeCount:0,pokedexSeen:1,pokedexCaught:1});
const receive=(opcode:number,data:unknown)=>network.listeners.get(opcode)?.forEach(listener=>listener(data));
beforeEach(()=>{
 useGameStatusStore.setState({isTrainerCardOpen:false,isInventoryOpen:false,isPokedexOpen:false});
 useGameScreenStore.getState().setScreen("characterSelect");
 usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 useGameScreenStore.getState().setScreen("game");
 useGameStatusStore.setState({isTrainerCardOpen:true});
 usePokedexStore.setState({species:[],isLoaded:false,statusMap:new Map(),trainerCard:null});
 network.send.mockReset();network.listeners.clear();
});
afterEach(()=>{useGameScreenStore.getState().setScreen("characterSelect");vi.useRealTimers();});
test("only its correlated response applies; untagged and duplicate replies do not",async()=>{
 const run=refreshTrainerCard();await Promise.resolve();const id=network.send.mock.calls[0][1];
 receive(OpCodes.TrainerCardResponse,card(""));expect(usePokedexStore.getState().trainerCard).toBeNull();
 receive(OpCodes.TrainerCardResponse,card(id));await run;
 receive(OpCodes.TrainerCardResponse,card(id,999));expect(usePokedexStore.getState().trainerCard?.money).toBe(100);
 expect(network.listeners.get(OpCodes.TrainerCardResponse)?.size).toBe(0);
});
test("superseded reads cannot rewind a newer card",async()=>{
 const first=refreshTrainerCard();await Promise.resolve();const old=network.send.mock.calls[0][1];
 const second=refreshTrainerCard();await Promise.resolve();const current=network.send.mock.calls[1][1];
 receive(OpCodes.TrainerCardResponse,card(old,100));receive(OpCodes.TrainerCardResponse,card(current,200));
 await Promise.all([first,second]);expect(usePokedexStore.getState().trainerCard?.money).toBe(200);
});
test("quit clears private data and same-character reentry rejects old responses",async()=>{
 const old=refreshTrainerCard();await Promise.resolve();const id=network.send.mock.calls[0][1];
 useGameStatusStore.setState({isTrainerCardOpen:false});
 useGameScreenStore.getState().setScreen("characterSelect");useGameScreenStore.getState().setScreen("game");
 receive(OpCodes.TrainerCardResponse,card(id));await old;
 expect(usePokedexStore.getState().trainerCard).toBeNull();expect(usePokedexStore.getState().statusMap.size).toBe(0);
});
test("transport retirement fences a resolved response before its application",async()=>{
 const run=refreshTrainerCard();await Promise.resolve();receive(OpCodes.TrainerCardResponse,card(network.send.mock.calls[0][1]));
 network.generation++;network.retire.forEach(retire=>retire());await run;
 expect(usePokedexStore.getState().trainerCard).toBeNull();expect(usePokedexStore.getState().isLoaded).toBe(false);
});
test("timeout releases subscriptions and retry has a fresh identity",async()=>{
 vi.useFakeTimers();const run=refreshTrainerCard();await Promise.resolve();const old=network.send.mock.calls[0][1];
 await vi.advanceTimersByTimeAsync(10000);await run;
 expect(network.listeners.get(OpCodes.TrainerCardResponse)?.size).toBe(0);
 const retry=refreshTrainerCard();await Promise.resolve();const fresh=network.send.mock.calls[1][1];expect(fresh).not.toBe(old);
 receive(OpCodes.TrainerCardResponse,card(old));receive(OpCodes.TrainerCardResponse,card(fresh));await retry;
 expect(usePokedexStore.getState().trainerCard?.money).toBe(100);
});
test("Pokedex applies complete species/status only for its owned request",async()=>{
 useGameStatusStore.setState({isPokedexOpen:true});const run=refreshPokedex();await Promise.resolve();const id=network.send.mock.calls[0][1];
 receive(OpCodes.PokedexListResponse,{success:true,requestId:id,characterId:42,species:[],status:[{pokemonId:25,seen:true,caught:true}]});await run;
 expect(usePokedexStore.getState().isLoaded).toBe(true);expect(usePokedexStore.getState().isCaught(25)).toBe(true);
});
test("unmount cancellation and wrong-character replies cannot publish",async()=>{
 const controller=new AbortController();const first=refreshTrainerCard(controller.signal);await Promise.resolve();const old=network.send.mock.calls[0][1];controller.abort();
 receive(OpCodes.TrainerCardResponse,card(old));await first;expect(usePokedexStore.getState().trainerCard).toBeNull();
 const second=refreshTrainerCard();await Promise.resolve();receive(OpCodes.TrainerCardResponse,{...card(network.send.mock.calls[1][1]),characterId:99});await second;
 expect(usePokedexStore.getState().trainerCard).toBeNull();expect(network.listeners.get(OpCodes.TrainerCardResponse)?.size).toBe(0);
});
test("send failure settles and permits a new read",async()=>{
 network.send.mockRejectedValueOnce(new Error("send failed"));await refreshTrainerCard();
 expect(network.listeners.get(OpCodes.TrainerCardResponse)?.size).toBe(0);
 const retry=refreshTrainerCard();await Promise.resolve();receive(OpCodes.TrainerCardResponse,card(network.send.mock.calls[1][1]));await retry;
 expect(usePokedexStore.getState().trainerCard?.money).toBe(100);
});
test("resource notices request current data for the visible owner",async()=>{
 acceptPokedexResourceChange({success:true,resourcesChanged:true,characterId:99});expect(network.send).not.toHaveBeenCalled();
 acceptPokedexResourceChange({success:true,resourcesChanged:true,characterId:42});await Promise.resolve();
 receive(OpCodes.TrainerCardResponse,card(network.send.mock.calls[0][1],200));await Promise.resolve();
 expect(usePokedexStore.getState().trainerCard?.money).toBe(200);
});
test("missing response data is rejected instead of silently completing",async()=>{
 const run=refreshTrainerCard();await Promise.resolve();receive(OpCodes.TrainerCardResponse,{success:true,requestId:network.send.mock.calls[0][1],characterId:42});await run;
 expect(usePokedexStore.getState().trainerCard).toBeNull();
});
test("a partial card cannot become a successful presentation snapshot",async()=>{
 const run=refreshTrainerCard();await Promise.resolve();receive(OpCodes.TrainerCardResponse,{success:true,requestId:network.send.mock.calls[0][1],characterId:42,badges:[]});await run;
 expect(usePokedexStore.getState().trainerCard).toBeNull();
});
test("a status reply cannot replace the species catalog by carrying list fields",async()=>{
 usePokedexStore.setState({species:[],isLoaded:true});useGameStatusStore.setState({isPokedexOpen:true});
 const run=refreshPokedex();await Promise.resolve();receive(OpCodes.PokedexStatusResponse,{success:true,requestId:network.send.mock.calls[0][1],characterId:42,status:[],species:[{id:999,name:"wrong catalog"}]});await run;
 expect(usePokedexStore.getState().species).toEqual([]);
});

test("same-turn cleanup and replacement send only the surviving demand",async()=>{
 const controller=new AbortController();const abandoned=refreshTrainerCard(controller.signal);controller.abort();
 const current=refreshTrainerCard();await Promise.resolve();
 expect(network.send).toHaveBeenCalledTimes(1);
 receive(OpCodes.TrainerCardResponse,card(network.send.mock.calls[0][1]));await Promise.all([abandoned,current]);
 expect(usePokedexStore.getState().trainerCard?.money).toBe(100);
});
