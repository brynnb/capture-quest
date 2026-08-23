export class MapSnapshotCache<T> {
  private readonly entries = new Map<number, T>();

  constructor(
    private readonly maximumEntries: number,
    private readonly pinnedKeys: ReadonlySet<number> = new Set(),
  ) {}

  get(key: number): T | undefined {
    const value = this.entries.get(key);
    if (value === undefined) return undefined;
    this.entries.delete(key);
    this.entries.set(key, value);
    return value;
  }

  set(key: number, value: T): void {
    this.entries.delete(key);
    this.entries.set(key, value);
    this.trim();
  }

  clear(): void {
    this.entries.clear();
  }

  get size(): number {
    return this.entries.size;
  }

  private trim(): void {
    while (this.entries.size > this.maximumEntries) {
      let removableKey: number | undefined;
      for (const key of this.entries.keys()) {
        if (!this.pinnedKeys.has(key)) {
          removableKey = key;
          break;
        }
      }
      if (removableKey === undefined) return;
      this.entries.delete(removableKey);
    }
  }
}
