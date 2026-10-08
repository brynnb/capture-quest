import { create } from "zustand";

import type { PokedexSpeciesEntry, PokedexStatusEntry, TrainerCardResponse, PokedexListResponse, PokedexStatusResponse } from "@/net/generated/protocol";

function statusMap(status:PokedexStatusEntry[]) {
  return new Map(status.map(s=>[s.pokemonId,{seen:s.seen||s.caught,caught:s.caught}]));
}

interface PokedexState {
  species: PokedexSpeciesEntry[];
  statusMap: Map<number, { seen: boolean; caught: boolean }>;
  isLoaded: boolean;
  trainerCard: TrainerCardResponse | null;

  applyRead: (reply:TrainerCardResponse|PokedexListResponse|PokedexStatusResponse)=>void;
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

  // A list is one server snapshot; observers must not see half its publication.
  applyRead: (reply)=>{
    if("badges" in reply)set({trainerCard:reply});
    else if("species" in reply)set({species:reply.species,isLoaded:true,statusMap:statusMap(reply.status)});
    else set({statusMap:statusMap(reply.status)});
  },

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
