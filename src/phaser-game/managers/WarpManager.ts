import { Scene } from "phaser";
import { CameraController } from "../controllers/CameraController";
import { PlayerMovementController } from "../controllers/PlayerMovementController";
import { MapDataService } from "../services/MapDataService";
import { UiManager } from "./UiManager";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import type { PhaserActor, PhaserWarp } from "@/net/generated/world_api";
import { isWorldInputFrozen } from "../utils/worldInputGuard";
import AudioManager from "@/services/audio/AudioManager";

export class WarpManager {
  private scene: Scene;
  private mapDataService: MapDataService;
  private playerMovementController: PlayerMovementController;
  private uiManager: UiManager;

  // Pending warp state
  private pendingWarpX: number | null = null;
  private pendingWarpY: number | null = null;

  // Warp lookup by tile position: "x,y" -> warp data
  private warpLookup: Map<string, PhaserWarp> = new Map();

  // Callbacks
  private getPlayerActor: () => PhaserActor | null;

  constructor(
    scene: Scene,
    mapDataService: MapDataService,
    _cameraController: CameraController,
    playerMovementController: PlayerMovementController,
    uiManager: UiManager,
    callbacks: {
      onResetScene: (resetCamera: boolean) => void;
      getPlayerActor: () => PhaserActor | null;
      setPlayerActor: (actor: PhaserActor) => void;
      getIsOverworldMode: () => boolean;
    },
  ) {
    this.scene = scene;
    this.mapDataService = mapDataService;
    this.playerMovementController = playerMovementController;
    this.uiManager = uiManager;
    this.getPlayerActor = callbacks.getPlayerActor;
  }

  getPendingWarpX(): number | null {
    return this.pendingWarpX;
  }

  getPendingWarpY(): number | null {
    return this.pendingWarpY;
  }

  setPendingWarpPosition(x: number | null, y: number | null) {
    this.pendingWarpX = x;
    this.pendingWarpY = y;
  }

  cancelPendingWarp(): void {
    // Normal warps are activated immediately from loaded client warp data.
  }

  private isWorldInputFrozen(): boolean {
    const scene = this.scene as Scene & { isWorldInputFrozen?: () => boolean };
    return scene.isWorldInputFrozen?.() ?? isWorldInputFrozen();
  }

  applyPendingWarpToActor(actor: PhaserActor): boolean {
    if (this.pendingWarpX !== null && this.pendingWarpY !== null) {
      console.log(
        `[WarpManager] Applying pending warp position to player: (${this.pendingWarpX}, ${this.pendingWarpY})`,
      );
      actor.x = this.pendingWarpX;
      actor.y = this.pendingWarpY;
      this.pendingWarpX = null;
      this.pendingWarpY = null;
      return true;
    }
    return false;
  }

  /**
   * Update the warp lookup map when warps are loaded for a map.
   */
  setWarps(warps: PhaserWarp[]): void {
    this.warpLookup.clear();
    for (const w of warps) {
      this.warpLookup.set(`${w.x},${w.y}`, w);
    }
    console.log(
      `[WarpManager] Built warp lookup with ${this.warpLookup.size} entries`,
    );
  }

  hasWarpAt(x: number, y: number): boolean {
    return this.warpLookup.has(`${x},${y}`);
  }

  getWarpAt(x: number, y: number): PhaserWarp | null {
    return this.warpLookup.get(`${x},${y}`) ?? null;
  }

  /**
   * Normal warp activation is client-owned. The client already has the loaded
   * warp data, so it transitions immediately and reports the final position.
   */
  setupKeyboardWarpHandlers() {
    this.scene.events.on(
      "playerSteppedOnTile",
      (
        x: number,
        y: number,
        inputSource: "click" | "keyboard",
        direction: string,
        reachedMoveDestination: boolean,
      ) => {
        if (
          this.isWorldInputFrozen() ||
          (inputSource !== "keyboard" && !reachedMoveDestination)
        ) {
          return;
        }

        const warp = this.getWarpAt(x, y);
        if (!warp) return;

        const normalizedDirection = direction.trim().toUpperCase();
        const warpDirection = warp.warpDirection?.trim().toUpperCase();
        if (inputSource === "click") {
          this.activateWarp(warp, warpDirection || normalizedDirection);
          return;
        }

        if (warpDirection && warpDirection !== normalizedDirection) return;

        this.activateWarp(warp, normalizedDirection);
      },
    );
  }

