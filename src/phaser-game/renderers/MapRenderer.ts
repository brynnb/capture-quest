import { Scene } from "phaser";
import { PhaserTile, PhaserActor, PhaserWarp } from "@/net/generated/world_api";
import { TILE_SIZE } from "../constants";
import { ActorMovementController } from "../controllers/ActorMovementController";
import { ActorManager } from "../managers/ActorManager";
import { isWorldInputFrozen } from "../utils/worldInputGuard";

interface LocalActorPositionOverride {
  x: number;
  y: number;
  direction?: string;
}

export class MapRenderer {
  // Maximum texture dimension before falling back to individual sprites
  private static readonly MAX_RENDER_TEXTURE_SIZE = 8192;

  private scene: Scene;
  private mapContainer: Phaser.GameObjects.Container;
  private tileRenderTexture: Phaser.GameObjects.RenderTexture | null = null;
  private tileDataMap: Map<string, number> = new Map(); // "x,y" -> tileImageId for updateTile
  private mapOriginX: number = 0; // tile-coordinate origin of the RenderTexture
  private mapOriginY: number = 0;
  private actorSprites: Map<number, Phaser.GameObjects.Sprite> = new Map();
  private actorZones: Map<number, Phaser.GameObjects.Zone> = new Map();
  private localActorPositionOverrides: Map<number, LocalActorPositionOverride> = new Map();
  private nameLabels: Map<number, Phaser.GameObjects.Text> = new Map();
  private chatBubbles: Map<number, Phaser.GameObjects.Container> = new Map();
  private chatBubbleTimers: Map<number, Phaser.Time.TimerEvent> = new Map();
  private movementController: ActorMovementController;
  private actorManager: ActorManager;
  private warnedMissingActors: Set<number> = new Set();
  private itemPickupHandler: ((e: Event) => void) | null = null;
  public localPlayerInternalId: number | null = null;

  // Tile editor: individual sprites for tiles placed outside the RenderTexture bounds
  private userTileSprites: Map<string, Phaser.GameObjects.Image> = new Map();
  // Tile editor: cursor preview sprites
  private cursorPreviewSprites: Phaser.GameObjects.GameObject[] = [];
  private cursorPreviewContainer: Phaser.GameObjects.Container | null = null;

  // Tile animations: tileImageId -> array of frame image paths + duration
  private tileAnimations: Map<number, { frames: string[]; durationMs: number }> = new Map();
  private tileAnimationTimer: ReturnType<typeof setInterval> | null = null;
  private tileAnimationFrameIndex: number = 0;

  constructor(
    scene: Scene,
    mapContainer: Phaser.GameObjects.Container,
    actorManager: ActorManager,
  ) {
    this.scene = scene;
    this.mapContainer = mapContainer;
    this.actorManager = actorManager;
    this.movementController = new ActorMovementController(scene);

    // Create the actor-fallback texture eagerly so it exists before any sprite references it
    if (!this.scene.textures.exists("actor-fallback")) {
      const graphics = this.scene.make.graphics({ x: 0, y: 0 });
      graphics.fillStyle(0x0000ff);
      graphics.fillRect(0, 0, TILE_SIZE, TILE_SIZE);
      graphics.lineStyle(1, 0xffffff);
      graphics.strokeRect(0, 0, TILE_SIZE, TILE_SIZE);
      graphics.generateTexture("actor-fallback", TILE_SIZE, TILE_SIZE);
      graphics.destroy();
    }

    // Listen for item pickup events to remove actor sprites
    this.itemPickupHandler = (e: Event) => {
      const { actorId, itemName } = (e as CustomEvent).detail;
      this.removeActor(actorId);
      console.log(`[MapRenderer] Removed item ball sprite for ${itemName} (actor ${actorId})`);
    };
    window.addEventListener("itemPickedUp", this.itemPickupHandler);

    // Load tile animation definitions from server
    this.loadTileAnimations();
  }

  private isWorldInputFrozen(): boolean {
    const scene = this.scene as Scene & { isWorldInputFrozen?: () => boolean };
    return scene.isWorldInputFrozen?.() ?? isWorldInputFrozen();
  }

  private consumeInteractivePointer(
    event?: Phaser.Types.Input.EventData,
  ): void {
    event?.stopPropagation();
    this.scene.events.emit("mapInteractivePointerDown");
  }

  private configureActorSpriteInteraction(
    actorSprite: Phaser.GameObjects.Sprite,
    actor: PhaserActor,
  ): void {
    if (actor.objectType !== "npc" && actor.objectType !== "item") {
      return;
    }

    actorSprite.disableInteractive();
    this.createActorClickZone(actor, true);
  }

  private configureActorZoneInteraction(
    zone: Phaser.GameObjects.Zone,
    actor: PhaserActor,
  ): void {
    const zoneWithActor = zone as Phaser.GameObjects.Zone & {
      clickActor?: PhaserActor;
    };
    zoneWithActor.clickActor = actor;
    zone.setInteractive({ useHandCursor: true });
    zone.off("pointerdown");
    zone.on("pointerdown", (
      _pointer: Phaser.Input.Pointer,
      _localX: number,
      _localY: number,
      event?: Phaser.Types.Input.EventData,
    ) => {
      this.consumeInteractivePointer(event);
      if (this.isWorldInputFrozen()) return;
      const latestActor =
        zoneWithActor.clickActor ??
        (this.actorSprites.get(actor.id) as any)?.actorData ?? // eslint-disable-line @typescript-eslint/no-explicit-any
        actor;
      this.scene.events.emit("actorClicked", latestActor);
    });
  }

