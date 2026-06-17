import { Scene } from "phaser";
import {
  DEFAULT_ZOOM,
  FOLLOW_ZOOM,
  OVERWORLD_MODE,
  TILE_SIZE,
} from "../constants";
import { CameraController } from "../controllers/CameraController";
import { PlayerMovementController } from "../controllers/PlayerMovementController";
import { MapRenderer } from "../renderers/MapRenderer";
import { MapDataService } from "../services/MapDataService";
import { MapLoader } from "../services/MapLoader";
import { TileManager, ActorManager, UiManager, WarpManager } from "../managers";
import * as PhaserNet from "../services/PhaserNetworkService";
import WebFont from "webfontloader";

import {
  PhaserMapInfo,
  PhaserTile,
  PhaserActor,
  PhaserWarp,
} from "@/net/generated/world_api";

import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import {
  getWorldInputFreezeReason,
} from "../utils/worldInputGuard";
import type { WorldInputFreezeReason } from "../utils/worldInputGuard";
import { CutsceneSpriteController } from "./tile-viewer/CutsceneSpriteController";
import { ProceduralOverworldPreview } from "./tile-viewer/ProceduralOverworldPreview";
import { TileViewerDebugOverlay } from "./tile-viewer/TileViewerDebugOverlay";
import { TileViewerEventBridge } from "./tile-viewer/TileViewerEventBridge";
import { TileViewerInteractionController } from "./tile-viewer/TileViewerInteractionController";
import { TileViewerOverlays } from "./tile-viewer/TileViewerOverlays";
import { TileViewerWarpEvents } from "./tile-viewer/TileViewerWarpEvents";
import { TrainerEncounterPresenter } from "./tile-viewer/TrainerEncounterPresenter";

export interface TileUpdateEvent {
  tileId: number;
  newTileImageId: number;
}

export interface ActorUpdateEvent {
  actor: PhaserActor;
}

export class TileViewer extends Scene {
  // Services and managers
  private mapDataService: MapDataService;
  private tileManager!: TileManager;
  private actorManager!: ActorManager;
  private mapRenderer!: MapRenderer;
  private cameraController!: CameraController;
  private playerMovementController!: PlayerMovementController;
  private uiManager!: UiManager;
  private warpManager!: WarpManager;
  private mapLoader!: MapLoader;
  private overlays!: TileViewerOverlays;
  private interactionController!: TileViewerInteractionController;
  private proceduralOverworldPreview!: ProceduralOverworldPreview;
  private trainerEncounterPresenter!: TrainerEncounterPresenter;
  private cutsceneController!: CutsceneSpriteController;
  private warpEvents!: TileViewerWarpEvents;
  private eventBridge!: TileViewerEventBridge;
  private debugOverlay!: TileViewerDebugOverlay;

  // Data
  private tiles: PhaserTile[] = [];
  private mapInfo: PhaserMapInfo | null = null;
  private items: any[] = []; // Items might not have a Phaser-specific struct yet
  private actors: PhaserActor[] = [];
  private actorCache: Map<number, PhaserActor> = new Map();
  private playerActor: PhaserActor | null = null; // Store player separately so it's never lost
  private warps: PhaserWarp[] = [];
  private tileLookup: Map<string, PhaserTile> = new Map();
  private actorUpdateUnsubscribe: (() => void) | null = null;
  private actorDespawnUnsubscribe: (() => void) | null = null;
  private actorsUnsubscribe: (() => void) | null = null;
  private gameStatusUnsubscribe: (() => void) | null = null;
  private battleInputFreezeUnsubscribe: (() => void) | null = null;
  private trainerEncounterUnsubscribe: (() => void) | null = null;
  private surfSuccessHandler: ((event: Event) => void) | null = null;
  private cutsceneInputLocked: boolean = false;

  // View mode tracking
  private isOverworldMode: boolean = OVERWORLD_MODE;
  private viewedMapIds: Set<number> = new Set();
  public mapLoadInProgress: boolean = false;
  public warpDestX: number | null = null;
  public warpDestY: number | null = null;

  // Phaser elements
  private mapContainer!: Phaser.GameObjects.Container;

  // Loading text for preload phase
  private preloadText!: Phaser.GameObjects.Text;

  constructor() {
    super("TileViewer");
    this.mapDataService = new MapDataService();

  }

  preload() {
    // Create a simple loading text for the preload phase with a fallback font
    this.preloadText = this.add.text(10, 50, "Loading...", {
      fontFamily: "monospace, Arial",
      fontSize: "18px",
      color: "#ffffff",
      backgroundColor: "#000000",
      padding: { x: 5, y: 5 },
    });
    this.preloadText.setScrollFactor(0);
    this.preloadText.setDepth(1000);

    // Initialize managers
    this.tileManager = new TileManager(this);
    this.actorManager = new ActorManager(this);

    // Load the Pokemon font using WebFontLoader
    try {
      WebFont.load({
        custom: {
          families: ["Pokemon Pixel Font"],
          urls: ["/src/index.css"],
        },
        active: () => {
          console.log("Pokemon font loaded successfully");
          // Refresh UI elements when font is loaded
          if (this.uiManager) {
            this.uiManager.refreshTextElements();
          }
        },
        inactive: () => {
          console.warn("Pokemon font failed to load, using fallback fonts");
        },
      });
    } catch (e) {
      console.error("Error loading WebFont:", e);
    }

    // Add error handler for the item-marker (poke_ball) image
    this.load.on("loaderror", (fileObj: any) => {
      if (fileObj.key === "item-marker") {
        console.warn("Failed to load poke_ball.png, using fallback");
      }
    });

    // Preload common tiles
    this.tileManager.preloadCommonTiles();
  }

