import { describe, expect, test } from "vitest";
import { getPokedexPresentation } from "./pokedexPresentation";

describe("Pokédex presentation", () => {
  test("keeps an unseen species hidden and unavailable", () => {
    expect(getPokedexPresentation("PIKACHU", false, false)).toEqual({
      displayName: "???",
      canOpen: false,
      isCaught: false,
    });
  });

  test("reveals a seen species without showing the caught marker", () => {
    expect(getPokedexPresentation("PIKACHU", true, false)).toEqual({
      displayName: "pikachu",
      canOpen: true,
      isCaught: false,
    });
  });

  test("shows the caught marker and treats ownership as seen", () => {
    expect(getPokedexPresentation("PIKACHU", false, true)).toEqual({
      displayName: "pikachu",
      canOpen: true,
      isCaught: true,
    });
  });
});