  renderMap(
    tiles: PhaserTile[],
    items: any[], // eslint-disable-line @typescript-eslint/no-explicit-any
    warps: PhaserWarp[] = [],
    actors: PhaserActor[] = [],
  ) {
    console.log(
      `Rendering map with ${tiles.length} tiles, ${items.length} items, ${warps.length} warps, ${actors.length} actors`,
    );

    // Clear existing map - ensure proper cleanup
    this.clear();

    // Calculate bounds first so we can size the RenderTexture
    const bounds = this.calculateMapBounds(tiles);
    this.mapOriginX = bounds.minX;
    this.mapOriginY = bounds.minY;

    // Check if the map fits in a single RenderTexture
    const texWidth = bounds.width;
    const texHeight = bounds.height;

    // Query the actual GPU max texture size via Phaser's renderer
    let gpuMaxSize = MapRenderer.MAX_RENDER_TEXTURE_SIZE;
    try {
      const renderer = this.scene.game.renderer;
      if (renderer && "gl" in renderer) {
        const gl = (renderer as any).gl as WebGLRenderingContext; // eslint-disable-line @typescript-eslint/no-explicit-any
        if (gl) {
          gpuMaxSize = gl.getParameter(gl.MAX_TEXTURE_SIZE);
        }
      }
    } catch {
      /* ignore */
    }

    const maxSize = Math.min(MapRenderer.MAX_RENDER_TEXTURE_SIZE, gpuMaxSize);
    const canUseRenderTexture =
      texWidth > 0 &&
      texHeight > 0 &&
      texWidth <= maxSize &&
      texHeight <= maxSize;

    console.log(
      `[MapRenderer] Map bounds: ${texWidth}x${texHeight}px, origin: (${this.mapOriginX}, ${this.mapOriginY}), GPU max: ${gpuMaxSize}, using RenderTexture: ${canUseRenderTexture}`,
    );

    if (canUseRenderTexture && tiles.length > 0) {
      // Create a single RenderTexture for all tiles
      const rtX = this.mapOriginX * TILE_SIZE;
      const rtY = this.mapOriginY * TILE_SIZE;
      this.tileRenderTexture = this.scene.add.renderTexture(
        rtX,
        rtY,
        texWidth,
        texHeight,
      );
      this.tileRenderTexture.setOrigin(0, 0);

      // Stamp each tile onto the RenderTexture using batch mode for performance
      let stampedCount = 0;
      let skippedCount = 0;
      this.tileRenderTexture.beginDraw();
      for (const tile of tiles) {
        const { x, y, tileImageId } = tile;
        const tileKey = `tile-${tileImageId}`;
        const localX = (x - this.mapOriginX) * TILE_SIZE;
        const localY = (y - this.mapOriginY) * TILE_SIZE;

        if (this.scene.textures.exists(tileKey)) {
          this.tileRenderTexture.batchDrawFrame(
            tileKey,
            undefined,
            localX,
            localY,
          );
          stampedCount++;
        } else {
          skippedCount++;
        }

        // Store tile data for later updates
        this.tileDataMap.set(`${x},${y}`, tileImageId);
      }
      this.tileRenderTexture.endDraw();
      console.log(
        `[MapRenderer] RenderTexture: stamped ${stampedCount} tiles, skipped ${skippedCount} (missing textures)`,
      );

      // Add the single RenderTexture to the container
      this.mapContainer.add(this.tileRenderTexture);
    } else if (tiles.length > 0) {
      // Fallback: map too large for a single texture, use individual images
      console.warn(
        `Map too large for RenderTexture (${texWidth}x${texHeight}), falling back to individual sprites`,
      );
      for (const tile of tiles) {
        const { x, y, tileImageId } = tile;
        const posX = x * TILE_SIZE;
        const posY = y * TILE_SIZE;
        const tileKey = `tile-${tileImageId}`;

        const tileSprite = this.scene.add.image(posX, posY, tileKey);
        tileSprite.setOrigin(0, 0);
        this.mapContainer.add(tileSprite);

        this.tileDataMap.set(`${x},${y}`, tileImageId);
      }
    }

    // Render items
    for (const item of items) {
      // Use the item-marker texture (poke_ball.png)
      const textureKey = this.scene.textures.exists("item-marker")
        ? "item-marker"
        : "item-marker-fallback";

      const itemSprite = this.scene.add.image(
        item.x * TILE_SIZE + TILE_SIZE / 2,
        item.y * TILE_SIZE + TILE_SIZE / 2,
        textureKey,
      );

      // Set appropriate size based on the texture used
      if (textureKey === "item-marker-fallback") {
        itemSprite.setDisplaySize(TILE_SIZE / 2, TILE_SIZE / 2);
      } else {
        // Set an appropriate size for the poke_ball image
        itemSprite.setDisplaySize(TILE_SIZE * 0.75, TILE_SIZE * 0.75);
      }

      // Store item data in the sprite for hover info
      (itemSprite as any).itemData = item;

      // Add to container
      this.mapContainer.add(itemSprite);
    }

    // Render warps
    for (const warp of warps) {
      // Create an invisible hit target for each warp.
      const warpZone = this.scene.add.zone(
        warp.x * TILE_SIZE + TILE_SIZE / 2,
        warp.y * TILE_SIZE + TILE_SIZE / 2,
        TILE_SIZE,
        TILE_SIZE,
      );

      // Store warp data in the zone for hover/debug info.
      (warpZone as any).warpData = warp;

      // Make the warp interactive
      warpZone.setInteractive({ useHandCursor: true });

      // Emit an event when the warp is clicked
      warpZone.on("pointerdown", (
        _pointer: Phaser.Input.Pointer,
        _localX: number,
        _localY: number,
        event?: Phaser.Types.Input.EventData,
      ) => {
        this.consumeInteractivePointer(event);
        if (this.isWorldInputFrozen()) return;
        // Emit immediately so TileViewer can start pathing the player to the warp
        this.scene.events.emit("warpClicked", warp);
      });

      // Add to container
      this.mapContainer.add(warpZone);
    }

    // Render Actors — reuse existing sprites when possible (they survive clear())
    for (const actor of actors) {
      const renderActor = this.actorWithLocalOverride(actor);

      // Sign objects are tile-backed fixtures, so keep the click target invisible.
      if (renderActor.objectType === "sign") {
        this.createActorClickZone(renderActor);
        continue;
      }

      // Hidden interactive objects (hidden items, PCs): invisible but clickable zone.
      if (renderActor.movementType === "HIDDEN") {
        this.createActorClickZone(renderActor);
        continue;
      }

      // Actors without sprites are intentionally invisible.
      if (!renderActor.spriteName) {
        continue;
      }

      // Calculate position (center of the tile)
      const posX = (renderActor.x ?? 0) * TILE_SIZE + TILE_SIZE / 2;
      const posY = (renderActor.y ?? 0) * TILE_SIZE + TILE_SIZE / 2;

      // Check if we already have a sprite for this actor (preserved from before clear())
      const existingSprite = this.actorSprites.get(renderActor.id);
      if (existingSprite && existingSprite.active) {
        // Reuse the existing sprite — just update position and re-register
        existingSprite.setPosition(posX, posY);
        (existingSprite as any).actorData = renderActor; // eslint-disable-line @typescript-eslint/no-explicit-any
        this.configureActorSpriteInteraction(existingSprite, renderActor);

        // Ensure it's in the container (it should be, but be safe)
        if (!this.mapContainer.exists(existingSprite)) {
          this.mapContainer.add(existingSprite);
        }

        // If still on fallback or missing texture, trigger async load
        const needsTextureLoad =
          existingSprite.texture.key === "actor-fallback" ||
          existingSprite.texture.key === "__MISSING";
        if (needsTextureLoad) {
          this.loadActorSpriteAsync(renderActor, existingSprite);
        }

        // Ensure name label exists for other player actors and update position
        const existingLabel = this.nameLabels.get(renderActor.id);
        const isLocalPlayer = renderActor.internalId === this.localPlayerInternalId;
        if (renderActor.objectType === "player" && renderActor.name && !isLocalPlayer) {
          if (existingLabel && existingLabel.active) {
            existingLabel.setPosition(posX, posY + TILE_SIZE / 2 + 2);
            if (!this.mapContainer.exists(existingLabel)) {
              this.mapContainer.add(existingLabel);
            }
          } else {
            this.createNameLabel(renderActor, posX, posY);
          }
        }

        // Re-register with movement controller
        if (
          renderActor.objectType === "player" ||
          renderActor.actionType === "WALK" ||
          renderActor.moveSpeed > 0
        ) {
          const label = this.nameLabels.get(renderActor.id);
          this.movementController.registerActor(renderActor, existingSprite, label);
        }
        continue;
      }

      // Create new sprite with fallback texture while we load the real one
      const actorSprite = this.scene.add.sprite(posX, posY, "actor-fallback");

      // If we have frame and flipX from the server, apply them directly
      if (renderActor.frame != null) {
        actorSprite.setFrame(0); // Fallback only has frame 0
      }

      if (renderActor.flipX != null) {
        actorSprite.setFlipX(renderActor.flipX);
      }

      // Load the sprite asynchronously
      this.loadActorSpriteAsync(renderActor, actorSprite);

      // Store actor data in the sprite for hover info
      (actorSprite as any).actorData = renderActor; // eslint-disable-line @typescript-eslint/no-explicit-any
      this.configureActorSpriteInteraction(actorSprite, renderActor);

      // Add to container
      this.mapContainer.add(actorSprite);

      // Store reference
      this.actorSprites.set(renderActor.id, actorSprite);

      // Create name label for other player actors (not the local player)
      const isLocal = renderActor.internalId === this.localPlayerInternalId;
      if (renderActor.objectType === "player" && renderActor.name && !isLocal) {
        this.createNameLabel(renderActor, posX, posY);
      }

      // Register with movement controller for walking actors (and always for the player)
      if (
        renderActor.objectType === "player" ||
        renderActor.actionType === "WALK" ||
        renderActor.moveSpeed > 0
      ) {
        const label = this.nameLabels.get(renderActor.id);
        this.movementController.registerActor(renderActor, actorSprite, label);
      }
    }

    // Ensure all actors (including preserved players) are on top of the tiles/warps
    this.actorSprites.forEach((sprite) => {
      this.mapContainer.bringToTop(sprite);
    });

    // Ensure name labels are on top of actor sprites
    this.nameLabels.forEach((label) => {
      if (label.active) {
        this.mapContainer.bringToTop(label);
      }
    });

    // Ensure chat bubbles are on top of everything
    this.chatBubbles.forEach((bubble) => {
      if (bubble.active) {
        this.mapContainer.bringToTop(bubble);
      }
    });

    return this.calculateMapBounds(tiles);
  }

