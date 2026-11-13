import { beforeEach, afterEach, expect, test, vi } from "vitest";
const net = vi.hoisted(() => ({ listeners:new Map<number,Set<(reply:any)=>void>>(),send:vi.fn(),read:vi.fn(),generation:0,retired:new Set<()=>void>() }));
vi.mock("@/net",()=>({OpCodes:{GameCornerSlotPlayRequest:136,GameCornerSlotResultResponse:137,GameCornerPrizeListRequest:138,GameCornerPrizeListResponse:139,GameCornerPrizeBuyRequest:140,GameCornerPrizeBuyResponse:141,PokemonPCOpenRequest:108,PokemonPCOpenResponse:109,PokemonPCDepositRequest:110,PokemonPCDepositResponse:111,PokemonPCWithdrawRequest:112,PokemonPCWithdrawResponse:113,PokemonPCReleaseRequest:114,PokemonPCReleaseResponse:115,PokemonPCSwitchBoxRequest:116,PokemonPCSwitchBoxResponse:117,PokemonPartyReorderRequest:90,PokemonPartyReorderResponse:91,CQItemUseRequest:100,CQItemUseResponse:101,ItemPickupRequest:106,ItemPickupResponse:107,RepelUseRequest:143,RepelUseResponse:144,CQMerchantOpenRequest:94,CQMerchantOpenResponse:95,CQMerchantBuyRequest:96,CQMerchantBuyResponse:97,CQMerchantSellRequest:98,CQMerchantSellResponse:99},WorldSocket:{sendStreamJsonMessage:net.send,get sessionGeneration(){return net.generation;},subscribeSessionRetirement:(receive:()=>void)=>{net.retired.add(receive);return()=>net.retired.delete(receive);}}}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onInventoryCommand:(opcode:number,receive:(reply:any)=>void)=>{
  if (!net.listeners.has(opcode)) net.listeners.set(opcode,new Set());
  const listeners=net.listeners.get(opcode)!;listeners.add(receive);return()=>listeners.delete(receive);
}}));
vi.mock("./GameplayRecoveryService",async importOriginal=>({...await importOriginal<typeof import("./GameplayRecoveryService")>(),readCurrentGameplayState:net.read}));
vi.mock("@/services/audio/AudioManager",()=>({default:{playSFX:vi.fn()}}));
import {playSlotCommand,buyPrizeCommand,openPrizeWindow} from "./GameCornerCommandService";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import {showCoinCaseBalance,refreshSlotCoinBalance} from "./CoinBalanceService";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import useChatStore from "@/stores/ChatStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import {openPokemonPC,depositPokemon,withdrawPokemon,releasePokemon,switchPokemonBox} from "./PCCommandService";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import {openShopForActor,buyShopItem,sellShopItem} from "./ShopCommandService";

