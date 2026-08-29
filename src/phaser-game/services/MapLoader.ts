import { Scene } from "phaser";
import {
  DEFAULT_ZOOM,
  TILE_SIZE,
  UNIFIED_OVERWORLD_MAP_ID,
} from "../constants";
import { CameraController } from "../controllers/CameraController";
import { PlayerMovementController } from "../controllers/PlayerMovementController";
import { MapRenderer, type MapItem } from "../renderers/MapRenderer";
import { OverworldOverviewLayer } from "../renderers/OverworldOverviewLayer";
import { MapDataService } from "./MapDataService";
import {
  OverworldChunkStream,
  type CommittedOverworldTileUpdate,
} from "./OverworldChunkStream";
import {
  OVERWORLD_CHUNK_SIZE_TILES,
  type CameraWorldView,
  type InclusiveTileBounds,
} from "./OverworldChunkPlanner";
import { preferOverworldOverviewAtZoom } from "./OverworldLodPolicy";
import { TileManager, ActorManager, UiManager } from "../managers";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import * as PhaserNet from "./PhaserNetworkService";
import type {
  PhaserMapInfo,
  PhaserTile,
  PhaserActor,
  PhaserWarp,
} from "@/net/generated/world_api";
import { RuntimeAssetReloadRequestedError } from "./RuntimeAssetCompatibility";

export interface MapLoaderState {
  tiles: PhaserTile[];
  items: MapItem[];
  warps: PhaserWarp[];
  actors: PhaserActor[];
  mapInfo: PhaserMapInfo | null;
  isOverworldMode: boolean;
  isMapOverviewMode: boolean;
  viewedMapIds: Set<number>;
}

interface MapLoadOptions {
  viewOnly?: boolean;
}

const OVERWORLD_STREAM_UPDATE_INTERVAL_MS = 100;

function numberOrNull(value: unknown): number | null {
  if (value === null || value === undefined) return null;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : null;
}

export class MapLoader {
  private scene: Scene;
  private mapDataService: MapDataService;
  private tileManager: TileManager;
  private actorManager: ActorManager;
  private mapRenderer: MapRenderer;
  private cameraController: CameraController;
  private playerMovementController: PlayerMovementController;
  private uiManager: UiManager;

  // Callbacks to update TileViewer state
  private setState: (partial: Partial<MapLoaderState>) => void;
  private getState: () => MapLoaderState;
  private getPlayerActor: () => PhaserActor | null;
  private updateCameraFollow: () => void;
  private removeMapLegend: () => void;
  private createMapLegend: (maps: PhaserMapInfo[]) => void;
  private prepareActorsForLoadedView: (actors: PhaserActor[]) => PhaserActor[];
  private mapLoadGeneration = 0;
  private overworldChunkStream: OverworldChunkStream | null = null;
  private readonly overworldOverviewLayer: OverworldOverviewLayer;
  private lastOverworldStreamUpdateAt = 0;
  private preferOverworldOverviewForZoom = false;

  constructor(
    scene: Scene,
    mapDataService: MapDataService,
    tileManager: TileManager,
    actorManager: ActorManager,
    mapRenderer: MapRenderer,
    mapContainer: Phaser.GameObjects.Container,
    cameraController: CameraController,
    playerMovementController: PlayerMovementController,
    uiManager: UiManager,
    callbacks: {
      getState: () => MapLoaderState;
      setState: (partial: Partial<MapLoaderState>) => void;
      getPlayerActor: () => PhaserActor | null;
      updateCameraFollow: () => void;
      removeMapLegend: () => void;
      createMapLegend: (maps: PhaserMapInfo[]) => void;
      prepareActorsForLoadedView: (actors: PhaserActor[]) => PhaserActor[];
    },
  ) {
    this.scene = scene;
    this.mapDataService = mapDataService;
    this.tileManager = tileManager;
    this.actorManager = actorManager;
    this.mapRenderer = mapRenderer;
    this.overworldOverviewLayer = new OverworldOverviewLayer(
      scene,
      mapContainer,
      OVERWORLD_CHUNK_SIZE_TILES,
    );
    this.cameraController = cameraController;
    this.playerMovementController = playerMovementController;
    this.uiManager = uiManager;
    this.getState = callbacks.getState;
    this.setState = callbacks.setState;
    this.getPlayerActor = callbacks.getPlayerActor;
    this.updateCameraFollow = callbacks.updateCameraFollow;
    this.removeMapLegend = callbacks.removeMapLegend;
    this.createMapLegend = callbacks.createMapLegend;
    this.prepareActorsForLoadedView = callbacks.prepareActorsForLoadedView;
  }

