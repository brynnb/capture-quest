import type { PhaserActor } from "@/net/generated/world_api";

// A read never replaces a live event newer than its captured presentation view.
// Despawn markers also protect actors that were absent from the initial cache.
export class ActorReadView {
  private readonly before: Map<number, PhaserActor>;
  private readonly despawns: Map<number, object>;
  constructor(cache: ReadonlyMap<number, PhaserActor>, despawns: ReadonlyMap<number, object>) {
    this.before = new Map(cache);
    this.despawns = new Map(despawns);
  }
  changes(incoming: PhaserActor[], cache: ReadonlyMap<number, PhaserActor>, despawns: ReadonlyMap<number, object>) {
    const unchanged = (id: number) => this.before.get(id) === cache.get(id) && this.despawns.get(id) === despawns.get(id);
    const ids = new Set(incoming.map(actor => actor.id));
    return {
      updates: incoming.filter(actor => unchanged(actor.id)),
      removals: [...this.before.values()].filter(actor => !ids.has(actor.id) && unchanged(actor.id)),
    };
  }
}
