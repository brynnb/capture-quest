import { describe, expect, test, vi } from "vitest";
import { PlayerMovementController } from "./PlayerMovementController";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import type { PhaserTile } from "@/net/generated/world_api";
import type { Scene } from "phaser";

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
  const controller = new PlayerMovementController({} as Scene);
  controller.buildCollisionMap([
    tile(1, 10, 0, 1, 0x2c),
    tile(2, 10, 1, 0, 0x37),
    tile(3, 10, 2, 1),
    tile(4, 9, 0, 1),
    tile(5, 9, 1, 1),
    tile(6, 9, 2, 1),
  ]);
  controller.setPlayer(1, 10, 0, UNIFIED_OVERWORLD_MAP_ID, mapRenderer);
  return { controller, updates };
}

describe("PlayerMovementController ledges", () => {
  test("WASD jumps directly over a valid ledge instead of pathing around to the landing tile", () => {
    const { controller, updates } = buildLedgeController();

    expect(controller.handleKeyboardMove("DOWN")).toBe(true);

    expect(updates).toHaveLength(1);
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

  test("clicking a ledge landing tile starts with the one-way jump, not the walk-around route", () => {
    const { controller, updates } = buildLedgeController();

    controller.handleTileClick(10 * TILE_SIZE + 8, 2 * TILE_SIZE + 8);

    expect(updates).toHaveLength(1);
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
