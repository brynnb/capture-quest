import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import { TILE_SIZE } from "../constants";
import { getSpriteUrl } from "../services/PhaserNetworkService";

// Define direction frame indices
enum DirectionFrame {
  DOWN = 0, // 0th frame for facing down
  UP = 1, // 1st frame for facing up
  LEFT = 2, // 2nd frame for facing left
  RIGHT = 2, // Same as LEFT but will be flipped
  WALKING_DOWN = 3, // 3rd frame for walking down
  WALKING_UP = 4, // 4th frame for walking up
  WALKING_LEFT = 5, // 5th frame for walking left
  WALKING_RIGHT = 5, // Same as WALKING_LEFT but will be flipped
}

export class ActorManager {
  private scene: Scene;
  private actorSpriteCache: Map<string, string> = new Map();
  private pendingSpriteLoads: Map<string, Promise<string>> = new Map();

  constructor(scene: Scene) {
    this.scene = scene;
  }

  /**
   * Get the frame index for an actor based on its actionType and actionDirection
   * @param actionType The actor's action type (e.g., "STAY", "WALK")
   * @param actionDirection The actor's action direction (e.g., "DOWN", "UP", "LEFT", "RIGHT")
   * @returns An object with the frame index and whether to flip the sprite horizontally
   */
  getFrameForDirection(
    actionType: string,
    actionDirection: string,
  ): { frame: number; flipX: boolean } {
    // Default to facing down
    let frame = DirectionFrame.DOWN;
    let flipX = false;

    if (!actionDirection) {
      return { frame, flipX };
    }

    // Convert to uppercase for consistency
    const direction = (actionDirection || "DOWN").toUpperCase();
    const type = (actionType || "STAY").toUpperCase();

    // Handle different action types
    const isWalking = type === "WALK";

    // Check if we have a specific frame and flipX from the server
    if (
      (actionType as any)?.frame !== undefined &&
      (actionType as any)?.flipX !== undefined
    ) {
      return {
        frame: (actionType as any).frame,
        flipX: (actionType as any).flipX,
      };
    }

    // Determine the frame based on direction
    switch (direction) {
      case "DOWN":
        frame = isWalking ? DirectionFrame.WALKING_DOWN : DirectionFrame.DOWN;
        break;
      case "UP":
        frame = isWalking ? DirectionFrame.WALKING_UP : DirectionFrame.UP;
        break;
      case "LEFT":
        frame = isWalking ? DirectionFrame.WALKING_LEFT : DirectionFrame.LEFT;
        flipX = false;
        break;
      case "RIGHT":
        frame = isWalking ? DirectionFrame.WALKING_RIGHT : DirectionFrame.RIGHT;
        flipX = true;
        break;
      case "UP_DOWN":
      case "LEFT_RIGHT":
      case "ANY_DIR":
      case "NONE":
        // For movement patterns or NONE direction, default to DOWN
        frame = DirectionFrame.DOWN;
        break;
      default:
        console.warn(`Unknown direction: ${direction}, defaulting to DOWN`);
        frame = DirectionFrame.DOWN;
    }

    return { frame, flipX };
  }

  /**
   * Get the sprite key for a given sprite name without loading it.
   */
  getSpriteKey(spriteName: string): string {
    if (!spriteName) return "actor-fallback";
    return this.getSpriteInfo(spriteName).spriteKey;
  }

  async loadActorSprite(spriteName: string): Promise<string> {
    if (!spriteName) {
      return "actor-fallback";
    }

    const { spriteFileName, spriteKey } = this.getSpriteInfo(spriteName);

    // Check if we've already loaded this sprite
    if (this.scene.textures.exists(spriteKey)) {
      return spriteKey;
    }

    const pendingLoad = this.pendingSpriteLoads.get(spriteKey);
    if (pendingLoad) {
      return pendingLoad;
    }

    const loadPromise = this.loadActorSpriteOnce(
      spriteName,
      spriteFileName,
      spriteKey,
    );
    this.pendingSpriteLoads.set(spriteKey, loadPromise);

    try {
      return await loadPromise;
    } finally {
      this.pendingSpriteLoads.delete(spriteKey);
    }
  }

  private getSpriteInfo(spriteName: string): {
    spriteFileName: string;
    spriteKey: string;
  } {
    // Convert sprite name format (e.g., SPRITE_BRUNETTE_GIRL -> brunette_girl.png)
    // Remove SPRITE_ prefix, convert to lowercase, and add .png extension.
    const spriteFileName =
      spriteName.replace("SPRITE_", "").toLowerCase() + ".png";
    return {
      spriteFileName,
      spriteKey: `actor-${spriteFileName.replace(".png", "")}`,
    };
  }

  private waitForLoaderIdle(): Promise<void> {
    if (!this.scene.load.isLoading()) {
      return Promise.resolve();
    }

    return new Promise((resolve) => {
      const checkLoader = () => {
        if (!this.scene.load.isLoading()) {
          resolve();
          return;
        }
        this.scene.time.delayedCall(16, checkLoader);
      };
      checkLoader();
    });
  }

