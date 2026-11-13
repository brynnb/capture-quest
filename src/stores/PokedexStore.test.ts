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
    usePokedexStore.getState().applyRead({success:true,requestId:"normalize",characterId:42,status:[
      { pokemonId: 1, seen: true, caught: false },
      { pokemonId: 4, seen: false, caught: true },
      { pokemonId: 7, seen: false, caught: false },
    ]});

    const state = usePokedexStore.getState();
    expect(state.isSeen(1)).toBe(true);
    expect(state.isSeen(4)).toBe(true);
    expect(state.isCaught(4)).toBe(true);
    expect(state.getSeenCount()).toBe(2);
    expect(state.getCaughtCount()).toBe(1);
  });
  test("publishes a complete list snapshot in one store update",()=>{
    const publications:number[]=[];
    const stop=usePokedexStore.subscribe(state=>publications.push(state.statusMap.size));
    usePokedexStore.getState().applyRead({success:true,requestId:"read",characterId:42,species:[],status:[{pokemonId:25,seen:true,caught:true}]});
    stop();
    expect(publications).toEqual([1]);
    expect(usePokedexStore.getState().isLoaded).toBe(true);
  });
});
