import { Scene } from "phaser";
import { IS_LOCAL_DEV } from "@/config";
import useWorldDebugStore from "@/stores/WorldDebugStore";
import { TILE_SIZE } from "../constants";
import type {
  PhaserActor,
  PhaserMapInfo,
  PhaserTile,
  PhaserWarp,
} from "@/net/generated/world_api";

interface DebugItem {
  x: number;
  y: number;
  name?: string;
  description?: string;
}

export class UiManager {
  private scene: Scene;
  private infoText!: Phaser.GameObjects.Text;
  private modeText!: Phaser.GameObjects.Text;
  private loadingText!: Phaser.GameObjects.Text;
  private tileHighlight!: Phaser.GameObjects.Graphics;
  private instantWarpTargetMarker!: Phaser.GameObjects.Container;
  private instantWarpTargetBeacon!: Phaser.GameObjects.Container;
  private instantWarpTargetPulseTween: Phaser.Tweens.Tween | null = null;
  private debugOverlayEnabled = import.meta.env.DEV && IS_LOCAL_DEV;
  private tileHighlightEnabled = true;
  private debugInfoText = "";
  private debugModeText = "Overworld View";

  constructor(scene: Scene) {
    this.scene = scene;

    // Check for existing UI elements and destroy them
    this.cleanupExistingUi();

    this.createUiElements();
    this.createTileHighlight();
    this.createInstantWarpTargetMarker();
  }

  cleanupExistingUi() {
    document.getElementById("capturequest-debug-overlay")?.remove();

    // Check for existing UI elements by name
    const uiElementNames = [
      "infoText",
      "modeText",
      "loadingText",
      "tileHighlight",
      "instantWarpTargetMarker",
    ];

    for (const name of uiElementNames) {
      for (const existingElement of this.scene.children.getChildren()) {
        if (existingElement.name !== name) continue;
        existingElement.destroy();
      }
    }
  }

  createUiElements() {
    // Add info text for displaying tile information
    this.infoText = this.scene.add.text(10, 10, "", {
      fontFamily: "'Pokemon GB', monospace, Arial",
      fontSize: "12px",
      color: "#ffffff",
      backgroundColor: "#000000",
      padding: { x: 5, y: 5 },
    });
    this.infoText.setDepth(1000); // Ensure it's always on top
    this.infoText.setScrollFactor(0);
    this.infoText.name = "infoText";
    this.infoText.setVisible(false);

    // Add view mode indicator
    this.modeText = this.scene.add.text(10, 30, "Overworld View", {
      fontFamily: "'Pokemon GB', monospace, Arial",
      fontSize: "12px",
      color: "#ffffff",
      backgroundColor: "#000000",
      padding: { x: 5, y: 5 },
    });
    this.modeText.setDepth(1000); // Ensure it's always on top
    this.modeText.setScrollFactor(0);
    this.modeText.name = "modeText";
    this.modeText.setVisible(false);

    // Add loading text
    this.loadingText = this.scene.add.text(10, 50, "Loading map data...", {
      fontFamily: "'Pokemon GB', monospace, Arial",
      fontSize: "12px",
      color: "#ffffff",
      backgroundColor: "#000000",
      padding: { x: 5, y: 5 },
    });
    this.loadingText.setScrollFactor(0);
    this.loadingText.setDepth(1000); // Ensure it's always on top
    this.loadingText.name = "loadingText";

    // Position elements correctly
    this.renderDebugOverlay();
    this.updateElementPositions();
  }

  createTileHighlight() {
    this.tileHighlight = this.scene.add.graphics();
    this.tileHighlight.setDepth(500); // Set depth to be above tiles but below UI
    this.tileHighlight.name = "tileHighlight";
    this.tileHighlight.setVisible(this.tileHighlightEnabled);
  }