  create(data?: any) {
    console.log("Creating TileViewer scene", data);

    // Ensure we're starting with a clean state
    this.tiles = [];
    this.items = [];
    this.warps = [];
    this.actors = [];
    this.mapInfo = null;
    this.tileLookup.clear();

    // Set up scene cleanup
    this.events.once("shutdown", this.cleanupResources, this);

    // Create map container - ensure any existing one is destroyed first
    const existingContainer = this.children.getByName("mapContainer");
    if (existingContainer) {
      console.log("Found existing map container, destroying it");
      existingContainer.destroy();
    }

    // Create a fresh map container
    this.mapContainer = this.add.container(0, 0);
    this.mapContainer.name = "mapContainer"; // Add a name for easier debugging

    // Initialize managers and controllers
    this.mapRenderer = new MapRenderer(
      this,
      this.mapContainer,
      this.actorManager,
    );
    this.mapRenderer.localPlayerInternalId =
      usePlayerCharacterStore.getState().characterProfile?.id ?? null;
    this.cameraController = new CameraController(this);
    this.playerMovementController = new PlayerMovementController(this);
    this.playerMovementController.setInputFrozenChecker(() =>
      this.isWorldInputFrozen(),
    );
    this.surfSuccessHandler = (event: Event) => {
      const detail = (event as CustomEvent<{
        x: number;
        y: number;
        mapId: number;
        direction?: string;
      }>).detail;
      if (!detail) return;
      this.playerMovementController.applySurfingSuccess(
        detail.x,
        detail.y,
        detail.mapId,
        detail.direction,
      );
    };
    window.addEventListener("pokeSurfingSuccess", this.surfSuccessHandler);
    this.overlays = new TileViewerOverlays(this);
    this.proceduralOverworldPreview = new ProceduralOverworldPreview({
      scene: this,
      mapRenderer: () => this.mapRenderer,
      tiles: () => this.tiles,
      mapInfo: () => this.mapInfo,
      isOverworldMode: () => this.isOverworldMode,
      tileLookup: () => this.tileLookup,
    });
    this.proceduralOverworldPreview.register();
    this.trainerEncounterPresenter = new TrainerEncounterPresenter({
      scene: this,
      mapRenderer: () => this.mapRenderer,
      setInputLocked: (locked) => this.setCutsceneInputLocked(locked),
    });

    // Ensure camera follow is enabled by default when entering the world
    useGameStatusStore.getState().setCameraFollowEnabled(true);

    // Subscribe to real-time actor updates from WebTransport
    this.actorUpdateUnsubscribe = PhaserNet.onActorUpdate((actor: any) => {
      this.handleActorUpdate({ actor });
      // If this was our player, we might need to update follow
      if (actor.objectType === "player" && actor.id === this.playerActor?.id) {
        this.updateCameraFollow();
      }
    });

    // Subscribe to actor despawn events
    this.actorDespawnUnsubscribe = PhaserNet.onActorDespawn(
      (data: { id: number }) => {
        this.handleActorDespawn(data.id);
      },
    );

    // Subscribe to trainer encounter notifications (Phase 4.6)
    this.trainerEncounterUnsubscribe = PhaserNet.onTrainerEncounter((data) => {
      void this.trainerEncounterPresenter.handleEncounter(data);
    });

    // Connect movement controller callbacks
    const movementController = this.mapRenderer.getMovementController();
    movementController.setOnStepComplete((actorId, x, y) => {
      this.playerMovementController.onStepComplete(actorId, x, y);
    });

    // Subscribe to game status changes (for camera follow toggle)
    this.gameStatusUnsubscribe = useGameStatusStore.subscribe(
      (state) => state.isCameraFollowEnabled,
      () => {
        this.updateCameraFollow();
      },
    );

    let wasInBattle = usePokeBattleStore.getState().isInBattle;
    this.battleInputFreezeUnsubscribe = usePokeBattleStore.subscribe((state) => {
      if (state.isInBattle && !wasInBattle) {
        this.cancelWorldInput();
      }
      wasInBattle = state.isInBattle;
    });

    this.warpEvents = new TileViewerWarpEvents({
      scene: this,
      mapDataService: this.mapDataService,
      mapRenderer: () => this.mapRenderer,
      playerMovementController: () => this.playerMovementController,
      getPlayerActor: () => this.playerActor,
      setPlayerActor: (actor) => {
        this.playerActor = actor;
      },
      resetScene: (resetCamera) => this.resetScene(resetCamera),
    });
    this.warpEvents.register();

    this.eventBridge = new TileViewerEventBridge({
      scene: this,
      overlays: this.overlays,
      mapRenderer: () => this.mapRenderer,
      actors: () => this.actors,
      resetScene: (resetCamera) => this.resetScene(resetCamera),
    });
    this.eventBridge.register();

    // Subscribe to pushed actor lists (e.g. player spawn)
    this.actorsUnsubscribe = PhaserNet.onActors((actors: any[]) => {
      if (actors && actors.length > 0) {
        console.log(
          `[TileViewer] Received ${actors.length} actors via onActors broadcast`,
        );
        actors.forEach((incomingActor) => {
          let actor = incomingActor as PhaserActor;
          const isLocalPlayer = this.isLocalPlayerActor(actor);

          if (isLocalPlayer) {
            // Apply pending warp position if available
            this.warpManager.applyPendingWarpToActor(actor);

            // Store player separately so it's never lost during map clears
            const isFirstSpawn = !this.playerActor;
            actor = this.cacheActor(actor);
            this.playerActor = actor;

            // Request party data on first spawn so the HUD is populated immediately
            if (isFirstSpawn) {
              PhaserNet.sendPokemonPartyRequest();
            }
          } else {
            actor = this.cacheActor(actor);
          }

          if (isLocalPlayer || this.actorBelongsToLoadedView(actor)) {
            // Check if actor already exists
            const existingIndex = this.actors.findIndex(
              (a) => a.id === actor.id,
            );
            if (existingIndex !== -1) {
              this.actors[existingIndex] = actor;
            } else {
              this.actors.push(actor);
            }
            this.playerMovementController.updateBlockingActor(actor);

            // Skip sprite creation while a map load is in progress —
            // renderMap handles all sprite creation to avoid timing issues
            // with async texture loading and clear() destroying in-flight sprites.
            if (this.mapLoadInProgress) {
              return;
            }

            // Preload and render
            this.actorManager.preloadActorSprites([actor]).then(() => {
              // Re-check flag: map load may have started while we were preloading
              if (this.mapLoadInProgress) {
                return;
              }
              this.mapRenderer.renderActor(actor);

              // Center camera on player once when they first appear
              if (isLocalPlayer) {
                console.log(
                  `[TileViewer] Local player spawned/updated, centering camera`,
                );
                const actorX = actor.x ?? 0;
                const actorY = actor.y ?? 0;
                const posX = actorX * TILE_SIZE + TILE_SIZE / 2;
                const posY = actorY * TILE_SIZE + TILE_SIZE / 2;
                this.cameraController.centerOnMap(posX, posY);

                // Also start following if enabled
                this.updateCameraFollow();

                // Register with player movement controller
                const sprite = this.mapRenderer.getActorSprite(actor.id);
                if (sprite) {
                  this.playerMovementController.setPlayer(
                    actor.id,
                    actorX,
                    actorY,
                    actor.mapId,
                    this.mapRenderer,
                  );
                }
              }
            });
          }
        });
      }
    });

    // Check if we have data passed from resetScene
    let destinationMapId = null;
    let useOverworldSavedCamera = null;

    if (data) {
      destinationMapId = data.destinationMapId;
      useOverworldSavedCamera = data.useOverworldSavedCamera;

      // If we have destination coordinates, store them to apply when player actor arrives
      if (data.destinationX !== undefined && data.destinationY !== undefined) {
        const pendingX =
          data.destinationX !== null ? Number(data.destinationX) : null;
        const pendingY =
          data.destinationY !== null ? Number(data.destinationY) : null;
        this.warpManager.setPendingWarpPosition(pendingX, pendingY);
        // Also store on scene directly so MapLoader can use them independently
        this.warpDestX = pendingX;
        this.warpDestY = pendingY;
        console.log(
          `[TileViewer] Set pending warp destination: (${pendingX}, ${pendingY})`,
        );
      }
    } else {
      // Fall back to registry if no data was passed
      destinationMapId = this.game.registry.get("destinationMapId");
      useOverworldSavedCamera = this.game.registry.get(
        "useOverworldSavedCamera",
      );
    }

    // Check if we should preserve camera state
    const shouldPreserveCamera =
      useOverworldSavedCamera === true ||
      destinationMapId !== null;

    // Only reset the camera if we're not returning to overworld or warping to a map
    if (!shouldPreserveCamera) {
      this.cameraController.resetCamera();
    }

    this.uiManager = new UiManager(this);

    // Initialize WarpManager and MapLoader (after uiManager is created)
    this.warpManager = new WarpManager(
      this,
      this.mapDataService,
      this.cameraController,
      this.playerMovementController,
      this.uiManager,
      {
        onResetScene: (resetCamera: boolean) => this.resetScene(resetCamera),
        getPlayerActor: () => this.playerActor,
        setPlayerActor: (actor: PhaserActor) => {
          this.playerActor = actor;
        },
        getIsOverworldMode: () => this.isOverworldMode,
      },
    );
    this.playerMovementController.setWarpTileChecker((x, y) =>
      this.warpManager?.hasWarpAt(x, y) ?? false,
    );
    this.playerMovementController.setWarpAtProvider((x, y) =>
      this.warpManager?.getWarpAt(x, y) ?? null,
    );
    this.debugOverlay = new TileViewerDebugOverlay({
      scene: this,
      uiManager: () => this.uiManager,
      cameraController: () => this.cameraController,
      tileLookup: () => this.tileLookup,
      items: () => this.items,
      mapInfo: () => this.mapInfo,
      warps: () => this.warps,
      actors: () => this.actors,
    });

    this.mapLoader = new MapLoader(
      this,
      this.mapDataService,
      this.tileManager,
      this.actorManager,
      this.mapRenderer,
      this.cameraController,
      this.playerMovementController,
      this.uiManager,
      {
        getState: () => ({
          tiles: this.tiles,
          items: this.items,
          warps: this.warps,
          actors: this.actors,
          mapInfo: this.mapInfo,
          isOverworldMode: this.isOverworldMode,
          viewedMapIds: this.viewedMapIds,
        }),
        setState: (partial) => {
          if (partial.tiles !== undefined) {
            this.proceduralOverworldPreview?.clear();
            this.tiles = partial.tiles;
            this.tileLookup.clear();
            for (const t of partial.tiles) {
              this.tileLookup.set(`${t.x},${t.y}`, t);
            }
          }
          if (partial.items !== undefined) this.items = partial.items;
          if (partial.warps !== undefined) {
            this.warps = partial.warps;
            this.warpManager.setWarps(partial.warps);
          }
          if (partial.actors !== undefined) this.actors = partial.actors;
          if (partial.mapInfo !== undefined) this.mapInfo = partial.mapInfo;
          if (partial.isOverworldMode !== undefined)
            this.isOverworldMode = partial.isOverworldMode;
          if (partial.viewedMapIds !== undefined)
            this.viewedMapIds = partial.viewedMapIds;
        },
        getPlayerActor: () => this.playerActor,
        updateCameraFollow: () => this.updateCameraFollow(),
        removeMapLegend: () => this.debugOverlay.removeMapLegend(),
        createMapLegend: (maps) => this.debugOverlay.createMapLegend(maps),
        prepareActorsForLoadedView: (actors) =>
          this.prepareActorsForLoadedView(actors),
      },
    );

    // Try to refresh UI elements if the font is already loaded
    try {
      if (document.fonts && document.fonts.check("12px 'Pokemon Pixel Font'")) {
        console.log("Pokemon font already loaded, refreshing UI");
        this.uiManager.refreshTextElements();
      }
    } catch (e) {
      console.warn("Font check not supported:", e);
    }

    this.interactionController = new TileViewerInteractionController({
      scene: this,
      mapRenderer: () => this.mapRenderer,
      cameraController: () => this.cameraController,
      playerMovementController: () => this.playerMovementController,
      warpManager: () => this.warpManager,
      overlays: () => this.overlays,
      getPlayerActor: () => this.playerActor,
      actors: () => this.actors,
      viewedMapIds: () => this.viewedMapIds,
      currentActorById: (actorId) => this.currentActorById(actorId),
      isWorldInputFrozen: () => this.isWorldInputFrozen(),
      handleTileEditorClick: (worldX, worldY) =>
        this.handleTileEditorClick(worldX, worldY),
    });
    this.playerMovementController.setHeldKeyboardDirectionProvider(() =>
      this.interactionController.getKeyboardDirection(),
    );
    this.interactionController.setup();

    // Configure camera to ignore UI elements
    this.cameraController.configureIgnoreElements(
      this.uiManager.getUiElements(),
      this.mapContainer,
      this.uiManager.getWorldOverlayElements(),
    );

    this.debugOverlay.register();

    // Handle warp click event via WarpManager
    this.warpManager.setupWarpClickHandler();
    this.warpManager.setupKeyboardWarpHandlers();

    this.cutsceneController = new CutsceneSpriteController({
      scene: this,
      mapContainer: () => this.mapContainer,
      actorManager: () => this.actorManager,
      mapRenderer: () => this.mapRenderer,
      getPlayerActor: () => this.playerActor,
      setInputLocked: (locked) => this.setCutsceneInputLocked(locked),
      onHideObject: (actorId) => this.handleActorDespawn(actorId),
    });
    this.cutsceneController.registerCallbacks();

    // Handle window resize
    this.scale.on("resize", this.handleResize, this);

    // Set initial zoom based on view mode
    this.cameraController.setViewMode(this.isOverworldMode);

    // Remove preload text
    if (this.preloadText) {
      this.preloadText.destroy();
    }

    if (destinationMapId !== null && destinationMapId !== undefined) {
      // We're coming from a warp, load the destination map
      const mapId = parseInt(destinationMapId.toString(), 10);
      const isOverworldDest = this.mapDataService.isOverworld(mapId);

      console.log(`Loading destination map ${mapId} from warp (overworld: ${isOverworldDest})`);

      if (isOverworldDest) {
        // Warping to overworld
        this.isOverworldMode = true;
        this.cameraController.setViewMode(true);
        this.mapLoader.loadOverworldData();
      } else {
        // Warping to interior map
        this.isOverworldMode = false;
        this.cameraController.setViewMode(false);
        this.mapLoader.loadMapData(mapId);

        // Safari Zone: check for existing session when warping into a safari zone map
        const safariZoneMapIds = [217, 218, 219, 220];
        if (safariZoneMapIds.includes(mapId)) {
          import("@/phaser-game/services/PhaserNetworkService").then(
            (PhaserNet) => {
              PhaserNet.requestSafariZoneStatus();
            },
          );
        }
      }

      // Clear registry flags after loading
      this.game.registry.remove("useOverworldSavedCamera");
      this.game.registry.remove("destinationMapId");
      this.game.registry.remove("destinationX");
      this.game.registry.remove("destinationY");
    } else {
      // Normal startup - check character's current map to decide what to load
      const profile = usePlayerCharacterStore.getState().characterProfile;
      const charMapId = profile?.mapId ?? profile?.zoneId;
      const isCharOnOverworld =
        !charMapId || this.mapDataService.isOverworld(charMapId);

      if (isCharOnOverworld) {
        // Character is on an overworld map (or unknown) - load overworld
        this.isOverworldMode = true;
        this.cameraController.setViewMode(true);
        this.mapLoader.loadOverworldData();
      } else {
        // Character is on an interior map - load that map directly
        console.log(
          `[TileViewer] Character is on interior map ${charMapId}, loading it`,
        );
        this.isOverworldMode = false;
        this.cameraController.setViewMode(false);
        this.mapLoader.loadMapData(charMapId);

        // Safari Zone: check for existing session if reconnecting on a safari zone map
        const safariReconnectIds = [217, 218, 219, 220];
        if (safariReconnectIds.includes(charMapId)) {
          import("@/phaser-game/services/PhaserNetworkService").then(
            (PhaserNet) => {
              PhaserNet.requestSafariZoneStatus();
            },
          );
        }
      }

      // Clear registry flags after loading
      this.game.registry.remove("useOverworldSavedCamera");
      this.game.registry.remove("destinationMapId");
    }

    // Set up tile editor listeners (broadcast, undo, drag painting, cursor preview)
    this.setupTileEditorListeners();
  }

