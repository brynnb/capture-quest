import { Scene } from "phaser";
import { TrainerEncounterNotifyPayload } from "@/net/generated/world_api";
import { TILE_SIZE } from "../../constants";
import { MapRenderer } from "../../renderers/MapRenderer";
import * as PhaserNet from "../../services/PhaserNetworkService";

interface TrainerEncounterPresenterDeps {
  scene: Scene;
  mapRenderer: () => MapRenderer;
  setInputLocked: (locked: boolean) => void;
}

export class TrainerEncounterPresenter {
  private activeTrainerActorId: number | null = null;
  private activeExclamation: Phaser.GameObjects.Text | null = null;

  constructor(private readonly deps: TrainerEncounterPresenterDeps) {}

  async handleEncounter(data: TrainerEncounterNotifyPayload): Promise<void> {
    console.log(
      `[TrainerEncounter] Trainer ${data.trainerName} (actor ${data.trainerActorId}) spotted player; local approach to (${data.approachToX}, ${data.approachToY})`,
    );

    if (this.activeTrainerActorId !== null) {
      console.warn(
        "[TrainerEncounter] Ignoring trainer encounter while another is active",
      );
      return;
    }

    this.activeTrainerActorId = data.trainerActorId;
    this.deps.setInputLocked(true);

    try {
      this.showExclamation(data.trainerActorId);
      await this.delayMs(450);
      this.destroyExclamation();

      const path = this.buildApproachPath(data);
      const finalDirection = this.directionFromTileToTile(
        data.approachToX,
        data.approachToY,
        data.playerX,
        data.playerY,
      );
      await this.deps
        .mapRenderer()
        .animateActorLocalPath(data.trainerActorId, path, finalDirection);
    } finally {
      this.destroyExclamation();
      this.deps.setInputLocked(false);
      this.activeTrainerActorId = null;
      PhaserNet.sendTrainerEncounterReady(data.trainerActorId);
    }
  }

  cleanup(): void {
    this.destroyExclamation();
    this.activeTrainerActorId = null;
  }

  private showExclamation(trainerActorId: number): void {
    this.destroyExclamation();

    const trainerSprite = this.deps.mapRenderer().getActorSprite(trainerActorId);
    if (!trainerSprite) return;

    const exclamation = this.deps.scene.add.text(
      trainerSprite.x,
      trainerSprite.y - TILE_SIZE,
      "!",
      {
        fontFamily: '"Pokemon Pixel Font", monospace',
        fontSize: "24px",
        color: "#ff0000",
        fontStyle: "bold",
        stroke: "#000000",
        strokeThickness: 3,
      },
    );
    exclamation.setOrigin(0.5, 1);
    exclamation.setDepth(9999);
    this.activeExclamation = exclamation;

    this.deps.scene.tweens.add({
      targets: exclamation,
      y: exclamation.y - 8,
      duration: 200,
      yoyo: true,
      repeat: 1,
      onComplete: () => {
        this.deps.scene.time.delayedCall(1000, () => {
          if (this.activeExclamation === exclamation) {
            this.activeExclamation = null;
          }
          if (exclamation.active) {
            exclamation.destroy();
          }
        });
      },
    });
  }

  private destroyExclamation(): void {
    if (!this.activeExclamation) return;

    this.deps.scene.tweens.killTweensOf(this.activeExclamation);
    this.activeExclamation.destroy();
    this.activeExclamation = null;
  }

  private buildApproachPath(
    data: TrainerEncounterNotifyPayload,
  ): { x: number; y: number }[] {
    const current =
      this.deps.mapRenderer().getActorTilePosition(data.trainerActorId) ?? {
        x: data.trainerX,
        y: data.trainerY,
        direction: "DOWN",
      };
    const targetX = data.approachToX;
    const targetY = data.approachToY;
    const stepX = Math.sign(targetX - current.x);
    const stepY = Math.sign(targetY - current.y);
    const path: { x: number; y: number }[] = [];
    let x = current.x;
    let y = current.y;
    let guard = 0;

    while ((x !== targetX || y !== targetY) && guard < 32) {
      if (x !== targetX) x += stepX;
      else if (y !== targetY) y += stepY;
      path.push({ x, y });
      guard++;
    }

    if (guard >= 32) {
      console.warn("[TrainerEncounter] Trainer approach path exceeded guard limit");
    }

    return path;
  }

  private directionFromTileToTile(
    fromX: number,
    fromY: number,
    toX: number,
    toY: number,
  ): "UP" | "DOWN" | "LEFT" | "RIGHT" {
    if (toY < fromY) return "UP";
    if (toY > fromY) return "DOWN";
    if (toX < fromX) return "LEFT";
    if (toX > fromX) return "RIGHT";
    return "DOWN";
  }

  private delayMs(ms: number): Promise<void> {
    return new Promise((resolve) => {
      this.deps.scene.time.delayedCall(ms, () => resolve());
    });
  }
}