  private normalizeWarpType(warp: PhaserWarp): "door" | "carpet" {
    const type = warp.warpType?.trim().toLowerCase();
    return type === "carpet" || type === "directional" ? "carpet" : "door";
  }

  private blockedDirectionalCarpetEntries(
    warp: PhaserWarp,
  ): Array<{ x: number; y: number }> {
    if (
      this.normalizeWarpType(warp) !== "carpet" ||
      this.playerMovementController.isWalkable(warp.x, warp.y)
    ) {
      return [];
    }

    return [
      { x: warp.x, y: warp.y - 1 },
      { x: warp.x, y: warp.y + 1 },
      { x: warp.x - 1, y: warp.y },
      { x: warp.x + 1, y: warp.y },
    ].filter((entry) =>
      this.playerMovementController.isWalkable(entry.x, entry.y),
    );
  }

  private canActivateWarpFromPosition(
    warp: PhaserWarp,
    x: number,
    y: number,
  ): boolean {
    const distance = Math.abs(x - warp.x) + Math.abs(y - warp.y);
    if (this.normalizeWarpType(warp) === "carpet") {
      if (distance === 0) return true;
      return this.blockedDirectionalCarpetEntries(warp).some(
        (entry) => entry.x === x && entry.y === y,
      );
    }
    return distance <= 1;
  }

  private findWarpActivationTarget(
    warp: PhaserWarp,
    playerPos: { x: number; y: number },
  ): { x: number; y: number } | null {
    if (this.normalizeWarpType(warp) === "carpet") {
      if (this.playerMovementController.isWalkable(warp.x, warp.y)) {
        return { x: warp.x, y: warp.y };
      }
      const candidates = this.blockedDirectionalCarpetEntries(warp);
      if (candidates.length === 0) return null;
      candidates.sort((a, b) => {
        const aDistance =
          Math.abs(a.x - playerPos.x) + Math.abs(a.y - playerPos.y);
        const bDistance =
          Math.abs(b.x - playerPos.x) + Math.abs(b.y - playerPos.y);
        return aDistance - bDistance;
      });
      return candidates[0];
    }

    const candidates = [
      { x: warp.x, y: warp.y },
      { x: warp.x, y: warp.y + 1 },
      { x: warp.x, y: warp.y - 1 },
      { x: warp.x - 1, y: warp.y },
      { x: warp.x + 1, y: warp.y },
    ].filter((candidate) =>
      this.playerMovementController.isWalkable(candidate.x, candidate.y),
    );

    if (candidates.length === 0) return null;
    candidates.sort((a, b) => {
      const aDistance =
        Math.abs(a.x - playerPos.x) + Math.abs(a.y - playerPos.y);
      const bDistance =
        Math.abs(b.x - playerPos.x) + Math.abs(b.y - playerPos.y);
      return aDistance - bDistance;
    });
    return candidates[0];
  }

  private directionFromPositionToWarp(
    warp: PhaserWarp,
    playerPos: { x: number; y: number },
  ): string | undefined {
    if (warp.x === playerPos.x && warp.y === playerPos.y) {
      return warp.warpDirection?.trim().toUpperCase() || undefined;
    }
    if (warp.x === playerPos.x && warp.y === playerPos.y - 1) return "UP";
    if (warp.x === playerPos.x && warp.y === playerPos.y + 1) return "DOWN";
    if (warp.x === playerPos.x - 1 && warp.y === playerPos.y) return "LEFT";
    if (warp.x === playerPos.x + 1 && warp.y === playerPos.y) return "RIGHT";
    return warp.warpDirection?.trim().toUpperCase() || undefined;
  }

