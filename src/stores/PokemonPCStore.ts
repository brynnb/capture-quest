import { create } from "zustand";
import type { PokemonDTO } from "@/net/generated/world_api";

interface PokemonPCState {
  isOpen: boolean;
  currentBox: number;
  boxCount: number;
  boxSize: number;
  boxPokemon: PokemonDTO[];
  party: PokemonDTO[];

  openPC: (data: {
    currentBox: number;
    boxCount: number;
    boxSize: number;
    box: PokemonDTO[];
    party: PokemonDTO[];
  }) => void;
  closePC: () => void;
  setBox: (currentBox: number, box: PokemonDTO[]) => void;
  setBoxAndParty: (box: PokemonDTO[], party: PokemonDTO[]) => void;
}

const usePokemonPCStore = create<PokemonPCState>((set) => ({
  isOpen: false,
  currentBox: 0,
  boxCount: 12,
  boxSize: 20,
  boxPokemon: [],
  party: [],

  openPC: (data) =>
    set({
      isOpen: true,
      currentBox: data.currentBox,
      boxCount: data.boxCount,
      boxSize: data.boxSize,
      boxPokemon: data.box,
      party: data.party,
    }),
  closePC: () => set({ isOpen: false }),
  setBox: (currentBox, box) => set({ currentBox, boxPokemon: box }),
  setBoxAndParty: (box, party) => set({ boxPokemon: box, party }),
}));

export default usePokemonPCStore;
