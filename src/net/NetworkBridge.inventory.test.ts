import { beforeEach, expect, test, vi } from "vitest";
import { NetworkBridge } from "./NetworkBridge";
import { WorldSocket } from "./index";
import { CQInventoryResponse, CQMerchantBuyResponse, CQMerchantSellResponse } from "./generated/opcodes";
import type { CQInventoryItem } from "./generated/cqitems";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import AudioManager from "@/services/audio/AudioManager";

const stack = (id: number, quantity: number): CQInventoryItem => ({
  instance: { id, itemId: 1, quantity, charges: 0, ownerType: 0 },
  item: { id: 1, name: "Potion" } as CQInventoryItem["item"],
});

beforeEach(() => {
  NetworkBridge.initialize();
  useCQInventoryStore.getState().setInventory([stack(1, 95)], 1000,0);
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42, pokedollars: 1000 });
  vi.spyOn(AudioManager, "playSFX").mockResolvedValue(undefined);
});

test.each([CQMerchantBuyResponse,CQMerchantSellResponse])("unsolicited shop reply %d cannot apply a historical bag", async opcode => {
  WorldSocket.onJson?.(opcode,{success:true,requestId:"retired",inventory:{items:[],money:0,shopRevision:1}});
  await Promise.resolve();
  expect(useCQInventoryStore.getState().items).toEqual([stack(1,95)]);
  expect(useCQInventoryStore.getState().money).toBe(1000);
});

test("empty bag reads replace the whole bag and synchronize both money views", () => {
  WorldSocket.onJson?.(CQInventoryResponse,{success:true,items:[],money:0,shopRevision:0});
  expect(useCQInventoryStore.getState().items).toEqual([]);
  expect(useCQInventoryStore.getState().money).toBe(0);
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(0);
});

test("an older shop revision cannot rewind the standalone inventory view",()=>{
  useCQInventoryStore.getState().setInventory([stack(1,95)],1000,3);
  WorldSocket.onJson?.(CQInventoryResponse,{success:true,items:[],money:0,shopRevision:2});
  expect(useCQInventoryStore.getState()).toMatchObject({money:1000,shopRevision:3});
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