  activateWarp(warp: PhaserWarp, direction?: string): void {
    if (
      !warp ||
      warp.destinationMapId === undefined ||
      warp.destinationMapId === null ||
      warp.destinationX === undefined ||
      warp.destinationY === undefined
    ) {
      return;
    }

    const playerActor = this.getPlayerActor();
    const currentMapId = this.scene.game.registry.get("currentMapId");
    const sourceMapId =
      typeof currentMapId === "number" ? currentMapId : playerActor?.mapId;
    const normalizedDirection =
      direction?.trim().toUpperCase() ||
      this.directionFromPositionToWarp(
        warp,
        this.playerMovementController.getCurrentPosition(),
      ) ||
      "DOWN";

    const destinationIsOverworld = this.mapDataService.isOverworld(
      warp.destinationMapId,
    );
    const sourceIsOverworld =
      typeof sourceMapId === "number" && this.mapDataService.isOverworld(sourceMapId);

    let destinationX = warp.destinationX;
    let destinationY = warp.destinationY;
    const detail: Record<string, unknown> = {
      mapId: warp.destinationMapId,
      x: destinationX,
      y: destinationY,
      direction: normalizedDirection,
    };

    if (
      destinationIsOverworld &&
      !sourceIsOverworld &&
      normalizedDirection === "DOWN"
    ) {
      destinationY += 1;
      detail.x = destinationX;
      detail.y = destinationY;
      detail.animateExitStep = true;
      detail.animationStartX = warp.destinationX;
      detail.animationStartY = warp.destinationY;
    }

    void AudioManager.playGeneratedSFX("warp", 0.85);
    window.dispatchEvent(
      new CustomEvent("warpTileTeleport", {
        detail,
      }),
    );
  }

  setupWarpClickHandler() {
    this.scene.events.on("warpClicked", (warp: PhaserWarp) => {
      if (this.isWorldInputFrozen()) return;
      console.log("[WarpManager] Warp clicked, pathing player to door:", warp);

      if (
        !warp ||
        warp.destinationMapId === undefined ||
        warp.destinationMapId === null
      ) {
        if (warp && warp.destinationMap) {
          const errorMsg = `Error: Warp destination ${warp.destinationMap} is not configured in database.`;
          console.error(errorMsg, warp);
          useChatStore
            .getState()
            .addMessage(errorMsg, MessageType.SYSTEM_ERROR);
          this.uiManager.setLoadingText(errorMsg);
        } else {
          console.warn("Invalid warp data or missing destination map ID", warp);
        }
        return;
      }

      const playerPos = this.playerMovementController.getCurrentPosition();
      if (this.canActivateWarpFromPosition(warp, playerPos.x, playerPos.y)) {
        console.log(
          `[WarpManager] Player already near warp, activating locally`,
        );
        this.activateWarp(warp, this.directionFromPositionToWarp(warp, playerPos));
        return;
      }

      const walkTarget = this.findWarpActivationTarget(warp, playerPos);
      if (!walkTarget) {
        console.warn(
          `[WarpManager] No walkable activation tile for warp (${warp.x}, ${warp.y}), cannot path`,
        );
        return;
      }

      console.log(
        `[WarpManager] Pathing locally to tile (${walkTarget.x}, ${walkTarget.y}) near warp (${warp.x}, ${warp.y})`,
      );
      if (!this.playerMovementController.requestMoveTo(
        walkTarget.x,
        walkTarget.y,
        this.playerMovementController.getCurrentMapId(),
        "click",
        warp.id,
      )) {
        return;
      }
      if (walkTarget.x !== warp.x || walkTarget.y !== warp.y) {
        this.playerMovementController.setArrivalCallback((x, y) => {
          if (x !== walkTarget.x || y !== walkTarget.y) {
            return false;
          }
          if (!this.isWorldInputFrozen()) {
            this.activateWarp(
              warp,
              this.directionFromPositionToWarp(warp, { x, y }),
            );
          }
          return true;
        });
      }
    });
  }

  cleanup() {
    this.pendingWarpX = null;
    this.pendingWarpY = null;
    this.warpLookup.clear();
    this.scene.events.off("warpClicked");
    this.scene.events.off("playerSteppedOnTile");
    this.scene.events.off("playerFacedDirection");
  }
}