  private createActorClickZone(
    actor: PhaserActor,
    preserveSprite: boolean = false,
  ): Phaser.GameObjects.Zone {
    if (!preserveSprite) {
      this.removeActorSprite(actor.id);
    }

    const posX = (actor.x ?? 0) * TILE_SIZE + TILE_SIZE / 2;
    const posY = (actor.y ?? 0) * TILE_SIZE + TILE_SIZE / 2;
    const existingZone = this.actorZones.get(actor.id);

    if (existingZone && existingZone.active) {
      existingZone.setPosition(posX, posY);
      this.configureActorZoneInteraction(existingZone, actor);
      if (!this.mapContainer.exists(existingZone)) {
        this.mapContainer.add(existingZone);
      }
      this.mapContainer.bringToTop(existingZone);
      return existingZone;
    }

    if (existingZone) {
      existingZone.destroy();
    }

    const zone = this.scene.add.zone(posX, posY, TILE_SIZE, TILE_SIZE);
    this.configureActorZoneInteraction(zone, actor);
    this.mapContainer.add(zone);
    this.mapContainer.bringToTop(zone);
    this.actorZones.set(actor.id, zone);
    return zone;
  }

  private async loadActorSpriteAsync(
    actor: PhaserActor,
    actorSprite: Phaser.GameObjects.Sprite,
  ) {
    try {
      const spriteKey = await this.actorManager.loadActorSprite(
        actor.spriteName ?? "",
      );

      // Update the sprite texture if it still exists and is active
      if (!actorSprite || !actorSprite.active) {
        return;
      }

      if (!this.scene.textures.exists(spriteKey)) {
        console.warn(
          `Actor sprite texture "${spriteKey}" not found for ${actor.name}`,
        );
        return;
      }

      // Determine frame and flip state
      let frame = 0;
      let flipX = false;

      if (actor.frame != null && actor.flipX != null) {
        frame = actor.frame;
        flipX = actor.flipX;
      } else {
        const frameInfo = this.actorManager.getFrameForDirection(
          actor.actionType || "STAY",
          actor.actionDirection || "DOWN",
        );
        frame = frameInfo.frame;
        flipX = frameInfo.flipX;
      }

      // Set the texture with the correct frame
      const texture = this.scene.textures.get(spriteKey);
      const frameStr = frame.toString();
      if (texture && texture.has(frameStr)) {
        actorSprite.setTexture(spriteKey, frame);
      } else {
        actorSprite.setTexture(spriteKey, 0);
      }
      actorSprite.setFlipX(flipX);
      actorSprite.setOrigin(0.5, 0.5);
    } catch (error) {
      console.error(`Error loading actor sprite for ${actor.name}:`, error);
    }
  }

  refreshActorSprite(actor: PhaserActor): boolean {
    const renderActor = this.actorWithLocalOverride(actor);
    const actorSprite = this.actorSprites.get(renderActor.id);
    if (!actorSprite || !actorSprite.active || !renderActor.spriteName) {
      return false;
    }

    (actorSprite as any).actorData = renderActor; // eslint-disable-line @typescript-eslint/no-explicit-any
    this.configureActorSpriteInteraction(actorSprite, renderActor);
    this.movementController.updateActorMetadata(renderActor);
    this.loadActorSpriteAsync(renderActor, actorSprite);
    return true;
  }

  updateActorSpriteName(
    actorId: number,
    spriteName: string,
    direction?: string,
  ): PhaserActor | null {
    const actorSprite = this.actorSprites.get(actorId);
    if (!actorSprite || !actorSprite.active) {
      return null;
    }

    const actorData = (actorSprite as any).actorData as PhaserActor | undefined; // eslint-disable-line @typescript-eslint/no-explicit-any
    if (!actorData) {
      return null;
    }

    const nextActor: PhaserActor = {
      ...actorData,
      spriteName,
      actionDirection: direction || actorData.actionDirection,
    };
    this.refreshActorSprite(nextActor);
    return nextActor;
  }

