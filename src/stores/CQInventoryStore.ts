import { create } from "zustand";

import type {
  CQItem as CQItemTemplate,
  CQItemInstance,
  CQInventoryItem,
  CQMerchantItem,
} from "@/net/generated/cqitems";

export type { CQItemTemplate, CQItemInstance, CQInventoryItem, CQMerchantItem };

// Item type constants
export const ITEM_TYPE_MISC = 0;
export const ITEM_TYPE_POKEBALL = 1;
export const ITEM_TYPE_MEDICINE = 2;
export const ITEM_TYPE_BATTLE_ITEM = 3;
export const ITEM_TYPE_FIELD_ITEM = 4;
export const ITEM_TYPE_TM = 5;
export const ITEM_TYPE_HM = 6;
export const ITEM_TYPE_EVOLUTION_STONE = 9;

export interface PendingTMHM {
  instanceId: number;
  partySlot: number;
  itemName: string;
  moveId?: number;
  moveName?: string;
  message: string;
}

interface CQInventoryState {
  items: CQInventoryItem[];
  money: number;

  // Merchant/shop state
  shopOpen: boolean;
  shopName: string;
  shopMerchantId: number | null;
  shopItems: CQMerchantItem[];

  // TM/HM pending move forget
  pendingTMHM: PendingTMHM | null;

  // Actions
  setInventory: (items: CQInventoryItem[], money: number) => void;
  setMoney: (money: number) => void;
  setPendingTMHM: (pending: PendingTMHM | null) => void;
  openShop: (
    merchantId: number,
    name: string,
    items: CQMerchantItem[],
    money: number,
  ) => void;
  closeShop: () => void;
}

const useCQInventoryStore = create<CQInventoryState>((set) => ({
  items: [],
  money: 0,
  shopOpen: false,
  shopName: "",
  shopMerchantId: null,
  shopItems: [],
  pendingTMHM: null,

  setInventory: (items, money) => set({ items, money }),

  setMoney: (money) => set({ money }),

  setPendingTMHM: (pending) => set({ pendingTMHM: pending }),

  openShop: (merchantId, name, items, money) =>
    set({
      shopOpen: true,
      shopMerchantId: merchantId,
      shopName: name,
      shopItems: items,
      money,
    }),

  closeShop: () =>
    set({
      shopOpen: false,
      shopName: "",
      shopMerchantId: null,
      shopItems: [],
    }),
}));

export default useCQInventoryStore;