import {watchInteractionPosition,bindInventoryScene,sendPickupCommand,sendPartyItemCommand,sendRepelItemCommand,sendPartyReorderCommand,refreshOwnedGameplayResources,acceptResourceChangeNotification} from "./InventoryCommandService";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import AudioManager from "@/services/audio/AudioManager";
const party = [{rowId:7,curHp:21}] as any;
const actions = ["buy","sell","party","repel","reorder","pcDeposit","pcWithdraw","pcRelease","pcSwitch"] as const;
const send = (action:typeof actions[number]) => action==="pcDeposit"?depositPokemon(7,0):action==="pcWithdraw"?withdrawPokemon(0,7):action==="pcRelease"?releasePokemon(0,7):action==="pcSwitch"?switchPokemonBox(0):action==="buy"?buyShopItem(1,1,10):action==="sell"?sellShopItem(7):action==="repel"?sendRepelItemCommand(1):action==="reorder"?sendPartyReorderCommand([7]):sendPartyItemCommand(1,0);
const opcode = (action:typeof actions[number]) => action==="pcDeposit"?111:action==="pcWithdraw"?113:action==="pcRelease"?115:action==="pcSwitch"?117:action==="buy"?97:action==="sell"?99:action==="repel"?144:action==="reorder"?91:101;
const reply = (requestId:string, revision=5) => ({success:true,requestId,characterId:42,sourceId:10,pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[{id:10,mapId:50,x:7,y:7,direction:"UP"}]},instanceId:1,message:"Repel started",stepsLeft:100,inventory:{items:[],money:900,commandRevision:revision},party,outcome:{instanceId:1,partySlot:0,message:"Healed"}});
let retire:()=>void;
const id=()=>net.send.mock.calls.at(-1)![1].requestId;
const emit=(opcode:number,reply:any)=>net.listeners.get(opcode)?.forEach(receive=>receive(reply));
beforeEach(()=>{
  useGameScreenStore.getState().setScreen("game"); net.generation=0;net.retired.clear();
  vi.useFakeTimers();net.listeners.clear();net.send.mockReset().mockResolvedValue(undefined);net.read.mockReset();
  vi.mocked(AudioManager.playSFX).mockClear();
  useGameStatusStore.setState({isInventoryOpen:true,playerTileContext:{mapId:50,x:7,y:8,direction:"UP"}});
  useCQInventoryStore.setState({items:[],money:1000,coins:10,commandRevision:4,shopOpen:true,inventoryCommandPending:false});
  usePlayerCharacterStore.getState().setCharacterProfile({id:42,pokedollars:1000});
  usePokemonPartyStore.getState().setParty(party);
  retire=bindInventoryScene();
  usePokemonPCStore.getState().openPC({sourceId:10,currentBox:0,boxCount:12,boxSize:20,box:[],party});
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
  else if(action.startsWith("pc")) expect(net.send.mock.calls[0][1].sourceId).toBe(10);
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
  net.read.mockResolvedValue({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],coins: 0, wallet:{characterId:42,pokedollars:800},commandRevision:6,party:recoveredParty});
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
  retire();resolve({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],coins: 0, wallet:{characterId:42,pokedollars:0},commandRevision:5,party:[]});
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
 net.read.mockResolvedValue({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],coins: 0, wallet:{characterId:42,pokedollars:1000},commandRevision:5,party});
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
  net.read.mockResolvedValue({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:items,coins: 0, wallet:{characterId:42,pokedollars:900},commandRevision:5,party});
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
 resolve({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],coins: 0, wallet:{characterId:42,pokedollars:900},commandRevision:5,party:recoveredParty});
 await pending;
 expect(useCQInventoryStore.getState()).toMatchObject({money:900,commandRevision:5,shopOpen:false,inventoryCommandPending:false});
 expect(usePokemonPartyStore.getState().party).toEqual(recoveredParty);
 expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test.each(["reply", "timeout"])("Repel reconciles after closing the bag on %s without late effects",async mode=>{
 net.read.mockResolvedValue({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],coins: 0, wallet:{characterId:42,pokedollars:900},commandRevision:5,party});
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
 net.read.mockResolvedValue({position:{mapId:50},eventFlags:[],pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, inventory:[],coins: 0, wallet:{characterId:42,pokedollars:800},commandRevision:6,party});
 const pending=sendRepelItemCommand(1); emit(144,{...reply(id()),...invalid}); await pending;
 expect(net.read).toHaveBeenCalledTimes(1);
 expect(useCQInventoryStore.getState()).toMatchObject({commandRevision:6,money:800});
 expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test.each(["pcDeposit","pcWithdraw","pcRelease","pcSwitch"] as const)("%s reconciles a delayed reply after closing without reopening or sound",async action=>{
  const pending=send(action); const requestId=id();
  usePokemonPCStore.getState().closePC();
  expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(true);
  emit(opcode(action),reply(requestId)); await pending;
  expect(usePokemonPCStore.getState()).toMatchObject({isOpen:false,sourceId:null,currentBox:0});
  expect(useCQInventoryStore.getState()).toMatchObject({commandRevision:5,inventoryCommandPending:false});
  expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test.each(["pc","shop"])("delayed %s opening retires after leaving its source position",async kind=>{
  usePokemonPCStore.getState().closePC();
  useCQInventoryStore.getState().closeShop();
  const pending=kind==="pc"?openPokemonPC(10):openShopForActor(1001);
  const requestId=id();
  useGameStatusStore.getState().setPlayerTileContext({mapId:50,x:8,y:8,direction:"RIGHT"});
  await pending;
  emit(kind==="pc"?109:95,{...reply(requestId),merchantId:1,name:"Shop",items:[],money:900});
  expect(usePokemonPCStore.getState().isOpen).toBe(false);
  expect(useCQInventoryStore.getState().shopOpen).toBe(false);
  expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(false);
  expect(net.read).not.toHaveBeenCalled(); expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test("delayed PC opening is inert after scene replacement",async()=>{
  usePokemonPCStore.getState().closePC();
  const pending=openPokemonPC(10), requestId=id();
  retire(); const next=bindInventoryScene();
  emit(109,reply(requestId)); await pending;
  expect(usePokemonPCStore.getState().isOpen).toBe(false);
  expect(useCQInventoryStore.getState().money).toBe(1000);
  expect(AudioManager.playSFX).not.toHaveBeenCalled(); next();
});

test.each(["pc","shop"])("facing-only changes follow the %s source policy",async kind=>{
  usePokemonPCStore.getState().closePC(); useCQInventoryStore.getState().closeShop();
  const pending=kind==="pc"?openPokemonPC(10):openShopForActor(1001), requestId=id();
  useGameStatusStore.getState().setPlayerTileContext({mapId:50,x:7,y:8,direction:"LEFT"});
  emit(kind==="pc"?109:95,{...reply(requestId),merchantId:1,name:"Shop",items:[],money:900}); await pending;
  expect(usePokemonPCStore.getState().isOpen).toBe(false);
  expect(useCQInventoryStore.getState().shopOpen).toBe(kind==="shop");
});

test("PC source movement preserves an already sent mutation's reconciliation",async()=>{
  const pending=depositPokemon(7,0), requestId=id();
  useGameStatusStore.getState().setPlayerTileContext({mapId:50,x:8,y:8,direction:"RIGHT"});
  emit(111,reply(requestId)); await pending;
  expect(usePokemonPCStore.getState().isOpen).toBe(false);
  expect(useCQInventoryStore.getState().commandRevision).toBe(5);
  expect(AudioManager.playSFX).not.toHaveBeenCalled(); expect(net.read).not.toHaveBeenCalled();
});

test("explicit resource refresh shares admission and applies one current projection",async()=>{
  let resolve!:(value:any)=>void; net.read.mockImplementation(()=>new Promise(done=>{resolve=done}));
  const pending=refreshOwnedGameplayResources();
  await depositPokemon(7,0); expect(net.send).not.toHaveBeenCalled();
  resolve({position:{mapId:50},eventFlags:[],pc:{currentBox:2,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:800},commandRevision:6,party});
  await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({money:800,commandRevision:6,inventoryCommandPending:false});
  expect(usePokemonPCStore.getState().currentBox).toBe(2);
  expect(AudioManager.playSFX).not.toHaveBeenCalled();
});

test("retiring an explicit refresh removes its ownership before late resolution",async()=>{
  let resolve!:(value:any)=>void; net.read.mockImplementation(()=>new Promise(done=>{resolve=done}));
  const pending=refreshOwnedGameplayResources(); retire();
  resolve({position:{mapId:50},eventFlags:[],pc:{currentBox:2,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:0},commandRevision:6,party:[]});
  await pending; expect(useCQInventoryStore.getState().money).toBe(1000);
  expect(usePokemonPartyStore.getState().party).toEqual(party);
});

test("an explicit refresh overtaken by a newer resource view stays inert",async()=>{
  let resolve!:(value:any)=>void; net.read.mockImplementation(()=>new Promise(done=>{resolve=done}));
  const pending=refreshOwnedGameplayResources();
  usePokemonPartyStore.getState().setParty([{rowId:9}] as any);
  resolve({position:{mapId:50},eventFlags:[],pc:{currentBox:2,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:0},commandRevision:6,party:[]});
  await pending; expect(usePokemonPartyStore.getState().party).toEqual([{rowId:9}]);
  expect(useCQInventoryStore.getState().money).toBe(1000);
});

test("resource notices coalesce behind a sent command and read current state",async()=>{
  net.read.mockResolvedValue({position:{mapId:50},eventFlags:[],pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:700},commandRevision:6,party});
  const pending=sendRepelItemCommand(1),requestId=id();
  for(let i=0;i<5;i++) acceptResourceChangeNotification({success:true,resourcesChanged:true,characterId:42,items:["historical"]});
  expect(net.read).not.toHaveBeenCalled();
  emit(144,reply(requestId)); await pending;
  await vi.waitFor(()=>expect(useCQInventoryStore.getState().commandRevision).toBe(6));
  expect(net.read).toHaveBeenCalledOnce(); expect(net.send).toHaveBeenCalledOnce();
  expect(useCQInventoryStore.getState().money).toBe(700);
});

test("untagged, foreign and retired-scene resource notices stay inert",()=>{
  acceptResourceChangeNotification({success:true,items:[],money:0});
  acceptResourceChangeNotification({success:true,resourcesChanged:true,characterId:43});
  retire(); acceptResourceChangeNotification({success:true,resourcesChanged:true,characterId:42});
  expect(net.read).not.toHaveBeenCalled(); expect(useCQInventoryStore.getState().money).toBe(1000);
});

test("character replacement discards an old owner's queued notice",async()=>{
  const pending=sendRepelItemCommand(1);
  acceptResourceChangeNotification({success:true,resourcesChanged:true,characterId:42});
  usePlayerCharacterStore.getState().setCharacterProfile({id:43,pokedollars:500}); await pending;
  expect(net.read).not.toHaveBeenCalled();
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


test.each(["screen","transport"])("inventory mutation retires on %s change without recovering on a replacement owner",async(kind)=>{
 const run=sendPartyReorderCommand([7]);const request=id();
 if(kind==="screen")useGameScreenStore.getState().setScreen("title");
 else{net.generation++;net.retired.forEach(receive=>receive());}
 emit(91,reply(request));await run;
 expect(useCQInventoryStore.getState().money).toBe(1000);
 expect(useCQInventoryStore.getState().inventoryCommandPending).toBe(false);
 expect(net.read).not.toHaveBeenCalled();expect(net.retired.size).toBe(0);
 expect(net.listeners.get(91)?.size).toBe(0);
});

test.each(["screen","transport"])("%s retirement discards a queued old resource hint",async(kind)=>{
 const run=sendPartyReorderCommand([7]);
 acceptResourceChangeNotification({success:true,resourcesChanged:true,characterId:42});
 if(kind==="screen")useGameScreenStore.getState().setScreen("title");
 else{net.generation++;net.retired.forEach(receive=>receive());}
 await run;
 expect(net.read).not.toHaveBeenCalled();expect(net.send).toHaveBeenCalledOnce();expect(useCQInventoryStore.getState().inventoryCommandError).toBeNull();
});

test.each(["screen","transport"])("%s retirement during recovery ignores an eventual old snapshot",async(kind)=>{
 let resolve!:(snapshot:any)=>void;net.read.mockImplementationOnce(()=>new Promise(done=>{resolve=done;}));
 const run=sendPartyReorderCommand([7]);await vi.advanceTimersByTimeAsync(10001);
 const signal=net.read.mock.calls[0][0] as AbortSignal;
 if(kind==="screen")useGameScreenStore.getState().setScreen("title");
 else{net.generation++;net.retired.forEach(receive=>receive());}
 expect(signal.aborted).toBe(true);
 resolve({position:{mapId:50},eventFlags:[],pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:800},commandRevision:5,party:[]});
 await run;expect(useCQInventoryStore.getState().money).toBe(1000);expect(net.send).toHaveBeenCalledOnce();expect(net.retired.size).toBe(0);
});


test("pickup joins inventory admission and reconciles actors after the committed bag",async()=>{
 const reconcile=vi.fn(async()=>{expect(useCQInventoryStore.getState().commandRevision).toBe(5);});
 const run=sendPickupCommand(14,reconcile);await sendPickupCommand(14,reconcile);
 expect(net.send).toHaveBeenCalledOnce();expect(net.send.mock.calls[0][1]).toMatchObject({actorId:14,command:{characterId:42,revision:4}});
 emit(107,{...reply(id()),actorId:14,itemId:1,itemName:"Potion"});await run;
 expect(reconcile).toHaveBeenCalledOnce();expect(net.read).not.toHaveBeenCalled();expect(useCQInventoryStore.getState().money).toBe(900);
});

test("pickup timeout recovers bag and actors without another grant or late reply application",async()=>{
 net.read.mockResolvedValueOnce({position:{mapId:50},eventFlags:[],pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:800},commandRevision:5,party:[]});
 const reconcile=vi.fn(async()=>{});const run=sendPickupCommand(14,reconcile);const request=id();
 await vi.advanceTimersByTimeAsync(10001);await run;
 expect(reconcile).toHaveBeenCalledOnce();expect(net.send).toHaveBeenCalledOnce();expect(useCQInventoryStore.getState().money).toBe(800);
 emit(107,{...reply(request),actorId:14,itemId:1,itemName:"Potion"});expect(reconcile).toHaveBeenCalledOnce();expect(useCQInventoryStore.getState().money).toBe(800);
});

test("retiring during actor reconciliation prevents pickup presentation in the next scene",async()=>{
 let finish!:()=>void;let signal!:AbortSignal;
 const reconcile=vi.fn((owned:AbortSignal)=>{signal=owned;return new Promise<void>(done=>{finish=done;});});
 const run=sendPickupCommand(14,reconcile);emit(107,{...reply(id()),actorId:14,itemId:1,itemName:"Potion"});await Promise.resolve();
 retire();retire=bindInventoryScene();expect(signal.aborted).toBe(true);finish();await run;
 expect(AudioManager.playSFX).not.toHaveBeenCalled();expect(net.send).toHaveBeenCalledOnce();
});


test("pickup actor-read failure recovers current bag and retries only reconciliation",async()=>{
 const reconcile=vi.fn().mockRejectedValueOnce(new Error("actor read failed")).mockResolvedValueOnce(undefined);
 net.read.mockResolvedValueOnce({position:{mapId:50},eventFlags:[],pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[],coins: 0, wallet:{characterId:42,pokedollars:900},commandRevision:5,party:[]});
 const run=sendPickupCommand(14,reconcile);emit(107,{...reply(id()),actorId:14,itemId:1,itemName:"Potion"});await run;
 expect(net.send).toHaveBeenCalledOnce();expect(net.read).toHaveBeenCalledOnce();expect(reconcile).toHaveBeenCalledTimes(2);expect(AudioManager.playSFX).not.toHaveBeenCalled();expect(useCQInventoryStore.getState().commandRevision).toBe(5);
});


test("source menu watcher retires once on same-position scene replacement",()=>{
 const closed=vi.fn();const stop=watchInteractionPosition(closed);
 const oldRetire=retire;retire=bindInventoryScene();expect(closed).toHaveBeenCalledTimes(1);
 oldRetire();retire();expect(closed).toHaveBeenCalledTimes(1);stop();
});

test("removed source watcher and obsolete scene cleanup cannot retire a newer menu",()=>{
 const closed=vi.fn();const stop=watchInteractionPosition(closed);stop();
 const oldRetire=retire;retire=bindInventoryScene();expect(closed).not.toHaveBeenCalled();
 const current=vi.fn();const stopCurrent=watchInteractionPosition(current);
 oldRetire();expect(current).not.toHaveBeenCalled();retire();expect(current).toHaveBeenCalledTimes(1);stopCurrent();
});


const coinCase = {instance:{id:69,quantity:1},item:{shortName:"COIN_CASE",name:"Coin Case"}} as any;
const coinSnapshot = (coins=60,money=0) => ({position:{mapId:50,x:7,y:8},eventFlags:[],pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]},inventory:[coinCase],coins,wallet:{characterId:42,pokedollars:money},commandRevision:5,party});

test("Coin Case reads current owned item, coins and wallet through the shared resource owner",async()=>{
  useCQInventoryStore.setState({items:[coinCase]});
  net.read.mockResolvedValueOnce(coinSnapshot());
  const messages=useChatStore.getState().messages.length;
  await showCoinCaseBalance(69);
  expect(net.send).not.toHaveBeenCalled();expect(net.read).toHaveBeenCalledOnce();
  expect(useCQInventoryStore.getState()).toMatchObject({money:0,coins:60,commandRevision:5});
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(0);
  expect(useChatStore.getState().messages.slice(messages).at(-1)?.text).toBe("You have 60 coins.");
});

test.each(["missing item","wrong item","missing coins","failed read"])("Coin Case %s cannot publish a guessed balance",async mode=>{
  useCQInventoryStore.setState({items:[coinCase]});
  const snapshot:any=coinSnapshot();
  if(mode==="missing item")snapshot.inventory=[];
  if(mode==="wrong item")snapshot.inventory=[{...coinCase,item:{shortName:"POTION"}}];
  if(mode==="missing coins")delete snapshot.coins;
  if(mode==="failed read")net.read.mockRejectedValueOnce(new Error("unavailable"));else net.read.mockResolvedValueOnce(snapshot);
  await showCoinCaseBalance(69);
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,coins:10,inventoryCommandError:"Could not read your Coin Case. Please try again."});
  expect(net.send).not.toHaveBeenCalled();
});

