import { create } from "zustand";

export type GameScreen =
  | "title"
  | "login"
  | "register"
  | "characterSelect"
  | "characterCreate"
  | "game";

interface GameScreenStore {
  currentScreen: GameScreen;
  setScreen: (screen: GameScreen) => void;
}

const useGameScreenStore = create<GameScreenStore>((set) => ({
  currentScreen: "title",
  setScreen: (screen) => set({ currentScreen: screen }),
}));

export default useGameScreenStore;
