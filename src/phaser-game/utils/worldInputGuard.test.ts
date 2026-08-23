import { beforeEach, describe, expect, it } from "vitest";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import { getWorldInputFreezeReason } from "./worldInputGuard";

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