test.each(["bag","item","character","screen","transport"])("Coin Case %s retirement ignores late read and balance text",async mode=>{
  useCQInventoryStore.setState({items:[coinCase]});
  let finish!:(value:any)=>void;net.read.mockImplementationOnce(()=>new Promise(done=>{finish=done;}));
  const messages=useChatStore.getState().messages.length;const run=showCoinCaseBalance(69);
  if(mode==="bag")useGameStatusStore.setState({isInventoryOpen:false});
  if(mode==="item")useCQInventoryStore.setState({items:[]});
  if(mode==="character")usePlayerCharacterStore.getState().setCharacterProfile({id:43});
  if(mode==="screen")useGameScreenStore.getState().setScreen("title");
  if(mode==="transport"){net.generation++;net.retired.forEach(receive=>receive());}
  finish(coinSnapshot());await run;
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,coins:10});
  expect(useChatStore.getState().messages).toHaveLength(messages);
});

test("slot opening shows only a coherent owned read and never sends legacy balance requests",async()=>{
  useSlotMachineStore.getState().openSlotMachine(6,8);
  let finish!:(value:any)=>void;net.read.mockImplementationOnce(()=>new Promise(done=>{finish=done;}));
  const run=refreshSlotCoinBalance(new AbortController().signal);
  expect(useSlotMachineStore.getState()).toMatchObject({coins:0,coinsReady:false,coinsLoading:true});
  finish(coinSnapshot());await run;
  expect(net.send).not.toHaveBeenCalled();
  expect(useCQInventoryStore.getState()).toMatchObject({money:0,coins:60});
  expect(useSlotMachineStore.getState()).toMatchObject({coins:60,coinsReady:true,coinsLoading:false});
});

