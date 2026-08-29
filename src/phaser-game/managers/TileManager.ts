import { Scene } from "phaser";
import { TileImageCacheEntry, getTileImageUrl, getSpriteUrl } from "../api";
import { TILE_SIZE } from "../constants";

interface TileImageRecord {
  id: number;
}

const TILE_IMAGE_XHR_TIMEOUT_MS = 12_000;
const TILE_IMAGE_LOAD_WATCHDOG_MS = 15_000;

export class TileManager {
  private scene: Scene;
  private tileImageCache: Map<number, TileImageCacheEntry> = new Map();
  private loadingTextures: Set<string> = new Set();
  private tileImageLoadQueue: Promise<void> = Promise.resolve();

  constructor(scene: Scene) {
    this.scene = scene;
    this.createFallbackTextures();
  }

  createFallbackTextures() {
    // Create a fallback texture for missing tiles
    if (!this.scene.textures.exists("missing-tile")) {
      const graphics = this.scene.make.graphics({ x: 0, y: 0 });
      graphics.fillStyle(0xff00ff); // Magenta color for missing tiles
      graphics.fillRect(0, 0, TILE_SIZE, TILE_SIZE);
      graphics.lineStyle(1, 0x000000);
      graphics.strokeRect(0, 0, TILE_SIZE, TILE_SIZE);
      graphics.generateTexture("missing-tile", TILE_SIZE, TILE_SIZE);
      graphics.destroy();
    }

    // Create a fallback item marker (red circle) if the poke_ball image fails to load
    if (!this.scene.textures.exists("item-marker-fallback")) {
      const itemGraphics = this.scene.make.graphics({ x: 0, y: 0 });
      itemGraphics.fillStyle(0xff0000);
      itemGraphics.fillCircle(TILE_SIZE / 4, TILE_SIZE / 4, TILE_SIZE / 4);
      itemGraphics.generateTexture(
        "item-marker-fallback",
        TILE_SIZE / 2,
        TILE_SIZE / 2,
      );
      itemGraphics.destroy();
    }

    // Create a placeholder tile if it doesn't exist
    if (!this.scene.textures.exists("placeholder-tile")) {
      const graphics = this.scene.make.graphics({ x: 0, y: 0 });
      graphics.fillStyle(0xcccccc);
      graphics.fillRect(0, 0, TILE_SIZE, TILE_SIZE);
      graphics.lineStyle(1, 0x000000);
      graphics.strokeRect(0, 0, TILE_SIZE, TILE_SIZE);
      graphics.generateTexture("placeholder-tile", TILE_SIZE, TILE_SIZE);
      graphics.destroy();
    }
  }

  loadTileImages(tileImagesData: TileImageRecord[]): Promise<void> {
    const records = [...tileImagesData];
    const load = this.tileImageLoadQueue.then(() =>
      this.loadTileImagesNow(records),
    );
    // Keep later requests moving after a failed batch while preserving the
    // failure for the caller that owns this specific request.
    this.tileImageLoadQueue = load.catch(() => undefined);
    return load;
  }

  private async loadTileImagesNow(tileImagesData: TileImageRecord[]) {
    // Track which textures we need to load
    const texturesToLoad = new Set<number>();

    // Process all tile images
    for (const tileImage of tileImagesData) {
      const tileKey = `tile-${tileImage.id}`;

      // Add to the set of textures to load
      if (!this.scene.textures.exists(tileKey)) {
        texturesToLoad.add(tileImage.id);
      }

      // Store the image path for later use
      const imgUrl = getTileImageUrl(tileImage.id);
      this.tileImageCache.set(tileImage.id, {
        key: tileKey,
        path: imgUrl,
      });
    }

    // Load all textures in a single batch
    if (texturesToLoad.size === 0) return;

    const completions: Promise<void>[] = [];
    for (const tileId of texturesToLoad) {
      const tileKey = `tile-${tileId}`;
      if (this.scene.textures.exists(tileKey)) continue;

      this.loadingTextures.add(tileKey);
      completions.push(
        new Promise<void>((resolve) => {
          const completeEvent = `filecomplete-image-${tileKey}`;
          const timeoutId = setTimeout(() => {
            cleanup();
            console.warn(`Timed out loading tile ${tileId}, using fallback`);
            this.createFallbackTile(tileKey, tileId.toString());
            resolve();
          }, TILE_IMAGE_LOAD_WATCHDOG_MS);
          const cleanup = () => {
            clearTimeout(timeoutId);
            this.scene.load.off(completeEvent, onComplete);
            this.scene.load.off("loaderror", onError);
            this.loadingTextures.delete(tileKey);
          };
          const onComplete = () => {
            cleanup();
            resolve();
          };
          const onError = (fileObj: Phaser.Loader.File) => {
            if (fileObj.key !== tileKey) return;
            cleanup();
            const displayId = fileObj.key.replace("tile-", "");
            console.warn(`Error loading tile ${displayId}, using fallback`);
            this.createFallbackTile(fileObj.key, displayId);
            resolve();
          };

          this.scene.load.once(completeEvent, onComplete);
          this.scene.load.on("loaderror", onError);
          this.scene.load.image(tileKey, getTileImageUrl(tileId), {
            responseType: "blob",
            timeout: TILE_IMAGE_XHR_TIMEOUT_MS,
          });
        }),
      );
    }

    if (completions.length === 0) return;
    if (!this.scene.load.isLoading()) this.scene.load.start();
    await Promise.all(completions);
  }

  // Helper method to create a fallback tile texture
  createFallbackTile(key: string, tileId: string) {
    // Create a new graphics object for this specific missing tile
    const graphics = this.scene.make.graphics({ x: 0, y: 0 });
    graphics.fillStyle(0xcccccc); // Light gray color for placeholder tiles
    graphics.fillRect(0, 0, TILE_SIZE, TILE_SIZE);
    graphics.lineStyle(1, 0x000000);
    graphics.strokeRect(0, 0, TILE_SIZE, TILE_SIZE);

    // Draw the tile ID on the texture for debugging
    const text = this.scene.add.text(TILE_SIZE / 2, TILE_SIZE / 2, tileId, {
      fontSize: "8px",
      color: "#000000",
    });
    text.setOrigin(0.5);

    // Generate the texture
    graphics.generateTexture(key, TILE_SIZE, TILE_SIZE);

    // Clean up
    graphics.destroy();
    text.destroy();
  }

  preloadCommonTiles() {
    // Preload some common tile images using the API endpoint
    for (let i = 1; i <= 10; i++) {
      this.scene.load.image(`tile-${i}`, getTileImageUrl(i));
    }

    // Load the poke_ball image for item markers
    this.scene.load.image("item-marker", getSpriteUrl("poke_ball.png"));
  }

  getTileImageCache() {
    return this.tileImageCache;
  }

  clearCache() {
    this.tileImageCache.clear();
    this.loadingTextures.clear();
  }
}
