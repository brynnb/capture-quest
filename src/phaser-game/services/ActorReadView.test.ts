import { expect, test } from "vitest";
import type { PhaserActor } from "@/net/generated/world_api";
import { ActorReadView } from "./ActorReadView";
const actor = (id: number, x = 0) => ({ id, x, y: 0, mapId: 50, objectType: "npc" } as PhaserActor);

test("unchanged actor read applies updates and absent removals", () => {
  const first = actor(1), removed = actor(2);
  const cache = new Map([[1, first], [2, removed]]);
  const view = new ActorReadView(cache, new Map());
  const update = actor(1, 4);
  expect(view.changes([update], cache, new Map())).toEqual({ updates: [update], removals: [removed] });
});

test("stream moves and spawns survive an older read through final projection", () => {
  const first = actor(1);
  const cache = new Map([[1, first]]);
  const view = new ActorReadView(cache, new Map());
  const stale = [actor(1, 1)];
  // This can happen after the response while map sprites are loading.
  cache.set(1, actor(1, 9)); cache.set(2, actor(2));
  expect(view.changes(stale, cache, new Map())).toEqual({ updates: [], removals: [] });
});

test("known and never-cached despawns cannot be resurrected by a late read", () => {
  const cache = new Map([[1, actor(1)]]);
  const despawns = new Map<number, object>();
  const view = new ActorReadView(cache, despawns);
  cache.delete(1); despawns.set(1, {}); despawns.set(2, {});
  expect(view.changes([actor(1), actor(2)], cache, despawns)).toEqual({ updates: [], removals: [] });
});