  getActorSpriteName(actorId: number): string | undefined {
    const actorSprite = this.actorSprites.get(actorId);
    const actorData = (actorSprite as any)?.actorData as PhaserActor | undefined; // eslint-disable-line @typescript-eslint/no-explicit-any
    return actorData?.spriteName;
  }

  updateTile(x: number, y: number, newTileImageId: number): boolean {
    const key = `${x},${y}`;
    if (!this.tileDataMap.has(key)) {
      console.warn(`No tile found at position (${x}, ${y})`);
      return false;
    }

    const newTileKey = `tile-${newTileImageId}`;

    if (!this.scene.textures.exists(newTileKey)) {
      console.warn(`Texture ${newTileKey} does not exist`);
      return false;
    }

    // Update the stored tile data
    this.tileDataMap.set(key, newTileImageId);

    // Re-stamp the tile on the RenderTexture
    if (this.tileRenderTexture) {
      const localX = (x - this.mapOriginX) * TILE_SIZE;
      const localY = (y - this.mapOriginY) * TILE_SIZE;
      // Clear the old tile region with a solid fill, then draw the new tile
      this.tileRenderTexture.fill(
        0x000000,
        1,
        localX,
        localY,
        TILE_SIZE,
        TILE_SIZE,
      );
      this.tileRenderTexture.drawFrame(newTileKey, undefined, localX, localY);
    }

    return true;
  }

  updateActorPosition(
    actorId: number,
    oldX: number,
    oldY: number,
    newX: number,
    newY: number,
    direction?: string,
    actor?: PhaserActor,
  ): boolean {
    // Check if actor is registered with movement controller
    const actorState = this.movementController.getActorState(actorId);
    const actorSprite = this.actorSprites.get(actorId);

    if (actorState) {
      if (actor && actorSprite) {
        (actorSprite as any).actorData = actor; // eslint-disable-line @typescript-eslint/no-explicit-any
        this.configureActorSpriteInteraction(actorSprite, actor);
        this.movementController.updateActorMetadata(actor);
      }
      // Use the movement controller for smooth queued animation
      const moveDirection =
        direction || this.calculateDirection(oldX, oldY, newX, newY);
      this.movementController.handlePositionUpdate(
        actorId,
        newX,
        newY,
        moveDirection,
      );
      return true;
    }

    // Fall back to direct sprite manipulation for non-walking actors
    if (!actorSprite) {
      if (!this.warnedMissingActors.has(actorId)) {
        this.warnedMissingActors.add(actorId);
        console.warn(`No actor sprite found with ID ${actorId}`);
      }
      return false;
    }

    // Calculate new position
    const posX = newX * TILE_SIZE + TILE_SIZE / 2;
    const posY = newY * TILE_SIZE + TILE_SIZE / 2;

    // Determine the direction of movement
    const moveDir =
      direction || this.calculateDirection(oldX, oldY, newX, newY);

    // Update actor data
    const actorData = (actorSprite as any).actorData;
    if (actorData) {
      actorData.x = newX;
      actorData.y = newY;
      actorData.actionDirection = moveDir;
      this.configureActorSpriteInteraction(actorSprite, actorData);
    }

    // Simple tween for non-walking actors (should be rare)
    this.scene.tweens.add({
      targets: actorSprite,
      x: posX,
      y: posY,
      duration: 300,
      ease: "Linear",
    });

    return true;
  }

  snapActorPosition(
    actorId: number,
    x: number,
    y: number,
    direction: string = "DOWN",
    actor?: PhaserActor,
  ): boolean {
    const actorSprite = this.actorSprites.get(actorId);
    if (actor && actorSprite) {
      (actorSprite as any).actorData = actor; // eslint-disable-line @typescript-eslint/no-explicit-any
      this.configureActorSpriteInteraction(actorSprite, actor);
      this.movementController.updateActorMetadata(actor);
    }

    const actorState = this.movementController.getActorState(actorId);
    if (actorState) {
      this.movementController.snapActorToPosition(actorId, x, y, direction);
      return true;
    }

    if (!actorSprite) {
      return false;
    }

    actorSprite.setPosition(
      x * TILE_SIZE + TILE_SIZE / 2,
      y * TILE_SIZE + TILE_SIZE / 2,
    );
    const actorData = (actorSprite as any).actorData; // eslint-disable-line @typescript-eslint/no-explicit-any
    if (actorData) {
      actorData.x = x;
      actorData.y = y;
      actorData.actionDirection = direction;
    }
    return true;
  }

  waitForActorIdle(actorId: number): Promise<void> {
    return this.movementController.waitForActorIdle(actorId);
  }

  /**
   * Calculate movement direction from position delta
   */
  private calculateDirection(
    oldX: number,
    oldY: number,
    newX: number,
    newY: number,
  ): string {
    if (newY < oldY) return "UP";
    if (newY > oldY) return "DOWN";
    if (newX < oldX) return "LEFT";
    if (newX > oldX) return "RIGHT";
    return "DOWN";
  }

  updateActorAnimation(
    actorId: number,
    frame?: number,
    flipX?: boolean,
  ): boolean {
    // Get the actor sprite
    const actorSprite = this.actorSprites.get(actorId);

    if (!actorSprite) {
      console.warn(
        `No actor sprite found with ID ${actorId} for animation update`,
      );
      return false;
    }

    // Update the sprite's frame if provided
    if (frame !== undefined) {
      // Safety check: ensure the frame exists in the texture
      const texture = actorSprite.texture;
      const frameStr = frame.toString();
      if (texture && (texture.has(frameStr) || texture.has(frame as any))) {
        actorSprite.setFrame(frame);
      } else {
        // Fallback to frame 0 if requested frame doesn't exist
        actorSprite.setFrame(0);
      }

      // Also update the frame in the actor data
      const actorData = (actorSprite as any).actorData;
      if (actorData) {
        actorData.frame = frame;
      }
    }

    // Update the sprite's flip state if provided
    if (flipX !== undefined) {
      actorSprite.setFlipX(flipX);

      // Also update the flipX in the actor data
      const actorData = (actorSprite as any).actorData;
      if (actorData) {
        actorData.flipX = flipX;
      }
    }

    return true;
  }

  private createNameLabel(actor: PhaserActor, posX: number, posY: number): void {
    // Remove old label if it exists
    const oldLabel = this.nameLabels.get(actor.id);
    if (oldLabel) {
      oldLabel.destroy();
      this.nameLabels.delete(actor.id);
    }

    const label = this.scene.add.text(
      posX,
      posY + TILE_SIZE / 2 + 2,
      actor.name || "",
      {
        fontSize: "9px",
        fontFamily: "monospace",
        color: "#000000",
        align: "center",
      },
    );
    label.setOrigin(0.5, 0);
    // Tag it so clear() preserves it alongside actor sprites
    (label as any).actorData = actor; // eslint-disable-line @typescript-eslint/no-explicit-any
    this.mapContainer.add(label);
    this.nameLabels.set(actor.id, label);
  }

