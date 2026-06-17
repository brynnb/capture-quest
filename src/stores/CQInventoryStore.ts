import { create } from "zustand";

export interface CQItemTemplate {
  id: number;
  name: string;
  shortName: string;
  price: number;
  vendingPrice?: number;
  itemType: number;
  isUsable: boolean;
  usesPartyMenu: boolean;
  isKeyItem: boolean;
  isGuardDrink: boolean;
  moveId?: number;
  stackable: boolean;
  stackSize: number;
  bonusHp: number;
  bonusAttack: number;
  bonusDefense: number;
  bonusSpeed: number;
  bonusSpecial: number;
  bonusAccuracy: number;
  bonusEvasion: number;
  bonusCatchRate: number;
  bonusExp: number;
  bonusEncounterRate: number;
  bonusCrit: number;
  bonusFlee: number;
  healAmount: number;
  statusCure?: string;
  ppRestore: number;
  revivePercent: number;
  ballModifier: number;
  loreText?: string;
  icon: number;
}

export interface CQItemInstance {
  id: number;
  itemId: number;
  charges: number;
  quantity: number;
  ownerId?: number;
  ownerType: number;
}

export interface CQInventoryItem {
  instance: CQItemInstance;
  item: CQItemTemplate;
}

export interface CQMerchantItem {
  itemId: number;
  displayOrder: number;
  priceOverride?: number;
  quantity: number;
  item: CQItemTemplate;
}

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
  updateAfterBuy: (
    itemId: number,
    quantity: number,
    instanceId: number,
    money: number,
    item: CQItemTemplate,
  ) => void;
  updateAfterSell: (instanceId: number, money: number) => void;
}

const useCQInventoryStore = create<CQInventoryState>((set, get) => ({
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

  updateAfterBuy: (itemId, quantity, instanceId, money, item) => {
    const { items } = get();
    // Check if we stacked onto an existing item
    const existingIdx = items.findIndex(
      (i) => i.instance.id === instanceId,
    );
    if (existingIdx >= 0) {
      // Update quantity on existing stack
      const updated = [...items];
      updated[existingIdx] = {
        ...updated[existingIdx],
        instance: {
          ...updated[existingIdx].instance,
          quantity: updated[existingIdx].instance.quantity + quantity,
        },
      };
      set({ items: updated, money });
    } else {
      // New item in inventory
      const newItem: CQInventoryItem = {
        instance: {
          id: instanceId,
          itemId,
          charges: 0,
          quantity,
          ownerType: 0,
        },
        item,
      };
      set({ items: [...items, newItem], money });
    }
  },

  updateAfterSell: (instanceId, money) => {
    const { items } = get();
    set({
      items: items.filter((i) => i.instance.id !== instanceId),
      money,
    });
  },
}));

export default useCQInventoryStore;
