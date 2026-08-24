import { describe, expect, test, vi } from "vitest";
import { ActorMovementController } from "./ActorMovementController";
import type { PhaserActor } from "@/net/generated/world_api";
import type { Scene } from "phaser";

type TweenConfig = {
  duration: number;
  onUpdate?: (tween: { progress: number }) => void;
  onComplete?: () => void;
};

function fakeScene(durations: number[], tweens: TweenConfig[] = []): Scene {
  return {
    tweens: {
      add: vi.fn((config: TweenConfig) => {
        durations.push(config.duration);
        tweens.push(config);
        return { stop: vi.fn() };
      }),
    },
    add: {
      ellipse: vi.fn(() => ({
        setDepth: vi.fn(),
        setPosition: vi.fn(),
        destroy: vi.fn(),
      })),
    },
  } as unknown as Scene;
}

function fakeSprite(): Phaser.GameObjects.Sprite {
  const sprite = {
    active: true,
    depth: 5,
    scene: {},
    texture: {
      key: "player",
      has: vi.fn(() => true),
      frameTotal: 6,
    },
    x: 8,
    y: 8,
  } as unknown as Phaser.GameObjects.Sprite;
  sprite.setFrame = vi.fn();
  sprite.setFlipX = vi.fn();
  sprite.setX = vi.fn((x: number) => {
    sprite.x = x;
    return sprite;
  });
  sprite.setY = vi.fn((y: number) => {
    sprite.y = y;
    return sprite;
  });
  return sprite;
}

function actor(overrides: Partial<PhaserActor> = {}): PhaserActor {
  return {
    id: 1,
    x: 0,
    y: 0,
    mapId: 9999,
    objectType: "player",
    moveSpeed: 200,
    ...overrides,
  };
}

describe("ActorMovementController", () => {
  test("scales animation duration by tile distance for ledge-style two-tile movement", () => {
    const durations: number[] = [];
    const controller = new ActorMovementController(fakeScene(durations));
    controller.registerActor(actor(), fakeSprite());

    controller.handlePositionUpdate(1, 0, 2, "DOWN");

    expect(durations).toEqual([400]);
  });

  test("ledge jump uses a moving ground shadow and lifts the sprite mid-jump", () => {
    const durations: number[] = [];
    const tweens: TweenConfig[] = [];
    const scene = fakeScene(durations, tweens);
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);

    controller.handlePositionUpdate(1, 0, 2, "DOWN", { ledgeJump: true });

    const shadow = vi.mocked(scene.add.ellipse).mock.results[0].value;
    expect(scene.add.ellipse).toHaveBeenCalledWith(8, 13, 12, 4, 0x000000, 0.65);
    expect(shadow.setDepth).toHaveBeenCalledWith(4.9);

    tweens[0].onUpdate?.({ progress: 0.5 });

    expect(shadow.setPosition).toHaveBeenCalledWith(8, 13);
    expect(sprite.setY).toHaveBeenCalledWith(-2);

    tweens[0].onComplete?.();

    expect(sprite.setY).toHaveBeenLastCalledWith(40);
    expect(shadow.destroy).toHaveBeenCalledTimes(1);
  });
});
