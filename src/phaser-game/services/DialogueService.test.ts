import { describe, expect, it } from "vitest";
import { parseDialogueText } from "./DialogueService";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";

describe("dialogue text normalization", () => {
  it("normalizes Game Boy Poke glyphs in fetched dialogue", () => {
    expect(
      parseDialogueText("A POKéMON is asleep!\nUse the POKé FLUTE and POKéDEX."),
    ).toEqual(["A POKÉMON is asleep!\nUse the POKÉ FLUTE and POKÉDEX."]);
  });

  it("normalizes direct dialogue store lines and choices", () => {
    const store = usePokemonDialogueStore.getState();
    store.resetDialogueState();

    store.openDialogue(["Welcome to our POKéMON CENTER!"]);
    expect(usePokemonDialogueStore.getState().lines).toEqual([
      "Welcome to our POKÉMON CENTER!",
    ]);

    store.showChoice("Use a POKé BALL?", () => {});
    expect(usePokemonDialogueStore.getState().choicePrompt).toBe("Use a POKÉ BALL?");

    usePokemonDialogueStore.getState().resetDialogueState();
  });
});