  showChatBubble(actorId: number, text: string): void {
    // Remove existing bubble for this actor
    this.removeChatBubble(actorId);

    const sprite = this.actorSprites.get(actorId);
    if (!sprite || !sprite.active) return;

    const bubbleX = sprite.x;
    const bubbleY = sprite.y - TILE_SIZE / 2 - 2;

    // Truncate long messages
    const displayText = text.length > 40 ? text.substring(0, 37) + "..." : text;

    const chatText = this.scene.add.text(0, 0, displayText, {
      fontSize: "8px",
      fontFamily: "monospace",
      color: "#000000",
      wordWrap: { width: 80 },
      align: "center",
    });
    chatText.setOrigin(0.5, 1);

    // Background rounded rect
    const padding = 3;
    const bgWidth = chatText.width + padding * 2;
    const bgHeight = chatText.height + padding * 2;
    const bg = this.scene.add.graphics();
    bg.fillStyle(0xffffff, 0.9);
    bg.fillRoundedRect(
      -bgWidth / 2,
      -bgHeight,
      bgWidth,
      bgHeight,
      3,
    );
    bg.lineStyle(1, 0x000000, 0.5);
    bg.strokeRoundedRect(
      -bgWidth / 2,
      -bgHeight,
      bgWidth,
      bgHeight,
      3,
    );

    // Adjust text position relative to background
    chatText.setPosition(0, -padding);

    const container = this.scene.add.container(bubbleX, bubbleY, [bg, chatText]);
    (container as any).actorData = { id: actorId }; // eslint-disable-line @typescript-eslint/no-explicit-any
    this.mapContainer.add(container);
    this.mapContainer.bringToTop(container);
    this.chatBubbles.set(actorId, container);

    // Register with movement controller so it follows the sprite
    this.movementController.setChatBubble(actorId, container);

    // Auto-remove after 5 seconds
    const timer = this.scene.time.delayedCall(5000, () => {
      this.removeChatBubble(actorId);
    });
    this.chatBubbleTimers.set(actorId, timer);
  }

  private removeChatBubble(actorId: number): void {
    const existing = this.chatBubbles.get(actorId);
    if (existing) {
      existing.destroy();
      this.chatBubbles.delete(actorId);
    }
    const timer = this.chatBubbleTimers.get(actorId);
    if (timer) {
      timer.destroy();
      this.chatBubbleTimers.delete(actorId);
    }
    // Clear from movement controller
    this.movementController.setChatBubble(actorId, undefined);
  }

  renderActor(actor: any): boolean {
    const renderActor = this.actorWithLocalOverride(actor as PhaserActor);

    if (renderActor.objectType === "sign" || renderActor.movementType === "HIDDEN") {
      this.createActorClickZone(renderActor);
      return true;
    }

    // Check if this actor is already rendered
    const existingSprite = this.actorSprites.get(renderActor.id);
    if (existingSprite) {
      (existingSprite as any).actorData = renderActor; // eslint-disable-line @typescript-eslint/no-explicit-any
      this.configureActorSpriteInteraction(existingSprite, renderActor);
      return false;
    }

    // Skip actors without sprites
    if (!renderActor.spriteName) {
      // console.log(`Skipping invisible actor in renderActor: ${actor.name || "Unnamed"} at (${actor.x}, ${actor.y})`);
      return false;
    }

    // Calculate position (center of the tile)
    const posX = (renderActor.x ?? 0) * TILE_SIZE + TILE_SIZE / 2;
    const posY = (renderActor.y ?? 0) * TILE_SIZE + TILE_SIZE / 2;

    // Create a temporary fallback sprite while we load the real one
    const actorSprite = this.scene.add.sprite(posX, posY, "actor-fallback");

    if (renderActor.frame != null) {
      actorSprite.setFrame(0); // Fallback only has frame 0
    }

    if (renderActor.flipX != null) {
      actorSprite.setFlipX(renderActor.flipX);
    }

    // Load the sprite asynchronously
    this.loadActorSpriteAsync(renderActor, actorSprite);

    // Store actor data in the sprite for hover info
    (actorSprite as any).actorData = renderActor;
    this.configureActorSpriteInteraction(actorSprite, renderActor);

    // Add to container
    this.mapContainer.add(actorSprite);

    // Store reference
    this.actorSprites.set(renderActor.id, actorSprite);

    // Create name label for other player actors (not the local player)
    const isLocalActor = renderActor.internalId === this.localPlayerInternalId;
    if (renderActor.objectType === "player" && renderActor.name && !isLocalActor) {
      this.createNameLabel(renderActor, posX, posY);
    }

    // Register with movement controller for walking actors (and always for the player)
    if (
      renderActor.objectType === "player" ||
      renderActor.actionType === "WALK" ||
      renderActor.moveSpeed > 0
    ) {
      const label = this.nameLabels.get(renderActor.id);
      this.movementController.registerActor(renderActor, actorSprite, label);
    }

    return true;
  }

  calculateMapBounds(tiles: any[]) {
    if (tiles.length === 0) {
      return { minX: 0, minY: 0, maxX: 0, maxY: 0, width: 0, height: 0 };
    }

    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;

    // Find the bounds of the map
    for (const tile of tiles) {
      minX = Math.min(minX, tile.x);
      minY = Math.min(minY, tile.y);
      maxX = Math.max(maxX, tile.x);
      maxY = Math.max(maxY, tile.y);
    }

    const mapWidth = (maxX - minX + 1) * TILE_SIZE;
    const mapHeight = (maxY - minY + 1) * TILE_SIZE;

    return {
      minX,
      minY,
      maxX,
      maxY,
      width: mapWidth,
      height: mapHeight,
      centerX: minX * TILE_SIZE + mapWidth / 2,
      centerY: minY * TILE_SIZE + mapHeight / 2,
    };
  }

  getActorSprite(actorId: number) {
    return this.actorSprites.get(actorId);
  }

  getActorTilePosition(actorId: number): { x: number; y: number; direction: string } | null {
    const tracked = this.movementController.getActorTilePosition(actorId);
    if (tracked) return tracked;

    const sprite = this.actorSprites.get(actorId);
    if (!sprite) return null;

    const actorData = (sprite as any).actorData as PhaserActor | undefined; // eslint-disable-line @typescript-eslint/no-explicit-any
    if (actorData && actorData.x !== undefined && actorData.y !== undefined) {
      return {
        x: actorData.x,
        y: actorData.y,
        direction: actorData.actionDirection || "DOWN",
      };
    }

    return {
      x: Math.floor(sprite.x / TILE_SIZE),
      y: Math.floor(sprite.y / TILE_SIZE),
      direction: "DOWN",
    };
  }

