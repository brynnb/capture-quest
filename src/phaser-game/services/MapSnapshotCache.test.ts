import { describe, expect, test } from "vitest";
import { MapSnapshotCache } from "./MapSnapshotCache";

describe("MapSnapshotCache", () => {
  test("keeps the overworld pinned while bounding retained interiors", () => {
    const cache = new MapSnapshotCache<string>(3, new Set([0]));
    cache.set(0, "overworld");
    cache.set(74, "interior-a");
    cache.set(75, "interior-b");
    cache.set(76, "interior-c");

    expect(cache.size).toBe(3);
    expect(cache.get(0)).toBe("overworld");
    expect(cache.get(74)).toBeUndefined();
    expect(cache.get(75)).toBe("interior-b");
    expect(cache.get(76)).toBe("interior-c");
  });

  test("refreshes recently used entries", () => {
    const cache = new MapSnapshotCache<string>(2);
    cache.set(1, "one");
    cache.set(2, "two");
    expect(cache.get(1)).toBe("one");
    cache.set(3, "three");

    expect(cache.get(1)).toBe("one");
    expect(cache.get(2)).toBeUndefined();
  });
});
