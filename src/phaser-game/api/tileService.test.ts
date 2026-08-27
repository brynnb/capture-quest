import { describe, expect, it } from "vitest";

import { getTileImageUrl } from "./tileService";

describe("getTileImageUrl", () => {
  it("maps database IDs to versioned zero-indexed tile artwork", () => {
    expect(getTileImageUrl(1)).toMatch(
      /^\/phaser\/tile_images\/tile_0\.png\?v=[0-9a-f]{64}$/,
    );
    expect(getTileImageUrl(826)).toMatch(
      /^\/phaser\/tile_images\/tile_825\.png\?v=[0-9a-f]{64}$/,
    );
  });
});