test.each(["source","close","reopen","scene","character","screen","transport","abort"])("slot %s retirement rejects late balance application",async mode=>{
  useSlotMachineStore.getState().openSlotMachine(6,8);
  const controller=new AbortController();let finish!:(value:any)=>void;net.read.mockImplementationOnce(()=>new Promise(done=>{finish=done;}));
  const run=refreshSlotCoinBalance(controller.signal);
  if(mode==="source")useGameStatusStore.getState().setPlayerTileContext({mapId:50,x:8,y:8,direction:"RIGHT"});
  if(mode==="close"||mode==="reopen")useSlotMachineStore.getState().closeSlotMachine();
  if(mode==="reopen")useSlotMachineStore.getState().openSlotMachine(6,8);
  if(mode==="scene")retire();
  if(mode==="character")usePlayerCharacterStore.getState().setCharacterProfile({id:43});
  if(mode==="screen")useGameScreenStore.getState().setScreen("title");
  if(mode==="transport"){net.generation++;net.retired.forEach(receive=>receive());}
  if(mode==="abort")controller.abort();
  finish(coinSnapshot());await run;
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,coins:10});
  expect(useSlotMachineStore.getState()).toMatchObject({coins:0,coinsReady:false});
});

test("failed slot read keeps spin unavailable and an explicit retry can recover",async()=>{
  useSlotMachineStore.getState().openSlotMachine(6,8);
  const signal=new AbortController().signal;
  net.read.mockRejectedValueOnce(new Error("timeout"));await refreshSlotCoinBalance(signal);
  expect(useSlotMachineStore.getState()).toMatchObject({coins:0,coinsReady:false,coinsLoading:false});
  useSlotMachineStore.getState().startSpin();expect(useSlotMachineStore.getState().isSpinning).toBe(false);
  net.read.mockResolvedValueOnce(coinSnapshot(1,900));await refreshSlotCoinBalance(signal);
  expect(useSlotMachineStore.getState()).toMatchObject({coins:1,coinsReady:true,coinsLoading:false});
  expect(net.send).not.toHaveBeenCalled();expect(net.read).toHaveBeenCalledTimes(2);
});


