import { create } from "zustand";

export interface SlotMachineState {
  isOpen: boolean;
  coins: number;
  bet: number;
  isSpinning: boolean;
  reelPositions: [number, number, number];
  reelStopped: [boolean, boolean, boolean];
  payout: number;
  message: string;
  matchLine: string;
  isLuckyMachine: boolean;

  openSlotMachine: (isLucky: boolean) => void;
  closeSlotMachine: () => void;
  setBet: (bet: number) => void;
  setCoins: (coins: number) => void;
  addCoins: (amount: number) => void;
  startSpin: () => void;
  stopReel: (reelIndex: number) => void;
  setReelPositions: (positions: [number, number, number]) => void;
  setReelStopped: (stopped: [boolean, boolean, boolean]) => void;
  setPayout: (payout: number) => void;
  setMessage: (message: string) => void;
  setMatchLine: (line: string) => void;
  setIsSpinning: (spinning: boolean) => void;
}

const useSlotMachineStore = create<SlotMachineState>((set, get) => ({
  isOpen: false,
  coins: 50,
  bet: 1,
  isSpinning: false,
  reelPositions: [0, 0, 0],
  reelStopped: [false, false, false],
  payout: 0,
  message: "Insert coins and pull the lever!",
  matchLine: "",
  isLuckyMachine: false,

  openSlotMachine: (isLucky: boolean) =>
    set({
      isOpen: true,
      isLuckyMachine: isLucky,
      isSpinning: false,
      reelStopped: [false, false, false],
      payout: 0,
      matchLine: "",
      message: "Insert coins and pull the lever!",
    }),

  closeSlotMachine: () => set({ isOpen: false }),

  setBet: (bet: number) => {
    if (!get().isSpinning) {
      set({ bet: Math.max(1, Math.min(3, bet)) });
    }
  },

  setCoins: (coins: number) => set({ coins }),
  addCoins: (amount: number) => set({ coins: get().coins + amount }),

  startSpin: () => {
    const state = get();
    if (state.isSpinning) return;
    if (state.coins < state.bet) {
      set({ message: "Not enough coins!" });
      return;
    }
    set({
      coins: state.coins - state.bet,
      isSpinning: true,
      reelStopped: [false, false, false],
      payout: 0,
      message: "Press STOP or click reels!",
    });
  },

  stopReel: (reelIndex: number) => {
    const state = get();
    if (!state.isSpinning) return;
    const stopped = [...state.reelStopped] as [boolean, boolean, boolean];
    // Must stop in order: 0, 1, 2
    const nextToStop = stopped.findIndex((s) => !s);
    if (nextToStop !== reelIndex) return;
    stopped[reelIndex] = true;
    set({ reelStopped: stopped });
  },

  setReelPositions: (positions) => set({ reelPositions: positions }),
  setReelStopped: (stopped) => set({ reelStopped: stopped }),
  setPayout: (payout) => set({ payout }),
  setMessage: (message) => set({ message }),
  setMatchLine: (line) => set({ matchLine: line }),
  setIsSpinning: (spinning) => set({ isSpinning: spinning }),
}));

export default useSlotMachineStore;