  async animateActorLocalPath(
    actorId: number,
    path: { x: number; y: number }[],
    finalDirection?: string,
  ): Promise<void> {
    this.ensureActorRegisteredForMovement(actorId);
    await this.movementController.animateActorLocalPath(actorId, path, finalDirection);
    const finalPosition = this.getActorTilePosition(actorId);
    if (finalPosition) {
      this.setLocalActorPositionOverride(
        actorId,
        finalPosition.x,
        finalPosition.y,
        finalDirection || finalPosition.direction,
      );
    }
  }

  clearLocalActorPositionOverrides(): void {
    this.localActorPositionOverrides.clear();
  }

  private setLocalActorPositionOverride(
    actorId: number,
    x: number,
    y: number,
    direction?: string,
  ): void {
    this.localActorPositionOverrides.set(actorId, { x, y, direction });
  }

  private actorWithLocalOverride(actor: PhaserActor): PhaserActor {
    const override = this.localActorPositionOverrides.get(actor.id);
    if (!override) return actor;
    return {
      ...actor,
      x: override.x,
      y: override.y,
      actionDirection: override.direction || actor.actionDirection,
    };
  }

  private ensureActorRegisteredForMovement(actorId: number): void {
    if (this.movementController.getActorTilePosition(actorId)) {
      return;
    }

    const sprite = this.actorSprites.get(actorId);
    if (!sprite) {
      return;
    }
    const actorData = (sprite as any).actorData as PhaserActor | undefined; // eslint-disable-line @typescript-eslint/no-explicit-any
    if (!actorData) {
      return;
    }

    const label = this.nameLabels.get(actorId);
    this.movementController.registerActor(actorData, sprite, label);
  }

  getTileAt(x: number, y: number) {
    return this.tileDataMap.get(`${x},${y}`);
  }

  clear() {
    try {
      // Collect non-actor children to destroy (RenderTexture, warps, items).
      // ALL actor sprites are preserved — renderMap will reuse them.
      // Uses collect-then-destroy to avoid iterate-while-mutating issues.
      if (this.mapContainer) {
        const toDestroy: Phaser.GameObjects.GameObject[] = [];
        this.mapContainer.each((child: Phaser.GameObjects.GameObject) => {
          if (child) {
            const actorData = (child as any).actorData; // eslint-disable-line @typescript-eslint/no-explicit-any
            if (actorData) {
              // Preserve ALL actor sprites (players and NPCs)
              return;
            }
            toDestroy.push(child);
          }
        });
        for (const child of toDestroy) {
          child.destroy();
        }
      }

      // Destroy the RenderTexture if it exists
      if (this.tileRenderTexture) {
        this.tileRenderTexture.destroy();
        this.tileRenderTexture = null;
      }

      // Clear the tile data map
      this.tileDataMap.clear();
      this.actorZones.clear();

      // Keep ALL actor sprites and movement controller registrations intact.
      // renderMap will reuse existing sprites and re-register with movement controller.

      // Reset warned missing actors so warnings can appear again for new map
      this.warnedMissingActors.clear();
    } catch (error) {
      console.error("Error clearing map renderer:", error);
    }
  }

  /**
   * Get the movement controller for external access (e.g., for handling server updates)
   */
  getMovementController(): ActorMovementController {
    return this.movementController;
  }

  destroy() {
    // Clean up all name labels
    this.nameLabels.forEach((label) => label.destroy());
    this.nameLabels.clear();

    // Clean up all chat bubbles and timers
    this.chatBubbles.forEach((bubble) => bubble.destroy());
    this.chatBubbles.clear();
    this.chatBubbleTimers.forEach((timer) => timer.destroy());
    this.chatBubbleTimers.clear();

    this.clear();
    if (this.itemPickupHandler) {
      window.removeEventListener("itemPickedUp", this.itemPickupHandler);
      this.itemPickupHandler = null;
    }
  }

  // --- Tile Editor Methods ---

  /**
   * Add a tile at the given position. If the position is within the RenderTexture bounds,
   * stamp it there. Otherwise, create an individual sprite.
   */
  addTile(x: number, y: number, tileImageId: number): boolean {
    const key = `${x},${y}`;
    const tileKey = `tile-${tileImageId}`;

    if (!this.scene.textures.exists(tileKey)) {
      return false;
    }

    // Update the data map
    this.tileDataMap.set(key, tileImageId);

    // Check if within RenderTexture bounds
    if (this.tileRenderTexture) {
      const localX = (x - this.mapOriginX) * TILE_SIZE;
      const localY = (y - this.mapOriginY) * TILE_SIZE;
      const rtWidth = this.tileRenderTexture.width;
      const rtHeight = this.tileRenderTexture.height;

      if (localX >= 0 && localY >= 0 && localX < rtWidth && localY < rtHeight) {
        // Within bounds — stamp onto RenderTexture using batch mode to avoid sub-pixel gaps
        this.tileRenderTexture.beginDraw();
        this.tileRenderTexture.batchDrawFrame(tileKey, undefined, localX, localY);
        this.tileRenderTexture.endDraw();

        // Remove any individual sprite at this position (if it was previously out-of-bounds)
        const existingSprite = this.userTileSprites.get(key);
        if (existingSprite) {
          existingSprite.destroy();
          this.userTileSprites.delete(key);
        }
        return true;
      }
    }

    // Outside RenderTexture bounds (or no RT) — use individual sprite
    const existingSprite = this.userTileSprites.get(key);
    if (existingSprite) {
      existingSprite.setTexture(tileKey);
    } else {
      const posX = x * TILE_SIZE;
      const posY = y * TILE_SIZE;
      const sprite = this.scene.add.image(posX, posY, tileKey);
      sprite.setOrigin(0, 0);
      sprite.setDepth(-1); // Below actors
      this.mapContainer.add(sprite);
      // Send to back so actors render on top
      this.mapContainer.sendToBack(sprite);
      this.userTileSprites.set(key, sprite);
    }

    return true;
  }

