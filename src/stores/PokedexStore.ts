import { create } from "zustand";

import type { PokedexSpeciesEntry, PokedexStatusEntry, TrainerCardResponse } from "@/net/generated/protocol";

interface PokedexState {
  species: PokedexSpeciesEntry[];
  statusMap: Map<number, { seen: boolean; caught: boolean }>;
  isLoaded: boolean;
  trainerCard: TrainerCardResponse | null;

  setSpecies: (species: PokedexSpeciesEntry[]) => void;
  setStatus: (status: PokedexStatusEntry[]) => void;
  setTrainerCard: (card: TrainerCardResponse) => void;
  isSeen: (pokemonId: number) => boolean;
  isCaught: (pokemonId: number) => boolean;
  getSeenCount: () => number;
  getCaughtCount: () => number;
}

const usePokedexStore = create<PokedexState>((set, get) => ({
  species: [],
  statusMap: new Map(),
  isLoaded: false,
  trainerCard: null,

  setSpecies: (species) => set({ species, isLoaded: true }),

  setStatus: (status) => {
    const map = new Map<number, { seen: boolean; caught: boolean }>();
    for (const s of status) {
      map.set(s.pokemonId, { seen: s.seen || s.caught, caught: s.caught });
    }
    set({ statusMap: map });
  },

  setTrainerCard: (card) => set({ trainerCard: card }),

  isSeen: (pokemonId) => {
    const entry = get().statusMap.get(pokemonId);
    return entry?.seen ?? false;
  },

  isCaught: (pokemonId) => {
    const entry = get().statusMap.get(pokemonId);
    return entry?.caught ?? false;
  },

  getSeenCount: () => {
    let count = 0;
    for (const entry of get().statusMap.values()) {
      if (entry.seen) count++;
    }
    return count;
  },

  getCaughtCount: () => {
    let count = 0;
    for (const entry of get().statusMap.values()) {
      if (entry.caught) count++;
    }
    return count;
  },
}));

export default usePokedexStore;
