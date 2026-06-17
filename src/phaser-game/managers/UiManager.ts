import { Scene } from "phaser";
import { IS_LOCAL_DEV } from "@/config";
import { TILE_SIZE } from "../constants";

export class UiManager {
  private scene: Scene;
  private infoText!: Phaser.GameObjects.Text;
  private modeText!: Phaser.GameObjects.Text;
  private loadingText!: Phaser.GameObjects.Text;
  private tileHighlight!: Phaser.GameObjects.Graphics;
  private debugOverlayEnabled = IS_LOCAL_DEV;
  private tileHighlightEnabled = true;
  private debugOverlayElement: HTMLDivElement | null = null;
  private debugInfoText = "";
  private debugModeText = "Overworld View";
  private debugOverlayResizeHandler = () => this.positionDebugOverlay();

  constructor(scene: Scene) {
    this.scene = scene;

    // Check for existing UI elements and destroy them
    this.cleanupExistingUi();

    this.createUiElements();
    this.createTileHighlight();
  }

  cleanupExistingUi() {
    document.getElementById("capturequest-debug-overlay")?.remove();

    // Check for existing UI elements by name
    const uiElementNames = [
      "infoText",
      "modeText",
      "loadingText",
      "tileHighlight",
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
      fontFamily: "'Pokemon Pixel Font', monospace, Arial",
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
      fontFamily: "'Pokemon Pixel Font', monospace, Arial",
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
      fontFamily: "'Pokemon Pixel Font', monospace, Arial",
      fontSize: "12px",
      color: "#ffffff",
      backgroundColor: "#000000",
      padding: { x: 5, y: 5 },
    });
    this.loadingText.setScrollFactor(0);
    this.loadingText.setDepth(1000); // Ensure it's always on top
    this.loadingText.name = "loadingText";

    // Position elements correctly
    this.createDebugOverlay();
    this.updateElementPositions();
  }

  private createDebugOverlay() {
    if (!this.debugOverlayEnabled) {
      return;
    }

    const overlay = document.createElement("div");
    overlay.id = "capturequest-debug-overlay";
    overlay.style.position = "fixed";
    overlay.style.zIndex = "40";
    overlay.style.boxSizing = "border-box";
    overlay.style.width = "260px";
    overlay.style.maxWidth = "calc(100vw - 16px)";
    overlay.style.maxHeight = "calc(100vh - 16px)";
    overlay.style.overflow = "auto";
    overlay.style.pointerEvents = "none";
    overlay.style.whiteSpace = "pre-line";
    overlay.style.fontFamily = "'Pokemon Pixel Font', monospace, Arial";
    overlay.style.fontSize = "12px";
    overlay.style.lineHeight = "1.35";
    overlay.style.color = "#ffffff";
    overlay.style.background = "rgba(0, 0, 0, 0.82)";
    overlay.style.border = "1px solid rgba(255, 255, 255, 0.22)";
    overlay.style.padding = "8px";
    overlay.style.boxShadow = "0 3px 12px rgba(0, 0, 0, 0.25)";
    document.body.appendChild(overlay);

    this.debugOverlayElement = overlay;
    window.addEventListener("resize", this.debugOverlayResizeHandler);
    this.renderDebugOverlay();
  }

  createTileHighlight() {
    this.tileHighlight = this.scene.add.graphics();
    this.tileHighlight.setDepth(500); // Set depth to be above tiles but below UI
    this.tileHighlight.name = "tileHighlight";
    this.tileHighlight.setVisible(this.tileHighlightEnabled);
  }

  updateElementPositions() {
    if (!this.debugOverlayEnabled) {
      this.loadingText.setPosition(10, 10);
      return;
    }

    this.loadingText.setPosition(10, 10);
    this.positionDebugOverlay();
  }

  updateTileInfo(
    pointer: Phaser.Input.Pointer,
    tileLookup: Map<string, any>,
    items: any[],
    mapInfo: any,
    getWorldPoint: (x: number, y: number) => Phaser.Math.Vector2,
    warps: any[] = [],
    npcs: any[] = [],
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

      // Use mapName directly from the tile object
      if (tile.mapName) {
        info += ` (${tile.mapName})`;
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
    this.loadingText.setText(text);
    this.loadingText.setVisible(true);
    this.updateElementPositions();
  }

  hideLoadingText() {
    this.loadingText.setVisible(false);
  }

  setModeText(text: string) {
    if (!this.debugOverlayEnabled) {
      return;
    }
    this.debugModeText = text;
    this.renderDebugOverlay();
  }

  private renderDebugOverlay() {
    if (!this.debugOverlayElement) {
      return;
    }

    const lines = [this.debugInfoText, this.debugModeText].filter(Boolean);
    this.debugOverlayElement.textContent = lines.join("\n\n");
    this.positionDebugOverlay();
  }

  private positionDebugOverlay() {
    if (!this.debugOverlayElement) {
      return;
    }

    const canvas = this.scene.game.canvas;
    const rect = canvas.getBoundingClientRect();
    const margin = 12;
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const overlayWidth = this.debugOverlayElement.offsetWidth || 260;
    const overlayHeight = this.debugOverlayElement.offsetHeight || 120;

    let left: number;
    let top = Math.max(8, Math.min(rect.top, viewportHeight - overlayHeight - 8));

    if (rect.left >= overlayWidth + margin) {
      left = rect.left - overlayWidth - margin;
    } else if (viewportWidth - rect.right >= overlayWidth + margin) {
      left = rect.right + margin;
    } else {
      left = Math.max(8, Math.min(rect.left, viewportWidth - overlayWidth - 8));
      const aboveTop = rect.top - overlayHeight - margin;
      const belowTop = rect.bottom + margin;
      if (aboveTop >= 8) {
        top = aboveTop;
      } else if (belowTop + overlayHeight <= viewportHeight - 8) {
        top = belowTop;
      }
    }

    this.debugOverlayElement.style.left = `${Math.round(left)}px`;
    this.debugOverlayElement.style.top = `${Math.round(top)}px`;
  }

  getUiElements() {
    return [
      this.infoText,
      this.modeText,
      this.loadingText,
    ];
  }

  getWorldOverlayElements() {
    return [this.tileHighlight];
  }

  handleResize() {
    if (!this.debugOverlayEnabled) {
      this.loadingText.setPosition(10, 10);
      return;
    }

    this.updateElementPositions();

  }

  destroy() {
    window.removeEventListener("resize", this.debugOverlayResizeHandler);
    this.debugOverlayElement?.remove();
    this.debugOverlayElement = null;
  }

  // Add this method to refresh text elements after fonts are loaded
  refreshTextElements() {
    // Force a redraw of text elements by setting their text again
    if (this.infoText) {
      const currentText = this.infoText.text;
      this.infoText.setText(currentText);
    }

    if (this.modeText) {
      const currentText = this.modeText.text;
      this.modeText.setText(currentText);
    }

    if (this.loadingText) {
      const currentText = this.loadingText.text;
      this.loadingText.setText(currentText);
    }

    this.positionDebugOverlay();
  }
}
