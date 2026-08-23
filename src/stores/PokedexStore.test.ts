import { beforeEach, describe, expect, test } from "vitest";
import usePokedexStore from "./PokedexStore";

describe("PokedexStore", () => {
  beforeEach(() => {
    usePokedexStore.setState({
      species: [],
      statusMap: new Map(),
      isLoaded: false,
      trainerCard: null,
    });
  });

  test("normalizes caught species to seen and reports both totals", () => {
    usePokedexStore.getState().setStatus([
      { pokemonId: 1, seen: true, caught: false },
      { pokemonId: 4, seen: false, caught: true },
      { pokemonId: 7, seen: false, caught: false },
    ]);

    const state = usePokedexStore.getState();
    expect(state.isSeen(1)).toBe(true);
    expect(state.isSeen(4)).toBe(true);
    expect(state.isCaught(4)).toBe(true);
    expect(state.getSeenCount()).toBe(2);
    expect(state.getCaughtCount()).toBe(1);
  });
});