  async loadMapData(mapId: number) {
    const loadGeneration = ++this.mapLoadGeneration;
    this.stopOverworldStreaming();
    (this.scene as any).mapLoadInProgress = true; // eslint-disable-line @typescript-eslint/no-explicit-any
    try {
      // Check if network is ready
      if (!this.mapDataService.isReady()) {
        this.uiManager.setLoadingText(
          "Waiting for server connection...\nPlease log in to continue.",
        );
        console.log("[MapLoader] Waiting for WebTransport connection...");
        return;
      }
      await this.mapDataService.ensureRuntimeTileCatalogCurrent(true);
      if (!this.isLoadCurrent(loadGeneration)) return;

      // Show loading text
      this.uiManager.setLoadingText(`Loading map data for map ${mapId}...`);

      // Always clear existing data
      this.mapRenderer.clearLocalActorPositionOverrides();
      this.mapRenderer.clear();
      const localPlayer = this.getPlayerActor();
      const playerActors = localPlayer ? [localPlayer] : [];

      // Remove any map legend (which is only present in overworld view)
      this.removeMapLegend();

      // Update state
      const viewedMapIds = new Set<number>();
      viewedMapIds.add(mapId);
      this.setState({
        tiles: [],
        items: [],
        warps: [],
        actors: playerActors,
        isOverworldMode: false,
        isMapOverviewMode: false,
        viewedMapIds,
      });

      // Set camera to non-overworld mode - this will save the overworld camera state if needed
      this.cameraController.setViewMode(false);

      // Fetch map info — pass warp destination coordinates so the server
      // atomically updates the player position before we request actors.
      const sceneForDest = this.scene as any; // eslint-disable-line @typescript-eslint/no-explicit-any
      const warpDX = sceneForDest.warpDestX ?? undefined;
      const warpDY = sceneForDest.warpDestY ?? undefined;
      const cached = this.mapDataService.getSnapshot(mapId);
      const hasWarpDestination = warpDX !== undefined && warpDY !== undefined;
      const mapInfo = hasWarpDestination || !cached
        ? await this.mapDataService.fetchMapInfo(mapId, warpDX, warpDY)
        : cached.mapInfo;
      if (!this.isLoadCurrent(loadGeneration)) return;

      if (!mapInfo) {
        throw new Error(`Map ${mapId} not found`);
      }

      this.setState({ mapInfo });
      void useGameStatusStore.getState().setCurrentMap(mapId);

      this.uiManager.setLoadingText("Loading tiles...");

      // Fetch tiles
      const tiles = cached?.tiles ?? await this.mapDataService.fetchTiles(mapId);
      if (!this.isLoadCurrent(loadGeneration)) return;

      this.uiManager.setLoadingText("Loading tile images...");

      // Fetch tile images
      const tileImagesData = await this.mapDataService.fetchTileImages();
      if (!this.isLoadCurrent(loadGeneration)) return;

      // Load tile images
      await this.tileManager.loadTileImages(tileImagesData);
      if (!this.isLoadCurrent(loadGeneration)) return;

      this.uiManager.setLoadingText("Loading items...");

      let items: any[] = []; // eslint-disable-line @typescript-eslint/no-explicit-any
      try {
        // Fetch items
        const allItems = await this.mapDataService.fetchItems();
        if (!this.isLoadCurrent(loadGeneration)) return;

        // Filter items for this map
        if (Array.isArray(allItems)) {
          items = allItems.filter((item: any) => item.mapId === mapId); // eslint-disable-line @typescript-eslint/no-explicit-any
        }
      } catch (itemError) {
        console.error("Error loading items:", itemError);
      }
      if (!this.isLoadCurrent(loadGeneration)) return;

      this.uiManager.setLoadingText("Loading warps...");

      let warps: PhaserWarp[] = [];
      try {
        // Fetch warps for this map
        warps = cached?.warps ?? await this.mapDataService.fetchWarps(mapId);
      } catch (warpError) {
        console.error("Error loading warps:", warpError);
      }
      if (!this.isLoadCurrent(loadGeneration)) return;

      this.uiManager.setLoadingText("Loading Actors...");

      let actors = [...playerActors];
      try {
        // Fetch actors for this map (includes static and walking)
        const allActors = await this.mapDataService.fetchActors(mapId);
        if (!this.isLoadCurrent(loadGeneration)) return;
        // Initialize with default if null, but preserve existing actors (like player)
        if (!allActors) {
          actors = this.prepareActorsForLoadedView(actors);
          this.setState({ tiles, items, warps, actors });
          return;
        }

        // Preload actor sprites before rendering
        if (allActors.length > 0) {
          this.uiManager.setLoadingText(
            `Preloading ${allActors.length} actor sprites...`,
          );
          await this.actorManager.preloadActorSprites(allActors);
          if (!this.isLoadCurrent(loadGeneration)) return;
        }

        // Add to our actor list (preserving players)
        allActors.forEach((actor) => {
          if (!actors.find((a) => a.id === actor.id)) {
            actors.push(actor);
          }
        });
      } catch (actorError) {
        console.error("Error loading actors:", actorError);
        actors = [];
      }
      if (!this.isLoadCurrent(loadGeneration)) return;

      // Apply warp destination coordinates to the player actor before rendering.
      // The scene stores warpDestX/warpDestY directly so we don't depend on
      // timing of the onActors broadcast or warpManager pending state.
      const sceneAny = this.scene as any; // eslint-disable-line @typescript-eslint/no-explicit-any
      if (sceneAny.warpDestX !== null && sceneAny.warpDestX !== undefined &&
          sceneAny.warpDestY !== null && sceneAny.warpDestY !== undefined) {
        const warpX = sceneAny.warpDestX;
        const warpY = sceneAny.warpDestY;
        const renderX = numberOrNull(sceneAny.warpAnimationStartX) ?? warpX;
        const renderY = numberOrNull(sceneAny.warpAnimationStartY) ?? warpY;
        const playerInList = actors.find((a) => a.objectType === "player");
        if (playerInList) {
          console.log(`[MapLoader] Overriding player position with warp destination: (${renderX}, ${renderY}) (was ${playerInList.x}, ${playerInList.y})`);
          playerInList.x = renderX;
          playerInList.y = renderY;
        }
        // Also update the scene's local playerActor reference
        if (sceneAny.playerActor) {
          sceneAny.playerActor.x = renderX;
          sceneAny.playerActor.y = renderY;
        }
        // Clear after applying
        sceneAny.warpDestX = null;
        sceneAny.warpDestY = null;
      }

      actors = this.prepareActorsForLoadedView(actors);

      this.mapDataService.setSnapshot(mapId, {
        mapInfo,
        tiles,
        warps,
        actors: actors.filter((actor) => actor.objectType !== "player"),
      });

      // Update state with all loaded data
      this.setState({ tiles, items, warps, actors });

      // Wait a short time for updates to arrive
      await new Promise((resolve) => setTimeout(resolve, 300));
      if (!this.isLoadCurrent(loadGeneration)) return;

      // Render the map with all actors
      const mapBounds = this.mapRenderer.renderMap(tiles, items, warps, actors);

      // Update collision map for movement
      this.playerMovementController.buildCollisionMap(tiles);
      this.playerMovementController.setBlockingActors(actors);

      const playerActor = this.getPlayerActor();
      if (playerActor) {
        this.playerMovementController.setPlayer(
          playerActor.id,
          playerActor.x ?? 0,
          playerActor.y ?? 0,
          playerActor.mapId,
          this.mapRenderer,
        );
      }

      // Center camera and start following the player if enabled
      if (useGameStatusStore.getState().isCameraFollowEnabled && playerActor) {
        this.updateCameraFollow();
      } else if (mapBounds.centerX !== undefined && mapBounds.centerY !== undefined) {
        // Fall back to centering on the map if follow is disabled or no player
        this.cameraController.centerOnMap(mapBounds.centerX, mapBounds.centerY);
      }

      // Apply preserved facing direction from warp
      this.applyDestinationDirection();

      // Remove any map legend
      this.removeMapLegend();

      // Update mode text to show we're viewing a specific map
      if (mapInfo && mapInfo.name) {
        this.uiManager.setModeText(`Map View: ${mapInfo.name}`);
      }

      // Fade in from black (matches the fade-out in WarpManager)
      this.scene.cameras.main.fadeIn(200, 0, 0, 0);

      // Hide loading text
      this.uiManager.hideLoadingText();
      await (this.scene as any).playPendingWarpExitAnimation?.(200); // eslint-disable-line @typescript-eslint/no-explicit-any
      if (!this.isLoadCurrent(loadGeneration)) return;
      if (mapInfo.name) {
        PhaserNet.requestMapScripts(mapInfo.name);
      }
    } catch (error: unknown) {
      if (!this.isLoadCurrent(loadGeneration)) return;
      if (error instanceof RuntimeAssetReloadRequestedError) {
        this.uiManager.setLoadingText("Updating game assets...");
        return;
      }
      console.error("Error loading map data:", error);
      this.uiManager.setLoadingText(
        `Error loading map data: ${
          error instanceof Error ? error.message : "Unknown error"
        }. Check console for details.`,
      );
    } finally {
      if (this.isLoadCurrent(loadGeneration)) {
        (this.scene as any).mapLoadInProgress = false; // eslint-disable-line @typescript-eslint/no-explicit-any
      }
    }
  }

