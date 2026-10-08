import { beforeEach, describe, expect, it } from "vitest";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import { getWorldInputFreezeReason, isInteractiveControlTarget } from "./worldInputGuard";

function resetInputState(): void {
  usePokeBattleStore.setState({ isInBattle: false });
  usePokemonDialogueStore.setState({
    isOpen: false,
    isChoicePending: false,
  });
  useCQInventoryStore.setState({ shopOpen: false });
  usePokemonPCStore.setState({ isOpen: false });
  useSlotMachineStore.setState({ isOpen: false });
  useGameStatusStore.setState({
    isInventoryOpen: false,
    isPokedexOpen: false,
    isTrainerCardOpen: false,
    isOptionsOpen: false,
    isHelpOpen: false,
    isTileManagerOpen: false,
    isArtStudioOpen: false,
    isGroupOpen: true,
  });
}

describe("world input guard", () => {
  beforeEach(resetInputState);

  it("keeps the world active for the compact party HUD", () => {
    expect(
      getWorldInputFreezeReason({ includeCutscene: false }),
    ).toBeNull();
  });

  it("freezes movement behind responsive panels", () => {
    useGameStatusStore.setState({ isInventoryOpen: true });
    expect(getWorldInputFreezeReason({ includeCutscene: false })).toBe("panel");

    useGameStatusStore.setState({
      isInventoryOpen: false,
      isTileManagerOpen: true,
    });
    expect(getWorldInputFreezeReason({ includeCutscene: false })).toBe("panel");
  });

  it("freezes movement behind full-screen game modals", () => {
    usePokemonPCStore.setState({ isOpen: true });
    expect(getWorldInputFreezeReason({ includeCutscene: false })).toBe("modal");

    usePokemonPCStore.setState({ isOpen: false });
    useSlotMachineStore.setState({ isOpen: true });
    expect(getWorldInputFreezeReason({ includeCutscene: false })).toBe("modal");
  });
});

describe("responsive HUD panel state", () => {
  beforeEach(resetInputState);

  it("keeps primary panels mutually exclusive", () => {
    useGameStatusStore.getState().toggleInventory();
    expect(useGameStatusStore.getState().isInventoryOpen).toBe(true);
    expect(useGameStatusStore.getState().isGroupOpen).toBe(false);

    useGameStatusStore.getState().toggleHelp();
    expect(useGameStatusStore.getState().isInventoryOpen).toBe(false);
    expect(useGameStatusStore.getState().isHelpOpen).toBe(true);

    useGameStatusStore.getState().toggleGroup();
    expect(useGameStatusStore.getState().isHelpOpen).toBe(false);
    expect(useGameStatusStore.getState().isGroupOpen).toBe(true);
  });
});


describe("global gesture control ownership",()=>{
 it("recognizes control descendants without claiming ordinary background clicks",()=>{
 for(const tag of ["button","a","input","select","textarea","div"]){
 const control=document.createElement(tag);
 if(tag==="a")control.setAttribute("href","#");
 if(tag==="div")control.setAttribute("role","button");
 const icon=document.createElementNS("http://www.w3.org/2000/svg","svg");control.append(icon);
 expect(isInteractiveControlTarget(control)).toBe(true);expect(isInteractiveControlTarget(icon)).toBe(true);
 }
 const background=document.createElement("div");expect(isInteractiveControlTarget(background)).toBe(false);
 const editor=document.createElement("div");editor.setAttribute("contenteditable","true");expect(isInteractiveControlTarget(editor)).toBe(true);
 expect(isInteractiveControlTarget(null)).toBe(false);
 });
});