  addTiles(
    tiles: { x: number; y: number; tileImageId: number }[],
  ): { rendered: number; skipped: number } {
    let rendered = 0;
    let skipped = 0;
    let renderTextureDrawing = false;

    const beginRenderTextureDraw = () => {
      if (!this.tileRenderTexture || renderTextureDrawing) return;
      this.tileRenderTexture.beginDraw();
      renderTextureDrawing = true;
    };

    for (const tile of tiles) {
      const key = `${tile.x},${tile.y}`;
      const tileKey = `tile-${tile.tileImageId}`;

      if (!this.scene.textures.exists(tileKey)) {
        skipped++;
        continue;
      }

      this.tileDataMap.set(key, tile.tileImageId);

      if (this.tileRenderTexture) {
        const localX = (tile.x - this.mapOriginX) * TILE_SIZE;
        const localY = (tile.y - this.mapOriginY) * TILE_SIZE;
        const rtWidth = this.tileRenderTexture.width;
        const rtHeight = this.tileRenderTexture.height;

        if (localX >= 0 && localY >= 0 && localX < rtWidth && localY < rtHeight) {
          beginRenderTextureDraw();
          this.tileRenderTexture.batchDrawFrame(tileKey, undefined, localX, localY);

          const existingSprite = this.userTileSprites.get(key);
          if (existingSprite) {
            existingSprite.destroy();
            this.userTileSprites.delete(key);
          }

          rendered++;
          continue;
        }
      }

      const existingSprite = this.userTileSprites.get(key);
      if (existingSprite) {
        existingSprite.setTexture(tileKey);
      } else {
        const sprite = this.scene.add.image(tile.x * TILE_SIZE, tile.y * TILE_SIZE, tileKey);
        sprite.setOrigin(0, 0);
        sprite.setDepth(-1);
        this.mapContainer.add(sprite);
        this.mapContainer.sendToBack(sprite);
        this.userTileSprites.set(key, sprite);
      }
      rendered++;
    }

    if (renderTextureDrawing) {
      this.tileRenderTexture?.endDraw();
    }

    return { rendered, skipped };
  }

  /**
   * Remove a tile at the given position (erase to black).
   */
  removeTile(x: number, y: number): boolean {
    const key = `${x},${y}`;

    if (!this.tileDataMap.has(key)) {
      return false;
    }

    this.tileDataMap.delete(key);

    // Clear from RenderTexture if within bounds
    if (this.tileRenderTexture) {
      const localX = (x - this.mapOriginX) * TILE_SIZE;
      const localY = (y - this.mapOriginY) * TILE_SIZE;
      const rtWidth = this.tileRenderTexture.width;
      const rtHeight = this.tileRenderTexture.height;

      if (localX >= 0 && localY >= 0 && localX < rtWidth && localY < rtHeight) {
        this.tileRenderTexture.fill(0x000000, 1, localX, localY, TILE_SIZE, TILE_SIZE);
      }
    }

    // Remove individual sprite if it exists
    const sprite = this.userTileSprites.get(key);
    if (sprite) {
      sprite.destroy();
      this.userTileSprites.delete(key);
    }

    return true;
  }

  /**
   * Show a semi-transparent cursor preview at the given tile coordinates.
   * brushSize determines the NxN square of tiles to preview.
   */
  showCursorPreview(tileX: number, tileY: number, tileImageId: number, brushSize: number): void {
    this.hideCursorPreview();

    const tileKey = `tile-${tileImageId}`;
    if (!this.scene.textures.exists(tileKey)) return;

    if (!this.cursorPreviewContainer) {
      this.cursorPreviewContainer = this.scene.add.container(0, 0);
      this.cursorPreviewContainer.setDepth(999);
      this.mapContainer.add(this.cursorPreviewContainer);
    }

    const halfBrush = Math.floor(brushSize / 2);
    for (let dy = 0; dy < brushSize; dy++) {
      for (let dx = 0; dx < brushSize; dx++) {
        const px = (tileX - halfBrush + dx) * TILE_SIZE;
        const py = (tileY - halfBrush + dy) * TILE_SIZE;
        const preview = this.scene.add.image(px, py, tileKey);
        preview.setOrigin(0, 0);
        preview.setAlpha(0.5);
        this.cursorPreviewContainer.add(preview);
        this.cursorPreviewSprites.push(preview);
      }
    }

    this.mapContainer.bringToTop(this.cursorPreviewContainer);
  }

  /**
   * Show an eraser cursor preview (red-tinted squares).
   */
  showEraserPreview(tileX: number, tileY: number, brushSize: number): void {
    this.hideCursorPreview();

    if (!this.cursorPreviewContainer) {
      this.cursorPreviewContainer = this.scene.add.container(0, 0);
      this.cursorPreviewContainer.setDepth(999);
      this.mapContainer.add(this.cursorPreviewContainer);
    }

    const halfBrush = Math.floor(brushSize / 2);
    for (let dy = 0; dy < brushSize; dy++) {
      for (let dx = 0; dx < brushSize; dx++) {
        const px = (tileX - halfBrush + dx) * TILE_SIZE + TILE_SIZE / 2;
        const py = (tileY - halfBrush + dy) * TILE_SIZE + TILE_SIZE / 2;
        const rect = this.scene.add.rectangle(px, py, TILE_SIZE, TILE_SIZE, 0xff0000, 0.3);
        this.cursorPreviewContainer.add(rect);
        this.cursorPreviewSprites.push(rect);
      }
    }

    this.mapContainer.bringToTop(this.cursorPreviewContainer);
  }

  /**
   * Show a semi-transparent stamp preview at the given tile coordinates.
   * The stamp's top-left corner is centered on the cursor tile.
   */
  showStampPreview(tileX: number, tileY: number, tileImageIds: number[][], widthTiles: number, heightTiles: number): void {
    this.hideCursorPreview();

    if (!this.cursorPreviewContainer) {
      this.cursorPreviewContainer = this.scene.add.container(0, 0);
      this.cursorPreviewContainer.setDepth(999);
      this.mapContainer.add(this.cursorPreviewContainer);
    }

    // Center the stamp on the cursor: offset so cursor is at the center of the stamp
    const offsetX = Math.floor(widthTiles / 2);
    const offsetY = Math.floor(heightTiles / 2);

    for (let row = 0; row < heightTiles; row++) {
      for (let col = 0; col < widthTiles; col++) {
        const tid = tileImageIds[row]?.[col];
        if (!tid) continue;
        const tileKey = `tile-${tid}`;
        if (!this.scene.textures.exists(tileKey)) continue;

        const px = (tileX - offsetX + col) * TILE_SIZE;
        const py = (tileY - offsetY + row) * TILE_SIZE;
        const preview = this.scene.add.image(px, py, tileKey);
        preview.setOrigin(0, 0);
        preview.setAlpha(0.5);
        this.cursorPreviewContainer.add(preview);
        this.cursorPreviewSprites.push(preview);
      }
    }

    this.mapContainer.bringToTop(this.cursorPreviewContainer);
  }

  /**
   * Hide the cursor preview.
   */
  hideCursorPreview(): void {
    for (const sprite of this.cursorPreviewSprites) {
      sprite.destroy();
    }
    this.cursorPreviewSprites = [];
  }

  /**
   * Load a tile texture if it doesn't exist yet (for dynamically loading tile images).
   * Returns a promise that resolves when the texture is ready.
   */
  async loadTileTextureIfNeeded(tileImageId: number): Promise<boolean> {
    const tileKey = `tile-${tileImageId}`;
    if (this.scene.textures.exists(tileKey)) return true;

    const { getTileImageUrl } = await import("../api/tileService");
    const url = getTileImageUrl(tileImageId);

    return new Promise<boolean>((resolve) => {
      this.scene.load.image(tileKey, url);
      this.scene.load.once("complete", () => resolve(true));
      this.scene.load.once("loaderror", () => resolve(false));
      this.scene.load.start();
    });
  }