  async loadOverworldData(options: MapLoadOptions = {}) {
    const loadGeneration = ++this.mapLoadGeneration;
    let readyForWorldInput = false;
    this.stopOverworldStreaming();
    (this.scene as any).mapLoadInProgress = true; // eslint-disable-line @typescript-eslint/no-explicit-any
    try {
      // Check if network is ready
      if (!this.mapDataService.isReady()) {
        this.uiManager.setLoadingText(
          "Waiting for server connection...\nPlease log in to continue.",
        );
        console.log("[MapLoader] Waiting for WebTransport connection...");
        return;
      }
      await this.mapDataService.ensureRuntimeTileCatalogCurrent(true);
      if (!this.isLoadCurrent(loadGeneration)) return;

      // Show loading text
      this.uiManager.setLoadingText("Loading overworld data...");

      // Always clear existing data
      this.mapRenderer.clearLocalActorPositionOverrides();
      this.mapRenderer.clear();
      const localPlayer = this.getPlayerActor();
      const playerActors = options.viewOnly ? [] : localPlayer ? [localPlayer] : [];

      // Map ID for the unified overworld
      const mapId = UNIFIED_OVERWORLD_MAP_ID;

      // Update state
      const viewedMapIds = new Set<number>();
      viewedMapIds.add(mapId);
      this.setState({
        tiles: [],
        items: [],
        warps: [],
        actors: playerActors,
        isOverworldMode: true,
        isMapOverviewMode: options.viewOnly === true,
        viewedMapIds,
      });

      // Set camera to overworld mode
      this.cameraController.setViewMode(true);

      this.uiManager.setLoadingText("Loading unified overworld...");

      // Fetch unified map info. Pass warp destination coordinates so the server
      // atomically updates the player position before we request actors.
      const sceneForDest = this.scene as any; // eslint-disable-line @typescript-eslint/no-explicit-any
      const warpDX = sceneForDest.warpDestX ?? undefined;
      const warpDY = sceneForDest.warpDestY ?? undefined;
      const cached = this.mapDataService.getSnapshot(mapId);
      const hasWarpDestination = warpDX !== undefined && warpDY !== undefined;
      const mapInfo = hasWarpDestination || !cached
        ? await this.mapDataService.fetchMapInfo(mapId, warpDX, warpDY)
        : cached.mapInfo;
      if (!this.isLoadCurrent(loadGeneration)) return;
      this.setState({ mapInfo });
      if (!options.viewOnly) {
        void useGameStatusStore.getState().setCurrentMap(mapId);
      }

      // Exact tiles are loaded after camera placement from aligned chunks.
      // Keeping this base render tile-free avoids allocating one texture for
      // the complete overworld before the camera-derived stream is ready.
      const character = usePlayerCharacterStore.getState().characterProfile;
      const centerX = Math.round(numberOrNull(warpDX) ?? numberOrNull(character.x) ?? 0);
      const centerY = Math.round(numberOrNull(warpDY) ?? numberOrNull(character.y) ?? 0);
      const tiles: PhaserTile[] = [];

      // Items are no longer fetched separately - they come as part of NPCs/objects
      const items: any[] = []; // eslint-disable-line @typescript-eslint/no-explicit-any

      this.uiManager.setLoadingText("Loading actors and warps...");

      // Load actors and warps for the unified map
      const allActors = options.viewOnly && cached
        ? cached.actors
        : await this.mapDataService.fetchActors(mapId);
      if (!this.isLoadCurrent(loadGeneration)) return;
      const warps = cached?.warps ?? await this.mapDataService.fetchWarps(mapId);
      if (!this.isLoadCurrent(loadGeneration)) return;

      // Preload actor sprites before rendering
      if (allActors.length > 0) {
        this.uiManager.setLoadingText(
          `Preloading ${allActors.length} actor sprites...`,
        );
        await this.actorManager.preloadActorSprites(allActors);
        if (!this.isLoadCurrent(loadGeneration)) return;
      }

      // Add to our actor list (preserving players)
      let actors = [...playerActors];
      allActors.forEach((actor) => {
        if (!actors.find((a) => a.id === actor.id)) {
          actors.push(actor);
        }
      });

      // Apply warp destination coordinates to the player actor (same as loadMapData)
      const sceneAnyOW = this.scene as any; // eslint-disable-line @typescript-eslint/no-explicit-any
      if (sceneAnyOW.warpDestX !== null && sceneAnyOW.warpDestX !== undefined &&
          sceneAnyOW.warpDestY !== null && sceneAnyOW.warpDestY !== undefined) {
        const warpX = sceneAnyOW.warpDestX;
        const warpY = sceneAnyOW.warpDestY;
        const renderX = numberOrNull(sceneAnyOW.warpAnimationStartX) ?? warpX;
        const renderY = numberOrNull(sceneAnyOW.warpAnimationStartY) ?? warpY;
        const pl = actors.find((a) => a.objectType === "player");
        if (pl) {
          console.log(`[MapLoader] Overriding player position with warp destination (overworld): (${renderX}, ${renderY}) (was ${pl.x}, ${pl.y})`);
          pl.x = renderX;
          pl.y = renderY;
          pl.mapId = UNIFIED_OVERWORLD_MAP_ID;
        }
        if (sceneAnyOW.playerActor) {
          sceneAnyOW.playerActor.x = renderX;
          sceneAnyOW.playerActor.y = renderY;
          sceneAnyOW.playerActor.mapId = UNIFIED_OVERWORLD_MAP_ID;
        }
        sceneAnyOW.warpDestX = null;
        sceneAnyOW.warpDestY = null;
      }

      actors = this.prepareActorsForLoadedView(actors);

      this.mapDataService.setSnapshot(mapId, {
        mapInfo,
        tiles: [],
        warps,
        actors: actors.filter((actor) => actor.objectType !== "player"),
      });

      // Update state
      this.setState({ tiles, items, warps, actors });

      // Render the map with all actors
      const explicitTileBounds =
        mapInfo.tileMinX !== undefined && mapInfo.tileMinY !== undefined &&
        mapInfo.tileMaxX !== undefined && mapInfo.tileMaxY !== undefined
          ? {
              minX: mapInfo.tileMinX,
              minY: mapInfo.tileMinY,
              maxX: mapInfo.tileMaxX,
              maxY: mapInfo.tileMaxY,
            }
          : undefined;
      const mapBounds = this.mapRenderer.renderMap(
        tiles,
        items,
        warps,
        actors,
        explicitTileBounds,
      );

      // Update collision map for movement
      if (!options.viewOnly) {
        this.playerMovementController.buildCollisionMap(tiles);
        this.playerMovementController.setBlockingActors(actors);
      }

      const playerActor = this.getPlayerActor();
      if (!options.viewOnly && playerActor) {
        this.playerMovementController.setPlayer(
          playerActor.id,
          playerActor.x ?? 0,
          playerActor.y ?? 0,
          playerActor.mapId,
          this.mapRenderer,
        );
      }

      // Check if we're explicitly told to use the saved camera
      const useSavedCamera =
        this.scene.game.registry.get("useOverworldSavedCamera") === true;

      // Get any specific destination map ID
      const destinationMapIdStr =
        this.scene.game.registry.get("destinationMapId");

      // Get the current camera state from registry
      const savedCameraState = this.scene.game.registry.get(
        "overworldCameraState",
      );

      // Restore saved camera position only when explicitly requested by map-view UI.
      // Normal gameplay loads should fall back to player follow/default zoom.
      let restored = false;
      if (options.viewOnly && savedCameraState?.saved) {
        restored = this.cameraController.restoreOverworldCameraState();
      } else if (useSavedCamera) {
        // Explicitly requested restoration (e.g. from "Back to Overworld" button)
        if (savedCameraState && savedCameraState.saved) {
          restored = this.cameraController.restoreOverworldCameraState();
        }
        this.scene.game.registry.remove("useOverworldSavedCamera");
      } else if (savedCameraState && savedCameraState.saved) {
        this.cameraController.clearOverworldCameraState();
      }

      // Check if we have a specific destination map to center on
      if (destinationMapIdStr && !restored) {
        const destMapId = parseInt(destinationMapIdStr.toString(), 10);
        console.log(
          `Centering on destination map ${destMapId} at (${centerX}, ${centerY})`,
        );
        this.cameraController.centerOnMap(
          centerX * TILE_SIZE + TILE_SIZE / 2,
          centerY * TILE_SIZE + TILE_SIZE / 2,
        );
        this.cameraController.setZoom(1.0);
        restored = true;
      }

      // Only center the camera on the entire overworld if we couldn't restore or center on a specific map
      if (
        !restored &&
        mapBounds.centerX !== undefined &&
        mapBounds.centerY !== undefined
      ) {
        this.cameraController.centerOnMap(mapBounds.centerX, mapBounds.centerY);
        // Set default zoom since we're centering
        this.cameraController.setZoom(DEFAULT_ZOOM);
      }

      // Create a legend for the maps
      this.createMapLegend([mapInfo]);

      // Apply preserved facing direction from warp
      if (!options.viewOnly) {
        this.applyDestinationDirection();
      }

      // Update mode text
      this.uiManager.setModeText("Overworld View");

      // Re-apply camera follow if enabled (overrides the zoom/center settings above)
      if (!options.viewOnly && useGameStatusStore.getState().isCameraFollowEnabled && playerActor) {
        console.log(`[MapLoader] Re-applying camera follow after map load`);
        this.updateCameraFollow();
      }

      const tileBounds = this.overworldBoundsFromMapInfo(mapInfo);
      if (!tileBounds) {
        throw new Error("Unified overworld map info did not include tile bounds");
      }
      this.overworldChunkStream = new OverworldChunkStream({
        mapBounds: tileBounds,
        viewOnly: options.viewOnly === true,
        mapDataService: this.mapDataService,
        tileManager: this.tileManager,
        mapRenderer: this.mapRenderer,
        movementController: this.playerMovementController,
        overviewLayer: this.overworldOverviewLayer,
        onTilesChanged: (streamedTiles) => {
          if (
            loadGeneration === this.mapLoadGeneration &&
            this.getState().mapInfo?.id === UNIFIED_OVERWORLD_MAP_ID
          ) {
            this.setState({ tiles: streamedTiles });
          }
        },
      });
      this.uiManager.setLoadingText("Loading nearby overworld chunks...");
      const initialCamera = this.currentCameraWorldView();
      await this.overworldChunkStream.initialize(
        initialCamera,
        this.updateOverworldZoomPreference(),
      );
      if (loadGeneration !== this.mapLoadGeneration) return;

      // Fade in from black (matches the fade-out in WarpManager)
      this.scene.cameras.main.fadeIn(200, 0, 0, 0);

      // Hide loading text
      this.uiManager.hideLoadingText();
      await (this.scene as any).playPendingWarpExitAnimation?.(200); // eslint-disable-line @typescript-eslint/no-explicit-any
      if (!this.isLoadCurrent(loadGeneration)) return;
      readyForWorldInput = true;
    } catch (error: unknown) {
      if (!this.isLoadCurrent(loadGeneration)) return;
      if (error instanceof RuntimeAssetReloadRequestedError) {
        this.uiManager.setLoadingText("Updating game assets...");
        return;
      }
      // Initialization may already have installed actors and an empty base
      // collision map. Tear down every partial exact/overview layer and leave
      // world input frozen so the player cannot walk through an unverified map.
      this.stopOverworldStreaming();
      if (!options.viewOnly) {
        this.playerMovementController.buildCollisionMap([]);
      }
      this.setState({ tiles: [] });
      console.error("Error loading overworld data:", error);
      this.uiManager.setLoadingText(
        `Error loading overworld data: ${
          error instanceof Error ? error.message : "Unknown error"
        }. Check console for details.`,
      );
    } finally {
      if (this.isLoadCurrent(loadGeneration) && readyForWorldInput) {
        (this.scene as any).mapLoadInProgress = false; // eslint-disable-line @typescript-eslint/no-explicit-any
      }
    }
  }

