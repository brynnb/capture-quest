import { describe, expect, it, vi } from "vitest";

import { MAP_TILE_CHUNK_SIZE, MapRenderer } from "./MapRenderer";

type ChunkTiles = Parameters<MapRenderer["upsertTileChunk"]>[3];

function tile(x: number, y: number, tileImageId: number): ChunkTiles[number] {
  return { x, y, tileImageId } as ChunkTiles[number];
}

function createRendererHarness() {
  const renderTextures: Array<Record<string, unknown>> = [];
  const renderTexture = vi.fn(
    (x: number, y: number, width: number, height: number) => {
      const texture = {
        x,
        y,
        width,
        height,
        setOrigin: vi.fn(),
        beginDraw: vi.fn(),
        batchDrawFrame: vi.fn(),
        endDraw: vi.fn(),
        drawFrame: vi.fn(),
        destroy: vi.fn(),
      };
      renderTextures.push(texture);
      return texture;
    },
  );
  const addImage = vi.fn();
  const containerList: Array<{ name?: string }> = [];
  const mapContainer = {
    list: containerList,
    add: vi.fn((child: { name?: string }) => containerList.push(child)),
    addAt: vi.fn((child: { name?: string }, index: number) =>
      containerList.splice(index, 0, child),
    ),
    sendToBack: vi.fn((child: { name?: string }) => {
      const index = containerList.indexOf(child);
      if (index >= 0) containerList.splice(index, 1);
      containerList.unshift(child);
    }),
    bringToTop: vi.fn(),
    each: vi.fn(),
  };
  const harness = Object.create(MapRenderer.prototype) as Record<
    string,
    unknown
  >;
  const tileDataMap = new Map<string, number>();

  harness.scene = {
    textures: { exists: () => true },
    add: { renderTexture, image: addImage },
    game: { renderer: {} },
  };
  harness.mapContainer = mapContainer;
  harness.tileRenderTexture = null;
  harness.tileDataMap = tileDataMap;
  harness.mapOriginX = 0;
  harness.mapOriginY = 0;
  harness.tileChunkRenderTextures = new Map();
  harness.tileChunkKeyByGridPosition = new Map();
  harness.tileChunkOwnerByCoordinate = new Map();
  harness.userTileSprites = new Map();
  harness.actorZones = new Map();
  harness.actorSprites = new Map();
  harness.nameLabels = new Map();
  harness.chatBubbles = new Map();
  harness.warnedMissingActors = new Set();

  return {
    renderer: harness as unknown as MapRenderer,
    tileDataMap,
    renderTexture,
    renderTextures,
    addImage,
    mapContainer,
    containerList,
  };
}