  updateCameraFollow() {
    const isFollowEnabled = useGameStatusStore.getState().isCameraFollowEnabled;

    if (isFollowEnabled) {
      if (this.playerActor) {
        const sprite = this.mapRenderer.getActorSprite(this.playerActor.id);

        if (sprite) {
          this.cameraController.setZoom(FOLLOW_ZOOM);
          // Snap instantly to the player
          this.cameraController.centerOnMap(sprite.x, sprite.y);
          // Start following
          this.cameraController.follow(sprite);

          // Also check if movement controller needs sprite refresh (e.g. after map load)
          this.playerMovementController.setPlayer(
            this.playerActor.id,
            this.playerActor.x ?? 0,
            this.playerActor.y ?? 0,
            this.playerActor.mapId,
            this.mapRenderer,
          );

          return;
        }
      }
    } else {
      this.cameraController.stopFollow();
      // Revert to overworld zoom if we were in overworld mode
      if (this.isOverworldMode) {
        this.cameraController.setZoom(DEFAULT_ZOOM);
      }
    }
  }

  handleTileUpdate(event: TileUpdateEvent) {
    // Find the tile in our local data
    const tileIndex = this.tiles.findIndex((tile) => tile.id === event.tileId);

    if (tileIndex !== -1) {
      // Update the tile image ID in our local data
      this.tiles[tileIndex].tileImageId = event.newTileImageId;

      // Get the tile's position
      const tile = this.tiles[tileIndex];

      // Update the tile sprite in the renderer
      this.mapRenderer.updateTile(tile.x, tile.y, event.newTileImageId);
    }
  }