  updateOverworldStreaming(): void {
    if (
      (this.scene as any).mapLoadInProgress || // eslint-disable-line @typescript-eslint/no-explicit-any
      !this.overworldChunkStream ||
      this.getState().mapInfo?.id !== UNIFIED_OVERWORLD_MAP_ID
    ) {
      return;
    }
    const now = this.scene.time.now;
    if (
      now - this.lastOverworldStreamUpdateAt <
      OVERWORLD_STREAM_UPDATE_INTERVAL_MS
    ) {
      return;
    }
    this.lastOverworldStreamUpdateAt = now;
    this.overworldChunkStream.update(
      this.currentCameraWorldView(),
      this.updateOverworldZoomPreference(),
    );
  }

  ensureOverworldTileAvailable(x: number, y: number): Promise<boolean> {
    return this.overworldChunkStream?.ensureTileAvailable(x, y) ??
      Promise.resolve(false);
  }

  invalidateOverworldTileAt(x: number, y: number): void {
    this.overworldChunkStream?.invalidateAt(x, y);
  }

  applyCommittedOverworldTileUpdates(
    updates: readonly CommittedOverworldTileUpdate[],
  ): void {
    this.overworldChunkStream?.applyCommittedTileUpdates(updates);
  }

  invalidateOverworldOverviewCatalog(): void {
    this.overworldChunkStream?.invalidateOverviewCatalog();
  }

