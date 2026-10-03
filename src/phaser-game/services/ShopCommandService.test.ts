import { beforeEach, afterEach, expect, test, vi } from "vitest";
const net = vi.hoisted(() => ({ listeners:new Map<number,Set<(reply:any)=>void>>(),send:vi.fn(),read:vi.fn() }));
vi.mock("@/net",()=>({OpCodes:{CQMerchantOpenRequest:94,CQMerchantOpenResponse:95,CQMerchantBuyRequest:96,CQMerchantBuyResponse:97,CQMerchantSellRequest:98,CQMerchantSellResponse:99},WorldSocket:{sendStreamJsonMessage:net.send}}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onShopCommand:(opcode:number,receive:(reply:any)=>void)=>{
  if (!net.listeners.has(opcode)) net.listeners.set(opcode,new Set());
  const listeners=net.listeners.get(opcode)!;listeners.add(receive);return()=>listeners.delete(receive);
}}));
vi.mock("./GameplayRecoveryService",()=>({readCurrentGameplayState:net.read}));
vi.mock("@/services/audio/AudioManager",()=>({default:{playSFX:vi.fn()}}));
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import {bindShopScene,openShopForActor,buyShopItem,sellShopItem} from "./ShopCommandService";

let retire:()=>void;
const id=()=>net.send.mock.calls.at(-1)![1].requestId;
const emit=(opcode:number,reply:any)=>net.listeners.get(opcode)?.forEach(receive=>receive(reply));
beforeEach(()=>{
  vi.useFakeTimers();net.listeners.clear();net.send.mockReset().mockResolvedValue(undefined);net.read.mockReset();
  useCQInventoryStore.setState({items:[],money:1000,shopRevision:4,shopOpen:true,shopCommandPending:false});
  usePlayerCharacterStore.getState().setCharacterProfile({id:42,pokedollars:1000});
  retire=bindShopScene();
  useCQInventoryStore.getState().openShop(1,"Shop",[],1000);
});
afterEach(()=>{retire();vi.useRealTimers();});

test.each(["buy","sell"])("%s awaits correlation, carries durable identity and accepts one next revision",async action=>{
  const opcode=action==="buy"?97:99;
  const pending=action==="buy"?buyShopItem(1,1,10):sellShopItem(7);
  expect(net.send.mock.calls[0][1].shop).toEqual({characterId:42,revision:4});
  expect(useCQInventoryStore.getState().shopCommandPending).toBe(true);
  await buyShopItem(1,1,1);expect(net.send).toHaveBeenCalledTimes(1);
  emit(opcode,{success:true,requestId:"other",inventory:{items:[],money:900,shopRevision:5}});
  expect(useCQInventoryStore.getState().money).toBe(1000);
  emit(opcode,{success:true,requestId:id(),inventory:{items:[],money:900,shopRevision:5}});
  await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({money:900,shopRevision:5,shopCommandPending:false});
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(900);
  expect(net.read).not.toHaveBeenCalled();
});

test.each(["timeout","rejection","malformed","overtaken"])("%s reads current authority without resending a mutation",async mode=>{
  net.read.mockResolvedValue({inventory:[],wallet:{characterId:42,pokedollars:800},shopRevision:5});
  const pending=buyShopItem(1,1,10);
  if (mode==="timeout") await vi.advanceTimersByTimeAsync(10000);
  else {
    if(mode==="overtaken") useCQInventoryStore.getState().setMoney(850);
    emit(97,mode==="rejection"?{success:false,requestId:id(),error:"stale"}:{success:true,requestId:id(),inventory:{items:[],money:900,shopRevision:mode==="malformed"?99:5}});
  }
  await pending;
  expect(net.read).toHaveBeenCalledTimes(1);expect(net.send).toHaveBeenCalledTimes(1);
  expect(useCQInventoryStore.getState()).toMatchObject({money:800,shopRevision:5,shopCommandPending:false});
  emit(97,{success:true,requestId:id(),inventory:{items:[],money:900,shopRevision:5}});
  expect(useCQInventoryStore.getState().money).toBe(800);
});

test.each(["scene","character","close"])("%s retirement cancels subscriptions and ignores late results",async mode=>{
  const pending=buyShopItem(1,1,1);const requestId=id();
  if(mode==="scene") retire();
  if(mode==="character") usePlayerCharacterStore.getState().setCharacterProfile({id:43,pokedollars:700});
  if(mode==="close") useCQInventoryStore.getState().closeShop();
  await pending;
  expect(net.listeners.get(97)?.size).toBe(0);
  emit(97,{success:true,requestId,inventory:{items:[],money:900,shopRevision:5}});
  expect(useCQInventoryStore.getState().money).toBe(1000);expect(net.read).not.toHaveBeenCalled();
  expect(useCQInventoryStore.getState().shopCommandPending).toBe(false);
});

test("failed recovery leaves the bag intact and reports a controlled reconnect error",async()=>{
  net.read.mockRejectedValue(new Error("read timeout"));const pending=buyShopItem(1,1,1);
  await vi.advanceTimersByTimeAsync(10000);await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,shopRevision:4,shopCommandPending:false,shopCommandError:"Could not restore shop state. Please reconnect."});
  expect(net.send).toHaveBeenCalledTimes(1);
});

test("older scene cleanup cannot release or settle a newer scene's command",async()=>{
  const first=buyShopItem(1,1,1);const firstId=id();const oldRetire=retire;
  retire=bindShopScene();useCQInventoryStore.getState().openShop(1,"Shop",[],1000);const second=buyShopItem(1,1,1);const secondId=id();
  oldRetire();await first;
  expect(useCQInventoryStore.getState().shopCommandPending).toBe(true);
  emit(97,{success:true,requestId:firstId,inventory:{items:[],money:900,shopRevision:5}});
  expect(useCQInventoryStore.getState().money).toBe(1000);
  emit(97,{success:true,requestId:secondId,inventory:{items:[],money:900,shopRevision:5}});
  await second;expect(useCQInventoryStore.getState().shopCommandPending).toBe(false);
});

const menu = (requestId:string) => ({ success:true, requestId, characterId:42, merchantId:1, name:"Shop", items:[], money:1000 });
test("opening is correlated and bound to the selected character",async()=>{
  useCQInventoryStore.getState().closeShop();
  const pending=openShopForActor(1001);
  expect(net.send.mock.calls[0]).toEqual([94,{requestId:id(),characterId:42,actorId:1001}]);
  emit(95,menu("unrelated")); expect(useCQInventoryStore.getState().shopOpen).toBe(false);
  emit(95,menu(id())); await pending;
  expect(useCQInventoryStore.getState()).toMatchObject({shopOpen:true,shopCommandPending:false});
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
  expect(useCQInventoryStore.getState().shopCommandError).toBeTruthy();
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
