import { describe, expect, test } from "vitest";
import { TILE_SIZE } from "../constants";
import {
  MAXIMUM_LIVE_CHUNKS_PER_AXIS,
  MAXIMUM_LIVE_TILE_SPAN,
  MINIMUM_LIVE_TILE_SPAN,
  OVERWORLD_CHUNK_SIZE_TILES,
  cameraWorldViewToTileBounds,
  overworldChunkAt,
  overworldChunksCoveringBounds,
  planOverworldChunks,
} from "./OverworldChunkPlanner";

const broadMap = { minX: -512, minY: -512, maxX: 511, maxY: 511 };

function tileCamera(x: number, y: number, width: number, height: number) {
  return {
    x: x * TILE_SIZE,
    y: y * TILE_SIZE,
    width: width * TILE_SIZE,
    height: height * TILE_SIZE,
  };
}

describe("OverworldChunkPlanner", () => {
  test("uses a 50-tile minimum intent on a stable 64-tile grid", () => {
    const plan = planOverworldChunks({
      camera: tileCamera(20, 20, 10, 10),
      mapBounds: broadMap,
    });

    expect(plan.mode).toBe("exact");
    if (plan.mode !== "exact") return;

    expect(OVERWORLD_CHUNK_SIZE_TILES).toBe(64);
    expect(MINIMUM_LIVE_TILE_SPAN).toBe(50);
    expect(plan.visibleTileBounds).toEqual({
      minX: 20,
      minY: 20,
      maxX: 29,
      maxY: 29,
    });
    expect(plan.requiredTileBounds).toEqual({
      minX: 0,
      minY: 0,
      maxX: 49,
      maxY: 49,
    });
    expect(plan.requiredChunks.map((chunk) => chunk.key)).toEqual([
      "overworld:0:0",
    ]);
  });

  test("prefetches a one-chunk ring without exceeding three chunks per axis", () => {
    const plan = planOverworldChunks({
      camera: tileCamera(7, 7, 50, 50),
      mapBounds: broadMap,
    });

    expect(plan.mode).toBe("exact");
    if (plan.mode !== "exact") return;

    expect(plan.requiredChunks).toHaveLength(1);
    expect(plan.desiredChunks).toHaveLength(9);
    expect(plan.preloadChunks).toHaveLength(8);
    expect(plan.desiredChunks.map((chunk) => chunk.key)).toEqual([
      "overworld:-1:-1",
      "overworld:0:-1",
      "overworld:1:-1",
      "overworld:-1:0",
      "overworld:0:0",
      "overworld:1:0",
      "overworld:-1:1",
      "overworld:0:1",
      "overworld:1:1",
    ]);
    expect(plan.retentionChunkKeys).toHaveLength(25);
  });

  test("prefetches only axes whose complete margin fits the live cap", () => {
    const plan = planOverworldChunks({
      camera: tileCamera(0, 7, 65, 50),
      mapBounds: broadMap,
    });

    expect(plan.mode).toBe("exact");
    if (plan.mode !== "exact") return;

    expect(plan.requiredChunks.map((chunk) => chunk.key)).toEqual([
      "overworld:0:0",
      "overworld:1:0",
    ]);
    expect(new Set(plan.desiredChunks.map((chunk) => chunk.chunkX))).toEqual(
      new Set([0, 1]),
    );
    expect(new Set(plan.desiredChunks.map((chunk) => chunk.chunkY))).toEqual(
      new Set([-1, 0, 1]),
    );
    expect(plan.desiredChunks).toHaveLength(6);
  });

  test("uses floor division for negative tile coordinates and deterministic keys", () => {
    const input = {
      camera: tileCamera(-60, -60, 50, 50),
      mapBounds: broadMap,
    };
    const first = planOverworldChunks(input);
    const second = planOverworldChunks(input);

    expect(first).toEqual(second);
    expect(first.mode).toBe("exact");
    if (first.mode !== "exact") return;

    expect(first.requiredChunks).toHaveLength(1);
    expect(first.requiredChunks[0]).toEqual({
      key: "overworld:-1:-1",
      chunkX: -1,
      chunkY: -1,
      canonicalBounds: { minX: -64, minY: -64, maxX: -1, maxY: -1 },
      bounds: { minX: -64, minY: -64, maxX: -1, maxY: -1 },
    });
  });

  test("clamps requests to map bounds and shifts the minimum intent inward", () => {
    const plan = planOverworldChunks({
      camera: tileCamera(-100, -100, 10, 10),
      mapBounds: { minX: 10, minY: 20, maxX: 80, maxY: 70 },
    });

    expect(plan.mode).toBe("exact");
    if (plan.mode !== "exact") return;

    expect(plan.visibleTileBounds).toEqual({
      minX: 10,
      minY: 20,
      maxX: 10,
      maxY: 20,
    });
    expect(plan.requiredTileBounds).toEqual({
      minX: 10,
      minY: 20,
      maxX: 59,
      maxY: 69,
    });
    expect(plan.desiredTileBounds).toEqual({
      minX: 10,
      minY: 20,
      maxX: 80,
      maxY: 70,
    });
    for (const chunk of plan.desiredChunks) {
      expect(chunk.bounds.minX).toBeGreaterThanOrEqual(10);
      expect(chunk.bounds.minY).toBeGreaterThanOrEqual(20);
      expect(chunk.bounds.maxX).toBeLessThanOrEqual(80);
      expect(chunk.bounds.maxY).toBeLessThanOrEqual(70);
    }
  });

  test("keeps an aligned 192x192 exact view but switches wider views to overview", () => {
    const exact = planOverworldChunks({
      camera: tileCamera(
        64,
        64,
        MAXIMUM_LIVE_TILE_SPAN,
        MAXIMUM_LIVE_TILE_SPAN,
      ),
      mapBounds: broadMap,
    });
    expect(MAXIMUM_LIVE_CHUNKS_PER_AXIS).toBe(3);
    expect(MAXIMUM_LIVE_TILE_SPAN).toBe(192);
    expect(exact.mode).toBe("exact");
    if (exact.mode === "exact") {
      expect(exact.requiredChunks).toHaveLength(9);
      expect(exact.preloadChunks).toHaveLength(0);
      expect(exact.desiredTileBounds).toEqual({
        minX: 64,
        minY: 64,
        maxX: 255,
        maxY: 255,
      });
    }

    // Although this camera is only 180 tiles wide, its unaligned footprint
    // crosses four canonical chunks and would require a 256-tile live span.
    const overview = planOverworldChunks({
      camera: tileCamera(50, 64, 180, 64),
      mapBounds: broadMap,
    });
    expect(overview).toMatchObject({
      mode: "overview",
      reason: "visible-area-exceeds-exact-limit",
      desiredChunks: [],
    });
  });

  test("treats exact right and bottom pixel edges as exclusive", () => {
    expect(
      cameraWorldViewToTileBounds(
        { x: 0, y: 0, width: TILE_SIZE, height: TILE_SIZE },
        broadMap,
      ),
    ).toEqual({ minX: 0, minY: 0, maxX: 0, maxY: 0 });
  });

  test("rejects invalid camera and map bounds", () => {
    expect(() =>
      planOverworldChunks({
        camera: { x: 0, y: 0, width: 0, height: TILE_SIZE },
        mapBounds: broadMap,
      }),
    ).toThrow("camera width and height must be greater than zero");
    expect(() =>
      planOverworldChunks({
        camera: tileCamera(0, 0, 1, 1),
        mapBounds: { minX: 1, minY: 0, maxX: 0, maxY: 1 },
      }),
    ).toThrow("mapBounds must describe a non-empty inclusive area");
  });

  test("enumerates signed overview chunks with clipped edge requests", () => {
    expect(
      overworldChunksCoveringBounds(
        { minX: -1, minY: -65, maxX: 64, maxY: 0 },
        { minX: -50, minY: -234, maxX: 289, maxY: 125 },
      ),
    ).toEqual([
      overworldChunkAt(-1, -2, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(0, -2, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(1, -2, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(-1, -1, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(0, -1, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(1, -1, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(-1, 0, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(0, 0, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
      overworldChunkAt(1, 0, {
        minX: -50,
        minY: -234,
        maxX: 289,
        maxY: 125,
      }),
    ]);
  });

  test("clamps overview prefetch to the map's canonical chunk range", () => {
    const chunks = overworldChunksCoveringBounds(
      { minX: -50, minY: -234, maxX: -50, maxY: -234 },
      { minX: -50, minY: -234, maxX: 289, maxY: 125 },
      1,
    );

    expect(chunks.map((chunk) => chunk.key)).toEqual([
      "overworld:-1:-4",
      "overworld:0:-4",
      "overworld:-1:-3",
      "overworld:0:-3",
    ]);
    expect(chunks[0].bounds).toEqual({
      minX: -50,
      minY: -234,
      maxX: -1,
      maxY: -193,
    });
  });
});
