import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import { TILE_SIZE } from "../../constants";
import { ActorManager } from "../../managers/ActorManager";
import { MapRenderer } from "../../renderers/MapRenderer";
import {
  registerCutsceneCallbacks,
  unregisterCutsceneCallbacks,
} from "../../services/CutsceneService";

interface CutsceneSpriteControllerDeps {
  scene: Scene;
  mapContainer: () => Phaser.GameObjects.Container;
  actorManager: () => ActorManager;
  mapRenderer: () => MapRenderer;
  getPlayerActor: () => PhaserActor | null;
  syncPlayerMovement: (x: number, y: number, direction: string) => void;
  setInputLocked: (locked: boolean) => void;
  onHideObject: (actorId: number) => void;
}

export class CutsceneSpriteController {
  private readonly sprites = new Map<string, Phaser.GameObjects.Sprite>();

  constructor(private readonly deps: CutsceneSpriteControllerDeps) {}

  registerCallbacks(): void {
    registerCutsceneCallbacks({
      onMove: (actor: string, movements: string[]): Promise<void> => {
        return this.moveActor(actor, movements);
      },
      onShowActor: (actor: string, x: number, y: number, sprite: string) => {
        this.showActor(actor, x, y, sprite);
      },
      onHideActor: (actor: string) => {
        this.hideActor(actor);
      },
      onHideObject: (actorId: number) => {
        this.deps.onHideObject(actorId);
      },
      onFace: (actor: string, direction: string) => {
        this.faceActor(actor, direction);
      },
      onInputLock: (locked: boolean) => {
        this.deps.setInputLocked(locked);
      },
    });
  }

  cleanup(): void {
    this.sprites.forEach((sprite) => {
      if (sprite.active) sprite.destroy();
    });
    this.sprites.clear();
    this.deps.setInputLocked(false);
    unregisterCutsceneCallbacks();
  }

  private showActor(
    actor: string,
    x: number,
    y: number,
    spriteName: string,
  ): void {
    this.hideActor(actor);

    const posX = x * TILE_SIZE + TILE_SIZE / 2;
    const posY = y * TILE_SIZE + TILE_SIZE / 2;
    const actorManager = this.deps.actorManager();
    const initialSpriteKey = spriteName
      ? actorManager.getSpriteKey(spriteName)
      : "actor-fallback";
    const initialTexture = this.deps.scene.textures.exists(initialSpriteKey)
      ? initialSpriteKey
      : "actor-fallback";

    const sprite = this.deps.scene.add.sprite(posX, posY, initialTexture);
    this.setSpriteFrame(sprite, "DOWN", false);
    this.deps.mapContainer().add(sprite);
    this.deps.mapContainer().bringToTop(sprite);
    this.sprites.set(actor, sprite);

    if (spriteName) {
      actorManager
        .loadActorSprite(spriteName)
        .then((spriteKey) => {
          if (
            sprite.active &&
            spriteKey !== "actor-fallback" &&
            this.deps.scene.textures.exists(spriteKey)
          ) {
            sprite.setTexture(spriteKey);
            this.setSpriteFrame(sprite, "DOWN", false);
          }
        })
        .catch((error) => {
          console.warn(
            `[Cutscene] Failed to load actor sprite ${spriteName}:`,
            error,
          );
        });
    }
  }

  private hideActor(actor: string): void {
    const sprite = this.sprites.get(actor);
    if (!sprite) return;

    sprite.destroy();
    this.sprites.delete(actor);
  }

  private faceActor(actor: string, direction: string): void {
    const sprite = this.sprites.get(actor);
    if (!sprite || !sprite.active) return;

    this.setSpriteFrame(sprite, direction, false);
  }

  private moveActor(actor: string, movements: string[]): Promise<void> {
    if (actor === "__PLAYER__" && this.deps.getPlayerActor()) {
      return this.movePlayerSprite(movements);
    }

    const sprite = this.sprites.get(actor);
    if (!sprite || !sprite.active) return Promise.resolve();

    return new Promise<void>((resolve) => {
      let stepIndex = 0;
      let stepCount = 0;
      let lastMove = "DOWN";

      const doNextStep = () => {
        if (stepIndex >= movements.length || !sprite.active) {
          this.faceActor(actor, lastMove);
          resolve();
          return;
        }

        const move = movements[stepIndex].toUpperCase()
          .replace("NPC_MOVEMENT_", "");
        stepIndex++;

        const delta = this.movementDelta(move);
        if (!delta) {
          doNextStep();
          return;
        }

        lastMove = move;
        this.faceActor(actor, move);
        stepCount++;
        const alternateFlip = stepCount % 2 === 1;

        this.deps.scene.tweens.add({
          targets: sprite,
          x: sprite.x + delta.dx * TILE_SIZE,
          y: sprite.y + delta.dy * TILE_SIZE,
          duration: 300,
          ease: "Linear",
          onUpdate: (tween) => {
            this.setSpriteFrame(
              sprite,
              move,
              tween.progress >= 0.5,
              alternateFlip,
            );
          },
          onComplete: () => {
            this.faceActor(actor, move);
            doNextStep();
          },
        });
      };

      doNextStep();
    });
  }