  getOverworldChunkStatus() {
    return this.overworldChunkStream?.getStatus() ?? null;
  }

  cleanup(): void {
    this.mapLoadGeneration += 1;
    this.stopOverworldStreaming();
    (this.scene as any).mapLoadInProgress = false; // eslint-disable-line @typescript-eslint/no-explicit-any
  }

  private isLoadCurrent(loadGeneration: number): boolean {
    return loadGeneration === this.mapLoadGeneration;
  }

  private stopOverworldStreaming(): void {
    this.overworldChunkStream?.stop();
    this.overworldChunkStream = null;
    this.overworldOverviewLayer.clear();
    this.lastOverworldStreamUpdateAt = 0;
    this.preferOverworldOverviewForZoom = false;
  }

  private currentCameraWorldView(): CameraWorldView {
    const camera = this.scene.cameras.main;
    const topLeft = camera.getWorldPoint(0, 0);
    const bottomRight = camera.getWorldPoint(camera.width, camera.height);
    return {
      x: topLeft.x,
      y: topLeft.y,
      width: Math.max(1, bottomRight.x - topLeft.x),
      height: Math.max(1, bottomRight.y - topLeft.y),
    };
  }

  private updateOverworldZoomPreference(): boolean {
    this.preferOverworldOverviewForZoom = preferOverworldOverviewAtZoom(
      this.preferOverworldOverviewForZoom,
      this.cameraController.getZoom(),
    );
    return this.preferOverworldOverviewForZoom;
  }