  // --- Tile Editor ---

  private tileEditorBroadcastHandler: ((e: Event) => void) | null = null;
  private tileEditorUndoHandler: ((e: Event) => void) | null = null;
  private tileImageReplacedHandler: ((e: Event) => void) | null = null;
  private tileEditorPointerMoveHandler: ((pointer: Phaser.Input.Pointer) => void) | null = null;
  private tileEditorPointerDownHandler: ((pointer: Phaser.Input.Pointer) => void) | null = null;
  private tileEditorPointerUpHandler: (() => void) | null = null;
  private tileEditorDragging = false;
  private tileEditorDragBatchOld: { x: number; y: number; tileImageId: number }[] = [];
  private tileEditorDragBatchNew: { x: number; y: number; tileImageId: number }[] = [];
  private tileEditorFlushTimer: ReturnType<typeof setTimeout> | null = null;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private tileEditorStoreCache: any = null;

  private getTileEditorStore() {
    if (!this.tileEditorStoreCache) {
      // Lazy-load to avoid circular dependency
      import("@/stores/TileEditorStore").then((mod) => {
        this.tileEditorStoreCache = mod.default;
      });
    }
    return this.tileEditorStoreCache;
  }

  private getTileCollisionType(tileImageId: number): number {
    const store = this.getTileEditorStore();
    if (!store) return 1; // Default to walkable if store not loaded yet
    const props = store.getState().tileProperties as { tileImageId: number; collisionType: number }[];
    const prop = props.find((p: { tileImageId: number }) => p.tileImageId === tileImageId);
    return prop ? prop.collisionType : 1; // Default to walkable if not found
  }

