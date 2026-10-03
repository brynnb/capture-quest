import { afterEach } from "vitest";
import { CorrelatedResponseError } from "../services/CorrelatedRequest";
import * as movement from "../services/PlayerMovementService";
vi.mock("../services/PlayerMovementService", () => ({
  requestPlayerFacing: vi.fn(async () => ({ success: true, requestId: "face", mapId: 9999, x: 10, y: 0, direction: "LEFT" })),
  requestPlayerStep: vi.fn(async () => ({ success: true, requestId: "accepted", stepToken: "issued", mapId: 9999, x: 10, y: 2, direction: "DOWN", ledgeJump: true })),
  completePlayerStep: vi.fn(async () => ({ success: true, requestId: "complete", mapId: 9999, x: 10, y: 2, direction: "DOWN" })),
  readOwnedPlayerPosition: vi.fn(async () => ({ success: true, requestId: "owned", mapId: 9999, x: 9, y: 0, direction: "LEFT", serverMovementPending: false })),
}));
afterEach(() => vi.clearAllMocks());
import { describe, expect, test, vi } from "vitest";
import { PlayerMovementController } from "./PlayerMovementController";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import type { PhaserTile } from "@/net/generated/world_api";
import type { Scene } from "phaser";
import type { MapRenderer } from "../renderers/MapRenderer";
import { NetworkBridge } from "@/net/NetworkBridge";
import * as OpCodes from "@/net/generated/opcodes";

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
    snapActorPosition: vi.fn(),
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
  return { controller, updates, visualMovement: movementController, mapRenderer };
}

describe("PlayerMovementController ledges", () => {
  test("a server snap updates movement context without echoing a position write", () => {
    const send = vi.spyOn(NetworkBridge, "send");
    const { controller } = buildLedgeController();
    controller.handleKeyboardMove("DOWN");
    controller.onStepComplete(1, 8, 9, "UP", "snap");
    expect(controller.getCurrentPosition()).toEqual({ x: 8, y: 9 });
    expect(controller.getCurrentDirection()).toBe("UP");
    expect(controller.getIsMoving()).toBe(false);
    expect(send).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
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
      snapActorPosition: vi.fn(),
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
    controller.onStepComplete(1, 1, 0, "RIGHT", "serverStep");

    expect(controller.getCurrentDirection()).toBe("RIGHT");
  });
});

