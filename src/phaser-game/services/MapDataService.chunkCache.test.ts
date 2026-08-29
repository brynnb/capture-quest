import type { PhaserTile } from "@/net/generated/world_api";
import { describe, expect, test } from "vitest";

import { MapDataService } from "./MapDataService";

function tile(id: number, x: number, y: number): PhaserTile {
  return {
    id,
    x,
    y,
    tileImageId: id,
    mapId: 9999,
    collisionType: 0,
    talkOverTile: false,
    isNativeGameData: true,
    coordinateOrigin: "native",
    contentOrigin: "native",
  };
}

describe("MapDataService overworld chunk cache", () => {
  test("uses least-recently-used eviction across its eighteen chunk budget", () => {
    const service = new MapDataService();
    for (let index = 0; index < 18; index += 1) {
      const minX = index * 64;
      service.setOverworldTileChunk(
        `chunk-${index}`,
        { minX, minY: 0, maxX: minX + 63, maxY: 63 },
        [tile(index, minX, 0)],
      );
    }

    expect(service.getOverworldTileChunk("chunk-0")).toBeDefined();
    service.setOverworldTileChunk(
      "chunk-18",
      { minX: 18 * 64, minY: 0, maxX: 19 * 64 - 1, maxY: 63 },
      [tile(18, 18 * 64, 0)],
    );

    expect(service.getOverworldTileChunk("chunk-1")).toBeUndefined();
    expect(service.getOverworldTileChunk("chunk-0")).toBeDefined();
    expect(service.getOverworldTileChunk("chunk-18")).toBeDefined();
  });

  test("invalidates only the cached chunk containing an edited coordinate", () => {
    const service = new MapDataService();
    service.setOverworldTileChunk(
      "negative",
      { minX: -64, minY: -64, maxX: -1, maxY: -1 },
      [tile(1, -1, -1)],
    );
    service.setOverworldTileChunk(
      "positive",
      { minX: 0, minY: 0, maxX: 63, maxY: 63 },
      [tile(2, 0, 0)],
    );

    service.invalidateOverworldTileChunkAt(-1, -1);

    expect(service.getOverworldTileChunk("negative")).toBeUndefined();
    expect(service.getOverworldTileChunk("positive")?.tiles).toEqual([
      tile(2, 0, 0),
    ]);
  });

  test("invalidates a neighboring cache whose render halo includes the edit", () => {
    const service = new MapDataService();
    service.setOverworldTileChunk(
      "north",
      { minX: 0, minY: -64, maxX: 63, maxY: -1 },
      [tile(1, 0, -1)],
      { minX: 0, minY: -64, maxX: 64, maxY: 0 },
      [tile(1, 0, -1), tile(2, 0, 0)],
    );
    service.setOverworldTileChunk(
      "south",
      { minX: 0, minY: 0, maxX: 63, maxY: 63 },
      [tile(2, 0, 0)],
    );

    service.invalidateOverworldTileChunkAt(0, 0);

    expect(service.getOverworldTileChunk("north")).toBeUndefined();
    expect(service.getOverworldTileChunk("south")).toBeUndefined();
  });

  test("clears cached chunks with the rest of the map-data cache", () => {
    const service = new MapDataService();
    service.setOverworldTileChunk(
      "chunk",
      { minX: 0, minY: 0, maxX: 63, maxY: 63 },
      [tile(1, 0, 0)],
    );

    service.clearCache();

    expect(service.getOverworldTileChunk("chunk")).toBeUndefined();
  });
});