  private getTileRawFootTileId(tileImageId: number): number | undefined {
    const store = this.getTileEditorStore();
    if (!store) return undefined;
    const props = store.getState().tileProperties as { tileImageId: number; rawFootTileId?: number }[];
    return props.find((p: { tileImageId: number }) => p.tileImageId === tileImageId)?.rawFootTileId;
  }

  private getTileTalkOverTile(tileImageId: number): boolean {
    const store = this.getTileEditorStore();
    if (!store) return false;
    const props = store.getState().tileProperties as { tileImageId: number; talkOverTile?: boolean }[];
    return props.find((p: { tileImageId: number }) => p.tileImageId === tileImageId)?.talkOverTile === true;
  }

  setupTileEditorListeners() {
    // Broadcast handler: other clients placed/erased tiles
    this.tileEditorBroadcastHandler = (e: Event) => {
      const payload = (e as CustomEvent).detail as {
        tiles: { x: number; y: number; tileImageId: number; collisionType: number; rawFootTileId?: number; talkOverTile?: boolean }[];
        mapId: number;
      };
      if (!payload || !Array.isArray(payload.tiles)) return;
      for (const tile of payload.tiles) {
        if (tile.tileImageId === 0) {
          this.mapRenderer.removeTile(tile.x, tile.y);
          this.playerMovementController.updateCollisionTile(tile.x, tile.y, 0, true);
        } else {
          this.mapRenderer.loadTileTextureIfNeeded(tile.tileImageId).then(() => {
            this.mapRenderer.addTile(tile.x, tile.y, tile.tileImageId);
          });
          this.playerMovementController.updateCollisionTile(
            tile.x,
            tile.y,
            tile.collisionType,
            false,
            tile.rawFootTileId,
            tile.talkOverTile === true,
          );
        }
      }
    };
    window.addEventListener("tileEditorBroadcast", this.tileEditorBroadcastHandler);

    // Undo handler: user clicked undo in the toolbar
    this.tileEditorUndoHandler = (e: Event) => {
      const entry = (e as CustomEvent).detail as { oldTiles: { x: number; y: number; tileImageId: number }[]; mapId: number };
      if (!entry || !Array.isArray(entry.oldTiles)) return;

      // Apply old tiles locally
      for (const tile of entry.oldTiles) {
        if (tile.tileImageId === 0) {
          this.mapRenderer.removeTile(tile.x, tile.y);
          this.playerMovementController.updateCollisionTile(tile.x, tile.y, 0, true);
        } else {
          this.mapRenderer.addTile(tile.x, tile.y, tile.tileImageId);
          const ct = this.getTileCollisionType(tile.tileImageId);
          this.playerMovementController.updateCollisionTile(
            tile.x,
            tile.y,
            ct,
            false,
            this.getTileRawFootTileId(tile.tileImageId),
            this.getTileTalkOverTile(tile.tileImageId),
          );
        }
      }

      // Send undo to server
      import("@/components/TileEditor/TileEditorNetwork").then((mod) => {
        mod.sendTileUndo(entry.oldTiles, entry.mapId);
      });
    };
    window.addEventListener("tileEditorUndo", this.tileEditorUndoHandler);

    // Pointer move for cursor preview
    this.tileEditorPointerMoveHandler = (pointer: Phaser.Input.Pointer) => {
      if (!useGameStatusStore.getState().isTileManagerOpen) {
        this.mapRenderer.hideCursorPreview();
        return;
      }

      const worldPoint = this.cameras.main.getWorldPoint(pointer.x, pointer.y);
      const tileX = Math.floor(worldPoint.x / TILE_SIZE);
      const tileY = Math.floor(worldPoint.y / TILE_SIZE);

      const store = this.getTileEditorStore();
      if (!store) return;
      const { selectedTool, selectedTileImageId, brushSize: bs } = store.getState();

      if (selectedTool === "eraser") {
        this.mapRenderer.showEraserPreview(tileX, tileY, bs);
      } else if (selectedTool === "stamp") {
        const { selectedStamp } = store.getState();
        if (selectedStamp) {
          this.mapRenderer.showStampPreview(tileX, tileY, selectedStamp.tileImageIds, selectedStamp.widthTiles, selectedStamp.heightTiles);
        } else {
          this.mapRenderer.hideCursorPreview();
        }
      } else if (selectedTileImageId && (selectedTool === "single" || selectedTool === "brush")) {
        const size = selectedTool === "brush" ? bs : 1;
        this.mapRenderer.showCursorPreview(tileX, tileY, selectedTileImageId, size);
      } else if (selectedTool === "fill" && selectedTileImageId) {
        this.mapRenderer.showCursorPreview(tileX, tileY, selectedTileImageId, 1);
      } else {
        this.mapRenderer.hideCursorPreview();
      }

      // Handle drag painting
      if (this.tileEditorDragging && (selectedTool === "single" || selectedTool === "brush" || selectedTool === "eraser")) {
        this.applyTileEditorAction(tileX, tileY);
      }
    };
    this.input.on("pointermove", this.tileEditorPointerMoveHandler);

    // Pointer down for drag start
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    this.tileEditorPointerDownHandler = (_pointer: Phaser.Input.Pointer) => {
      if (!useGameStatusStore.getState().isTileManagerOpen) return;
      const store = this.getTileEditorStore();
      if (!store) return;
      const { selectedTool } = store.getState();
      if (selectedTool === "single" || selectedTool === "brush" || selectedTool === "eraser") {
        this.tileEditorDragging = true;
        this.tileEditorDragBatchOld = [];
        this.tileEditorDragBatchNew = [];
      }
    };
    this.input.on("pointerdown", this.tileEditorPointerDownHandler);

    // Pointer up for drag end + flush batch
    this.tileEditorPointerUpHandler = () => {
      if (this.tileEditorDragging) {
        this.tileEditorDragging = false;
        this.flushTileEditorBatch();
      }
    };
    this.input.on("pointerup", this.tileEditorPointerUpHandler);

    // Tile image replaced handler: bust Phaser texture cache and re-render
    this.tileImageReplacedHandler = (e: Event) => {
      const { tileImageId } = (e as CustomEvent).detail as { tileImageId: number };
      if (!tileImageId) return;
      const tileKey = `tile-${tileImageId}`;
      // Remove the old texture so Phaser re-fetches it
      if (this.textures.exists(tileKey)) {
        this.textures.remove(tileKey);
      }
      // Reload the texture with a cache-buster query param
      import("@/phaser-game/api/tileService").then(({ getTileImageUrl }) => {
        const url = getTileImageUrl(tileImageId) + `?v=${Date.now()}`;
        this.load.image(tileKey, url);
        this.load.once("complete", () => {
          // Re-render all tiles using this tileImageId
          this.mapRenderer.refreshTilesWithImage(tileImageId);
        });
        this.load.start();
      });
    };
    window.addEventListener("tileImageReplaced", this.tileImageReplacedHandler);
  }

