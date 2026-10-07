import { beforeEach, afterEach, expect, test, vi } from "vitest";
const net = vi.hoisted(() => ({ listeners:new Map<number,Set<(reply:any)=>void>>(),send:vi.fn(),read:vi.fn() }));
vi.mock("@/net",()=>({OpCodes:{PokemonPartyReorderRequest:90,PokemonPartyReorderResponse:91,CQItemUseRequest:100,CQItemUseResponse:101,RepelUseRequest:143,RepelUseResponse:144,CQMerchantOpenRequest:94,CQMerchantOpenResponse:95,CQMerchantBuyRequest:96,CQMerchantBuyResponse:97,CQMerchantSellRequest:98,CQMerchantSellResponse:99},WorldSocket:{sendStreamJsonMessage:net.send}}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onInventoryCommand:(opcode:number,receive:(reply:any)=>void)=>{
  if (!net.listeners.has(opcode)) net.listeners.set(opcode,new Set());
  const listeners=net.listeners.get(opcode)!;listeners.add(receive);return()=>listeners.delete(receive);
}}));
vi.mock("./GameplayRecoveryService",()=>({readCurrentGameplayState:net.read}));
vi.mock("@/services/audio/AudioManager",()=>({default:{playSFX:vi.fn()}}));
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import {openShopForActor,buyShopItem,sellShopItem} from "./ShopCommandService";

import {bindInventoryScene,sendPartyItemCommand,sendRepelItemCommand,sendPartyReorderCommand} from "./InventoryCommandService";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import AudioManager from "@/services/audio/AudioManager";
const party = [{rowId:7,curHp:21}] as any;
const actions = ["buy","sell","party","repel","reorder"] as const;
const send = (action:typeof actions[number]) => action==="buy"?buyShopItem(1,1,10):action==="sell"?sellShopItem(7):action==="repel"?sendRepelItemCommand(1):action==="reorder"?sendPartyReorderCommand([7]):sendPartyItemCommand(1,0);
const opcode = (action:typeof actions[number]) => action==="buy"?97:action==="sell"?99:action==="repel"?144:action==="reorder"?91:101;
const reply = (requestId:string, revision=5) => ({success:true,requestId,instanceId:1,message:"Repel started",stepsLeft:100,inventory:{items:[],money:900,commandRevision:revision},party,outcome:{instanceId:1,partySlot:0,message:"Healed"}});
let retire:()=>void;
const id=()=>net.send.mock.calls.at(-1)![1].requestId;
const emit=(opcode:number,reply:any)=>net.listeners.get(opcode)?.forEach(receive=>receive(reply));
beforeEach(()=>{
  vi.useFakeTimers();net.listeners.clear();net.send.mockReset().mockResolvedValue(undefined);net.read.mockReset();
  vi.mocked(AudioManager.playSFX).mockClear();
  useGameStatusStore.setState({isInventoryOpen:true});
  useCQInventoryStore.setState({items:[],money:1000,commandRevision:4,shopOpen:true,inventoryCommandPending:false});
  usePlayerCharacterStore.getState().setCharacterProfile({id:42,pokedollars:1000});
  usePokemonPartyStore.getState().setParty(party);
  retire=bindInventoryScene();
  useCQInventoryStore.getState().openShop(1,"Shop",[],1000,1001);
});
afterEach(()=>{retire();vi.useRealTimers();});

test.each([null,0,-1])("shop mutations require the live menu actor %s",async actorId=>{
  useCQInventoryStore.setState({shopActorId:actorId});
  await buyShopItem(1,1,1); await sellShopItem(7);
  expect(net.send).not.toHaveBeenCalled();
  expect(net.read).not.toHaveBeenCalled();
});

