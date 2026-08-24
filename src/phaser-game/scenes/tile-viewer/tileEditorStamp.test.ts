import { describe, expect, it, vi } from "vitest";

import {
  MAX_CAPTURED_STAMP_TILES,
  captureTileStamp,
} from "./tileEditorStamp";

describe("captureTileStamp", () => {
  it("normalizes reverse drags and preserves empty gaps", () => {
    const tiles = new Map([
      ["3,4", 101],
      ["4,4", 102],
      ["3,5", 103],
    ]);

    const stamp = captureTileStamp(
      { x: 4, y: 5 },
      { x: 3, y: 4 },
      (x, y) => tiles.get(`${x},${y}`) ?? 0,
    );

    expect(stamp).toEqual({
      widthTiles: 2,
      heightTiles: 2,
      tileImageIds: [[101, 102], [103, 0]],
      populatedTiles: 3,
    });
  });

  it("accepts the largest server-safe selection", () => {
    const getTile = vi.fn(() => 7);
    const stamp = captureTileStamp(
      { x: 0, y: 0 },
      { x: 24, y: 19 },
      getTile,
    );

    expect(stamp).not.toBeNull();
    expect(stamp!.widthTiles * stamp!.heightTiles).toBe(MAX_CAPTURED_STAMP_TILES);
    expect(getTile).toHaveBeenCalledTimes(MAX_CAPTURED_STAMP_TILES);
  });

  it("rejects selections larger than one placement request", () => {
    expect(captureTileStamp(
      { x: 0, y: 0 },
      { x: 25, y: 19 },
      () => 7,
    )).toBeNull();
  });
});