  cleanupTileEditorListeners() {
    if (this.tileEditorBroadcastHandler) {
      window.removeEventListener("tileEditorBroadcast", this.tileEditorBroadcastHandler);
      this.tileEditorBroadcastHandler = null;
    }
    if (this.tileEditorUndoHandler) {
      window.removeEventListener("tileEditorUndo", this.tileEditorUndoHandler);
      this.tileEditorUndoHandler = null;
    }
    if (this.tileEditorFlushTimer) {
      clearTimeout(this.tileEditorFlushTimer);
      this.tileEditorFlushTimer = null;
    }
    if (this.tileImageReplacedHandler) {
      window.removeEventListener("tileImageReplaced", this.tileImageReplacedHandler);
      this.tileImageReplacedHandler = null;
    }
    this.mapRenderer.hideCursorPreview();
  }

  private applyTileEditorAction(tileX: number, tileY: number) {
    const store = this.getTileEditorStore();
    if (!store) return;
    const { selectedTool, selectedTileImageId, brushSize: bs } = store.getState();

    const size = (selectedTool === "brush" || selectedTool === "eraser") ? bs : 1;
    const halfBrush = Math.floor(size / 2);

    for (let dy = 0; dy < size; dy++) {
      for (let dx = 0; dx < size; dx++) {
        const tx = tileX - halfBrush + dx;
        const ty = tileY - halfBrush + dy;

        // Skip if already in this drag batch
        if (this.tileEditorDragBatchNew.some((t) => t.x === tx && t.y === ty)) continue;

        const oldTileImageId = this.mapRenderer.getTileImageIdAt(tx, ty);

        if (selectedTool === "eraser") {
          if (oldTileImageId === 0) continue; // Already empty
          this.mapRenderer.removeTile(tx, ty);
          this.playerMovementController.updateCollisionTile(tx, ty, 0, true);
          this.tileEditorDragBatchOld.push({ x: tx, y: ty, tileImageId: oldTileImageId });
          this.tileEditorDragBatchNew.push({ x: tx, y: ty, tileImageId: 0 });
        } else if (selectedTileImageId) {
          if (oldTileImageId === selectedTileImageId) continue; // Same tile
          this.mapRenderer.addTile(tx, ty, selectedTileImageId);
          const ct = this.getTileCollisionType(selectedTileImageId);
          this.playerMovementController.updateCollisionTile(
            tx,
            ty,
            ct,
            false,
            this.getTileRawFootTileId(selectedTileImageId),
            this.getTileTalkOverTile(selectedTileImageId),
          );
          this.tileEditorDragBatchOld.push({ x: tx, y: ty, tileImageId: oldTileImageId });
          this.tileEditorDragBatchNew.push({ x: tx, y: ty, tileImageId: selectedTileImageId });
        }
      }
    }

    // Schedule a flush every 100ms during drag
    if (!this.tileEditorFlushTimer && this.tileEditorDragBatchNew.length > 0) {
      this.tileEditorFlushTimer = setTimeout(() => {
        this.flushTileEditorBatch();
        this.tileEditorFlushTimer = null;
      }, 100);
    }
  }

  private flushTileEditorBatch() {
    if (this.tileEditorDragBatchNew.length === 0) return;

    const mapId = 9999; // Overworld only
    const store = this.getTileEditorStore();
    if (!store) return;

    // Push undo entry
    store.getState().pushUndo({
      oldTiles: [...this.tileEditorDragBatchOld],
      newTiles: [...this.tileEditorDragBatchNew],
      mapId,
    });

    // Separate into place and erase batches
    const placeTiles = this.tileEditorDragBatchNew.filter((t) => t.tileImageId !== 0);
    const eraseTiles = this.tileEditorDragBatchNew.filter((t) => t.tileImageId === 0);

    import("@/components/TileEditor/TileEditorNetwork").then((mod) => {
      if (placeTiles.length > 0) {
        mod.sendTilePlace(placeTiles, mapId);
      }
      if (eraseTiles.length > 0) {
        mod.sendTileErase(eraseTiles.map((t) => ({ x: t.x, y: t.y })), mapId);
      }
    });

    this.tileEditorDragBatchOld = [];
    this.tileEditorDragBatchNew = [];
  }

