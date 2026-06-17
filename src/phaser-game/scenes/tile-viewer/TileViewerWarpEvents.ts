import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import useGameStatusStore from "@/stores/GameStatusStore";
import { UNIFIED_OVERWORLD_MAP_ID } from "../../constants";
import { PlayerMovementController } from "../../controllers/PlayerMovementController";
import { MapRenderer } from "../../renderers/MapRenderer";
import { MapDataService } from "../../services/MapDataService";
import * as PhaserNet from "../../services/PhaserNetworkService";

interface TileViewerWarpEventsDeps {
  scene: Scene;
  mapDataService: MapDataService;
  mapRenderer: () => MapRenderer;
  playerMovementController: () => PlayerMovementController;
  getPlayerActor: () => PhaserActor | null;
  setPlayerActor: (actor: PhaserActor) => void;
  resetScene: (resetCamera: boolean) => void;
}

export class TileViewerWarpEvents {
  private blackoutWarpUnsubscribe: (() => void) | null = null;
  private warpTileTeleportHandler: ((e: Event) => void) | null = null;

  constructor(private readonly deps: TileViewerWarpEventsDeps) {}

  register(): void {
    this.blackoutWarpUnsubscribe = useGameStatusStore.subscribe(
      (state) => state.pendingBlackoutWarp,
      (pending) => {
        if (!pending) return;

        useGameStatusStore.getState().clearBlackoutWarp();
        const destMapId = pending.mapId;
        const destX = pending.x;
        const destY = pending.y;
        console.log(
          `[Blackout] Warping to Pokémon Center (map ${destMapId}, ${destX}, ${destY})`,
        );
        PhaserNet.sendPlayerPosition(destX, destY, destMapId);

        const playerActor = this.deps.getPlayerActor();
        if (playerActor) {
          playerActor.x = destX;
          playerActor.y = destY;
          playerActor.mapId = destMapId;
          this.deps.setPlayerActor(playerActor);
        }

        const movement = this.deps.playerMovementController();
        movement.stopMovement();
        movement.syncPosition(destX, destY);
        this.deps.scene.game.registry.set("destinationMapId", destMapId);
        this.deps.scene.game.registry.set("destinationX", destX);
        this.deps.scene.game.registry.set("destinationY", destY);
        this.deps.scene.game.registry.set("useOverworldSavedCamera", false);
        this.deps.resetScene(false);
      },
    );

    this.warpTileTeleportHandler = (event: Event) => {
      void this.handleWarpTileTeleport(event as CustomEvent<{
        mapId: number;
        x: number;
        y: number;
        direction?: string;
      }>);
    };
    window.addEventListener("warpTileTeleport", this.warpTileTeleportHandler);
  }

  cleanup(): void {
    if (this.blackoutWarpUnsubscribe) {
      this.blackoutWarpUnsubscribe();
      this.blackoutWarpUnsubscribe = null;
    }

    if (this.warpTileTeleportHandler) {
      window.removeEventListener("warpTileTeleport", this.warpTileTeleportHandler);
      this.warpTileTeleportHandler = null;
    }
  }

  private async handleWarpTileTeleport(
    event: CustomEvent<{
      mapId: number;
      x: number;
      y: number;
      direction?: string;
    }>,
  ): Promise<void> {
    const { mapId, x, y, direction } = event.detail;
    console.log(`[WarpTile] Teleporting to map ${mapId} (${x}, ${y})`);

    const normalizedPlayerMapId = this.deps.mapDataService.isOverworld(mapId)
      ? UNIFIED_OVERWORLD_MAP_ID
      : mapId;
    const playerActor = this.deps.getPlayerActor();
    if (playerActor?.id != null) {
      await Promise.race([
        this.deps.mapRenderer().waitForActorIdle(playerActor.id),
        new Promise<void>((resolve) => setTimeout(resolve, 1200)),
      ]);
      if (!this.deps.scene.sys.isActive()) return;
    }

    const movement = this.deps.playerMovementController();
    movement.stopMovement();
    movement.syncMapId(normalizedPlayerMapId);

    if (playerActor) {
      playerActor.x = x;
      playerActor.y = y;
      playerActor.mapId = normalizedPlayerMapId;
      this.deps.setPlayerActor(playerActor);
    }

    const currentMapId = this.deps.scene.game.registry.get("currentMapId");
    if (mapId !== currentMapId) {
      this.deps.scene.game.registry.set("destinationMapId", mapId);
      this.deps.scene.game.registry.set("destinationX", x);
      this.deps.scene.game.registry.set("destinationY", y);
      if (direction) {
        this.deps.scene.game.registry.set("destinationDirection", direction);
      }
      this.deps.scene.game.registry.set("useOverworldSavedCamera", false);
      this.deps.resetScene(false);
      return;
    }

    movement.syncPosition(x, y);
    if (direction) {
      movement.syncDirection(direction);
    }
    if (playerActor?.id != null) {
      this.deps.mapRenderer().snapActorPosition(
        playerActor.id,
        x,
        y,
        direction ?? playerActor.actionDirection ?? "DOWN",
        playerActor,
      );
    }
  }
}