for (const action of actions) {
 test(`${action} carries identity and accepts one correlated next revision`,async()=>{
  const pending=send(action);
  expect(net.send.mock.calls[0][1].command).toEqual({characterId:42,revision:4});
  if(action==="party") expect(net.send.mock.calls[0][1].pokemonRowId).toBe(7);
  else if(action==="reorder") expect(net.send.mock.calls[0][1].pokemonIds).toEqual([7]);
  else if(action!=="repel") expect(net.send.mock.calls[0][1].actorId).toBe(1001);
  else expect(net.send.mock.calls[0][1].instanceId).toBe(1);
  expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(true);
  for(const another of actions) await send(another);
  expect(net.send).toHaveBeenCalledTimes(1);
  emit(opcode(action),reply("other"));expect(useCQInventoryStore.getState().money).toBe(1000);
  const result=reply(id()); result.party=[{rowId:7,curHp:41}];
  emit(opcode(action),result);await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({money:900,commandRevision:5,inventoryCommandPending:false});
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(900);
  if(action==="party" || action==="reorder") expect(usePokemonPartyStore.getState().party).toEqual(result.party);
  expect(net.read).not.toHaveBeenCalled();
 });
 test.each(["timeout","rejection","malformed","overtaken","party update","send failure"])(`${action} %s recovers without resending`,async mode=>{
  const recoveredParty=[{rowId:7,curHp:41}];
  net.read.mockResolvedValue({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],wallet:{characterId:42,pokedollars:800},commandRevision:6,party:recoveredParty});
  if(mode==="send failure") net.send.mockRejectedValue(new Error("failed"));
  const pending=send(action);
  if(mode==="timeout") await vi.advanceTimersByTimeAsync(10000);
  else if(mode!=="send failure") {
   if(mode==="overtaken") useCQInventoryStore.getState().setMoney(850);
   if(mode==="party update") usePokemonPartyStore.getState().setParty([...party]);
   emit(opcode(action),mode==="rejection"?{success:false,requestId:id(),error:"stale"}:reply(id(),mode==="malformed"?99:5));
  }
  await pending;
  expect(net.read).toHaveBeenCalledTimes(1);expect(net.send).toHaveBeenCalledTimes(1);
  expect(useCQInventoryStore.getState()).toMatchObject({money:800,commandRevision:6,inventoryCommandPending:false});
  expect(usePokemonPartyStore.getState().party).toEqual(recoveredParty);
  emit(opcode(action),reply(id()));expect(useCQInventoryStore.getState().money).toBe(800);
 });
 test.each(["scene","character"])(`${action} %s retirement ignores late replies`,async mode=>{
  const pending=send(action);const requestId=id();
  if(action!=="party") useCQInventoryStore.getState().closeShop();
  if(mode==="scene") retire();
  if(mode==="character") usePlayerCharacterStore.getState().setCharacterProfile({id:43,pokedollars:700});
  await pending;
  expect(net.listeners.get(opcode(action))?.size).toBe(0);
  emit(opcode(action),reply(requestId));
  expect(useCQInventoryStore.getState().money).toBe(1000);expect(net.read).not.toHaveBeenCalled();
  expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(false);
 });
 test(`${action} retirement while recovering ignores a late snapshot`,async()=>{
  let resolve!:(value:any)=>void;net.read.mockImplementation(()=>new Promise(done=>{resolve=done}));
  const pending=send(action);await vi.advanceTimersByTimeAsync(10000);
  if(action!=="party") useCQInventoryStore.getState().closeShop();
  retire();resolve({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],wallet:{characterId:42,pokedollars:0},commandRevision:5,party:[]});
  await pending;expect(useCQInventoryStore.getState().money).toBe(1000);expect(usePokemonPartyStore.getState().party).toEqual(party);
 });
 test(`${action} failed recovery preserves state and asks to reconnect`,async()=>{
  net.read.mockRejectedValue(new Error("read timeout"));const pending=send(action);
  await vi.advanceTimersByTimeAsync(10000);await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,commandRevision:4,inventoryCommandPending:false,inventoryCommandError:"Could not restore inventory state. Please reconnect."});
  expect(net.send).toHaveBeenCalledTimes(1);
 });
}