const source={mapId:50,fromX:7,fromY:8};
const slotReceipt=(requestId:string)=>({success:true,version:1,kind:"slot",requestId,characterId:42,commandRevision:5,machineX:6,machineY:8,bet:2,reelPositions:[1,2,3],reels:[["cherry"],["cherry"],["cherry"]],payout:3,matchLine:"middle",isLucky:false});
const prizeReceipt=(requestId:string)=>({success:true,version:1,kind:"prize",requestId,characterId:42,commandRevision:5,prizeId:7,prize:{id:7,type:"pokemon",name:"ABRA",coinCost:180},prizeName:"ABRA",prizeLevel:9,addedToParty:true,pcBox:-1,pcSlot:-1,message:"Here you go!"});
function readySlot(){useSlotMachineStore.getState().openSlotMachine(6,8);useSlotMachineStore.setState({coins:10,coinsReady:true,isSpinning:true});}

test("slot receipt owns one wager and waits for current resources before animation",async()=>{
  readySlot();let finish!:(value:any)=>void;net.read.mockImplementationOnce(()=>new Promise(done=>{finish=done;}));
  const present=vi.fn();const run=playSlotCommand(2,6,8,present);
  expect(net.send.mock.calls[0]).toEqual([136,{...source,bet:2,machineX:6,machineY:8,requestId:id(),command:{characterId:42,revision:4}}]);
  await playSlotCommand(2,6,8,present);expect(net.send).toHaveBeenCalledOnce();
  emit(137,{...slotReceipt(id()),coins:9999,money:0});await Promise.resolve();await Promise.resolve();
  expect(present).not.toHaveBeenCalled();expect(useCQInventoryStore.getState()).toMatchObject({money:1000,coins:10});
  finish(coinSnapshot(7,800));await run;
  expect(useCQInventoryStore.getState()).toMatchObject({money:800,coins:7});
  expect(useSlotMachineStore.getState().coins).toBe(7);expect(present).toHaveBeenCalledOnce();
});