describe("issued player movement lifecycle", () => {
  test("animation waits for acceptance and completion sends only the issued token", async () => {
    const { controller, updates } = buildLedgeController();
    const legacy = vi.spyOn(NetworkBridge, "send");
    controller.handleKeyboardMove("DOWN");
    expect(updates).toHaveLength(0);
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    expect(movement.requestPlayerStep).toHaveBeenCalledWith({ mapId: 9999, fromX: 10, fromY: 0, direction: "DOWN" }, expect.any(AbortSignal));
    controller.onStepComplete(1, 10, 2, "DOWN");
    await vi.waitFor(() => expect(movement.completePlayerStep).toHaveBeenCalledWith("issued", expect.any(AbortSignal)));
    expect(legacy).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
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
  const legacy = vi.spyOn(NetworkBridge, "send");
  controller.handleKeyboardMove("DOWN");
  await vi.waitFor(() => expect(updates).toHaveLength(1));
  controller.stopMovement();
  controller.onStepComplete(1, 10, 2, "DOWN");
  await vi.waitFor(() => expect(movement.completePlayerStep).toHaveBeenCalledWith("issued", expect.any(AbortSignal)));
  expect(legacy).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
  legacy.mockRestore();
  controller.clear();
});

describe("issued movement exclusivity", () => {
  test("discarded future input cannot replace an issued animation or pending completion", async () => {
    let complete!: (value: Awaited<ReturnType<typeof movement.completePlayerStep>>) => void;
    vi.mocked(movement.completePlayerStep).mockImplementationOnce(() => new Promise((resolve) => { complete = resolve; }));
    const { controller, updates } = buildLedgeController();
    controller.handleKeyboardMove("DOWN");
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    controller.stopMovement();
    expect(controller.getIsMoving()).toBe(true);
    expect(controller.handleKeyboardMove("LEFT")).toBe(false);
    controller.handleTileClick(9 * TILE_SIZE + 8, 8);
    expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
    controller.onStepComplete(1, 10, 2, "DOWN");
    await vi.waitFor(() => expect(movement.completePlayerStep).toHaveBeenCalledTimes(1));
    expect(controller.handleKeyboardMove("LEFT")).toBe(false);
    controller.handleTileClick(9 * TILE_SIZE + 8, 2 * TILE_SIZE + 8);
    expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
    complete({ success: true, requestId: "complete", mapId: 9999, x: 10, y: 2, direction: "DOWN" });
    await vi.waitFor(() => expect(controller.getIsMoving()).toBe(false));
    controller.clear();
  });
});


describe("owned-source facing", () => {
  test("turning sends an expected source through facing without a position report", async () => {
    const { controller } = buildLedgeController();
    const legacy = vi.spyOn(NetworkBridge, "send");
    expect(controller.faceTile(9, 0)).toBe(true);
    expect(movement.requestPlayerFacing).toHaveBeenCalledWith({ mapId: 9999, fromX: 10, fromY: 0, direction: "LEFT" }, expect.any(AbortSignal));
    expect(controller.getCurrentPosition()).toEqual({ x: 10, y: 0 });
    expect(legacy).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
    controller.clear();
    legacy.mockRestore();
  });

  test("pending repeated facing coalesces and scene retirement cancels its listener", async () => {
    let settle!: (value: Awaited<ReturnType<typeof movement.requestPlayerFacing>>) => void;
    vi.mocked(movement.requestPlayerFacing).mockImplementationOnce(() => new Promise((resolve) => { settle = resolve; }));
    const { controller } = buildLedgeController();
    controller.faceTile(9, 0);
    controller.faceTile(9, 0);
    expect(movement.requestPlayerFacing).toHaveBeenCalledTimes(1);
    const signal = vi.mocked(movement.requestPlayerFacing).mock.calls[0][1]!;
    expect(signal.aborted).toBe(false);
    controller.clear();
    expect(signal.aborted).toBe(true);
    settle({ serverMovementPending: false, success: true, requestId: "face", mapId: 9999, x: 10, y: 0, direction: "LEFT" });
    await Promise.resolve();
    expect(controller.getIsMoving()).toBe(false);
  });
});


test("a rejected facing reconciles facing only and never snaps or writes position", async () => {
  const owned = { success: false as const, requestId: "face", error: "rejected", mapId: 9999, x: 10, y: 0, direction: "UP" };
  vi.mocked(movement.requestPlayerFacing).mockRejectedValueOnce(new CorrelatedResponseError(owned));
  const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
  const legacy = vi.spyOn(NetworkBridge, "send");
  const { controller, updates, visualMovement } = buildLedgeController();
  controller.faceTile(9, 0);
  await vi.waitFor(() => expect(controller.getCurrentDirection()).toBe("UP"));
  expect(controller.getCurrentPosition()).toEqual({ x: 10, y: 0 });
  expect(visualMovement.handleDirectionUpdate).toHaveBeenLastCalledWith(1, "UP");
  expect(updates).toHaveLength(0);
  expect(legacy).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
  controller.clear();
  legacy.mockRestore();
  warn.mockRestore();
});


test("committed server path projection stays busy until its final point and never echoes position", () => {
  const { controller } = buildLedgeController();
  const legacy = vi.spyOn(NetworkBridge, "send");
  controller.beginServerMovement(false);
  controller.onStepComplete(1, 10, 1, "DOWN", "serverStep");
  expect(controller.getCurrentPosition()).toEqual({ x: 10, y: 1 });
  expect(controller.getIsMoving()).toBe(true);
  expect(controller.handleKeyboardMove("LEFT")).toBe(false);
  controller.stopMovement();
  expect(controller.getIsMoving()).toBe(true);
  controller.beginServerMovement(true);
  controller.onStepComplete(1, 10, 2, "DOWN", "serverStep");
  expect(controller.getCurrentPosition()).toEqual({ x: 10, y: 2 });
  expect(controller.getIsMoving()).toBe(false);
  expect(legacy).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
  controller.clear();
  legacy.mockRestore();
});


test("facing acceptance reserves queued server movement before its first published point", async () => {
  let settle!: (value: Awaited<ReturnType<typeof movement.requestPlayerFacing>>) => void;
  vi.mocked(movement.requestPlayerFacing).mockImplementationOnce(() => new Promise((resolve) => { settle = resolve; }));
  const { controller } = buildLedgeController();
  controller.faceTile(9, 0);
  expect(controller.handleKeyboardMove("DOWN")).toBe(false);
  expect(controller.requestMoveTo(9, 0)).toBe(false);
  controller.handleTileClick(9 * TILE_SIZE + 8, 8);
  expect(movement.requestPlayerStep).not.toHaveBeenCalled();
  settle({ serverMovementPending: true, success: true, requestId: "face", mapId: 9999, x: 10, y: 0, direction: "LEFT" });
  await vi.waitFor(() => expect(controller.getIsMoving()).toBe(true));
  expect(controller.handleKeyboardMove("DOWN")).toBe(false);
  expect(controller.requestMoveTo(9, 0)).toBe(false);
  expect(movement.requestPlayerStep).not.toHaveBeenCalled();
  controller.beginServerMovement(true);
  controller.onStepComplete(1, 9, 0, "LEFT", "serverStep");
  expect(controller.getIsMoving()).toBe(false);
  controller.clear();
});


test("a rejected ordinary step preserves an owned unfinished server path", async () => {
  const error = { success: false as const, requestId: "busy", error: "server path busy", mapId: 9999, x: 10, y: 0, direction: "DOWN", serverMovementPending: true };
  vi.mocked(movement.requestPlayerStep).mockRejectedValueOnce(new CorrelatedResponseError(error));
  const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const { controller } = buildLedgeController();
  controller.handleKeyboardMove("DOWN");
  await vi.waitFor(() => expect(log).toHaveBeenCalled());
  expect(controller.getIsMoving()).toBe(true);
  expect(controller.handleKeyboardMove("LEFT")).toBe(false);
  expect(controller.requestMoveTo(9, 0)).toBe(false);
  controller.clear(); log.mockRestore();
});


test("an unissued ordinary animation cannot acknowledge or trigger arrival effects", async () => {
  const { controller } = buildLedgeController();
  const arrived = vi.fn(); controller.setArrivalCallback(arrived);
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const send = vi.spyOn(NetworkBridge, "send");
  controller.onStepComplete(1, 10, 2, "DOWN");
  await vi.waitFor(() => expect(error).toHaveBeenCalledWith("[PlayerMovement] Movement request failed", expect.any(Error)));
  expect(movement.completePlayerStep).not.toHaveBeenCalled();
  expect(send).not.toHaveBeenCalledWith(expect.anything(), OpCodes.PhaserPlayerPositionUpdate);
  expect(arrived).not.toHaveBeenCalled();
  expect(controller.getIsMoving()).toBe(false);
  controller.clear(); error.mockRestore(); send.mockRestore();
});

test("Surf success projects a committed step and releases input only at completion", () => {
  const { controller, updates } = buildLedgeController();
  controller.applySurfingSuccess(9, 0, 9999, "LEFT");
  expect(updates).toEqual([[1, 10, 0, 9, 0, "LEFT", undefined, { serverControlled: true }]]);
  expect(controller.handleKeyboardMove("DOWN")).toBe(false);
  controller.onStepComplete(1, 9, 0, "LEFT", "serverStep");
  expect(controller.getCurrentPosition()).toEqual({ x: 9, y: 0 });
  expect(controller.getIsMoving()).toBe(false);
  expect(movement.completePlayerStep).not.toHaveBeenCalled();
  controller.clear();
});

test("committed warp exit is a server projection and retirement ignores late completion", async () => {
  const { controller, updates, mapRenderer, visualMovement } = buildLedgeController();
  let idle!: () => void;
  Object.assign(visualMovement, { getActorState: () => ({}) });
  Object.assign(mapRenderer, { getActorSprite: () => ({}), waitForActorIdle: () => new Promise<void>(resolve => { idle = resolve; }) });
  const done = controller.animateCommittedStepToward(9, 0, "LEFT");
  expect(updates).toEqual([[1, 10, 0, 9, 0, "LEFT", undefined, { serverControlled: true }]]);
  expect(controller.getIsMoving()).toBe(true);
  controller.clear();
  controller.syncPosition(20, 20);
  idle(); await done;
  expect(controller.getCurrentPosition()).toEqual({ x: 20, y: 20 });
  expect(movement.completePlayerStep).not.toHaveBeenCalled();
});


test("a replayed receipt reconciles current ownership and does not trigger another arrival", async () => {
  vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "receipt", replayed: true, mapId: 9999, x: 10, y: 2, direction: "DOWN" });
  const { controller, mapRenderer } = buildLedgeController();
  const arrived = vi.fn(); controller.setArrivalCallback(arrived);
  controller.handleKeyboardMove("DOWN");
  await vi.waitFor(() => expect(movement.requestPlayerStep).toHaveBeenCalled());
  controller.onStepComplete(1, 10, 2, "DOWN");
  await vi.waitFor(() => expect(mapRenderer.snapActorPosition).toHaveBeenCalledWith(1, 9, 0, "LEFT"));
  expect(movement.readOwnedPlayerPosition).toHaveBeenCalledWith(expect.any(AbortSignal), "issued");
  expect(controller.getCurrentPosition()).toEqual({ x: 9, y: 0 });
  expect(controller.getIsMoving()).toBe(false);
  expect(arrived).not.toHaveBeenCalled();
  controller.clear();
});

