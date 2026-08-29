import { describe, expect, it } from "vitest";

import {
  OVERWORLD_OVERVIEW_ENTER_ZOOM,
  OVERWORLD_OVERVIEW_EXIT_ZOOM,
  preferOverworldOverviewAtZoom,
} from "./OverworldLodPolicy";

describe("overworld LOD zoom policy", () => {
  it("enters overview at the 4px source's 2x enlargement threshold", () => {
    expect(OVERWORLD_OVERVIEW_ENTER_ZOOM).toBe(0.5);
    expect(preferOverworldOverviewAtZoom(false, 0.5)).toBe(true);
    expect(preferOverworldOverviewAtZoom(false, 0.51)).toBe(false);
  });

  it("retains overview through a hysteresis band during pinch zoom", () => {
    expect(OVERWORLD_OVERVIEW_EXIT_ZOOM).toBe(0.625);
    expect(preferOverworldOverviewAtZoom(true, 0.6)).toBe(true);
    expect(preferOverworldOverviewAtZoom(true, 0.63)).toBe(false);
  });

  it("ignores invalid transient zoom samples", () => {
    expect(preferOverworldOverviewAtZoom(true, Number.NaN)).toBe(true);
    expect(preferOverworldOverviewAtZoom(false, 0)).toBe(false);
  });
});