test("lost spin reply refreshes current balance and clears animation without another wager",async()=>{
  readySlot();net.read.mockResolvedValueOnce(coinSnapshot(8,1000));const present=vi.fn();
  const run=playSlotCommand(2,6,8,present),request=id();await vi.advanceTimersByTimeAsync(10001);await run;
  expect(net.send).toHaveBeenCalledOnce();expect(net.read).toHaveBeenCalledOnce();expect(present).not.toHaveBeenCalled();
  expect(useSlotMachineStore.getState()).toMatchObject({coins:8,coinsReady:true,isSpinning:false,payout:0});
  emit(137,slotReceipt(request));expect(present).not.toHaveBeenCalled();expect(useSlotMachineStore.getState().coins).toBe(8);
});

test.each(["source","close","reopen"])("spin %s retires presentation but reconciles its possible commit",async mode=>{
  readySlot();net.read.mockResolvedValueOnce(coinSnapshot(8,1000));const present=vi.fn();const run=playSlotCommand(2,6,8,present);
  if(mode==="source")useGameStatusStore.getState().setPlayerTileContext({mapId:50,x:8,y:8,direction:"RIGHT"});
  else useSlotMachineStore.getState().closeSlotMachine();
  if(mode==="reopen")readySlot();
  emit(137,slotReceipt(id()));await run;
  expect(net.send).toHaveBeenCalledOnce();expect(net.read).toHaveBeenCalledOnce();expect(present).not.toHaveBeenCalled();
  expect(useCQInventoryStore.getState().coins).toBe(8);
  if(mode==="reopen")expect(useSlotMachineStore.getState()).toMatchObject({coins:10,coinsReady:true});
});

