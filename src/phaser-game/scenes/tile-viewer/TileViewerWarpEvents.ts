import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import useGameStatusStore from "@/stores/GameStatusStore";
import { UNIFIED_OVERWORLD_MAP_ID } from "../../constants";
import { PlayerMovementController } from "../../controllers/PlayerMovementController";
import { MapRenderer } from "../../renderers/MapRenderer";
import { MapDataService } from "../../services/MapDataService";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

interface WarpTileTeleportDetail {
  mapId: number;
  x: number;
  y: number;
  direction?: string;
  animateExitStep?: boolean;
  animationStartX?: number;
  animationStartY?: number;
  sfxAlreadyPlayed?: boolean;
  serverCommitted: true;
}

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
        void this.handleWarpTileTeleport(new CustomEvent("warpTileTeleport", {
          detail: { ...pending, direction: "DOWN", serverCommitted: true },
        }));
      },
    );

    this.warpTileTeleportHandler = (event: Event) => {
      void this.handleWarpTileTeleport(
        event as CustomEvent<WarpTileTeleportDetail>,
      );
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

  async reconcileOwnedPosition(position: { mapId: number; x: number; y: number; direction: string; serverMovementPending: boolean }): Promise<void> {
    const movement = this.deps.playerMovementController();
    // A view registry is not the owned movement map. Correct the existing actor
    // in place so an ordinary result cannot reload/retrigger a map script.
    if (movement.getCurrentMapId() === position.mapId) {
      movement.stopMovement(true);
      movement.syncPosition(position.x, position.y);
      movement.syncDirection(position.direction);
      const player = this.deps.getPlayerActor();
      if (player) {
        player.x = position.x;
        player.y = position.y;
        player.mapId = position.mapId;
        player.actionDirection = position.direction;
        this.deps.setPlayerActor(player);
        this.deps.mapRenderer().snapActorPosition(player.id, position.x, position.y, position.direction, player);
      }
      if (position.serverMovementPending) movement.beginServerMovement(false);
      return;
    }
    await this.handleWarpTileTeleport(new CustomEvent("warpTileTeleport", {
      detail: { ...position, serverCommitted: true, sfxAlreadyPlayed: true },
    }));
  }

  private async handleWarpTileTeleport(
    event: CustomEvent<WarpTileTeleportDetail>,
  ): Promise<void> {
    // This local event projects an accepted server result; it cannot establish
    // a destination. Reject stale callers before sound, movement or scene changes.
    if (event.detail?.serverCommitted !== true) {
      console.error("[WarpTile] Ignored teleport without a committed server result");
      return;
    }
    const {
      mapId,
      x,
      y,
      direction,
      animateExitStep,
      animationStartX,
      animationStartY,
      sfxAlreadyPlayed,
    } = event.detail;
    console.log(`[WarpTile] Teleporting to map ${mapId} (${x}, ${y})`);
    const normalizedPlayerMapId = this.deps.mapDataService.isOverworld(mapId)
      ? UNIFIED_OVERWORLD_MAP_ID
      : mapId;
    const currentMapId = this.deps.scene.game.registry.get("currentMapId");
    if (!sfxAlreadyPlayed) {
      const warpSfx = sfxPathForConstant(
        normalizedPlayerMapId === UNIFIED_OVERWORLD_MAP_ID
          ? "SFX_GO_OUTSIDE"
          : "SFX_GO_INSIDE",
      );
      if (warpSfx) {
        void AudioManager.playSFX(warpSfx, 0.85);
      }
    }

    const playerActor = this.deps.getPlayerActor();

    const movement = this.deps.playerMovementController();
    movement.stopMovement(true);
    movement.syncMapId(normalizedPlayerMapId);

    if (playerActor) {
      playerActor.x = x;
      playerActor.y = y;
      playerActor.mapId = normalizedPlayerMapId;
      this.deps.setPlayerActor(playerActor);
    }

    // Retire the source tween before projecting the committed arrival; its
    // completion must not activate another local walking/warp path.
    movement.syncPosition(x, y);
    if (direction) movement.syncDirection(direction);
    if (playerActor?.id != null) {
      this.deps.mapRenderer().snapActorPosition(
        playerActor.id, x, y, direction ?? "DOWN", playerActor,
      );
    }

    if (mapId !== currentMapId) {
      this.deps.scene.game.registry.set("destinationMapId", mapId);
      this.deps.scene.game.registry.set("destinationX", x);
      this.deps.scene.game.registry.set("destinationY", y);
      if (direction) {
        this.deps.scene.game.registry.set("destinationDirection", direction);
      }
      if (
        animateExitStep &&
        animationStartX != null &&
        animationStartY != null
      ) {
        this.deps.scene.game.registry.set("warpAnimationStartX", animationStartX);
        this.deps.scene.game.registry.set("warpAnimationStartY", animationStartY);
        this.deps.scene.game.registry.set("warpAnimationDestX", x);
        this.deps.scene.game.registry.set("warpAnimationDestY", y);
        if (direction) {
          this.deps.scene.game.registry.set("warpAnimationDirection", direction);
        }
      } else {
        this.deps.scene.game.registry.remove("warpAnimationStartX");
        this.deps.scene.game.registry.remove("warpAnimationStartY");
        this.deps.scene.game.registry.remove("warpAnimationDestX");
        this.deps.scene.game.registry.remove("warpAnimationDestY");
        this.deps.scene.game.registry.remove("warpAnimationDirection");
      }
      this.deps.scene.game.registry.set("useOverworldSavedCamera", false);
      this.deps.resetScene(false);
      return;
    }

    if (
      animateExitStep &&
      animationStartX != null &&
      animationStartY != null
    ) {
      const sceneWithAnimation = this.deps.scene as Scene & {
        warpAnimationStartX?: number | null;
        warpAnimationStartY?: number | null;
        warpAnimationDestX?: number | null;
        warpAnimationDestY?: number | null;
        warpAnimationDirection?: string | null;
        playPendingWarpExitAnimation?: (delayMs?: number) => Promise<void>;
      };
      sceneWithAnimation.warpAnimationStartX = animationStartX;
      sceneWithAnimation.warpAnimationStartY = animationStartY;
      sceneWithAnimation.warpAnimationDestX = x;
      sceneWithAnimation.warpAnimationDestY = y;
      sceneWithAnimation.warpAnimationDirection = direction ?? null;
      await sceneWithAnimation.playPendingWarpExitAnimation?.();
      return;
    }

    movement.syncPosition(x, y);
    if (direction) {
      movement.syncDirection(direction);
    }
  }
}
