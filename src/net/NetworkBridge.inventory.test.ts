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
  useCQInventoryStore.getState().setInventory([stack(1, 95)], 1000);
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42, pokedollars: 1000 });
  vi.spyOn(AudioManager, "playSFX").mockResolvedValue(undefined);
});

test("purchase uses the full committed split-stack bag even if the independent snapshot is lost", () => {
  const inventory = { items: [stack(1, 99), stack(2, 6)], money: 900 };
  const reply = { success: true, itemId: 1, instanceId: 1, quantity: 10, money: 900, inventory };
  WorldSocket.onJson?.(CQMerchantBuyResponse, reply);
  expect(useCQInventoryStore.getState().items).toEqual(inventory.items);
  expect(useCQInventoryStore.getState().money).toBe(900);
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(900);
  // Duplicate delivery and the compatibility stream cannot increment a stack.
  WorldSocket.onJson?.(CQMerchantBuyResponse, reply);
  WorldSocket.onJson?.(CQInventoryResponse, { success: true, ...inventory });
  expect(useCQInventoryStore.getState().items).toEqual(inventory.items);
});

test("sale and empty bag reads replace the whole bag and synchronize both money views", () => {
  WorldSocket.onJson?.(CQMerchantSellResponse, {
    success: true, instanceId: 1, itemName: "Potion", sellPrice: 4750,
    money: 5750, inventory: { items: [], money: 5750 },
  });
  expect(useCQInventoryStore.getState().items).toEqual([]);
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(5750);
  WorldSocket.onJson?.(CQInventoryResponse, { success: true, items: [], money: 0 });
  expect(useCQInventoryStore.getState().money).toBe(0);
  expect(usePlayerCharacterStore.getState().characterProfile.pokedollars).toBe(0);
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
