import { describe, expect, it } from "vitest";

import audioManifest from "../../constants/audio_manifest.json";

describe("runtime audio manifest policy", () => {
  it("preloads only the three small UI effects", () => {
    expect(audioManifest.global).toEqual([
      "/sound/button_1.mp3",
      "/sound/buttonclick.mp3",
      "/sound/SFX_TURN_ON_PC.mp3",
    ]);
    expect(
      audioManifest.global.some((path) =>
        ["/pokemon/music/", "/pokemon/cries/", "/pokemon/moves/"].some(
          (segment) => path.includes(segment),
        ),
      ),
    ).toBe(false);
  });

  it("keeps generated audio available on demand", () => {
    expect(audioManifest.library.length).toBeGreaterThan(500);
    expect(audioManifest.library).toContain(
      "/sound/pokemon/music/pallet_town.ogg",
    );
  });
});