  /**
   * Re-render all tiles that use the given tileImageId.
   * Called after a tile image file has been replaced and the texture reloaded.
   */
  refreshTilesWithImage(tileImageId: number): void {
    const tileKey = `tile-${tileImageId}`;
    if (!this.scene.textures.exists(tileKey)) return;

    // Re-stamp all matching tiles onto the RenderTexture and update individual sprites
    for (const [key, tid] of this.tileDataMap.entries()) {
      if (tid !== tileImageId) continue;
      const [xStr, yStr] = key.split(",");
      const x = parseInt(xStr, 10);
      const y = parseInt(yStr, 10);

      // Re-stamp on RenderTexture if within bounds
      if (this.tileRenderTexture) {
        const localX = (x - this.mapOriginX) * TILE_SIZE;
        const localY = (y - this.mapOriginY) * TILE_SIZE;
        const rtWidth = this.tileRenderTexture.width;
        const rtHeight = this.tileRenderTexture.height;

        if (localX >= 0 && localY >= 0 && localX < rtWidth && localY < rtHeight) {
          this.tileRenderTexture.beginDraw();
          this.tileRenderTexture.batchDrawFrame(tileKey, undefined, localX, localY);
          this.tileRenderTexture.endDraw();
        }
      }

      // Update individual sprite if it exists
      const sprite = this.userTileSprites.get(key);
      if (sprite) {
        sprite.setTexture(tileKey);
      }
    }
  }

  /**
   * Load tile animation definitions from the server and start the animation loop.
   */
  private async loadTileAnimations(): Promise<void> {
    try {
      const resp = await fetch("/api/tiles/animations");
      const data = await resp.json();
      if (!data.success || !Array.isArray(data.animations)) return;

      // Group frames by tileImageId
      const grouped = new Map<number, { frames: string[]; durationMs: number }>();
      for (const frame of data.animations as { tileImageId: number; frameIndex: number; frameImagePath: string; frameDurationMs: number }[]) {
        if (!grouped.has(frame.tileImageId)) {
          grouped.set(frame.tileImageId, { frames: [], durationMs: frame.frameDurationMs });
        }
        const entry = grouped.get(frame.tileImageId)!;
        entry.frames[frame.frameIndex] = `/phaser/tile_images/${frame.frameImagePath}`;
      }

      this.tileAnimations = grouped;

      if (this.tileAnimations.size > 0) {
        // Preload all animation frame textures
        let loadCount = 0;
        for (const [tileImageId, anim] of this.tileAnimations) {
          for (let i = 0; i < anim.frames.length; i++) {
            const frameKey = `tile-${tileImageId}-frame-${i}`;
            if (!this.scene.textures.exists(frameKey)) {
              this.scene.load.image(frameKey, anim.frames[i]);
              loadCount++;
            }
          }
        }
        if (loadCount > 0) {
          this.scene.load.once("complete", () => {
            this.startAnimationLoop();
          });
          this.scene.load.start();
        } else {
          this.startAnimationLoop();
        }
      }
    } catch {
      // Server may not be running — animations just won't play
    }
  }

  /**
   * Start the interval that cycles animated tile frames.
   */
  private startAnimationLoop(): void {
    if (this.tileAnimationTimer) return;
    // Use the minimum duration across all animations as the tick interval
    let minDuration = 500;
    for (const anim of this.tileAnimations.values()) {
      if (anim.durationMs < minDuration) minDuration = anim.durationMs;
    }
    this.tileAnimationTimer = setInterval(() => this.tickAnimations(), minDuration);
  }

  /**
   * Stop the animation loop (called on cleanup/destroy).
   */
  stopAnimationLoop(): void {
    if (this.tileAnimationTimer) {
      clearInterval(this.tileAnimationTimer);
      this.tileAnimationTimer = null;
    }
  }

  /**
   * Advance animated tiles to the next frame and re-render them.
   */
  private tickAnimations(): void {
    this.tileAnimationFrameIndex++;

    for (const [tileImageId, anim] of this.tileAnimations) {
      const totalFrames = anim.frames.length + 1; // +1 for the base tile
      const currentFrame = this.tileAnimationFrameIndex % totalFrames;

      // Determine which texture key to use
      let textureKey: string;
      if (currentFrame === 0) {
        textureKey = `tile-${tileImageId}`;
      } else {
        textureKey = `tile-${tileImageId}-frame-${currentFrame - 1}`;
      }

      if (!this.scene.textures.exists(textureKey)) continue;

      // Update all tiles on the map that use this tileImageId
      for (const [key, tid] of this.tileDataMap.entries()) {
        if (tid !== tileImageId) continue;

        // Update individual sprites
        const sprite = this.userTileSprites.get(key);
        if (sprite) {
          sprite.setTexture(textureKey);
        }

        // Re-stamp on RenderTexture
        if (this.tileRenderTexture) {
          const [xStr, yStr] = key.split(",");
          const x = parseInt(xStr, 10);
          const y = parseInt(yStr, 10);
          const localX = (x - this.mapOriginX) * TILE_SIZE;
          const localY = (y - this.mapOriginY) * TILE_SIZE;
          const rtWidth = this.tileRenderTexture.width;
          const rtHeight = this.tileRenderTexture.height;

          if (localX >= 0 && localY >= 0 && localX < rtWidth && localY < rtHeight) {
            this.tileRenderTexture.beginDraw();
            this.tileRenderTexture.batchDrawFrame(textureKey, undefined, localX, localY);
            this.tileRenderTexture.endDraw();
          }
        }
      }
    }
  }

  /**
   * Get the current tile image ID at a position, or 0 if empty.
   */
  getTileImageIdAt(x: number, y: number): number {
    return this.tileDataMap.get(`${x},${y}`) ?? 0;
  }

  private removeActorSprite(actorId: number): boolean {
    const sprite = this.actorSprites.get(actorId);
    if (!sprite) return false;

    // Cleanup from movement controller
    this.movementController.unregisterActor(actorId);

    // Remove name label
    const label = this.nameLabels.get(actorId);
    if (label) {
      label.destroy();
      this.nameLabels.delete(actorId);
    }

    // Remove chat bubble
    this.removeChatBubble(actorId);

    // Remove from display
    sprite.destroy();
    this.actorSprites.delete(actorId);

    return true;
  }

  removeActor(actorId: number): boolean {
    let removed = false;
    const zone = this.actorZones.get(actorId);
    if (zone) {
      zone.destroy();
      this.actorZones.delete(actorId);
      removed = true;
    }

    return this.removeActorSprite(actorId) || removed;
  }
}
