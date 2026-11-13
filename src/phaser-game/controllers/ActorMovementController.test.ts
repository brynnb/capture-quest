import { describe, expect, test, vi } from "vitest";
import { ActorMovementController } from "./ActorMovementController";
import type { PhaserActor } from "@/net/generated/world_api";
import type { Scene } from "phaser";

type Listener = {
  callback: (...args: number[]) => void;
  context: unknown;
  once: boolean;
};

function fakeScene() {
  const listeners = new Map<string, Listener[]>();
  const events = {
    on(event: string, callback: Listener["callback"], context: unknown) {
      const existing = listeners.get(event) ?? [];
      existing.push({ callback, context, once: false });
      listeners.set(event, existing);
    },
    once(event: string, callback: Listener["callback"], context: unknown) {
      const existing = listeners.get(event) ?? [];
      existing.push({ callback, context, once: true });
      listeners.set(event, existing);
    },
    off(event: string, callback: Listener["callback"], context: unknown) {
      listeners.set(
        event,
        (listeners.get(event) ?? []).filter(
          (listener) =>
            listener.callback !== callback || listener.context !== context,
        ),
      );
    },
    emit(event: string, ...args: number[]) {
      for (const listener of [...(listeners.get(event) ?? [])]) {
        if (listener.once) {
          events.off(event, listener.callback, listener.context);
        }
        listener.callback.apply(listener.context, args);
      }
    },
  };
  const scene = {
    events,
    add: {
      ellipse: vi.fn(() => ({
        setDepth: vi.fn(),
        setPosition: vi.fn(),
        destroy: vi.fn(),
      })),
    },
  } as unknown as Scene;
  let time = 0;
  return {
    scene,
    events,
    advance(delta: number) {
      time += delta;
      events.emit("update", time, delta);
    },
    listenerCount(event: string) {
      return listeners.get(event)?.length ?? 0;
    },
  };
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
  test("committed server movement completes with its projection kind", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    controller.registerActor(actor(), fakeSprite());
    const completed = vi.fn();
    controller.setOnStepComplete(completed);

    controller.handlePositionUpdate(1, 1, 0, "RIGHT", {
      serverControlled: true,
    });
    advance(100);
    expect(completed).not.toHaveBeenCalled();
    advance(100);
    expect(completed).toHaveBeenCalledWith(1, 1, 0, "RIGHT", "serverStep");
  });

  test.each([1000 / 120, 1000 / 60, 1000 / 30, 37])(
    "moves at exactly the configured speed across every tile with %fms frames",
    (frameDelta) => {
      const { scene, advance } = fakeScene();
      const controller = new ActorMovementController(scene);
      const sprite = fakeSprite();
      controller.registerActor(actor(), sprite);
      controller.setOnStepComplete((_actorId, x) => {
        if (x < 16) controller.handlePositionUpdate(1, x + 1, 0, "RIGHT");
      });
      controller.handlePositionUpdate(1, 1, 0, "RIGHT");

      let elapsed = 0;
      while (elapsed < 3200) {
        const delta = Math.min(frameDelta, 3200 - elapsed);
        elapsed += delta;
        const previousX = sprite.x;
        advance(delta);
        // Unlike a maximum-pause check, this rejects even one lost frame or
        // accelerated catch-up frame at any of the fifteen tile boundaries.
        expect(sprite.x - previousX).toBeCloseTo((16 * delta) / 200, 8);
        expect(sprite.x).toBeCloseTo(8 + (16 * elapsed) / 200, 8);
      }
      expect(sprite.x).toBeCloseTo(264, 8);
      expect(sprite.y).toBe(8);
      expect(controller.getActorTilePosition(1)).toMatchObject({ x: 16, y: 0 });
      expect(controller.getActorState(1)?.isAnimating).toBe(false);
    },
  );

  test("carries a frame through several steps and corners without losing time", async () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);
    const path = controller.animateActorLocalPath(
      1,
      [
        { x: 1, y: 0 },
        { x: 1, y: 1 },
        { x: 2, y: 1 },
        { x: 2, y: 2 },
      ],
      "LEFT",
    );

    advance(650);
    expect(sprite.x).toBe(40);
    expect(sprite.y).toBe(28);
    expect(controller.getActorTilePosition(1)).toMatchObject({ x: 2, y: 1 });
    expect(controller.getActorState(1)?.isAnimating).toBe(true);
    advance(150);
    await path;
    expect(sprite.x).toBe(40);
    expect(sprite.y).toBe(40);
    expect(controller.getActorState(1)?.isAnimating).toBe(false);
    expect(controller.getActorTilePosition(1)?.direction).toBe("LEFT");
  });

  test("does not accelerate a queued local route", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor({ moveSpeed: 250 }), sprite);
    void controller.animateActorLocalPath(
      1,
      Array.from({ length: 20 }, (_, index) => ({ x: index + 1, y: 0 })),
    );

    advance(1750);
    expect(sprite.x).toBe(120);
    expect(controller.getActorTilePosition(1)).toMatchObject({ x: 7, y: 0 });
    expect(controller.getActorState(1)?.isAnimating).toBe(true);
  });

  test("scales two-tile movement duration without changing the travel speed", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);

    controller.handlePositionUpdate(1, 0, 2, "DOWN");
    advance(200);
    expect(sprite.y).toBe(24);
    expect(controller.getActorTilePosition(1)?.y).toBe(0);
    advance(200);
    expect(sprite.y).toBe(40);
    expect(controller.getActorTilePosition(1)?.y).toBe(2);
  });

  test("ledge jump uses a moving ground shadow and lifts the sprite mid-jump", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);

    controller.handlePositionUpdate(1, 0, 2, "DOWN", { ledgeJump: true });
    const shadow = vi.mocked(scene.add.ellipse).mock.results[0].value;
    expect(scene.add.ellipse).toHaveBeenCalledWith(8, 13, 12, 4, 0x000000, 0.65);
    expect(shadow.setDepth).toHaveBeenCalledWith(4.9);

    advance(200);
    expect(shadow.setPosition).toHaveBeenLastCalledWith(8, 29);
    expect(sprite.y).toBe(14);
    advance(100);
    expect(shadow.setPosition).toHaveBeenLastCalledWith(8, 37);
    expect(sprite.y).toBeCloseTo(32 - Math.sin(Math.PI * 0.75) * 10);
    advance(100);
    expect(sprite.y).toBe(40);
    expect(shadow.destroy).toHaveBeenCalledTimes(1);
  });

  test("settles directly on the final facing frame when movement ends", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);

    controller.handlePositionUpdate(1, 1, 0, "RIGHT");
    advance(150);
    expect(sprite.setFrame).toHaveBeenLastCalledWith(5);
    advance(50);
    expect(sprite.setFrame).toHaveBeenLastCalledWith(2);
    expect(sprite.setFlipX).toHaveBeenLastCalledWith(true);
  });

  test("completion can supply the next step before idle or frame time is lost", async () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);
    controller.setOnStepComplete((_actorId, x) => {
      if (x === 1) controller.handlePositionUpdate(1, 2, 0, "RIGHT");
    });
    controller.handlePositionUpdate(1, 1, 0, "RIGHT");
    const idle = vi.fn();
    void controller.waitForActorIdle(1).then(idle);

    advance(250);
    expect(sprite.x).toBe(28);
    expect(controller.getActorTilePosition(1)).toMatchObject({
      x: 1,
      y: 0,
      direction: "RIGHT",
    });
    await Promise.resolve();
    expect(idle).not.toHaveBeenCalled();
    advance(150);
    await Promise.resolve();
    expect(idle).toHaveBeenCalledOnce();
    expect(sprite.x).toBe(40);
  });

  test("completion reports the direction derived from the completed tile step", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const completed = vi.fn();
    controller.registerActor(actor({ actionDirection: "DOWN" }), fakeSprite());
    controller.setOnStepComplete(completed);

    controller.handlePositionUpdate(1, 1, 0, "DOWN");
    advance(200);
    expect(completed).toHaveBeenCalledWith(1, 1, 0, "RIGHT", "step");
  });

  test("a snap retires the active motion and queue and reports a projection", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const completed = vi.fn();
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);
    controller.setOnStepComplete(completed);
    controller.handlePositionUpdate(1, 1, 0, "RIGHT");
    controller.handlePositionUpdate(1, 2, 0, "RIGHT");
    advance(50);

    controller.snapActorToPosition(1, 8, 9, "UP");
    advance(500);
    expect(controller.getActorState(1)).toMatchObject({
      currentX: 8,
      currentY: 9,
      isAnimating: false,
      currentMotion: null,
      queue: [],
    });
    expect(sprite.x).toBe(136);
    expect(sprite.y).toBe(152);
    expect(completed).toHaveBeenCalledOnce();
    expect(completed).toHaveBeenCalledWith(1, 8, 9, "UP", "snap");
  });

  test("a completion-time snap cannot spend old frame time on its replacement route", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);
    controller.setOnStepComplete((_actorId, _x, _y, _direction, kind) => {
      if (kind === "step") {
        controller.snapActorToPosition(1, 8, 9, "UP");
        controller.handlePositionUpdate(1, 9, 9, "RIGHT");
      }
    });
    controller.handlePositionUpdate(1, 1, 0, "RIGHT");

    advance(250);
    expect(sprite.x).toBe(136);
    expect(sprite.y).toBe(152);
    advance(50);
    expect(sprite.x).toBe(140);
  });

  test("a replacement sprite cannot inherit a retired registration's frame time", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const oldSprite = fakeSprite();
    const replacement = fakeSprite();
    replacement.setX(88);
    controller.registerActor(actor(), oldSprite);
    controller.setOnStepComplete(() => {
      controller.registerActor(actor({ x: 5 }), replacement);
      controller.handlePositionUpdate(1, 6, 0, "RIGHT");
    });
    controller.handlePositionUpdate(1, 1, 0, "RIGHT");

    advance(250);
    expect(oldSprite.x).toBe(24);
    expect(replacement.x).toBe(88);
    advance(50);
    expect(replacement.x).toBe(92);
  });

  test("reapplies the latest tracked facing after an async texture swap", () => {
    const { scene, advance } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor({ actionDirection: "UP" }), sprite);
    controller.handlePositionUpdate(1, 1, 0, "UP");
    advance(200);
    vi.mocked(sprite.setFrame).mockClear();
    vi.mocked(sprite.setFlipX).mockClear();

    expect(controller.refreshActorFacingFrame(1)).toBe(true);
    expect(sprite.setFrame).toHaveBeenLastCalledWith(2);
    expect(sprite.setFlipX).toHaveBeenLastCalledWith(true);
  });

  test("shutdown detaches the scene clock and retires animations and shadows", async () => {
    const { scene, advance, events, listenerCount } = fakeScene();
    const controller = new ActorMovementController(scene);
    const sprite = fakeSprite();
    controller.registerActor(actor(), sprite);
    controller.handlePositionUpdate(1, 0, 2, "DOWN", { ledgeJump: true });
    const shadow = vi.mocked(scene.add.ellipse).mock.results[0].value;
    const idle = controller.waitForActorIdle(1);

    events.emit("shutdown");
    await idle;
    advance(400);
    expect(controller.getActorState(1)).toBeUndefined();
    expect(sprite.x).toBe(8);
    expect(sprite.y).toBe(8);
    expect(shadow.destroy).toHaveBeenCalledOnce();
    expect(listenerCount("update")).toBe(0);
    expect(listenerCount("shutdown")).toBe(0);
    expect(listenerCount("destroy")).toBe(0);
  });
});