test("receipt recovery preserves a current server path instead of resuming user input", async () => {
  vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "receipt", replayed: true, mapId: 9999, x: 10, y: 2, direction: "DOWN" });
  vi.mocked(movement.readOwnedPlayerPosition).mockResolvedValueOnce({ success: true, requestId: "owned", mapId: 9999, x: 9, y: 0, direction: "LEFT", serverMovementPending: true });
  const { controller, mapRenderer } = buildLedgeController();
  controller.handleKeyboardMove("DOWN");
  await vi.waitFor(() => expect(movement.requestPlayerStep).toHaveBeenCalled());
  controller.onStepComplete(1, 10, 2, "DOWN");
  await vi.waitFor(() => expect(mapRenderer.snapActorPosition).toHaveBeenCalled());
  expect(controller.getIsMoving()).toBe(true);
  expect(controller.handleKeyboardMove("DOWN")).toBe(false);
  controller.clear();
});

test("retirement ignores a late receipt recovery and failed recovery keeps input locked", async () => {
  vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "receipt", replayed: true, mapId: 9999, x: 10, y: 2, direction: "DOWN" });
  let recover!: (value: Awaited<ReturnType<typeof movement.readOwnedPlayerPosition>>) => void;
  vi.mocked(movement.readOwnedPlayerPosition).mockImplementationOnce(() => new Promise(resolve => { recover = resolve; }));
  const { controller, mapRenderer } = buildLedgeController();
  controller.handleKeyboardMove("DOWN");
  await vi.waitFor(() => expect(movement.requestPlayerStep).toHaveBeenCalled());
  controller.onStepComplete(1, 10, 2, "DOWN");
  await vi.waitFor(() => expect(movement.readOwnedPlayerPosition).toHaveBeenCalled());
  controller.clear(); controller.syncPosition(20, 20);
  recover({ success: true, requestId: "late", mapId: 9999, x: 9, y: 0, direction: "LEFT", serverMovementPending: false });
  await Promise.resolve(); await Promise.resolve();
  expect(mapRenderer.snapActorPosition).not.toHaveBeenCalled();
  expect(controller.getCurrentPosition()).toEqual({ x: 20, y: 20 });

  vi.clearAllMocks();
  vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "receipt", replayed: true, mapId: 9999, x: 10, y: 2, direction: "DOWN" });
  vi.mocked(movement.readOwnedPlayerPosition).mockRejectedValueOnce(new Error("recovery unavailable"));
  const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const next = buildLedgeController().controller;
  next.handleKeyboardMove("DOWN");
  await vi.waitFor(() => expect(movement.requestPlayerStep).toHaveBeenCalled());
  next.onStepComplete(1, 10, 2, "DOWN");
  await vi.waitFor(() => expect(log).toHaveBeenCalled());
  expect(next.handleKeyboardMove("LEFT")).toBe(false);
  expect(next.requestMoveTo(9, 0)).toBe(false);
  next.clear(); log.mockRestore();
});
