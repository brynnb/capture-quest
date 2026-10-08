import { beforeEach, expect, test, vi } from "vitest";
import { NetworkBridge } from "./NetworkBridge";
import { WorldSocket } from "./index";
import { CQItemUseResponse, CQInventoryResponse, CQMerchantBuyResponse, CQMerchantSellResponse, RepelUseResponse, PokemonPartyResponse, PokemonPartyReorderResponse, PokemonPCOpenResponse, PokemonPCDepositResponse, PokemonPCWithdrawResponse, PokemonPCReleaseResponse, PokemonPCSwitchBoxResponse, ResourcesChangedNotify, GameplayStateRequest, GameplayStateResponse } from "./generated/opcodes";
import {bindInventoryScene} from "@/phaser-game/services/InventoryCommandService";
import type { CQInventoryItem } from "./generated/cqitems";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import AudioManager from "@/services/audio/AudioManager";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";

const stack = (id: number, quantity: number): CQInventoryItem => ({
  instance: { id, itemId: 1, quantity, charges: 0, ownerType: 0 },
  item: { id: 1, name: "Potion" } as CQInventoryItem["item"],
});

test.each([PokemonPCOpenResponse,PokemonPCDepositResponse,PokemonPCWithdrawResponse,PokemonPCReleaseResponse,PokemonPCSwitchBoxResponse])("PC reply %d reaches the shared command subscriber", async opcode => {
  const net=await import("@/phaser-game/services/PhaserNetworkService");
  const receive=vi.fn(); const stop=net.onInventoryCommand(opcode,receive);
  const reply={success:false,requestId:"pc-routing",error:"rejected"};
  WorldSocket.onJson?.(opcode,reply);
  await vi.waitFor(()=>expect(receive).toHaveBeenCalledWith(reply));
  expect(receive).toHaveBeenCalledOnce(); stop();
});

beforeEach(() => {
  NetworkBridge.initialize();
  useCQInventoryStore.getState().setInventory([stack(1, 95)], 1000,0);
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42, pokedollars: 1000 });
  vi.spyOn(AudioManager, "playSFX").mockResolvedValue(undefined);
});

test.each([CQMerchantBuyResponse,CQMerchantSellResponse,CQItemUseResponse,RepelUseResponse,PokemonPartyResponse,PokemonPartyReorderResponse,PokemonPCOpenResponse,PokemonPCDepositResponse,PokemonPCWithdrawResponse,PokemonPCReleaseResponse,PokemonPCSwitchBoxResponse])("unsolicited inventory reply %d cannot apply a historical bag or party", async opcode => {
  const party=usePokemonPartyStore.getState().party;
  WorldSocket.onJson?.(opcode,{success:true,requestId:"retired",party:[],inventory:{items:[],money:0,commandRevision:1}});
  await Promise.resolve();
  expect(useCQInventoryStore.getState().items).toEqual([stack(1,95)]);
  expect(useCQInventoryStore.getState().money).toBe(1000);
  expect(usePokemonPartyStore.getState().party).toBe(party);
});

test("an unowned historical empty bag cannot replace current resources", () => {
  WorldSocket.onJson?.(CQInventoryResponse,{success:true,items:[],money:0,commandRevision:0});
  expect(useCQInventoryStore.getState().items).toEqual([stack(1,95)]);
  expect(useCQInventoryStore.getState().money).toBe(1000);
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(1000);
});

test.each([ResourcesChangedNotify])("notice %d causes an owned current read, never payload application",async opcode=>{
  const net=await import("@/phaser-game/services/PhaserNetworkService");
  const connected=vi.spyOn(net,"isConnected").mockReturnValue(true);
  const retire=bindInventoryScene();
  let requestId="";
  const send=vi.spyOn(WorldSocket,"sendStreamJsonMessage").mockImplementation(async(type,payload)=>{
    expect(type).toBe(GameplayStateRequest); requestId=(payload as {requestId:string}).requestId;
  });
  WorldSocket.onJson?.(opcode,{success:true,resourcesChanged:true,characterId:42,items:[],money:0,party:[]});
  await vi.waitFor(()=>expect(requestId).not.toBe(""));
  expect(useCQInventoryStore.getState().money).toBe(1000);
  WorldSocket.onJson?.(GameplayStateResponse,{success:true,requestId,position:{mapId:50},inventory:[],wallet:{characterId:42,pokedollars:600},commandRevision:3,party:[],eventFlags:[],pc:{currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}});
  await vi.waitFor(()=>expect(useCQInventoryStore.getState().money).toBe(600));
  expect(send).toHaveBeenCalledOnce(); retire(); send.mockRestore(); connected.mockRestore();
});

test("an older shop revision cannot rewind the standalone inventory view",()=>{
  useCQInventoryStore.getState().setInventory([stack(1,95)],1000,3);
  WorldSocket.onJson?.(CQInventoryResponse,{success:true,items:[],money:0,commandRevision:2});
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,commandRevision:3});
  expect(useCQInventoryStore.getState().items).toEqual([stack(1,95)]);
});

test.each([
  { success: false, error: "Failed to load inventory" },
  { success: true, items: [] },
  { success: true, money: 0 },
  { success: true, items: [], money: -1 },
  { success: true, items: [], money: 0x100000000 },
])("failed or incompatible bag reply leaves existing views intact: %j", reply => {
  WorldSocket.onJson?.(CQInventoryResponse, reply);
  expect(useCQInventoryStore.getState().items).toEqual([stack(1, 95)]);
  expect(useCQInventoryStore.getState().money).toBe(1000);
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(1000);
});