  private async loadActorSpriteOnce(
    spriteName: string,
    spriteFileName: string,
    spriteKey: string,
  ): Promise<string> {
    try {
      // Store in cache for future reference
      this.actorSpriteCache.set(spriteName, spriteKey);

      // If a map preload is already fetching this sprite, wait for that batch
      // to finish so we do not attach a per-file listener after the file has
      // already completed or silently collide with an in-flight key.
      await this.waitForLoaderIdle();

      if (this.scene.textures.exists(spriteKey)) {
        return spriteKey;
      }

      // Load the sprite as a spritesheet with 6 frames (0-5)
      // Frame 0: Down facing
      // Frame 1: Up facing
      // Frame 2: Left facing
      // Frame 3: Walking down
      // Frame 4: Walking up
      // Frame 5: Walking left
      this.scene.load.spritesheet(spriteKey, getSpriteUrl(spriteFileName), {
        frameWidth: TILE_SIZE,
        frameHeight: TILE_SIZE,
      });

      await this.waitForSpriteSheet(spriteKey, spriteFileName);

      // Verify the texture was loaded
      if (this.scene.textures.exists(spriteKey)) {
        return spriteKey;
      } else {
        console.warn(
          `Actor sprite ${spriteKey} failed to load despite no error`,
        );
        return "actor-fallback";
      }
    } catch (error) {
      console.error(`Error loading actor sprite ${spriteName}:`, error);
      return "actor-fallback";
    }
  }

  async preloadActorSprites(actors: PhaserActor[]) {
    // Create a set of unique sprite names to load
    const spriteNames = new Set<string>();

    for (const actor of actors) {
      if (actor.spriteName) {
        spriteNames.add(actor.spriteName);
      }
    }

    // Filter to only sprites that aren't already loaded
    const toLoad: {
      spriteName: string;
      spriteKey: string;
      spriteFileName: string;
    }[] = [];
    for (const spriteName of spriteNames) {
      const spriteFileName =
        spriteName.replace("SPRITE_", "").toLowerCase() + ".png";
      const spriteKey = `actor-${spriteFileName.replace(".png", "")}`;
      if (!this.scene.textures.exists(spriteKey)) {
        toLoad.push({ spriteName, spriteKey, spriteFileName });
        this.actorSpriteCache.set(spriteName, spriteKey);
      }
    }

    if (toLoad.length === 0) {
      return;
    }

    await this.waitForLoaderIdle();

    const missingSprites = toLoad.filter(
      ({ spriteKey }) => !this.scene.textures.exists(spriteKey),
    );

    if (missingSprites.length === 0) {
      return;
    }

    // Queue ALL spritesheets into the loader at once
    for (const { spriteKey, spriteFileName } of missingSprites) {
      this.scene.load.spritesheet(spriteKey, getSpriteUrl(spriteFileName), {
        frameWidth: TILE_SIZE,
        frameHeight: TILE_SIZE,
      });
    }

    // Single "complete" event for the entire batch
    await new Promise<void>((resolve) => {
      this.scene.load.once("complete", () => {
        resolve();
      });
      this.scene.load.start();
    });

    // Warn about any failures
    for (const { spriteKey } of missingSprites) {
      if (!this.scene.textures.exists(spriteKey)) {
        console.warn(`Actor sprite ${spriteKey} failed to load`);
      }
    }
  }

  clearCache() {
    this.actorSpriteCache.clear();
  }

  private waitForSpriteSheet(
    spriteKey: string,
    spriteFileName: string,
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      let settled = false;
      const timeoutId = window.setTimeout(() => {
        settle(
          new Error(
            `Timed out loading ${spriteKey} from ${getSpriteUrl(spriteFileName)}`,
          ),
        );
      }, 5000);

      const settle = (error?: Error) => {
        if (settled) {
          return;
        }
        settled = true;
        window.clearTimeout(timeoutId);
        this.scene.load.off(
          `filecomplete-spritesheet-${spriteKey}`,
          handleFileComplete,
        );
        this.scene.load.off("complete", handleComplete);
        this.scene.load.off("loaderror", handleLoadError);

        if (error) {
          reject(error);
          return;
        }
        resolve();
      };

      const handleFileComplete = () => {
        settle();
      };

      const handleComplete = () => {
        if (this.scene.textures.exists(spriteKey)) {
          settle();
          return;
        }
        settle(
          new Error(
            `Loaded batch did not produce ${spriteKey} from ${getSpriteUrl(spriteFileName)}`,
          ),
        );
      };

      const handleLoadError = (fileObj: { key?: string }) => {
        if (fileObj.key !== spriteKey) {
          return;
        }
        settle(
          new Error(
            `Failed to load ${spriteKey} from ${getSpriteUrl(spriteFileName)}`,
          ),
        );
      };

      this.scene.load.once(
        `filecomplete-spritesheet-${spriteKey}`,
        handleFileComplete,
      );
      this.scene.load.once("complete", handleComplete);
      this.scene.load.on("loaderror", handleLoadError);
      this.scene.load.start();
    });
  }
}