test.each(["scene","character","screen","transport"])("spin %s owner retirement never recovers through the replacement",async mode=>{
  readySlot();const present=vi.fn();const run=playSlotCommand(2,6,8,present),request=id();
  if(mode==="scene")retire();
  if(mode==="character")usePlayerCharacterStore.getState().setCharacterProfile({id:43});
  if(mode==="screen")useGameScreenStore.getState().setScreen("title");
  if(mode==="transport"){net.generation++;net.retired.forEach(receive=>receive());}
  emit(137,slotReceipt(request));await run;
  expect(net.read).not.toHaveBeenCalled();expect(present).not.toHaveBeenCalled();expect(net.listeners.get(137)?.size).toBe(0);
});

test.each([{kind:"prize"},{version:2},{machineX:7},{characterId:43},{bet:1},{reelPositions:[-1,2,3]},{commandRevision:99}])("invalid slot receipt %j recovers current state without animation",async invalid=>{
  readySlot();net.read.mockResolvedValueOnce(coinSnapshot(8,1000));const present=vi.fn();const run=playSlotCommand(2,6,8,present);
  emit(137,{...slotReceipt(id()),...invalid});await run;
  expect(present).not.toHaveBeenCalled();expect(net.read).toHaveBeenCalledOnce();expect(net.send).toHaveBeenCalledOnce();
  expect(useSlotMachineStore.getState()).toMatchObject({coins:8,isSpinning:false,coinsReady:true});
});

test("failed post-spin current recovery stops the view and requires a read before another wager",async()=>{
  readySlot();net.read.mockRejectedValue(new Error("unavailable"));const present=vi.fn();const run=playSlotCommand(2,6,8,present);
  emit(137,slotReceipt(id()));await run;
  expect(useSlotMachineStore.getState()).toMatchObject({isSpinning:false,coinsReady:false,coinsLoading:false});
  await playSlotCommand(2,6,8,present);expect(net.send).toHaveBeenCalledOnce();expect(present).not.toHaveBeenCalled();
});

test("prize receipt projects current party and coins before source-owned confirmation",async()=>{
  net.read.mockResolvedValueOnce(coinSnapshot(30,1000));const dialogue=vi.spyOn(usePokemonDialogueStore.getState(),"openDialogue");
  const run=buyPrizeCommand(7,source,new AbortController().signal);
  expect(net.send.mock.calls[0]).toEqual([140,{...source,prizeId:7,requestId:id(),command:{characterId:42,revision:4}}]);
  emit(141,prizeReceipt(id()));await run;
  expect(useCQInventoryStore.getState().coins).toBe(30);expect(dialogue).toHaveBeenCalledWith(["Here you go!"]);dialogue.mockRestore();
});