  private createInstantWarpTargetMarker() {
    const tile = this.scene.add.graphics();
    tile.fillStyle(0xff1838, 0.34);
    tile.fillRect(-TILE_SIZE / 2, -TILE_SIZE / 2, TILE_SIZE, TILE_SIZE);
    tile.lineStyle(2, 0xffffff, 1);
    tile.strokeRect(-TILE_SIZE / 2, -TILE_SIZE / 2, TILE_SIZE, TILE_SIZE);
    tile.lineStyle(1, 0xff1838, 1);
    tile.strokeRect(
      -TILE_SIZE / 2 - 2,
      -TILE_SIZE / 2 - 2,
      TILE_SIZE + 4,
      TILE_SIZE + 4,
    );

    const pulse = this.scene.add.graphics();
    pulse.fillStyle(0xff1838, 0.2);
    pulse.fillCircle(0, 0, 19);
    pulse.lineStyle(3, 0xff1838, 1);
    pulse.strokeCircle(0, 0, 16);
    pulse.lineStyle(2, 0xffffff, 1);
    pulse.strokeCircle(0, 0, 11);
    pulse.beginPath();
    pulse.moveTo(-23, 0);
    pulse.lineTo(-8, 0);
    pulse.moveTo(8, 0);
    pulse.lineTo(23, 0);
    pulse.moveTo(0, -23);
    pulse.lineTo(0, -8);
    pulse.moveTo(0, 8);
    pulse.lineTo(0, 23);
    pulse.strokePath();

    this.instantWarpTargetBeacon = this.scene.add.container(0, 0, [pulse]);
    this.instantWarpTargetMarker = this.scene.add.container(0, 0, [
      tile,
      this.instantWarpTargetBeacon,
    ]);
    this.instantWarpTargetMarker.name = "instantWarpTargetMarker";
    this.instantWarpTargetMarker.setDepth(850);
    this.instantWarpTargetMarker.setVisible(false);

    this.instantWarpTargetPulseTween = this.scene.tweens.add({
      targets: pulse,
      scale: { from: 0.82, to: 1.22 },
      alpha: { from: 1, to: 0.42 },
      duration: 620,
      ease: "Sine.InOut",
      yoyo: true,
      repeat: -1,
    });
  }

  showInstantWarpTarget(tileX: number, tileY: number, cameraZoom: number) {
    this.instantWarpTargetMarker.setPosition(
      tileX * TILE_SIZE + TILE_SIZE / 2,
      tileY * TILE_SIZE + TILE_SIZE / 2,
    );
    this.updateInstantWarpTargetZoom(cameraZoom);
    this.instantWarpTargetMarker.setVisible(true);
  }

  clearInstantWarpTarget() {
    this.instantWarpTargetMarker?.setVisible(false);
  }

  isInstantWarpTargetVisible() {
    return this.instantWarpTargetMarker?.visible ?? false;
  }

  updateInstantWarpTargetZoom(cameraZoom: number) {
    if (!this.instantWarpTargetBeacon?.active) return;
    this.instantWarpTargetBeacon.setScale(1 / Math.max(cameraZoom, 0.01));
  }

  updateElementPositions() {
    if (!this.debugOverlayEnabled) {
      this.loadingText.setPosition(10, 10);
      return;
    }

    this.loadingText.setPosition(10, 10);
    this.renderDebugOverlay();
  }

  updateTileInfo(
    pointer: Phaser.Input.Pointer,
    tileLookup: Map<string, PhaserTile>,
    items: DebugItem[],
    mapInfo: PhaserMapInfo | null,
    getWorldPoint: (x: number, y: number) => Phaser.Math.Vector2,
    warps: PhaserWarp[] = [],
    npcs: PhaserActor[] = [],
  ) {
    // Convert screen coordinates to world coordinates
    const worldPoint = getWorldPoint(pointer.x, pointer.y);

    // Convert world coordinates to tile coordinates
    const tileX = Math.floor(worldPoint.x / TILE_SIZE);
    const tileY = Math.floor(worldPoint.y / TILE_SIZE);

    // Update the tile highlight position
    this.updateTileHighlight(tileX, tileY);

    if (!this.debugOverlayEnabled) {
      return;
    }

    // Check if we have map info
    if (!mapInfo) {
      this.debugInfoText = "No map info available";
      this.renderDebugOverlay();
      return;
    }

    // Build info text
    let info = `Tile: (${tileX}, ${tileY})`;

    // O(1) tile lookup by coordinate key
    const tile = tileLookup.get(`${tileX},${tileY}`);

    // Always show local coordinates, displaying "none" when not available
    if (tile && tile.localX !== undefined && tile.localY !== undefined) {
      info += `\nLocal Coords: (${tile.localX}, ${tile.localY})`;
    } else {
      info += `\nLocal Coords: none`;
    }

    // Always show Map ID, displaying "none" when not available
    if (tile && tile.mapId) {
      info += `\nMap ID: ${tile.mapId}`;

      // Use the source map name directly from the tile object.
      if (tile.sourceMapName) {
        info += ` (${tile.sourceMapName})`;
      } else {
        info += ` (no name)`;
      }
    } else {
      info += `\nMap ID: none`;
    }

    if (mapInfo.tilesetId) {
      info += `\nTileset ID: ${mapInfo.tilesetId}`;
    }

    // Always display Tile ID, showing "n/a" when no tile is found
    info += `\nTile ID: ${tile ? tile.tileImageId : "none"}`;

    // Find item at this position
    const item = items.find((i) => i.x === tileX && i.y === tileY);
    if (item) {
      info += `\nItem: ${item.name}`;
      if (item.description) {
        info += `\nDescription: ${item.description}`;
      }
    }

    // Find warp at this position
    const warp = warps.find((w) => w.x === tileX && w.y === tileY);
    if (warp) {
      info += `\nWarp: (${warp.x}, ${warp.y})`;
      info += `\nDestination: Map ${warp.destinationMap} at (${warp.destinationX}, ${warp.destinationY})`;
    }

    // Find NPC at this position
    const npc = npcs.find((n) => n.x === tileX && n.y === tileY);
    if (npc) {
      info += `\nNPC: ${npc.name || "Unnamed NPC"}`;
      if (npc.spriteName) {
        info += `\nSprite: ${npc.spriteName}`;
      }
    }

    this.debugInfoText = info;
    this.renderDebugOverlay();

    // Update the mode text with the current view name
    this.setModeText(`View: ${mapInfo.name}`);
  }

