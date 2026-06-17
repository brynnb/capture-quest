import { Scene } from "phaser";
import { TileImageCacheEntry, getTileImageUrl, getSpriteUrl } from "../api";
import { TILE_SIZE } from "../constants";

export class TileManager {
  private scene: Scene;
  private tileImageCache: Map<number, TileImageCacheEntry> = new Map();
  private loadingTextures: Set<string> = new Set();

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

  async loadTileImages(tileImagesData: any[]) {
    // Track which textures we need to load
    const texturesToLoad = new Set<number>();

    // Process all tile images
    for (const tileImage of tileImagesData) {
      const tileKey = `tile-${tileImage.id}`;

      // Add to the set of textures to load
      texturesToLoad.add(tileImage.id);

      // Store the image path for later use
      const imgUrl = getTileImageUrl(tileImage.id);
      this.tileImageCache.set(tileImage.id, {
        key: tileKey,
        path: imgUrl,
      });
    }

    // Load all textures in a single batch
    if (texturesToLoad.size > 0) {
      // Clear any previous listeners to avoid duplicates
      this.scene.load.off("loaderror");

      // Set up error handling
      this.scene.load.on("loaderror", (fileObj: any) => {
        // Handle loading errors by creating a fallback tile
        const tileId = fileObj.key.replace("tile-", "");
        console.warn(`Error loading tile ${tileId}, using fallback`);
        this.createFallbackTile(fileObj.key, tileId);
      });

      // Load all textures
      for (const tileId of texturesToLoad) {
        const tileKey = `tile-${tileId}`;
        const imgUrl = getTileImageUrl(tileId);

        // Load all textures
        this.scene.load.image(tileKey, imgUrl);
      }

      // Start the load
      await new Promise<void>((resolve) => {
        this.scene.load.once("complete", () => {
          resolve();
        });
        this.scene.load.start();
      });
    }
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
