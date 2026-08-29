import type { Scene } from "phaser";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { ensureRuntimeTileCatalogCurrentMock, getApiUrlMock } = vi.hoisted(
  () => ({
    ensureRuntimeTileCatalogCurrentMock: vi.fn(async () => undefined),
    getApiUrlMock: vi.fn(
      (path: string) => `https://capturequest.test/api${path}`,
    ),
  }),
);

vi.mock("@/config", () => ({
  getApiUrl: getApiUrlMock,
}));

vi.mock("../services/RuntimeAssetCompatibility", () => ({
  ensureRuntimeTileCatalogCurrent: ensureRuntimeTileCatalogCurrentMock,
}));

import { RUNTIME_TILE_ASSET_VERSION } from "@/constants/runtime_asset_version";
import { TILE_SIZE } from "../constants";
import {
  type OverviewChunkRef,
  OverworldOverviewLayer,
} from "./OverworldOverviewLayer";

type Listener = (...args: unknown[]) => void;

interface LoadedImageRequest {
  key: string;
  url: string;
  xhrSettings?: Phaser.Types.Loader.XHRSettingsObject;
  completed: boolean;
}

function createHarness(chunkSize = 64, autoComplete = true) {
  const listeners = new Map<string, Set<Listener>>();
  const requests: LoadedImageRequest[] = [];
  const textureKeys = new Set<string>();
  const images: Array<{
    x: number;
    y: number;
    textureKey: string;
    setOrigin: ReturnType<typeof vi.fn>;
    setDisplaySize: ReturnType<typeof vi.fn>;
    setDepth: ReturnType<typeof vi.fn>;
    setVisible: ReturnType<typeof vi.fn>;
    destroy: ReturnType<typeof vi.fn>;
  }> = [];
  let isLoading = false;

  const emit = (event: string, ...args: unknown[]) => {
    for (const listener of [...(listeners.get(event) ?? [])]) {
      listener(...args);
    }
  };
  const addListener = (event: string, listener: Listener) => {
    const eventListeners = listeners.get(event) ?? new Set<Listener>();
    eventListeners.add(listener);
    listeners.set(event, eventListeners);
  };

  const loader = {
    once: vi.fn((event: string, listener: Listener) => {
      const onceListener: Listener = (...args) => {
        loader.off(event, onceListener);
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
        url: string,
        xhrSettings?: Phaser.Types.Loader.XHRSettingsObject,
      ) => {
        requests.push({ key, url, xhrSettings, completed: false });
      },
    ),
    isLoading: vi.fn(() => isLoading),
    start: vi.fn(() => {
      isLoading = true;
      if (!autoComplete) return;
      queueMicrotask(() => {
        for (const request of requests.filter((item) => !item.completed)) {
          request.completed = true;
          textureKeys.add(request.key);
          emit(`filecomplete-image-${request.key}`);
        }
        isLoading = false;
      });
    }),
  };
  const addImage = vi.fn((x: number, y: number, textureKey: string) => {
    const image = {
      x,
      y,
      textureKey,
      setOrigin: vi.fn(),
      setDisplaySize: vi.fn(),
      setDepth: vi.fn(),
      setVisible: vi.fn(),
      destroy: vi.fn(),
    };
    images.push(image);
    return image;
  });
  const textures = {
    exists: vi.fn((key: string) => textureKeys.has(key)),
    remove: vi.fn((key: string) => textureKeys.delete(key)),
  };
  const mapContainer = {
    add: vi.fn(),
    sendToBack: vi.fn(),
  };
  const scene = {
    add: { image: addImage },
    load: loader,
    textures,
  } as unknown as Scene;

  const completeRequest = (key: string) => {
    const request = requests.find(
      (candidate) => candidate.key === key && !candidate.completed,
    );
    if (!request) throw new Error(`No pending image request for ${key}`);
    request.completed = true;
    textureKeys.add(request.key);
    emit(`filecomplete-image-${request.key}`);
    if (requests.every((candidate) => candidate.completed)) {
      isLoading = false;
    }
  };
  const failRequest = (key: string) => {
    const request = requests.find(
      (candidate) => candidate.key === key && !candidate.completed,
    );
    if (!request) throw new Error(`No pending image request for ${key}`);
    request.completed = true;
    emit("loaderror", { key: request.key });
    if (requests.every((candidate) => candidate.completed)) {
      isLoading = false;
    }
  };

  return {
    layer: new OverworldOverviewLayer(
      scene,
      mapContainer as unknown as Phaser.GameObjects.Container,
      chunkSize,
    ),
    addImage,
    images,
    mapContainer,
    requests,
    textures,
    completeRequest,
    failRequest,
  };
}

function chunk(
  column: number,
  row: number,
  key = `${column},${row}`,
): OverviewChunkRef {
  return { key, column, row };
}

describe("OverworldOverviewLayer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    ensureRuntimeTileCatalogCurrentMock.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads signed chunk coordinates through the shared API URL contract", async () => {
    const { layer, requests } = createHarness();

    await layer.sync([chunk(-2, 3)], () => 7);

    expect(getApiUrlMock).toHaveBeenCalledOnce();
    expect(getApiUrlMock).toHaveBeenCalledWith("/overworld/overview");
    expect(requests).toHaveLength(1);
    const requestUrl = new URL(requests[0].url);
    expect(`${requestUrl.origin}${requestUrl.pathname}`).toBe(
      "https://capturequest.test/api/overworld/overview",
    );
    expect(Object.fromEntries(requestUrl.searchParams)).toEqual({
      chunkX: "-2",
      chunkY: "3",
      catalog: RUNTIME_TILE_ASSET_VERSION,
      revision: "7",
    });
    expect(requests[0].xhrSettings?.timeout).toBe(12_000);
  });

  it("places each overview image on its tile-aligned bounds behind actors", async () => {
    const { layer, addImage, images, mapContainer } = createHarness();

    await layer.sync([chunk(-1, 2)], () => 0);

    const image = images[0];
    expect(addImage).toHaveBeenCalledWith(
      -64 * TILE_SIZE,
      128 * TILE_SIZE,
      "overworld-overview--1-2-0",
    );
    expect(image.setOrigin).toHaveBeenCalledWith(0, 0);
    expect(image.setDisplaySize).toHaveBeenCalledWith(
      64 * TILE_SIZE,
      64 * TILE_SIZE,
    );
    expect(image.setDepth).toHaveBeenCalledWith(-10);
    expect(image.setVisible).toHaveBeenCalledWith(false);
    expect(mapContainer.add).toHaveBeenCalledWith(image);
    expect(mapContainer.sendToBack).toHaveBeenCalledWith(image);
  });

  it("returns after the first successful chunk while a bounded queue continues", async () => {
    const { layer, images, requests, completeRequest } = createHarness(
      64,
      false,
    );
    const chunks = Array.from({ length: 6 }, (_, column) => chunk(column, 0));

    layer.setVisible(true);
    const ready = layer.sync(chunks, () => 0);

    expect(requests.map((request) => request.key)).toEqual([
      "overworld-overview-0-0-0",
      "overworld-overview-1-0-0",
      "overworld-overview-2-0-0",
      "overworld-overview-3-0-0",
    ]);

    completeRequest(requests[0].key);
    await ready;

    expect(images).toHaveLength(1);
    expect(images[0].setVisible).toHaveBeenCalledWith(true);
    await vi.waitFor(() => expect(requests).toHaveLength(5));
    expect(requests.filter((request) => !request.completed)).toHaveLength(4);

    while (images.length < chunks.length) {
      await vi.waitFor(() => {
        expect(requests.some((request) => !request.completed)).toBe(true);
      });
      const next = requests.find((request) => !request.completed);
      if (next) completeRequest(next.key);
      await Promise.resolve();
    }

    expect(requests).toHaveLength(chunks.length);
    expect(images).toHaveLength(chunks.length);
  });

  it("keeps the concurrency bound when a newer camera plan replaces the queue", async () => {
    const { layer, requests, completeRequest } = createHarness(64, false);
    const firstReady = layer.sync(
      Array.from({ length: 6 }, (_, column) => chunk(column, 0)),
      () => 0,
    );

    expect(requests).toHaveLength(4);
    const secondReady = layer.sync(
      Array.from({ length: 6 }, (_, offset) => chunk(10 + offset, 0)),
      () => 0,
    );
    await firstReady;

    // The four obsolete in-flight requests retain the global slots until they
    // settle; a rapid camera change cannot create a second four-request pool.
    expect(requests).toHaveLength(4);
    completeRequest(requests[0].key);
    await vi.waitFor(() => expect(requests).toHaveLength(5));
    expect(requests.filter((request) => !request.completed)).toHaveLength(4);

    completeRequest(requests[4].key);
    await secondReady;
    layer.clear();
  });

  it("checks the runtime catalog before surfacing an overview load failure", async () => {
    const { layer, requests, failRequest } = createHarness(64, false);
    const reloadRequested = new Error("runtime catalog reload requested");
    ensureRuntimeTileCatalogCurrentMock.mockRejectedValueOnce(reloadRequested);
    const warning = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);

    const loading = layer.sync([chunk(2, 3)], () => 0);
    failRequest(requests[0].key);

    await expect(loading).rejects.toBe(reloadRequested);
    expect(ensureRuntimeTileCatalogCurrentMock).toHaveBeenCalledOnce();
    expect(ensureRuntimeTileCatalogCurrentMock).toHaveBeenCalledWith(true);
    warning.mockRestore();
  });

  it("rejects initial readiness when an overview image never settles", async () => {
    vi.useFakeTimers();
    const { layer } = createHarness(64, false);
    const warning = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);

    const loading = layer.sync([chunk(2, 3)], () => 0);
    const rejection = expect(loading).rejects.toThrow(
      "Timed out loading overworld overview chunk 2,3",
    );
    await vi.runOnlyPendingTimersAsync();

    await rejection;
    warning.mockRestore();
  });

  it("bounds aggregate readiness instead of draining every stalled chunk", async () => {
    vi.useFakeTimers();
    const { layer, requests } = createHarness(64, false);
    const warning = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);
    const loading = layer.sync(
      Array.from({ length: 10 }, (_, column) => chunk(column, 0)),
      () => 0,
    );
    const rejection = expect(loading).rejects.toThrow(
      "Timed out waiting for an overworld overview chunk",
    );

    await vi.advanceTimersByTimeAsync(16_001);
    await rejection;
    await vi.runOnlyPendingTimersAsync();

    expect(requests.length).toBeLessThan(10);
    warning.mockRestore();
  });

  it("reports a failed background chunk after first-chunk readiness", async () => {
    const { layer, requests, completeRequest, failRequest } = createHarness(
      64,
      false,
    );
    const onBackgroundFailure = vi.fn();
    const warning = vi
      .spyOn(console, "warn")
      .mockImplementation(() => undefined);

    const loading = layer.sync(
      [chunk(0, 0), chunk(1, 0)],
      () => 0,
      onBackgroundFailure,
    );
    completeRequest(requests[0].key);
    await loading;
    failRequest(requests[1].key);
    await vi.waitFor(() => expect(onBackgroundFailure).toHaveBeenCalledOnce());

    expect(onBackgroundFailure.mock.calls[0][0]).toBeInstanceOf(Error);
    warning.mockRestore();
  });

  it("updates visibility and removes chunks omitted by the next sync", async () => {
    const { layer, images, textures } = createHarness();
    const first = chunk(0, 0);
    const second = chunk(1, 0);

    await layer.sync([first, second], () => 0);
    layer.setVisible(true);
    await layer.sync([second], () => 0);

    expect(images[0].setVisible).toHaveBeenLastCalledWith(true);
    expect(images[1].setVisible).toHaveBeenLastCalledWith(true);
    expect(images[0].destroy).toHaveBeenCalledOnce();
    expect(images[1].destroy).not.toHaveBeenCalled();
    expect(textures.remove).toHaveBeenCalledWith("overworld-overview-0-0-0");

    layer.setVisible(false);
    expect(images[1].setVisible).toHaveBeenLastCalledWith(false);
  });

  it("reloads an invalidated chunk at its new revision", async () => {
    const { layer, images, requests, textures } = createHarness();
    const target = chunk(-1, -2);

    await layer.sync([target], () => 4);
    layer.invalidate(target.key);
    await layer.sync([target], () => 5);

    expect(images).toHaveLength(2);
    expect(images[0].destroy).toHaveBeenCalledOnce();
    expect(textures.remove).toHaveBeenCalledWith("overworld-overview--1--2-4");
    expect(requests.map((request) => request.key)).toEqual([
      "overworld-overview--1--2-4",
      "overworld-overview--1--2-5",
    ]);
    expect(new URL(requests[1].url).searchParams.get("revision")).toBe("5");
  });

  it("discards a texture that finishes loading after the layer is cleared", async () => {
    const { layer, images, textures } = createHarness();
    const loading = layer.sync([chunk(2, -3)], () => 0);

    layer.clear();
    await loading;

    expect(images).toHaveLength(0);
    expect(textures.remove).toHaveBeenCalledWith("overworld-overview-2--3-0");
  });
});
