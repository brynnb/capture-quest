import { Scene } from "phaser";
import { PhaserMapInfo, PhaserTile } from "@/net/generated/world_api";
import { UNIFIED_OVERWORLD_MAP_ID } from "../../constants";
import { MapRenderer } from "../../renderers/MapRenderer";
import {
  PROCEDURAL_OVERWORLD_GENERATE_EVENT,
  PROCEDURAL_OVERWORLD_GENERATED_EVENT,
  generateProceduralOverworldTiles,
  tileKey,
  type ProceduralOverworldGenerateDetail,
  type ProceduralOverworldGeneratedDetail,
} from "../../procedural/overworldProceduralTiles";

interface ProceduralOverworldPreviewDeps {
  scene: Scene;
  mapRenderer: () => MapRenderer | null;
  tiles: () => PhaserTile[];
  mapInfo: () => PhaserMapInfo | null;
  isOverworldMode: () => boolean;
  tileLookup: () => Map<string, PhaserTile>;
}

export class ProceduralOverworldPreview {
  private readonly generatedTileKeys = new Set<string>();
  private generateHandler: ((e: Event) => void) | null = null;

  constructor(private readonly deps: ProceduralOverworldPreviewDeps) {}

  register(): void {
    if (this.generateHandler) return;

    this.generateHandler = (event: Event) => {
      const detail = (event as CustomEvent<ProceduralOverworldGenerateDetail>)
        .detail;
      void this.generate(detail);
    };
    window.addEventListener(
      PROCEDURAL_OVERWORLD_GENERATE_EVENT,
      this.generateHandler,
    );
  }

  unregister(): void {
    if (!this.generateHandler) return;

    window.removeEventListener(
      PROCEDURAL_OVERWORLD_GENERATE_EVENT,
      this.generateHandler,
    );
    this.generateHandler = null;
  }

  async generate(detail?: ProceduralOverworldGenerateDetail): Promise<void> {
    const renderer = this.deps.mapRenderer();
    const sourceTiles = this.deps.tiles();
    const mapInfo = this.deps.mapInfo();
    const seed = detail?.seed ?? Math.floor(Date.now() % 0x7fffffff);

    if (!renderer || sourceTiles.length === 0) {
      this.dispatchGenerated({
        seed,
        sourceTileCount: sourceTiles.length,
        generatedTileCount: 0,
        renderedTileCount: 0,
        skippedTileCount: 0,
        rockTileCount: 0,
        grassPatchTileCount: 0,
        waterTileCount: 0,
        error: "No loaded map tiles are available yet.",
      });
      return;
    }

    if (
      !this.deps.isOverworldMode() &&
      mapInfo?.id !== UNIFIED_OVERWORLD_MAP_ID &&
      mapInfo?.isOverworld !== 1
    ) {
      this.dispatchGenerated({
        seed,
        sourceTileCount: sourceTiles.length,
        generatedTileCount: 0,
        renderedTileCount: 0,
        skippedTileCount: 0,
        rockTileCount: 0,
        grassPatchTileCount: 0,
        waterTileCount: 0,
        error: "Load the overworld before generating preview terrain.",
      });
      return;
    }

    this.clear();

    const { tiles, summary } = generateProceduralOverworldTiles(sourceTiles, {
      mapId: mapInfo?.id ?? UNIFIED_OVERWORLD_MAP_ID,
      seed,
    });

    const uniqueTileImageIds = Array.from(
      new Set(tiles.map((tile) => tile.tileImageId)),
    );
    for (const tileImageId of uniqueTileImageIds) {
      await renderer.loadTileTextureIfNeeded(tileImageId);
    }

    const renderResult = renderer.addTiles(tiles);
    const lookup = this.deps.tileLookup();
    for (const tile of tiles) {
      const key = tileKey(tile.x, tile.y);
      this.generatedTileKeys.add(key);
      lookup.set(key, tile);
    }

    this.dispatchGenerated({
      ...summary,
      renderedTileCount: renderResult.rendered,
      skippedTileCount: renderResult.skipped,
    });
  }

  clear(): void {
    if (this.generatedTileKeys.size === 0) return;

    const renderer = this.deps.mapRenderer();
    const lookup = this.deps.tileLookup();
    for (const key of this.generatedTileKeys) {
      const [xRaw, yRaw] = key.split(",");
      const x = Number.parseInt(xRaw, 10);
      const y = Number.parseInt(yRaw, 10);
      if (Number.isNaN(x) || Number.isNaN(y)) continue;

      renderer?.removeTile(x, y);
      lookup.delete(key);
    }
    this.generatedTileKeys.clear();
  }

  private dispatchGenerated(detail: ProceduralOverworldGeneratedDetail): void {
    window.dispatchEvent(
      new CustomEvent(PROCEDURAL_OVERWORLD_GENERATED_EVENT, { detail }),
    );
  }
}
