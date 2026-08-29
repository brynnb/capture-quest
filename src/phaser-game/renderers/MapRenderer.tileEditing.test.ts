import { describe, expect, it, vi } from "vitest";

import { MapRenderer } from "./MapRenderer";

function createRendererHarness() {
  const drawFrame = vi.fn();
  const fill = vi.fn();
  const harness = Object.create(MapRenderer.prototype) as Record<string, unknown>;
  const tileDataMap = new Map<string, number>();

  harness.scene = { textures: { exists: () => true } };
  harness.tileDataMap = tileDataMap;
  harness.tileRenderTexture = {
    width: 160,
    height: 160,
    drawFrame,
    fill,
  };
  harness.mapOriginX = 0;
  harness.mapOriginY = 0;
  harness.tileChunkRenderTextures = new Map();
  harness.tileChunkKeyByGridPosition = new Map();
  harness.userTileSprites = new Map();

  return {
    renderer: harness as unknown as MapRenderer,
    harness,
    tileDataMap,
    drawFrame,
    fill,
  };
}

describe("MapRenderer incremental tile editing", () => {
  it("stamps a replacement frame directly without partially filling the cell", () => {
    const { renderer, drawFrame, fill } = createRendererHarness();

    expect(renderer.addTile(3, 4, 109)).toBe(true);

    expect(drawFrame).toHaveBeenCalledWith("tile-109", undefined, 48, 64);
    expect(fill).not.toHaveBeenCalled();
    expect(renderer.getTileImageIdAt(3, 4)).toBe(109);
  });

  it("erases the complete cell with an opaque frame", () => {
    const { renderer, tileDataMap, drawFrame, fill } = createRendererHarness();
    tileDataMap.set("3,4", 109);

    expect(renderer.removeTile(3, 4)).toBe(true);

    expect(drawFrame).toHaveBeenCalledWith(
      "tile-editor-empty",
      undefined,
      48,
      64,
    );
    expect(fill).not.toHaveBeenCalled();
    expect(renderer.getTileImageIdAt(3, 4)).toBe(0);
  });

  it("coalesces concurrent loads for every cell in a large brush", async () => {
    const { renderer, harness } = createRendererHarness();
    const listeners = new Map<string, (...args: unknown[]) => void>();
    const image = vi.fn();
    const start = vi.fn();

    harness.scene = {
      textures: { exists: () => false },
      load: {
        once: vi.fn((event: string, callback: (...args: unknown[]) => void) => {
          listeners.set(event, callback);
        }),
        image,
        isLoading: () => false,
        start,
      },
    };
    harness.tileTextureLoadPromises = new Map<number, Promise<boolean>>();

    const first = renderer.loadTileTextureIfNeeded(400);
    const second = renderer.loadTileTextureIfNeeded(400);

    expect(second).toBe(first);
    await vi.waitFor(() => expect(image).toHaveBeenCalledTimes(1));
    expect(start).toHaveBeenCalledTimes(1);

    listeners.get("filecomplete-image-tile-400")?.();
    await expect(first).resolves.toBe(true);
  });
});