describe("MapRenderer exact tile chunks", () => {
  it("renders an aligned 64x64 chunk into one bounded RenderTexture", () => {
    const { renderer, renderTexture, renderTextures, addImage } =
      createRendererHarness();

    const result = renderer.upsertTileChunk("2,-1", 2, -1, [
      tile(128, -64, 100),
      tile(191, -1, 101),
    ]);

    expect(MAP_TILE_CHUNK_SIZE).toBe(64);
    expect(result).toEqual({ rendered: 2, skipped: 0 });
    expect(renderTexture).toHaveBeenCalledWith(2048, -1024, 1040, 1040);
    expect(renderTextures[0].batchDrawFrame).toHaveBeenNthCalledWith(
      1,
      "tile-100",
      undefined,
      0,
      0,
    );
    expect(renderTextures[0].batchDrawFrame).toHaveBeenNthCalledWith(
      2,
      "tile-101",
      undefined,
      1008,
      1008,
    );
    expect(addImage).not.toHaveBeenCalled();
    expect(renderer.getTileImageIdAt(128, -64)).toBe(100);
  });

  it("renders a neighboring tile halo without claiming its data coordinate", () => {
    const { renderer, renderTextures, tileDataMap } = createRendererHarness();
    renderer.upsertTileChunk(
      "0,0",
      0,
      0,
      [tile(63, 0, 100)],
      [tile(63, 0, 100), tile(64, 0, 101)],
    );

    expect(renderTextures[0].batchDrawFrame).toHaveBeenLastCalledWith(
      "tile-101",
      undefined,
      1024,
      0,
    );
    expect(renderer.getTileImageIdAt(63, 0)).toBe(100);
    expect(tileDataMap.has("64,0")).toBe(false);
  });

  it("updates every loaded halo copy of a boundary tile", () => {
    const { renderer, renderTextures } = createRendererHarness();
    renderer.upsertTileChunk(
      "0,-1",
      0,
      -1,
      [tile(0, -1, 100)],
      [tile(0, -1, 100), tile(0, 0, 101)],
    );
    renderer.upsertTileChunk("0,0", 0, 0, [tile(0, 0, 101)]);

    expect(renderer.updateTile(0, 0, 202)).toBe(true);
    expect(renderTextures[0].drawFrame).toHaveBeenCalledWith(
      "tile-202",
      undefined,
      0,
      1024,
    );
    expect(renderTextures[1].drawFrame).toHaveBeenCalledWith(
      "tile-202",
      undefined,
      0,
      0,
    );
  });

  it("places exact chunks above overview imagery and below world objects", () => {
    const { renderer, renderTextures, containerList } = createRendererHarness();
    const overview = { name: "overworld-overview:0,0" };
    const actor = { name: "player" };
    containerList.push(overview, actor);

    renderer.upsertTileChunk("0,0", 0, 0, [tile(0, 0, 100)]);

    expect(containerList).toEqual([overview, renderTextures[0], actor]);
  });

  it("keeps editor paint, update, and erase operations on the owning chunk", () => {
    const { renderer, renderTextures, addImage } = createRendererHarness();
    renderer.upsertTileChunk("0,0", 0, 0, [tile(0, 0, 100)]);
    const chunkTexture = renderTextures[0];

    expect(renderer.addTile(1, 1, 200)).toBe(true);
    expect(chunkTexture.drawFrame).toHaveBeenCalledWith(
      "tile-200",
      undefined,
      16,
      16,
    );
    expect(renderer.updateTile(1, 1, 201)).toBe(true);
    expect(chunkTexture.drawFrame).toHaveBeenCalledWith(
      "tile-201",
      undefined,
      16,
      16,
    );
    expect(renderer.removeTile(1, 1)).toBe(true);
    expect(chunkTexture.drawFrame).toHaveBeenCalledWith(
      "tile-editor-empty",
      undefined,
      16,
      16,
    );
    expect(addImage).not.toHaveBeenCalled();
    expect(renderer.getTileImageIdAt(1, 1)).toBe(0);
  });

  it("replaces and unloads chunks without leaving stale tile lookup data", () => {
    const { renderer, renderTextures, tileDataMap } = createRendererHarness();
    renderer.upsertTileChunk("current", 0, 0, [
      tile(0, 0, 100),
      tile(1, 0, 101),
    ]);
    renderer.upsertTileChunk("current", 0, 0, [tile(0, 0, 200)]);

    expect(renderTextures[0].destroy).toHaveBeenCalledOnce();
    expect(tileDataMap.get("0,0")).toBe(200);
    expect(tileDataMap.has("1,0")).toBe(false);

    expect(renderer.unloadTileChunk("current")).toBe(true);
    expect(renderTextures[1].destroy).toHaveBeenCalledOnce();
    expect(tileDataMap.size).toBe(0);
    expect(renderer.unloadTileChunk("current")).toBe(false);
  });

  it("rejects tiles outside their declared chunk before allocating GPU state", () => {
    const { renderer, renderTexture } = createRendererHarness();

    expect(() =>
      renderer.upsertTileChunk("0,0", 0, 0, [tile(64, 0, 100)]),
    ).toThrow(/outside chunk/);
    expect(renderTexture).not.toHaveBeenCalled();
  });

  it("destroys all chunk GPU state during a full map clear", () => {
    const { renderer, renderTextures, tileDataMap } = createRendererHarness();
    renderer.upsertTileChunk("0,0", 0, 0, [tile(0, 0, 100)]);
    renderer.upsertTileChunk("1,0", 1, 0, [tile(64, 0, 101)]);

    renderer.clear();

    expect(renderTextures[0].destroy).toHaveBeenCalledOnce();
    expect(renderTextures[1].destroy).toHaveBeenCalledOnce();
    expect(tileDataMap.size).toBe(0);
  });

  it("preserves explicit world bounds when the base overworld has no exact tiles", () => {
    const { renderer, renderTexture } = createRendererHarness();

    const bounds = renderer.renderMap([], [], [], [], {
      minX: -50,
      minY: -234,
      maxX: 289,
      maxY: 125,
    });

    expect(renderTexture).not.toHaveBeenCalled();
    expect(bounds).toEqual({
      minX: -50,
      minY: -234,
      maxX: 289,
      maxY: 125,
      width: 340 * 16,
      height: 360 * 16,
      centerX: 120 * 16,
      centerY: -54 * 16,
    });
  });
});