  handleTileEditorClick(worldX: number, worldY: number) {
    const tileX = Math.floor(worldX / TILE_SIZE);
    const tileY = Math.floor(worldY / TILE_SIZE);

    const store = this.getTileEditorStore();
    if (!store) return;
    const { selectedTool, selectedTileImageId } = store.getState();

    if (selectedTool === "fill") {
      if (!selectedTileImageId) return;
      const mapId = 9999;

      // Send fill request to server (server does the BFS)
      import("@/components/TileEditor/TileEditorNetwork").then((mod) => {
        mod.sendTileFill(tileX, tileY, selectedTileImageId, mapId);
      });

      // We can't do optimistic fill locally (server determines the region),
      // so we rely on the broadcast to update the local map.
      return;
    }

    if (selectedTool === "stamp") {
      const { selectedStamp } = store.getState();
      if (!selectedStamp) return;
      const mapId = 9999;

      // Place all stamp tiles at once, centered on cursor
      const placeTiles: { x: number; y: number; tileImageId: number }[] = [];
      const oldTiles: { x: number; y: number; tileImageId: number }[] = [];
      const offsetX = Math.floor(selectedStamp.widthTiles / 2);
      const offsetY = Math.floor(selectedStamp.heightTiles / 2);

      for (let row = 0; row < selectedStamp.heightTiles; row++) {
        for (let col = 0; col < selectedStamp.widthTiles; col++) {
          const tx = tileX - offsetX + col;
          const ty = tileY - offsetY + row;
          const tid = selectedStamp.tileImageIds[row][col];
          if (!tid) continue;

          const oldTid = this.mapRenderer.getTileImageIdAt(tx, ty);
          oldTiles.push({ x: tx, y: ty, tileImageId: oldTid });
          placeTiles.push({ x: tx, y: ty, tileImageId: tid });

          // Optimistic local update
          this.mapRenderer.loadTileTextureIfNeeded(tid).then(() => {
            this.mapRenderer.addTile(tx, ty, tid);
          });
          const ct = this.getTileCollisionType(tid);
          this.playerMovementController.updateCollisionTile(
            tx,
                ty,
                ct,
                false,
                this.getTileRawFootTileId(tid),
                this.getTileTalkOverTile(tid),
              );
        }
      }

      if (placeTiles.length > 0) {
        // Push undo entry
        store.getState().pushUndo({ oldTiles, newTiles: placeTiles, mapId });

        // Send to server
        import("@/components/TileEditor/TileEditorNetwork").then((mod) => {
          mod.sendTilePlace(placeTiles, mapId);
        });
      }
      return;
    }

    // For single/brush/eraser, the drag handler already handles it via applyTileEditorAction
    // But for a simple click (no drag), we need to apply once
    this.tileEditorDragBatchOld = [];
    this.tileEditorDragBatchNew = [];
    this.applyTileEditorAction(tileX, tileY);
    this.flushTileEditorBatch();
  }

  private isLocalPlayerActor(actor: PhaserActor): boolean {
    const localCharId = usePlayerCharacterStore.getState().characterProfile?.id;
    return actor.objectType === "player" && actor.internalId === localCharId;
  }

  private actorBelongsToLoadedView(actor: PhaserActor): boolean {
    return (
      this.viewedMapIds.has(actor.mapId) ||
      (this.isOverworldMode && this.mapDataService.isOverworld(actor.mapId))
    );
  }

  private shouldRenderActorInCurrentView(actor: PhaserActor): boolean {
    return this.isLocalPlayerActor(actor) || this.actorBelongsToLoadedView(actor);
  }

  private currentActorById(actorId: number): PhaserActor | null {
    return (
      this.actorCache.get(actorId) ??
      this.actors.find((actor) => actor.id === actorId) ??
      null
    );
  }

  private cacheActor(actor: PhaserActor): PhaserActor {
    const existing = this.actorCache.get(actor.id);
    const cached = existing ? { ...existing, ...actor } : { ...actor };
    this.actorCache.set(actor.id, cached);
    return cached;
  }

  private prepareActorsForLoadedView(actors: PhaserActor[]): PhaserActor[] {
    const freshIds = new Set(actors.map((actor) => actor.id));
    for (const [actorId, cachedActor] of this.actorCache.entries()) {
      if (
        !this.isLocalPlayerActor(cachedActor) &&
        this.actorBelongsToLoadedView(cachedActor) &&
        !freshIds.has(actorId)
      ) {
        this.actorCache.delete(actorId);
      }
    }

    actors.forEach((actor) => this.cacheActor(actor));
    return Array.from(this.actorCache.values()).filter((actor) =>
      this.shouldRenderActorInCurrentView(actor),
    );
  }

  handleActorDespawn(actorId: number) {
    this.actorCache.delete(actorId);
    if (!this.mapRenderer || !this.sys.isActive()) return;

    console.log(`[TileViewer] Despawning actor ${actorId}`);

    // Remove from local data
    this.actors = this.actors.filter((actor) => actor.id !== actorId);
    this.playerMovementController.removeBlockingActor(actorId);

    // Remove from renderer
    this.mapRenderer.removeActor(actorId);

    // If it was the player, we might need a special check (though usually they just relog)
    if (this.playerActor && this.playerActor.id === actorId) {
      this.playerActor = null;
    }
  }

  handleActorUpdate(event: ActorUpdateEvent) {
    const incomingActor = event.actor;
    const isLocalUpdate = this.isLocalPlayerActor(incomingActor);
    let localPositionResult: "apply" | "confirmed" | "ignore" = "apply";
    let actorForCache = incomingActor;

    if (isLocalUpdate && incomingActor.x != null && incomingActor.y != null) {
      localPositionResult =
        this.playerMovementController.reconcileAuthoritativePosition(
          incomingActor.x,
          incomingActor.y,
          incomingActor.mapId,
          incomingActor.actionDirection,
          incomingActor.movementSeq,
        );
      if (localPositionResult === "ignore") {
        return;
      }
      if (localPositionResult === "confirmed") {
        const predicted = this.playerMovementController.getCurrentPosition();
        actorForCache = {
          ...incomingActor,
          x: predicted.x,
          y: predicted.y,
          mapId: this.playerMovementController.getCurrentMapId(),
        };
      }
    }

    const actor = this.cacheActor(actorForCache);

    // Safety check: ensure the scene is still active and mapRenderer exists
    if (!this.mapRenderer || !this.sys.isActive()) return;
    if (!this.shouldRenderActorInCurrentView(actor)) {
      if (!isLocalUpdate) {
        this.actorCache.delete(actor.id);
        this.actors = this.actors.filter(
          (existingActor) => existingActor.id !== actor.id,
        );
        this.mapRenderer.removeActor(actor.id);
      }
      this.playerMovementController.removeBlockingActor(actor.id);
      return;
    }
    this.playerMovementController.updateBlockingActor(actor);

    // Check regular actors
    const actorIndex = this.actors.findIndex(
      (existingActor) => existingActor.id === actor.id,
    );

    if (actorIndex !== -1) {
      // Store the old position
      const oldX = this.actors[actorIndex].x ?? 0;
      const oldY = this.actors[actorIndex].y ?? 0;
      const oldSpriteName = this.actors[actorIndex].spriteName;

      // Update the actor position and animation properties in our local data.
      this.actors[actorIndex] = actor;
      if (this.isLocalPlayerActor(actor)) {
        this.playerActor = actor;
      }
      if (oldSpriteName !== actor.spriteName) {
        this.mapRenderer.refreshActorSprite(actor);
      }
      this.mapRenderer.getMovementController().updateActorMetadata(actor);

      if (isLocalUpdate && localPositionResult === "confirmed") {
        return;
      }

      // Update the actor sprite in the renderer
      if (oldX !== actor.x || oldY !== actor.y) {
        // Position changed, use position update method to animate the movement
        this.mapRenderer.updateActorPosition(
          actor.id,
          oldX,
          oldY,
          actor.x ?? 0,
          actor.y ?? 0,
          actor.actionDirection,
          actor,
        );
      } else if (actor.actionDirection != null) {
        // Only direction changed
        const controller = this.mapRenderer.getMovementController();
        controller.handleDirectionUpdate(
          actor.id,
          actor.actionDirection,
        );
      } else if (actor.frame != null || actor.flipX != null) {
        // Only animation changed, update the sprite directly
        this.mapRenderer.updateActorAnimation(
          actor.id,
          actor.frame,
          actor.flipX,
        );
      }
    } else {
      // If the actor isn't in our local data, it might be a new actor moving into view
      this.actors.push(actor);
      this.playerMovementController.updateBlockingActor(actor);

      // Preload the actor sprite and then render it
      this.actorManager.preloadActorSprites([actor]).then(() => {
        this.mapRenderer.renderActor(actor);
      });
    }
  }