  private overworldBoundsFromMapInfo(
    mapInfo: PhaserMapInfo,
  ): InclusiveTileBounds | null {
    if (
      mapInfo.tileMinX === undefined ||
      mapInfo.tileMinY === undefined ||
      mapInfo.tileMaxX === undefined ||
      mapInfo.tileMaxY === undefined
    ) {
      return null;
    }
    return {
      minX: mapInfo.tileMinX,
      minY: mapInfo.tileMinY,
      maxX: mapInfo.tileMaxX,
      maxY: mapInfo.tileMaxY,
    };
  }

  /**
   * Apply the preserved facing direction from a warp transition.
   * Reads destinationDirection from the registry and updates the player actor's
   * sprite direction so they face the same way they were facing when entering.
   */
  private applyDestinationDirection() {
    const direction = this.scene.game.registry.get("destinationDirection");
    if (!direction) return;

    this.scene.game.registry.remove("destinationDirection");

    const playerActor = this.getPlayerActor();
    if (!playerActor) return;

    // Update the actor data
    playerActor.actionDirection = direction;

    // Update the actor movement controller's sprite direction
    const movementController = this.mapRenderer.getMovementController();
    if (movementController) {
      movementController.handleDirectionUpdate(playerActor.id, direction);
    }

    // Also sync the player movement controller's internal direction
    this.playerMovementController.syncDirection(direction);

    console.log(
      `[MapLoader] Applied preserved facing direction: ${direction}`,
    );
  }
}
