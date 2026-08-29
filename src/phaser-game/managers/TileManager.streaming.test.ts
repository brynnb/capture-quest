import { afterEach, describe, expect, it, vi } from "vitest";

import { TileManager } from "./TileManager";

type Listener = (...args: unknown[]) => void;

function createHarness() {
  const listeners = new Map<string, Set<Listener>>();
  const requests: Array<{
    key: string;
    complete: boolean;
    xhrSettings?: Phaser.Types.Loader.XHRSettingsObject;
  }> = [];
  const textures = new Set<string>();
  let isLoading = false;

  const addListener = (event: string, listener: Listener) => {
    const registered = listeners.get(event) ?? new Set<Listener>();
    registered.add(listener);
    listeners.set(event, registered);
  };
  const emit = (event: string, ...args: unknown[]) => {
    for (const listener of [...(listeners.get(event) ?? [])]) {
      listener(...args);
    }
  };
  const load = {
    once: vi.fn((event: string, listener: Listener) => {
      const onceListener: Listener = (...args) => {
        load.off(event, onceListener);
        listener(...args);
      };
      addListener(event, onceListener);
    }),
    on: vi.fn((event: string, listener: Listener) => {
      addListener(event, listener);
    }),
    off: vi.fn((event: string, listener: Listener) => {
      listeners.get(event)?.delete(listener);
    }),
    image: vi.fn(
      (
        key: string,
        _url: string,
        xhrSettings?: Phaser.Types.Loader.XHRSettingsObject,
      ) => {
        requests.push({ key, complete: false, xhrSettings });
      },
    ),
    isLoading: vi.fn(() => isLoading),
    start: vi.fn(() => {
      isLoading = true;
    }),
  };
  const complete = (key: string) => {
    const request = requests.find((candidate) => candidate.key === key);
    if (!request) throw new Error(`missing request ${key}`);
    request.complete = true;
    textures.add(key);
    emit(`filecomplete-image-${key}`);
    isLoading = requests.some((candidate) => !candidate.complete);
  };
  const fail = (key: string) => {
    const request = requests.find((candidate) => candidate.key === key);
    if (!request) throw new Error(`missing request ${key}`);
    request.complete = true;
    emit("loaderror", { key });
    isLoading = requests.some((candidate) => !candidate.complete);
  };

  const rawManager = Object.create(TileManager.prototype) as Record<
    string,
    unknown
  >;
  const createFallbackTile = vi.fn();
  rawManager.scene = {
    textures: { exists: (key: string) => textures.has(key) },
    load,
  };
  rawManager.tileImageCache = new Map();
  rawManager.loadingTextures = new Set();
  rawManager.tileImageLoadQueue = Promise.resolve();
  rawManager.createFallbackTile = createFallbackTile;

  return {
    manager: rawManager as unknown as TileManager,
    load,
    requests,
    complete,
    fail,
    emit,
    createFallbackTile,
  };
}

describe("TileManager streamed texture loading", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("serializes tile batches sharing Phaser's scene loader", async () => {
    const { manager, load, requests, complete } = createHarness();

    const first = manager.loadTileImages([{ id: 1 }]);
    const second = manager.loadTileImages([{ id: 2 }]);
    await vi.waitFor(() =>
      expect(requests.map(({ key }) => key)).toEqual(["tile-1"]),
    );

    complete("tile-1");
    await first;
    await vi.waitFor(() =>
      expect(requests.map(({ key }) => key)).toEqual(["tile-1", "tile-2"]),
    );
    complete("tile-2");
    await second;

    expect(load.start).toHaveBeenCalledTimes(2);
    expect(requests[0].xhrSettings?.timeout).toBe(12_000);
  });

  it("removes only its own error listener and resolves with a fallback", async () => {
    const { manager, load, requests, fail, emit, createFallbackTile } =
      createHarness();
    const unrelatedErrorListener = vi.fn();
    load.on("loaderror", unrelatedErrorListener);

    const loading = manager.loadTileImages([{ id: 7 }]);
    await vi.waitFor(() => expect(requests).toHaveLength(1));
    fail("tile-7");
    await loading;
    emit("loaderror", { key: "someone-else" });

    expect(createFallbackTile).toHaveBeenCalledWith("tile-7", "7");
    expect(unrelatedErrorListener).toHaveBeenCalledTimes(2);
    expect(load.off).not.toHaveBeenCalledWith("loaderror");
  });

  it("resolves a stalled tile request with a fallback after a bounded wait", async () => {
    vi.useFakeTimers();
    const { manager, requests, createFallbackTile } = createHarness();
    const warning = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);

    const loading = manager.loadTileImages([{ id: 9 }]);
    await Promise.resolve();
    const completion = expect(loading).resolves.toBeUndefined();
    await vi.runOnlyPendingTimersAsync();
    await completion;

    expect(requests[0].xhrSettings?.timeout).toBe(12_000);
    expect(createFallbackTile).toHaveBeenCalledWith("tile-9", "9");
    warning.mockRestore();
  });
});