  updateTileHighlight(tileX: number, tileY: number) {
    if (!this.tileHighlightEnabled) {
      return;
    }

    // Clear previous highlight
    this.tileHighlight.clear();

    const darkGrey = 0x444444;
    const x = tileX * TILE_SIZE;
    const y = tileY * TILE_SIZE;
    const size = TILE_SIZE;
    const bracketSize = 4; // Size of the corner brackets

    // Set line style
    this.tileHighlight.lineStyle(1, darkGrey);

    // Draw top-left corner bracket
    this.tileHighlight.beginPath();
    this.tileHighlight.moveTo(x, y + bracketSize);
    this.tileHighlight.lineTo(x, y);
    this.tileHighlight.lineTo(x + bracketSize, y);
    this.tileHighlight.strokePath();

    // Draw top-right corner bracket
    this.tileHighlight.beginPath();
    this.tileHighlight.moveTo(x + size - bracketSize, y);
    this.tileHighlight.lineTo(x + size, y);
    this.tileHighlight.lineTo(x + size, y + bracketSize);
    this.tileHighlight.strokePath();

    // Draw bottom-right corner bracket
    this.tileHighlight.beginPath();
    this.tileHighlight.moveTo(x + size, y + size - bracketSize);
    this.tileHighlight.lineTo(x + size, y + size);
    this.tileHighlight.lineTo(x + size - bracketSize, y + size);
    this.tileHighlight.strokePath();

    // Draw bottom-left corner bracket
    this.tileHighlight.beginPath();
    this.tileHighlight.moveTo(x + bracketSize, y + size);
    this.tileHighlight.lineTo(x, y + size);
    this.tileHighlight.lineTo(x, y + size - bracketSize);
    this.tileHighlight.strokePath();
  }

  setLoadingText(text: string) {
    if (!this.setTextSafely(this.loadingText, text)) {
      return;
    }
    this.setVisibleSafely(this.loadingText, true);
    this.updateElementPositions();
  }

  hideLoadingText() {
    this.setVisibleSafely(this.loadingText, false);
  }

  setModeText(text: string) {
    if (!this.debugOverlayEnabled) {
      return;
    }
    this.debugModeText = text;
    this.renderDebugOverlay();
  }

  private renderDebugOverlay() {
    if (!this.debugOverlayEnabled) {
      return;
    }
    const debugStore = useWorldDebugStore.getState();
    debugStore.setTileInfo(
      this.debugInfoText || "Move over the map to inspect a tile.",
    );
    debugStore.setViewMode(this.debugModeText);
  }

  getUiElements() {
    return [
      this.infoText,
      this.modeText,
      this.loadingText,
    ];
  }

  getWorldOverlayElements() {
    return [this.tileHighlight, this.instantWarpTargetMarker];
  }

  handleResize() {
    if (!this.debugOverlayEnabled) {
      this.loadingText.setPosition(10, 10);
      return;
    }

    this.updateElementPositions();
  }

  destroy() {
    useWorldDebugStore.getState().reset();
    this.instantWarpTargetPulseTween?.stop();
    this.instantWarpTargetPulseTween = null;
  }

  // Add this method to refresh text elements after fonts are loaded
  refreshTextElements() {
    // Force a redraw of text elements by setting their text again
    if (this.infoText) {
      const currentText = this.infoText.text;
      this.setTextSafely(this.infoText, currentText);
    }

    if (this.modeText) {
      const currentText = this.modeText.text;
      this.setTextSafely(this.modeText, currentText);
    }

    if (this.loadingText) {
      const currentText = this.loadingText.text;
      this.setTextSafely(this.loadingText, currentText);
    }

    this.renderDebugOverlay();
  }

  private setTextSafely(
    textObject: Phaser.GameObjects.Text | undefined,
    text: string,
  ) {
    if (!textObject?.active) {
      return false;
    }

    try {
      textObject.setText(text);
      return true;
    } catch {
      return false;
    }
  }

  private setVisibleSafely(
    gameObject:
      | (Phaser.GameObjects.GameObject & { setVisible: (visible: boolean) => unknown })
      | undefined,
    visible: boolean,
  ) {
    if (!gameObject?.active) {
      return;
    }

    try {
      gameObject.setVisible(visible);
    } catch {
      // Stale Phaser objects can briefly survive scene teardown.
    }
  }
}
