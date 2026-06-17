import { create } from "zustand";
import type { PokemonDTO } from "@/net/generated/world_api";

interface PokemonPartyState {
  party: PokemonDTO[];
  isLoaded: boolean;

  setParty: (party: PokemonDTO[]) => void;
  clearParty: () => void;
}

const usePokemonPartyStore = create<PokemonPartyState>((set) => ({
  party: [],
  isLoaded: false,

  setParty: (party) => set({ party, isLoaded: true }),
  clearParty: () => set({ party: [], isLoaded: false }),
}));

export default usePokemonPartyStore;
