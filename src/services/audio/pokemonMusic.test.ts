import { describe, expect, test } from "vitest";
import {
  musicTrackForMap,
  musicTrackForMapId,
  musicTrackForMapName,
  victoryMusicTrackForState,
} from "./pokemonMusic";

describe("Pokemon music metadata", () => {
  test("maps classic locations to imported Red/Blue music", () => {
    expect(musicTrackForMapId(0)).toBe("/sound/pokemon/music/pallet_town.ogg");
    expect(musicTrackForMapId(12)).toBe("/sound/pokemon/music/routes1.ogg");
    expect(musicTrackForMapId(22)).toBe("/sound/pokemon/music/routes4.ogg");
    expect(musicTrackForMapId(40)).toBe("/sound/pokemon/music/oaks_lab.ogg");
  });

  test("falls back to source map names when only a stitched-overworld name is available", () => {
    expect(musicTrackForMapName("Route 1")).toBe("/sound/pokemon/music/routes1.ogg");
    expect(musicTrackForMapName("OAKS_LAB")).toBe("/sound/pokemon/music/oaks_lab.ogg");
    expect(musicTrackForMap(null, "Route 1")).toBe("/sound/pokemon/music/routes1.ogg");
  });

  test("uses battle victory music constants without a delayed post-battle fallback", () => {
    expect(victoryMusicTrackForState("wild", null)).toBe(
      "/sound/pokemon/music/defeated_wild_mon.ogg",
    );
    expect(victoryMusicTrackForState("trainer", "BROCK")).toBe(
      "/sound/pokemon/music/defeated_gym_leader.ogg",
    );
  });
});