test.each([{party:[]},{party:[{rowId:8}]},{party:[{rowId:7},{rowId:7}]}])("reorder rejects a mismatched final party: %j",async ({party:returnedParty})=>{
 net.read.mockResolvedValue({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],wallet:{characterId:42,pokedollars:1000},commandRevision:5,party});
 const ids=[7]; const pending=sendPartyReorderCommand(ids); ids[0]=8;
 expect(net.send.mock.calls[0][1].pokemonIds).toEqual([7]);
 emit(91,{...reply(id()),party:returnedParty}); await pending;
 expect(net.read).toHaveBeenCalledTimes(1);
 expect(usePokemonPartyStore.getState().party).toEqual(party);
 expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test.each([{ids:[]},{ids:[undefined]},{ids:[0]},{ids:[7,7]}])("reorder rejects missing or repeated identities: %j",async ({ids})=>{
 await sendPartyReorderCommand(ids);
 expect(net.send).not.toHaveBeenCalled();
 expect(useCQInventoryStore.getState().inventoryCommandError).toContain("Party identity is unavailable");
});

for (const action of ["buy", "sell"] as const) {
 test.each(["delayed reply", "lost reply"])(`${action} reconciles after shop closure with %s`, async mode => {
  const items = [{instance:{id:12,quantity:3},item:{name:"Potion"}}] as any;
  net.read.mockResolvedValue({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:items,wallet:{characterId:42,pokedollars:900},commandRevision:5,party});
  const pending = send(action); const requestId = id();
  useCQInventoryStore.getState().closeShop();
  expect(useCQInventoryStore.getState()).toMatchObject({shopOpen:false,inventoryCommandPending:true});
  expect(net.listeners.get(opcode(action))?.size).toBe(1);
  await openShopForActor(1001); await sendPartyItemCommand(1,0);
  expect(net.send).toHaveBeenCalledTimes(1);
  if (mode === "lost reply") await vi.advanceTimersByTimeAsync(10000);
  else emit(opcode(action), {...reply(requestId),inventory:{items,money:900,commandRevision:5}});
  await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({items,money:900,commandRevision:5,shopOpen:false,inventoryCommandPending:false});
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(900);
  expect(AudioManager.playSFX).not.toHaveBeenCalled();
  expect(net.read).toHaveBeenCalledTimes(mode === "lost reply" ? 1 : 0);
  expect(net.send).toHaveBeenCalledTimes(1);
  // Reopening must not need a rejected stale mutation to refresh the revision.
  const opening = openShopForActor(1001);
  emit(95,{...menu(id()),money:900}); await opening;
  const next = send(action);
  expect(net.send.mock.calls.at(-1)![1].command).toEqual({characterId:42,revision:5});
  emit(opcode(action),reply(requestId));
  expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(true);
  emit(opcode(action),reply(id(),6)); await next;
  expect(useCQInventoryStore.getState().commandRevision).toBe(6);
  expect(AudioManager.playSFX).toHaveBeenCalledTimes(1);
 });
}

test("closing during recovery still applies the current gameplay snapshot", async () => {
 let resolve!:(value:any)=>void; net.read.mockImplementation(()=>new Promise(done=>{resolve=done}));
 const pending=buyShopItem(1,1,1); await vi.advanceTimersByTimeAsync(10000);
 useCQInventoryStore.getState().closeShop();
 expect(net.read.mock.calls[0][0].aborted).toBe(false);
 const recoveredParty=[{rowId:7,curHp:41}];
 resolve({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],wallet:{characterId:42,pokedollars:900},commandRevision:5,party:recoveredParty});
 await pending;
 expect(useCQInventoryStore.getState()).toMatchObject({money:900,commandRevision:5,shopOpen:false,inventoryCommandPending:false});
 expect(usePokemonPartyStore.getState().party).toEqual(recoveredParty);
 expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test.each(["reply", "timeout"])("Repel reconciles after closing the bag on %s without late effects",async mode=>{
 net.read.mockResolvedValue({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],wallet:{characterId:42,pokedollars:900},commandRevision:5,party});
 const pending=sendRepelItemCommand(1); const requestId=id();
 useGameStatusStore.setState({isInventoryOpen:false});
 await buyShopItem(1,1,1);
 expect(net.send).toHaveBeenCalledTimes(1);
 expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(true);
 if(mode==="timeout") await vi.advanceTimersByTimeAsync(10000);
 else emit(144,reply(requestId));
 await pending;
 expect(useCQInventoryStore.getState()).toMatchObject({commandRevision:5,money:900,inventoryCommandPending:false});
 expect(AudioManager.playSFX).not.toHaveBeenCalled();
 expect(net.read).toHaveBeenCalledTimes(mode==="timeout"?1:0);
});

test.each([{instanceId:99},{stepsLeft:0},{message:null}])("invalid Repel outcome recovers: %j",async invalid=>{
 net.read.mockResolvedValue({pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],wallet:{characterId:42,pokedollars:800},commandRevision:6,party});
 const pending=sendRepelItemCommand(1); emit(144,{...reply(id()),...invalid}); await pending;
 expect(net.read).toHaveBeenCalledTimes(1);
 expect(useCQInventoryStore.getState()).toMatchObject({commandRevision:6,money:800});
 expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test("older scene cleanup cannot release a newer command",async()=>{
 const first=buyShopItem(1,1,1);const firstId=id();const oldRetire=retire;
 retire=bindInventoryScene();const second=sendPartyItemCommand(1,0);const secondId=id();
 oldRetire();await first;
 expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(true);
 emit(97,reply(firstId));expect(useCQInventoryStore.getState().money).toBe(1000);
 emit(101,reply(secondId));await second;expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(false);
});

test("move selection retains the original Pokemon identity",async()=>{
 const pending=sendPartyItemCommand(1,0);
 emit(101,{...reply(id()),outcome:{instanceId:1,partySlot:0,needsMoveSlot:true,message:"Choose a move"}});
 await pending;
 expect(useCQInventoryStore.getState().pendingTMHM).toMatchObject({pokemonRowId:7,partySlot:0});
});

const menu = (requestId:string) => ({ success:true, requestId, characterId:42, merchantId:1, name:"Shop", items:[], money:1000 });
test("opening is correlated and bound to the selected character",async()=>{
  useCQInventoryStore.getState().closeShop();
  const pending=openShopForActor(1001);
  expect(net.send.mock.calls[0]).toEqual([94,{requestId:id(),characterId:42,actorId:1001}]);
  emit(95,menu("unrelated")); expect(useCQInventoryStore.getState().shopOpen).toBe(false);
  emit(95,menu(id())); await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({shopOpen:true,inventoryCommandPending:false});
});
test.each(["timeout","rejection","malformed","overtaken","send failure"])("open %s preserves state without retries",async mode=>{
  useCQInventoryStore.getState().closeShop();
  if(mode==="send failure") net.send.mockRejectedValue(new Error("failed"));
  const pending=openShopForActor(1001);
  if(mode==="timeout") await vi.advanceTimersByTimeAsync(10000);
  else if(mode!=="send failure") {
    if(mode==="overtaken") useCQInventoryStore.getState().setMoney(850);
    emit(95,mode==="rejection"?{success:false,requestId:id(),error:"denied"}:mode==="malformed"?{...menu(id()),characterId:99}:menu(id()));
  }
  await pending;
  expect(useCQInventoryStore.getState().shopOpen).toBe(false);
  expect(useCQInventoryStore.getState().inventoryCommandError).toBeTruthy();
  expect(net.send).toHaveBeenCalledTimes(1); expect(net.read).not.toHaveBeenCalled();
  emit(95,menu(id())); expect(useCQInventoryStore.getState().shopOpen).toBe(false);
});
test.each(["scene","character","close"])("open retires on %s and ignores a late menu",async mode=>{
  const pending=openShopForActor(1001); const requestId=id();
  if(mode==="scene") retire();
  else if(mode==="character") usePlayerCharacterStore.getState().setCharacterProfile({id:99});
  else useCQInventoryStore.getState().closeShop();
  await pending;
  emit(95,menu(requestId)); expect(useCQInventoryStore.getState().shopOpen).toBe(false);
  expect(net.listeners.get(95)?.size).toBe(0);
});