test("lost prize reply refreshes current resources once without another purchase or historical confirmation",async()=>{
  net.read.mockResolvedValueOnce(coinSnapshot(30,1000));const dialogue=vi.spyOn(usePokemonDialogueStore.getState(),"openDialogue");
  const run=buyPrizeCommand(7,source,new AbortController().signal),request=id();await vi.advanceTimersByTimeAsync(10001);await run;
  emit(141,prizeReceipt(request));expect(net.send).toHaveBeenCalledOnce();expect(net.read).toHaveBeenCalledOnce();expect(dialogue).not.toHaveBeenCalled();dialogue.mockRestore();
});

test("closing a prize source retires confirmation while preserving committed resource reconciliation",async()=>{
  net.read.mockResolvedValueOnce(coinSnapshot(30,1000));const dialogue=vi.spyOn(usePokemonDialogueStore.getState(),"openDialogue");const controller=new AbortController();
  const run=buyPrizeCommand(7,source,controller.signal);controller.abort();emit(141,prizeReceipt(id()));await run;
  expect(net.read).toHaveBeenCalledOnce();expect(useCQInventoryStore.getState().coins).toBe(30);expect(dialogue).not.toHaveBeenCalled();dialogue.mockRestore();
});

const prizeMenu=(requestId:string)=>({success:true,requestId,characterId:42,prizeWindow:1,prizes:[{id:7,type:"pokemon",name:"ABRA",coinCost:180}],snapshot:coinSnapshot(500,1000)});
test("source-owned prize read opens only its current menu and selection uses the shared mutation owner",async()=>{
  net.read.mockResolvedValueOnce({...coinSnapshot(320,1000),commandRevision:6});const show=vi.fn(async()=>7);
  const run=openPrizeWindow(1,show,new AbortController().signal);
  expect(net.send.mock.calls[0]).toEqual([138,{...source,prizeWindow:1,requestId:id(),characterId:42}]);
  emit(139,prizeMenu(id()));await vi.waitFor(()=>expect(net.send).toHaveBeenCalledTimes(2));
  expect(show).toHaveBeenCalledOnce();expect(net.send.mock.calls[1][1]).toMatchObject({...source,prizeId:7,command:{characterId:42,revision:5}});
  emit(141,{...prizeReceipt(id()),commandRevision:6});await run;
  expect(net.read).toHaveBeenCalledOnce();expect(useCQInventoryStore.getState().coins).toBe(320);
});

test.each(["source","character","cancel"])("prize list %s retirement cannot open a late menu",async mode=>{
  const show=vi.fn(async()=>undefined),controller=new AbortController();const run=openPrizeWindow(1,show,controller.signal),request=id();
  if(mode==="source")useGameStatusStore.getState().setPlayerTileContext({mapId:50,x:8,y:8,direction:"RIGHT"});
  if(mode==="character")usePlayerCharacterStore.getState().setCharacterProfile({id:43});
  if(mode==="cancel")controller.abort();
  emit(139,prizeMenu(request));await run;expect(show).not.toHaveBeenCalled();expect(net.send).toHaveBeenCalledOnce();
});

test("a displayed prize menu loses its choice when its scene retires",async()=>{
  let complete!:(choice:undefined)=>void;let viewSignal!:AbortSignal;
  const show=vi.fn((_reply:any,signal:AbortSignal)=>{viewSignal=signal;return new Promise<undefined>(done=>{complete=done;signal.addEventListener("abort",()=>done(undefined),{once:true});});});
  const run=openPrizeWindow(1,show,new AbortController().signal);emit(139,prizeMenu(id()));await vi.waitFor(()=>expect(show).toHaveBeenCalledOnce());
  retire();expect(viewSignal.aborted).toBe(true);complete(undefined);await run;expect(net.send).toHaveBeenCalledOnce();
});


test("a known spin receipt cannot project resources older than its commit",async()=>{
  readySlot();const stale={...coinSnapshot(999,0),commandRevision:4};
  net.read.mockResolvedValueOnce(stale).mockResolvedValueOnce(stale);
  const present=vi.fn();const run=playSlotCommand(2,6,8,present);emit(137,slotReceipt(id()));await run;
  expect(present).not.toHaveBeenCalled();expect(net.send).toHaveBeenCalledOnce();
  expect(useCQInventoryStore.getState()).toMatchObject({coins:10,money:1000,commandRevision:4});
  expect(useSlotMachineStore.getState()).toMatchObject({isSpinning:false,coinsReady:false});
});
