import { afterEach } from "vitest";
import * as movement from "../services/PlayerMovementService";
vi.mock("../services/PlayerMovementService", () => ({
  requestPlayerStep: vi.fn(async () => ({ success: true, requestId: "accepted", stepToken: "issued", mapId: 9999, x: 10, y: 2, direction: "DOWN", ledgeJump: true })),
  completePlayerStep: vi.fn(async () => ({ success: true, requestId: "complete", mapId: 9999, x: 10, y: 2, direction: "DOWN" })),
}));
afterEach(() => vi.clearAllMocks());
import { describe, expect, test, vi } from "vitest";
import { PlayerMovementController } from "./PlayerMovementController";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import type { PhaserTile } from "@/net/generated/world_api";
import type { Scene } from "phaser";
import type { MapRenderer } from "../renderers/MapRenderer";
import * as PhaserNet from "../services/PhaserNetworkService";

function tile(
  id: number,
  x: number,
  y: number,
  collisionType: number,
  rawFootTileId?: number,
): PhaserTile {
  return {
    id,
    x,
    y,
    tileImageId: id,
    mapId: UNIFIED_OVERWORLD_MAP_ID,
    collisionType,
    rawFootTileId,
    talkOverTile: false,
    isNativeGameData: true,
    coordinateOrigin: "native",
    contentOrigin: "native",
  };
}

function buildLedgeController() {
  const updates: unknown[][] = [];
  const movementController = {
    handleDirectionUpdate: vi.fn(),
  };
  const mapRenderer = {
    updateActorPosition: vi.fn((...args: unknown[]) => {
      updates.push(args);
    }),
    getMovementController: () => movementController,
  };
  const controller = new PlayerMovementController({ events: { emit: vi.fn(), once: vi.fn() } } as unknown as Scene);
  controller.buildCollisionMap([
    tile(1, 10, 0, 1, 0x2c),
    tile(2, 10, 1, 0, 0x37),
    tile(3, 10, 2, 1),
    tile(4, 9, 0, 1),
    tile(5, 9, 1, 1),
    tile(6, 9, 2, 1),
  ]);
  controller.setPlayer(
    1,
    10,
    0,
    UNIFIED_OVERWORLD_MAP_ID,
    mapRenderer as unknown as MapRenderer,
  );
  return { controller, updates };
}

describe("PlayerMovementController ledges", () => {
  test("a server snap updates movement context without echoing a position write", () => {
    const send = vi.spyOn(PhaserNet, "sendPlayerPosition");
    const { controller } = buildLedgeController();
    controller.handleKeyboardMove("DOWN");
    controller.onStepComplete(1, 8, 9, "UP", "snap");
    expect(controller.getCurrentPosition()).toEqual({ x: 8, y: 9 });
    expect(controller.getCurrentDirection()).toBe("UP");
    expect(controller.getIsMoving()).toBe(false);
    expect(send).not.toHaveBeenCalled();
    send.mockRestore();
  });
  test("WASD jumps directly over a valid ledge instead of pathing around to the landing tile", async () => {
    const { controller, updates } = buildLedgeController();

    expect(controller.handleKeyboardMove("DOWN")).toBe(true);

    await vi.waitFor(() => expect(updates).toHaveLength(1));
    expect(updates[0]).toEqual([
      1,
      10,
      0,
      10,
      2,
      "DOWN",
      undefined,
      { ledgeJump: true },
    ]);
  });

  test("clicking a ledge landing tile starts with the one-way jump, not the walk-around route", async () => {
    const { controller, updates } = buildLedgeController();

    controller.handleTileClick(10 * TILE_SIZE + 8, 2 * TILE_SIZE + 8);

    await vi.waitFor(() => expect(updates).toHaveLength(1));
    expect(updates[0]).toEqual([
      1,
      10,
      0,
      10,
      2,
      "DOWN",
      undefined,
      { ledgeJump: true },
    ]);
  });
});

describe("PlayerMovementController completed facing", () => {
  test("uses the completed visual step direction instead of a stale spawn direction", () => {
    const movementController = { handleDirectionUpdate: vi.fn() };
    const mapRenderer = {
      getMovementController: () => movementController,
    };
    const scene = { events: { emit: vi.fn() } } as unknown as Scene;
    const controller = new PlayerMovementController(scene);
    controller.setPlayer(
      1,
      0,
      0,
      UNIFIED_OVERWORLD_MAP_ID,
      mapRenderer as unknown as MapRenderer,
    );

    expect(controller.getCurrentDirection()).toBe("DOWN");
    controller.onStepComplete(1, 1, 0, "RIGHT");

    expect(controller.getCurrentDirection()).toBe("RIGHT");
  });
});

describe("issued player movement lifecycle", () => {
  test("animation waits for acceptance and completion sends only the issued token", async () => {
    const { controller, updates } = buildLedgeController();
    const legacy = vi.spyOn(PhaserNet, "sendPlayerPosition");
    controller.handleKeyboardMove("DOWN");
    expect(updates).toHaveLength(0);
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    expect(movement.requestPlayerStep).toHaveBeenCalledWith({ mapId: 9999, fromX: 10, fromY: 0, direction: "DOWN" }, expect.any(AbortSignal));
    controller.onStepComplete(1, 10, 2, "DOWN");
    await vi.waitFor(() => expect(movement.completePlayerStep).toHaveBeenCalledWith("issued", expect.any(AbortSignal)));
    expect(legacy).not.toHaveBeenCalled();
    legacy.mockRestore();
    controller.clear();
  });

  test("a retired scene ignores late acceptance", async () => {
    let accept!: (value: Awaited<ReturnType<typeof movement.requestPlayerStep>>) => void;
    vi.mocked(movement.requestPlayerStep).mockImplementationOnce(() => new Promise((resolve) => { accept = resolve; }));
    const { controller, updates } = buildLedgeController();
    controller.handleKeyboardMove("DOWN");
    controller.clear();
    accept({ success: true, requestId: "late", stepToken: "old", mapId: 9999, x: 10, y: 2, direction: "DOWN", ledgeJump: true });
    await Promise.resolve();
    expect(updates).toHaveLength(0);
    expect(controller.getIsMoving()).toBe(false);
  });
});

 test("discarding a path lets its current issued animation acknowledge without a legacy report", async () => {
    const { controller, updates } = buildLedgeController();
    const legacy = vi.spyOn(PhaserNet, "sendPlayerPosition");
    controller.handleKeyboardMove("DOWN");
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    controller.stopMovement();
    controller.onStepComplete(1, 10, 2, "DOWN");
    await vi.waitFor(() => expect(movement.completePlayerStep).toHaveBeenCalledWith("issued", expect.any(AbortSignal)));
    expect(legacy).not.toHaveBeenCalled();
    legacy.mockRestore();
    controller.clear();
 });
