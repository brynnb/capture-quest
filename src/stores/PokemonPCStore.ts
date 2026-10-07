import { create } from "zustand";
import type { PokemonDTO, PCStorageSnapshot, PCInteractionSource } from "@/net/generated/world_api";

interface PokemonPCState {
  isOpen: boolean;
  sourceId: number | null;
  currentBox: number;
  boxCount: number;
  boxSize: number;
  boxPokemon: PokemonDTO[];
  party: PokemonDTO[];
  sources: PCInteractionSource[];
  applySnapshot: (snapshot: PCStorageSnapshot, party: PokemonDTO[]) => void;

  openPC: (data: {
    sourceId: number;
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
  sourceId: null,
  currentBox: 0,
  boxCount: 12,
  boxSize: 20,
  boxPokemon: [],
  party: [],
  sources: [],
  applySnapshot: (snapshot, party) => set({ currentBox: snapshot.currentBox, boxCount: snapshot.boxCount, boxSize: snapshot.boxSize, boxPokemon: snapshot.box, sources: snapshot.sources, party }),

  openPC: (data) =>
    set({
      isOpen: true,
      sourceId: data.sourceId,
      currentBox: data.currentBox,
      boxCount: data.boxCount,
      boxSize: data.boxSize,
      boxPokemon: data.box,
      party: data.party,
    }),
  closePC: () => set({ isOpen: false, sourceId: null }),
  setBox: (currentBox, box) => set({ currentBox, boxPokemon: box }),
  setBoxAndParty: (box, party) => set({ boxPokemon: box, party }),
}));

export default usePokemonPCStore;

export function validatePCSnapshot(snapshot: PCStorageSnapshot): void {
  if (!snapshot || snapshot.boxCount !== 12 || snapshot.boxSize !== 20
    || !Number.isSafeInteger(snapshot.currentBox) || snapshot.currentBox < 0 || snapshot.currentBox >= snapshot.boxCount
    || !Array.isArray(snapshot.box) || snapshot.box.length > snapshot.boxSize || !Array.isArray(snapshot.sources)) {
    throw new Error("Incomplete PC snapshot");
  }
  const ids = new Set<number>(), slots = new Set<number>();
  for (const pokemon of snapshot.box) {
    if (!Number.isSafeInteger(pokemon.rowId) || pokemon.rowId! <= 0 || ids.has(pokemon.rowId!)
      || !Number.isSafeInteger(pokemon.boxSlot) || pokemon.boxSlot! < 0 || pokemon.boxSlot! >= snapshot.boxSize || slots.has(pokemon.boxSlot!)) throw new Error("Invalid PC membership");
    ids.add(pokemon.rowId!); slots.add(pokemon.boxSlot!);
  }
  const sources = new Set<number>();
  for (const source of snapshot.sources) {
    if (!Number.isSafeInteger(source.id) || source.id <= 0 || sources.has(source.id)
      || !Number.isSafeInteger(source.mapId) || source.mapId < 0
      || !Number.isSafeInteger(source.x) || source.x < 0 || !Number.isSafeInteger(source.y) || source.y < 0 || source.direction !== "UP") throw new Error("Invalid PC source");
    sources.add(source.id);
  }
}