  handleResize() {
    this.cameraController.handleResize();
    this.uiManager.handleResize();
    this.overlays?.positionSafariHUD();
  }

  update() {
    this.interactionController?.update();
  }

  public getWorldInputFreezeReason(): WorldInputFreezeReason | null {
    return getWorldInputFreezeReason({
      cutsceneInputLocked: this.cutsceneInputLocked,
    });
  }

  public isWorldInputFrozen(): boolean {
    return this.getWorldInputFreezeReason() !== null;
  }

  private setCutsceneInputLocked(locked: boolean): void {
    this.cutsceneInputLocked = locked;
    if (locked) {
      this.cancelWorldInput();
    }
  }

  private cancelWorldInput(): void {
    this.playerMovementController?.stopMovement();
    this.warpManager?.cancelPendingWarp();
  }

  cleanupResources() {
    // Reduced logging
    if (process.env.NODE_ENV === "development") {
      console.log("Cleaning up resources");
    }

    this.interactionController?.cleanup();
    this.warpEvents?.cleanup();
    this.eventBridge?.cleanup();
    this.debugOverlay?.cleanup();
    this.proceduralOverworldPreview?.unregister();
    this.proceduralOverworldPreview?.clear();
    this.trainerEncounterPresenter?.cleanup();
    this.overlays?.cleanup();
    this.cutsceneController?.cleanup();

    // Clean up WebTransport subscriptions
    if (this.actorUpdateUnsubscribe) {
      this.actorUpdateUnsubscribe();
      this.actorUpdateUnsubscribe = null;
    }

    if (this.actorDespawnUnsubscribe) {
      this.actorDespawnUnsubscribe();
      this.actorDespawnUnsubscribe = null;
    }

    if (this.actorsUnsubscribe) {
      this.actorsUnsubscribe();
      this.actorsUnsubscribe = null;
    }

    if (this.gameStatusUnsubscribe) {
      this.gameStatusUnsubscribe();
      this.gameStatusUnsubscribe = null;
    }

    if (this.battleInputFreezeUnsubscribe) {
      this.battleInputFreezeUnsubscribe();
      this.battleInputFreezeUnsubscribe = null;
    }

    if (this.trainerEncounterUnsubscribe) {
      this.trainerEncounterUnsubscribe();
      this.trainerEncounterUnsubscribe = null;
    }

    if (this.surfSuccessHandler) {
      window.removeEventListener("pokeSurfingSuccess", this.surfSuccessHandler);
      this.surfSuccessHandler = null;
    }

    if (this.uiManager) {
      this.uiManager.destroy();
    }

    // Clean up tile editor listeners
    this.cleanupTileEditorListeners();

    // Clear any pending warp state
    if (this.warpManager) {
      this.warpManager.cleanup();
    }

    if (this.playerMovementController) {
      this.playerMovementController.clear();
    }

    // Clean up other resources
    if (this.mapRenderer) {
      try {
        this.mapRenderer.clear();
      } catch (error) {
        console.error("Error clearing map renderer:", error);
      }
    }

    if (this.tileManager) {
      try {
        this.tileManager.clearCache();
      } catch (error) {
        console.error("Error clearing tile manager cache:", error);
      }
    }

    if (this.actorManager) {
      try {
        this.actorManager.clearCache();
      } catch (error) {
        console.error("Error clearing actor manager cache:", error);
      }
    }

    // Clear all data
    this.tiles = [];
    this.items = [];
    this.warps = [];
    this.actors = [];
    this.tileLookup.clear();

    // Remove the map container and all its children
    if (this.mapContainer) {
      try {
        this.mapContainer.removeAll(true);
        this.mapContainer.destroy();
      } catch (error) {
        console.error("Error destroying map container:", error);
      }
    }

    // Remove resize listener
    this.scale.off("resize", this.handleResize, this);

    // Remove all event listeners
    this.input.off("pointermove");
    this.events.off("warpClicked");
  }

  resetScene(resetCamera: boolean = true) {
    // First, store any data we need to pass to the new scene
    const data = {
      destinationMapId: this.game.registry.get("destinationMapId"),
      useOverworldSavedCamera: this.game.registry.get(
        "useOverworldSavedCamera",
      ),
      destinationX: this.game.registry.get("destinationX"),
      destinationY: this.game.registry.get("destinationY"),
    };

    // Only reset the camera if explicitly requested AND we're not trying to preserve state
    const preservingCameraState =
      data.useOverworldSavedCamera === true;

    if (resetCamera && !preservingCameraState && this.cameraController) {
      this.cameraController.resetCamera();
    }

    // Clean up resources
    this.cleanupResources();

    // Use a simpler approach - just restart the scene once
    this.scene.restart(data);
  }
}
