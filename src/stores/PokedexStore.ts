import { create } from "zustand";

export interface PokedexSpecies {
  id: number;
  name: string;
  type1: string;
  type2?: string;
  pokedexType?: string;
  height?: string;
  weight?: number;
  pokedexText?: string;
  iconImage?: string;
  crySfx?: string;
  cryPitch?: number;
  cryLength?: number;
}

export interface PokedexStatus {
  pokemonId: number;
  seen: boolean;
  caught: boolean;
}

export interface TrainerCardData {
  name: string;
  money: number;
  timePlayed: number;
  badges: string[];
  badgeCount: number;
  pokedexSeen: number;
  pokedexCaught: number;
}

interface PokedexState {
  species: PokedexSpecies[];
  statusMap: Map<number, { seen: boolean; caught: boolean }>;
  isLoaded: boolean;
  trainerCard: TrainerCardData | null;

  setSpecies: (species: PokedexSpecies[]) => void;
  setStatus: (status: PokedexStatus[]) => void;
  setTrainerCard: (card: TrainerCardData) => void;
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
      map.set(s.pokemonId, { seen: s.seen, caught: s.caught });
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
