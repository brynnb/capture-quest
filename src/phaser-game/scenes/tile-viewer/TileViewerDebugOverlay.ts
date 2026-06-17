import { Scene } from "phaser";
import { PhaserActor, PhaserMapInfo, PhaserTile, PhaserWarp } from "@/net/generated/world_api";
import { CameraController } from "../../controllers/CameraController";
import { UiManager } from "../../managers";

interface TileViewerDebugOverlayDeps {
  scene: Scene;
  uiManager: () => UiManager;
  cameraController: () => CameraController;
  tileLookup: () => Map<string, PhaserTile>;
  items: () => unknown[];
  mapInfo: () => PhaserMapInfo | null;
  warps: () => PhaserWarp[];
  actors: () => PhaserActor[];
}

export class TileViewerDebugOverlay {
  private pointerMoveHandler: ((pointer: Phaser.Input.Pointer) => void) | null =
    null;

  constructor(private readonly deps: TileViewerDebugOverlayDeps) {}

  register(): void {
    if (this.pointerMoveHandler) return;

    this.pointerMoveHandler = (pointer: Phaser.Input.Pointer) => {
      const mapInfo = this.deps.mapInfo();
      if (!mapInfo) return;

      this.deps.uiManager().updateTileInfo(
        pointer,
        this.deps.tileLookup(),
        this.deps.items(),
        mapInfo,
        (x, y) => this.deps.cameraController().getWorldPoint(x, y),
        this.deps.warps(),
        this.deps.actors(),
      );
    };
    this.deps.scene.input.on("pointermove", this.pointerMoveHandler);
  }

  cleanup(): void {
    if (this.pointerMoveHandler) {
      this.deps.scene.input.off("pointermove", this.pointerMoveHandler);
      this.pointerMoveHandler = null;
    }
    this.removeMapLegend();
  }

  createMapLegend(_maps: PhaserMapInfo[]): void {
    // Placeholder for a richer local-dev map legend.
  }

  removeMapLegend(): void {
    const existingLegend = this.deps.scene.children.getByName("map-legend");
    if (existingLegend) {
      existingLegend.destroy();
    }
  }
}