  private movePlayerSprite(movements: string[]): Promise<void> {
    const playerActor = this.deps.getPlayerActor();
    if (!playerActor) return Promise.resolve();

    const playerSprite = this.deps.mapRenderer().getActorSprite(playerActor.id);
    if (!playerSprite || !playerSprite.active) return Promise.resolve();

    const trackedPosition = this.deps
      .mapRenderer()
      .getActorTilePosition(playerActor.id);
    let tileX =
      trackedPosition?.x ??
      playerActor.x ??
      Math.round((playerSprite.x - TILE_SIZE / 2) / TILE_SIZE);
    let tileY =
      trackedPosition?.y ??
      playerActor.y ??
      Math.round((playerSprite.y - TILE_SIZE / 2) / TILE_SIZE);

    const syncPlayerTile = (direction?: string) => {
      const currentPlayer = this.deps.getPlayerActor();
      if (!currentPlayer) return;

      currentPlayer.x = tileX;
      currentPlayer.y = tileY;
      if (direction) {
        currentPlayer.actionDirection = direction;
      }
      const actorData = (playerSprite as unknown as { actorData?: PhaserActor })
        .actorData;
      if (actorData) {
        actorData.x = tileX;
        actorData.y = tileY;
        if (direction) {
          actorData.actionDirection = direction;
        }
      }
    };

    let lastMove =
      trackedPosition?.direction ?? playerActor.actionDirection ?? "DOWN";

    return new Promise<void>((resolve) => {
      let stepIndex = 0;
      let stepCount = 0;

      const doNextStep = () => {
        if (stepIndex >= movements.length || !playerSprite.active) {
          syncPlayerTile(lastMove);
          this.deps.syncPlayerMovement(tileX, tileY, lastMove);
          resolve();
          return;
        }

        const move = movements[stepIndex].toUpperCase()
          .replace("NPC_MOVEMENT_", "");
        stepIndex++;

        const delta = this.movementDelta(move);
        if (!delta) {
          doNextStep();
          return;
        }

        const nextX = tileX + delta.dx;
        const nextY = tileY + delta.dy;
        lastMove = move;
        syncPlayerTile(move);
        this.setSpriteFrame(playerSprite, move, false);
        stepCount++;
        const alternateFlip = stepCount % 2 === 1;

        this.deps.scene.tweens.add({
          targets: playerSprite,
          x: playerSprite.x + delta.dx * TILE_SIZE,
          y: playerSprite.y + delta.dy * TILE_SIZE,
          duration: 300,
          ease: "Linear",
          onUpdate: (tween) => {
            this.setSpriteFrame(
              playerSprite,
              move,
              tween.progress >= 0.5,
              alternateFlip,
            );
          },
          onComplete: () => {
            tileX = nextX;
            tileY = nextY;
            syncPlayerTile(move);
            this.deps
              .mapRenderer()
              .snapActorPosition(playerActor.id, tileX, tileY, move, playerActor);
            this.deps.syncPlayerMovement(tileX, tileY, move);
            this.setSpriteFrame(playerSprite, move, false);
            doNextStep();
          },
        });
      };

      doNextStep();
    });
  }

  private setSpriteFrame(
    sprite: Phaser.GameObjects.Sprite,
    direction: string,
    walking: boolean,
    alternateFlip: boolean = false,
  ): void {
    if (!sprite || !sprite.active || !sprite.texture) return;

    const dir = direction.toUpperCase();
    let frame = 0;
    let flipX = false;

    switch (dir) {
      case "UP":
        frame = walking ? 4 : 1;
        flipX = walking && alternateFlip;
        break;
      case "DOWN":
        frame = walking ? 3 : 0;
        flipX = walking && alternateFlip;
        break;
      case "LEFT":
        frame = walking ? 5 : 2;
        break;
      case "RIGHT":
        frame = walking ? 5 : 2;
        flipX = true;
        break;
      default:
        frame = 0;
        break;
    }

    if (sprite.texture.has(frame.toString())) {
      sprite.setFrame(frame);
    } else if (sprite.texture.has("0") || sprite.texture.frameTotal > 0) {
      sprite.setFrame(0);
    }
    sprite.setFlipX(flipX);
  }

  private movementDelta(move: string): { dx: number; dy: number } | null {
    switch (move) {
      case "UP":
        return { dx: 0, dy: -1 };
      case "DOWN":
        return { dx: 0, dy: 1 };
      case "LEFT":
        return { dx: -1, dy: 0 };
      case "RIGHT":
        return { dx: 1, dy: 0 };
      default:
        return null;
    }
  }
}
