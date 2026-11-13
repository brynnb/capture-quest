vi.mock("../services/BattleCommandService",()=>({suspendBattleCommandForEmergency:vi.fn(()=>()=>{}),recoverBattlePublication:vi.fn(async()=>true),recoverFieldPublication:vi.fn(async()=>true)}));
import * as battlePublication from "../services/BattleCommandService";
import { afterEach, beforeEach } from "vitest";
import { CorrelatedResponseError } from "../services/CorrelatedRequest";
import * as movement from "../services/PlayerMovementService";
vi.mock("../services/PlayerMovementService", () => ({
  readWarpHomeRevision:vi.fn(async()=>({success:true,requestId:"read",characterId:42,current:true,revision:2,moveName:"WARP_HOME",mapId:9999,message:"current"})),
  requestWarpHome:vi.fn(),
  requestElevator:vi.fn(),
  requestCut:vi.fn(),
  requestStateField:vi.fn(),
  requestSurf:vi.fn(),
  requestFishing:vi.fn(),
  requestEscapeRope: vi.fn(),
  requestBicycleState: vi.fn(),
  requestPlayerFacing: vi.fn(async () => ({ success: true, requestId: "face", mapId: 9999, x: 10, y: 0, direction: "LEFT" })),
  requestPlayerStep: vi.fn(async () => ({ success: true, requestId: "accepted", stepToken: "issued", mapId: 9999, x: 10, y: 2, direction: "DOWN", ledgeJump: true })),
  completePlayerStep: vi.fn(async () => ({ success: true, requestId: "complete", mapId: 9999, x: 10, y: 2, direction: "DOWN" })),
  cancelPlayerStep: vi.fn(async () => ({ success: true, cancelled: true, requestId: "cancel", mapId: 9999, x: 10, y: 2, direction: "DOWN" })),
  readOwnedPlayerPosition: vi.fn(async () => ({ success: true, requestId: "owned", mapId: 9999, x: 9, y: 0, direction: "LEFT", serverMovementPending: false })),
}));
vi.mock("../services/GameplayRecoveryService", () => ({ readCurrentGameplayState: vi.fn(), applyGameplayResourceSnapshot: vi.fn(),applyGameplaySnapshot:vi.fn() }));
import * as recovery from "../services/GameplayRecoveryService";
beforeEach(() => useGameScreenStore.getState().setScreen("game"));
afterEach(() => vi.clearAllMocks());
import { describe, expect, test, vi } from "vitest";
import { PlayerMovementController } from "./PlayerMovementController";
import { TILE_SIZE, UNIFIED_OVERWORLD_MAP_ID } from "../constants";
import type { PhaserTile } from "@/net/generated/world_api";
import type { Scene } from "phaser";
import type { MapRenderer } from "../renderers/MapRenderer";
import { NetworkBridge } from "@/net/NetworkBridge";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import { WorldSocket } from "@/net/index";
import useGameStatusStore from "@/stores/GameStatusStore";
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

test("Bicycle uncertain replies read current preference without retrying the setter",async()=>{
 const {controller}=buildLedgeController();
 usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 const request=vi.mocked(movement.requestBicycleState);
 request.mockResolvedValueOnce({success:true,requestId:"before",characterId:42,bicycle:{revision:0,wantsRiding:false,activeRiding:false,forcedRiding:false}});
 request.mockRejectedValueOnce(new Error("timeout"));
 request.mockResolvedValueOnce({success:true,requestId:"current",characterId:42,bicycle:{revision:1,wantsRiding:true,activeRiding:true,forcedRiding:false}});
 await controller.changeBicyclePreference(7);
 expect(request).toHaveBeenCalledTimes(3);
 expect(request.mock.calls[1][2]).toEqual({instanceId:7,wantsRiding:true,revision:0});
 expect(request.mock.calls[2][2]).toBeUndefined();
 expect(useAudioActivityStore.getState().wantsBicycle).toBe(true);
});

test("retired movement controllers ignore late Bicycle state",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 useAudioActivityStore.getState().resetTravelAudio();
 let resolve!:(value:any)=>void;vi.mocked(movement.requestBicycleState).mockImplementationOnce(()=>new Promise(done=>{resolve=done}));
 const pending=controller.changeBicyclePreference(7);controller.retireFieldCommands();
 resolve({success:true,requestId:"late",characterId:42,bicycle:{revision:1,wantsRiding:true,activeRiding:true,forcedRiding:false}});await pending;
 expect(useAudioActivityStore.getState().wantsBicycle).toBe(false);
});

test("character replacement cancels Bicycle admission and ignores a late state", async () => {
  const { controller } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  useAudioActivityStore.getState().resetTravelAudio();
  let resolve!: (value: import("@/net/generated/world_api").BicycleStateResponse) => void;
  const request = vi.mocked(movement.requestBicycleState);
  request.mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const pending = controller.changeBicyclePreference(7);
  const signal = request.mock.calls[0][1];
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 43 });
  expect(signal.aborted).toBe(true);
  resolve({ success: true, requestId: "old-character", characterId: 42,
    bicycle: { revision: 1, wantsRiding: true, activeRiding: true, forcedRiding: false } });
  await pending;
  expect(request).toHaveBeenCalledTimes(1);
  expect(useAudioActivityStore.getState().wantsBicycle).toBe(false);
});

test("Escape Rope uncertain replies project a current coherent read without retry", async () => {
  const { controller, mapRenderer } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  vi.mocked(movement.requestEscapeRope).mockRejectedValueOnce(new Error("timeout"));
  const snapshot = { position: { mapId: 9999, x: 9, y: 0, direction: "LEFT" } } as import("@/net/generated/world_api").GameplayStateResponse;
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce(snapshot);
  await controller.useEscapeRope(7);
  expect(movement.requestEscapeRope).toHaveBeenCalledTimes(1);
  expect(recovery.readCurrentGameplayState).toHaveBeenCalledTimes(1);
  expect(recovery.applyGameplayResourceSnapshot).toHaveBeenCalledWith(snapshot);
  expect(mapRenderer.snapActorPosition).toHaveBeenCalledWith(1, 9, 0, "LEFT");
});

test("retired movement owners ignore late Escape Rope recovery", async () => {
  const { controller } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  vi.mocked(movement.requestEscapeRope).mockResolvedValueOnce({ success: true, requestId: "rope", characterId: 42 });
  let resolve!: (value: import("@/net/generated/world_api").GameplayStateResponse) => void;
  vi.mocked(recovery.readCurrentGameplayState).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const pending = controller.useEscapeRope(7);
  await Promise.resolve();
  controller.retireFieldCommands();
  resolve({ position: { mapId: 59, x: 9, y: 9, direction: "DOWN" } } as import("@/net/generated/world_api").GameplayStateResponse);
  await pending;
  expect(recovery.applyGameplayResourceSnapshot).not.toHaveBeenCalled();
});

test("failed field recovery stays locked through actor snaps and blocks further commands", async () => {
  const { controller } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
  vi.mocked(movement.requestEscapeRope).mockRejectedValueOnce(new Error("timeout"));
  vi.mocked(recovery.readCurrentGameplayState).mockRejectedValueOnce(new Error("read unavailable"));
  await controller.useEscapeRope(7);
  expect(controller.handleKeyboardMove("LEFT")).toBe(false);
  controller.onStepComplete(1, 10, 0, "UP", "snap");
  await Promise.resolve();
  controller.stopMovement(true);
  expect(controller.handleKeyboardMove("LEFT")).toBe(false);
  expect(controller.requestMoveTo(9, 0)).toBe(false);
  await controller.useEscapeRope(7);
  await controller.changeBicyclePreference(8);
  expect(movement.requestEscapeRope).toHaveBeenCalledTimes(1);
  expect(movement.requestBicycleState).not.toHaveBeenCalled();
  expect(movement.requestPlayerStep).not.toHaveBeenCalled();
  controller.clear();
  const replacement = buildLedgeController().controller;
  expect(replacement.handleKeyboardMove("LEFT")).toBe(true);
  replacement.clear();
  warning.mockRestore();
});

test("controller clear aborts its field request and prevents late application", async () => {
  const { controller } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  useAudioActivityStore.getState().resetTravelAudio();
  let resolve!: (value: import("@/net/generated/world_api").BicycleStateResponse) => void;
  const request = vi.mocked(movement.requestBicycleState);
  request.mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const pending = controller.changeBicyclePreference(7);
  const signal = request.mock.calls[0][1];
  controller.clear();
  expect(signal.aborted).toBe(true);
  resolve({ success: true, requestId: "late-clear", characterId: 42,
    bicycle: { revision: 1, wantsRiding: true, activeRiding: true, forcedRiding: false } });
  await pending;
  expect(request).toHaveBeenCalledTimes(1);
  expect(useAudioActivityStore.getState().wantsBicycle).toBe(false);
});

describe("PlayerMovementController ledges", () => {
  test("facing a source tile publishes its direction without changing location",()=>{
    const {controller}=buildLedgeController();
    expect(controller.faceTile(9,0)).toBe(true);
    expect(useGameStatusStore.getState().playerTileContext).toMatchObject({mapId:9999,x:10,y:0,direction:"LEFT"});
  });
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
    expect(movement.requestPlayerStep).toHaveBeenCalledWith({ mapId: 9999, fromX: 10, fromY: 0, direction: "DOWN", lookAhead: [] }, expect.any(AbortSignal));
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
  next.onStepComplete(1, 10, 2, "DOWN", "snap");
  await Promise.resolve();
  expect(next.handleKeyboardMove("LEFT")).toBe(false);
  next.clear(); log.mockRestore();
});


test("unknown facing outcome reads current pose without retrying the turn", async () => {
  const { controller, mapRenderer } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  vi.mocked(movement.requestPlayerFacing).mockRejectedValueOnce(new Error("timeout"));
  const snapshot = { position: { mapId: 9999, x: 9, y: 0, direction: "LEFT", serverMovementPending: false } } as import("@/net/generated/world_api").GameplayStateResponse;
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce(snapshot);
  controller.faceTile(9, 0);
  await vi.waitFor(() => expect(mapRenderer.snapActorPosition).toHaveBeenCalledWith(1, 9, 0, "LEFT"));
  expect(movement.requestPlayerFacing).toHaveBeenCalledTimes(1);
  expect(recovery.readCurrentGameplayState).toHaveBeenCalledTimes(1);
  expect(recovery.applyGameplayResourceSnapshot).toHaveBeenCalledWith(snapshot);
});

test("failed facing recovery remains locked and retired recovery cannot apply", async () => {
  const { controller } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
  vi.mocked(movement.requestPlayerFacing).mockRejectedValueOnce(new Error("timeout"));
  vi.mocked(recovery.readCurrentGameplayState).mockRejectedValueOnce(new Error("read unavailable"));
  controller.faceTile(9, 0);
  await vi.waitFor(() => expect(warning).toHaveBeenCalled());
  controller.stopMovement(true);
  expect(controller.handleKeyboardMove("LEFT")).toBe(false);
  controller.clear();
  warning.mockRestore();

  vi.clearAllMocks();
  const next = buildLedgeController().controller;
  let resolve!: (value: import("@/net/generated/world_api").GameplayStateResponse) => void;
  vi.mocked(movement.requestPlayerFacing).mockRejectedValueOnce(new Error("timeout"));
  vi.mocked(recovery.readCurrentGameplayState).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  next.faceTile(9, 0);
  await vi.waitFor(() => expect(recovery.readCurrentGameplayState).toHaveBeenCalled());
  next.clear();
  resolve({ position: { mapId: 9999, x: 9, y: 0, direction: "LEFT" } } as import("@/net/generated/world_api").GameplayStateResponse);
  await Promise.resolve(); await Promise.resolve();
  expect(recovery.applyGameplayResourceSnapshot).not.toHaveBeenCalled();
});


test("a facing acceptance from a replaced character cannot reserve movement", async () => {
  const { controller } = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
  let resolve!: (value: Awaited<ReturnType<typeof movement.requestPlayerFacing>>) => void;
  vi.mocked(movement.requestPlayerFacing).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  controller.faceTile(9, 0);
  usePlayerCharacterStore.getState().setCharacterProfile({ id: 43 });
  resolve({ success: true, requestId: "old-owner", mapId: 9999, x: 10, y: 0, direction: "LEFT", serverMovementPending: true });
  await Promise.resolve(); await Promise.resolve();
  expect(controller.getIsMoving()).toBe(false);
  expect(recovery.readCurrentGameplayState).not.toHaveBeenCalled();
  controller.clear();
});

test("position read view changes for pose/facing without requiring route retirement",()=>{
 const {controller}=buildLedgeController();
 const generation=controller.getPositionGeneration();const initial=controller.capturePositionView();
 expect(controller.capturePositionView()).toBe(initial);
 controller.syncPosition(9,1);expect(controller.getPositionGeneration()).toBe(generation);expect(controller.capturePositionView()).not.toBe(initial);
 const moved=controller.capturePositionView();controller.syncDirection("LEFT");expect(controller.getPositionGeneration()).toBe(generation);expect(controller.capturePositionView()).not.toBe(moved);
});

test("fishing uncertainty reconciles once without a second mutation",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 let reject!:(error:Error)=>void;vi.mocked(movement.requestFishing).mockImplementationOnce(()=>new Promise((_ok,no)=>{reject=no}));
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{},battle:{battleId:"committed"}} as any);
 const pending=controller.fish(7);await controller.fish(7);expect(movement.requestFishing).toHaveBeenCalledOnce();reject(new Error("lost reply"));await pending;
 expect(movement.requestFishing).toHaveBeenCalledOnce();expect(battlePublication.recoverFieldPublication).toHaveBeenCalledTimes(1);expect(battlePublication.recoverFieldPublication).toHaveBeenCalledWith(expect.any(AbortSignal));
});
test("retired fishing ignores a late reply and does not reconcile the old scene",async()=>{
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{}} as any);
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});let resolve!:(reply:any)=>void;
 vi.mocked(movement.requestFishing).mockImplementationOnce(()=>new Promise(done=>{resolve=done}));const pending=controller.fish(7);await Promise.resolve();controller.retireFieldCommands();
 resolve({success:true,requestId:"old",characterId:42,instanceId:7,hooked:true,message:"Oh! A bite!"});await pending;expect(battlePublication.recoverFieldPublication).not.toHaveBeenCalled();
});

test("failed fishing recovery keeps later field mutations inadmissible",async()=>{
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{}} as any);
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(movement.requestFishing).mockResolvedValueOnce({success:true,requestId:"done",characterId:42,instanceId:7,hooked:true,message:"Oh! A bite!"});
 vi.mocked(battlePublication.recoverFieldPublication).mockResolvedValueOnce(false);
 await controller.fish(7);await controller.fish(7);expect(movement.requestFishing).toHaveBeenCalledTimes(1);
});


test.each(["screen", "transport"])("shared field slot retires Bicycle on %s replacement", async (kind) => {
  const {controller} = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({id:42});
  useAudioActivityStore.getState().resetTravelAudio();
  let resolve!: (reply: any) => void;
  const request = vi.mocked(movement.requestBicycleState);
  request.mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const pending = controller.changeBicyclePreference(7);
  const signal = request.mock.calls[0][1];
  if (kind === "screen") useGameScreenStore.getState().setScreen("title");
  else WorldSocket.close(false);
  expect(signal.aborted).toBe(true);
  resolve({success:true, requestId:"retired", characterId:42, bicycle:{revision:1,wantsRiding:true,activeRiding:true,forcedRiding:false}});
  await pending;
  expect(request).toHaveBeenCalledOnce();
  expect(useAudioActivityStore.getState().wantsBicycle).toBe(false);
});

test.each(["current", "facing-settled", "position", "character", "scene", "screen", "transport"])("SURF choice respects %s source lifetime", (kind) => {
  const {controller} = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({id:42});
  vi.spyOn(controller as any, "partyKnowsSurf").mockReturnValue(true);
  let choose!: (yes:boolean) => void;
  const choice = vi.spyOn(usePokemonDialogueStore.getState(), "showChoice").mockImplementation((_text, callback) => {choose = callback;});
  const send = vi.spyOn(controller, "surf").mockResolvedValue();
  if (kind === "facing-settled") (controller as any).facingAbort = new AbortController();
  (controller as any).promptSurfToWater(9,0,"LEFT");
  if (kind === "facing-settled") (controller as any).facingAbort = null;
  if (kind === "position") controller.syncPosition(9,1);
  if (kind === "character") usePlayerCharacterStore.getState().setCharacterProfile({id:43});
  if (kind === "scene") controller.retireFieldCommands();
  if (kind === "screen") useGameScreenStore.getState().setScreen("title");
  if (kind === "transport") WorldSocket.close(false);
  choose(true);
  if (kind === "current" || kind === "facing-settled") expect(send).toHaveBeenCalledOnce();
  else expect(send).not.toHaveBeenCalled();
  choice.mockRestore(); send.mockRestore();
});


test("SURF unknown delivery reconciles owned pose and battle once without resend", async () => {
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{surf:3}} as any);
  const {controller} = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({id:42});
  let reject!: (error:Error) => void;
  vi.mocked(movement.requestSurf).mockImplementationOnce(() => new Promise((_done, no) => {reject=no;}));
  const pending=controller.surf(9,0,"LEFT");
  await controller.surf(9,0,"LEFT");
  expect(movement.requestSurf).toHaveBeenCalledOnce();
  reject(new Error("lost reply")); await pending;
  expect(battlePublication.recoverFieldPublication).toHaveBeenCalledWith(expect.any(AbortSignal));
  expect(movement.requestSurf).toHaveBeenCalledOnce();
});

test("retired SURF ignores a late reply and cannot recover an old scene", async () => {
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{surf:3}} as any);
  const {controller} = buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({id:42});
  let resolve!: (reply:any)=>void;
  vi.mocked(movement.requestSurf).mockImplementationOnce(()=>new Promise(done=>{resolve=done;}));
  const pending=controller.surf(9,0,"LEFT");
  await Promise.resolve();
  controller.retireFieldCommands();
  resolve({success:true,requestId:"late",characterId:42,encounter:false,blackout:false});
  await pending;
  expect(battlePublication.recoverFieldPublication).not.toHaveBeenCalled();
});

test("failed SURF recovery keeps subsequent field mutations locked", async () => {
  vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{surf:3}} as any);
  const {controller}=buildLedgeController();
  usePlayerCharacterStore.getState().setCharacterProfile({id:42});
  vi.mocked(movement.requestSurf).mockRejectedValueOnce(new Error("timeout"));
  vi.mocked(battlePublication.recoverFieldPublication).mockResolvedValueOnce(false);
  await controller.surf(9,0,"LEFT"); await controller.surf(9,0,"LEFT");
  expect(movement.requestSurf).toHaveBeenCalledOnce();
});


test("state field uncertainty reads current state without resending",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{"field-state":2}} as any).mockResolvedValueOnce({fieldState:{mapId:9999,strength:true,flash:false},position:{mapId:9999,x:10,y:0,direction:"UP",serverMovementPending:false}} as any);
 vi.mocked(movement.requestStateField).mockRejectedValueOnce(new Error("lost reply"));
 await controller.activateFieldState("STRENGTH");expect(movement.requestStateField).toHaveBeenCalledOnce();expect(vi.mocked(movement.requestStateField).mock.calls[0][0]).toMatchObject({characterId:42,commandRevision:2,moveName:"STRENGTH"});expect(recovery.readCurrentGameplayState).toHaveBeenCalledTimes(2);
});
test("retired state field ignores late reply and cannot recover an old scene",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{}} as any);let resolve!:(reply:any)=>void;
 vi.mocked(movement.requestStateField).mockImplementationOnce(()=>new Promise(done=>{resolve=done;}));
 const pending=controller.activateFieldState("FLASH");await Promise.resolve();controller.retireFieldCommands();resolve({success:true,requestId:"old",characterId:42,moveName:"FLASH",mapId:9999,message:"Ready"});await pending;expect(recovery.readCurrentGameplayState).toHaveBeenCalledOnce();
});


test("historical field success cannot present activation over inactive owned state",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 const open=vi.spyOn(usePokemonDialogueStore.getState(),"openDialogue");
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{}} as any).mockResolvedValueOnce({fieldState:{mapId:9999,strength:false,flash:false},position:{mapId:9999,x:10,y:0,direction:"UP",serverMovementPending:false}} as any);
 vi.mocked(movement.requestStateField).mockResolvedValueOnce({success:true,requestId:"old",characterId:42,moveName:"FLASH",mapId:9999,message:"PIKACHU used FLASH."});
 await controller.activateFieldState("FLASH");expect(open).toHaveBeenCalledWith(["FLASH state refreshed for your current location."]);expect(movement.requestStateField).toHaveBeenCalledOnce();open.mockRestore();
});


test("CUT unknown delivery reconciles one current scoped tile without resend",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});const reconcile=vi.fn(async()=>true);controller.setTileReconciler(reconcile);
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({cutScope:"visit",fieldCommands:{cut:0}} as any).mockResolvedValueOnce({cutScope:"visit",position:{mapId:9999}} as any);
 vi.mocked(movement.requestCut).mockRejectedValueOnce(new Error("lost reply"));await controller.cut(9,0,"LEFT");expect(movement.requestCut).toHaveBeenCalledOnce();expect(reconcile).toHaveBeenCalledWith(9999,9,0,"visit",expect.any(AbortSignal));
});
test("CUT unknown result cannot reconcile a replacement visit",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});const reconcile=vi.fn(async()=>true);controller.setTileReconciler(reconcile);
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({cutScope:"old",fieldCommands:{}} as any).mockResolvedValueOnce({cutScope:"new",position:{mapId:9999}} as any);
 vi.mocked(movement.requestCut).mockRejectedValueOnce(new Error("lost reply"));await controller.cut(9,0,"LEFT");expect(reconcile).not.toHaveBeenCalled();expect(movement.requestCut).toHaveBeenCalledOnce();
});


test("CUT discarded projection cannot present a historical success",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});controller.setTileReconciler(async()=>false);
 const open=vi.spyOn(usePokemonDialogueStore.getState(),"openDialogue");
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({cutScope:"visit",fieldCommands:{}} as any).mockResolvedValueOnce({cutScope:"visit",position:{mapId:9999}} as any);
 vi.mocked(movement.requestCut).mockResolvedValueOnce({success:true,requestId:"done",characterId:42,moveName:"CUT",mapId:9999,cutScope:"visit",targetX:9,targetY:0,message:"Used CUT!"});
 await controller.cut(9,0,"LEFT");expect(open).not.toHaveBeenCalled();open.mockRestore();
});


test("ability travel uncertainty uses shared owned publication without resend",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{travel:2}} as any);vi.mocked(movement.requestStateField).mockRejectedValueOnce(new Error("lost reply"));
 await controller.travelByMove("TELEPORT");expect(movement.requestStateField).toHaveBeenCalledOnce();expect(vi.mocked(movement.requestStateField).mock.calls[0][0]).toMatchObject({moveName:"TELEPORT",characterId:42,commandRevision:2});expect(battlePublication.recoverFieldPublication).toHaveBeenCalledWith(expect.any(AbortSignal));
});
test("retired ability travel ignores old acknowledgement and recovery",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{}} as any);let resolve!:(reply:any)=>void;
 vi.mocked(movement.requestStateField).mockImplementationOnce(()=>new Promise(done=>{resolve=done;}));const pending=controller.travelByMove("DIG");await Promise.resolve();controller.retireFieldCommands();resolve({success:true,requestId:"old",characterId:42,moveName:"DIG",mapId:50,message:"Dug out"});await pending;expect(battlePublication.recoverFieldPublication).not.toHaveBeenCalled();
});

test("failed ability travel recovery keeps subsequent field mutations inadmissible",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{}} as any);vi.mocked(movement.requestStateField).mockRejectedValueOnce(new Error("lost reply"));vi.mocked(battlePublication.recoverFieldPublication).mockResolvedValueOnce(false);
 await controller.travelByMove("TELEPORT");await controller.travelByMove("TELEPORT");expect(movement.requestStateField).toHaveBeenCalledOnce();expect(battlePublication.recoverFieldPublication).toHaveBeenCalledOnce();
});

test("FLY forwards catalog identity separately from expected source",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{travel:3}} as any);vi.mocked(movement.requestStateField).mockRejectedValueOnce(new Error("lost reply"));
 await controller.travelByMove("FLY",{mapId:60,x:3,y:4});expect(movement.requestStateField).toHaveBeenCalledOnce();expect(vi.mocked(movement.requestStateField).mock.calls[0][0]).toMatchObject({mapId:9999,destinationMapId:60,targetX:3,targetY:4,moveName:"FLY",characterId:42,commandRevision:3});expect(battlePublication.recoverFieldPublication).toHaveBeenCalledOnce();
});


test("elevator uncertain selection shares travel recovery without resending",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(movement.requestElevator).mockResolvedValueOnce({success:true,requestId:"list",characterId:42,kind:"list",mapId:9999,floors:[{floorMapId:60,floorLabel:"2F",destX:3,destY:4}],message:""}).mockRejectedValueOnce(new Error("lost reply"));
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{travel:3}} as any);
 await controller.useElevator(async()=>60);expect(movement.requestElevator).toHaveBeenCalledTimes(2);expect(vi.mocked(movement.requestElevator).mock.calls[1][0]).toMatchObject({floorMapId:60,mapId:9999,fromX:10,fromY:0,characterId:42,commandRevision:3});expect(battlePublication.recoverFieldPublication).toHaveBeenCalledOnce();
});

test("retired elevator listing cannot open a menu or send a selection",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});let resolve!:(reply:any)=>void;
 vi.mocked(movement.requestElevator).mockImplementationOnce(()=>new Promise(done=>{resolve=done}));const choose=vi.fn();const pending=controller.useElevator(choose);
 controller.retireFieldCommands();resolve({success:true,requestId:"old",characterId:42,kind:"list",mapId:9999,floors:[],message:""});await pending;
 expect(choose).not.toHaveBeenCalled();expect(movement.requestElevator).toHaveBeenCalledOnce();expect(battlePublication.recoverFieldPublication).not.toHaveBeenCalled();
});

test("cancelled elevator selector sends no mutation and performs no recovery",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(movement.requestElevator).mockResolvedValueOnce({success:true,requestId:"list",characterId:42,kind:"list",mapId:9999,floors:[{floorMapId:60,floorLabel:"2F",destX:3,destY:4}],message:""});
 await controller.useElevator(async()=>undefined);expect(movement.requestElevator).toHaveBeenCalledOnce();expect(battlePublication.recoverFieldPublication).not.toHaveBeenCalled();
});


test("elevator list read retires after moving away and back to identical coordinates",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 useGameStatusStore.setState({playerTileContext:{mapId:9999,x:10,y:0,direction:"UP"}});
 let resolve!:(reply:any)=>void;vi.mocked(movement.requestElevator).mockImplementationOnce(()=>new Promise(done=>{resolve=done}));const choose=vi.fn();const pending=controller.useElevator(choose);
 const signal=vi.mocked(movement.requestElevator).mock.calls[0][2]!;
 useGameStatusStore.setState({playerTileContext:{mapId:9999,x:11,y:0,direction:"UP"}});useGameStatusStore.setState({playerTileContext:{mapId:9999,x:10,y:0,direction:"UP"}});
 expect(signal.aborted).toBe(true);resolve({success:true,requestId:"old",characterId:42,kind:"list",mapId:9999,floors:[{floorMapId:60,floorLabel:"2F",destX:3,destY:4}],message:""});await pending;
 expect(choose).not.toHaveBeenCalled();expect(movement.requestElevator).toHaveBeenCalledOnce();expect(battlePublication.recoverFieldPublication).not.toHaveBeenCalled();
});


test("emergency home rescues movement-recovery lock through owned recovery without resend",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{travel:2}} as any);vi.mocked(movement.readWarpHomeRevision).mockResolvedValueOnce({success:true,requestId:"read",characterId:42,current:true,revision:3,moveName:"WARP_HOME",mapId:9999,message:"current"});
 vi.mocked(movement.requestStateField).mockRejectedValueOnce(new Error("lost travel reply"));vi.mocked(battlePublication.recoverFieldPublication).mockResolvedValueOnce(false);
 await controller.travelByMove("TELEPORT");vi.mocked(movement.requestWarpHome).mockRejectedValueOnce(new Error("lost home reply"));
 await controller.warpHome();expect(movement.requestWarpHome).toHaveBeenCalledOnce();expect(vi.mocked(movement.requestWarpHome).mock.calls[0][0]).toEqual({characterId:42,commandRevision:3});
 expect(battlePublication.suspendBattleCommandForEmergency).toHaveBeenCalledOnce();expect(battlePublication.recoverFieldPublication).toHaveBeenLastCalledWith(expect.any(AbortSignal),true);
});

test("emergency home replaces an older field owner but rejects duplicate home clicks",async()=>{
 const {controller}=buildLedgeController();usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 vi.mocked(recovery.readCurrentGameplayState).mockResolvedValueOnce({fieldCommands:{travel:2}} as any);
 let finishOld!:(reply:any)=>void;vi.mocked(movement.requestStateField).mockImplementationOnce(()=>new Promise(done=>{finishOld=done}));const old=controller.travelByMove("TELEPORT");await Promise.resolve();
 let finishHome!:(reply:any)=>void;vi.mocked(movement.requestWarpHome).mockImplementationOnce(()=>new Promise(done=>{finishHome=done}));const home=controller.warpHome();await Promise.resolve();await controller.warpHome();
 expect(vi.mocked(movement.requestStateField).mock.calls[0][1]!.aborted).toBe(true);expect(movement.requestWarpHome).toHaveBeenCalledOnce();
 finishOld({success:true,requestId:"old",characterId:42,moveName:"TELEPORT",mapId:50,message:"old"});await old;
 finishHome({success:true,requestId:"home",characterId:42,current:false,revision:3,moveName:"WARP_HOME",mapId:9999,message:"home"});await home;expect(battlePublication.recoverFieldPublication).toHaveBeenCalledOnce();
});


describe("continuous issued movement", () => {
  beforeEach(() => {
    usePokemonDialogueStore.getState().resetDialogueState();
    vi.mocked(movement.requestPlayerStep).mockReset();
    vi.mocked(movement.completePlayerStep).mockReset();
    vi.mocked(movement.cancelPlayerStep).mockReset();
  });
  function step(x: number): import("@/net/generated/protocol").PlayerStepResponse {
    return { success: true, requestId: `step-${x}`, stepToken: `issued-${x}`, mapId: 9999, x, y: 0, direction: "RIGHT", ledgeJump: false };
  }
  function straightController() {
    const updates: { x: number; at: number }[] = [];
    const renderer = {
      updateActorPosition: vi.fn((_id: number, _fromX: number, _fromY: number, x: number) => updates.push({ x, at: Date.now() })),
      getMovementController: () => ({ handleDirectionUpdate: vi.fn() }),
      snapActorPosition: vi.fn(),
    };
    const controller = new PlayerMovementController({ events: { emit: vi.fn(), once: vi.fn() } } as unknown as Scene);
    controller.buildCollisionMap(Array.from({ length: 6 }, (_, x) => tile(x + 1, x, 0, 1)));
    controller.setPlayer(1, 0, 0, 9999, renderer as unknown as MapRenderer);
    return { controller, updates };
  }
  function delayedCompletion(x: number, next?: number) {
    return new Promise<import("@/net/generated/protocol").PlayerStepCompleteResponse>(resolve => setTimeout(() => resolve({
      success: true, requestId: `complete-${x}`, mapId: 9999, x, y: 0, direction: "RIGHT", ...(next ? { nextStep: step(next) } : {}),
    }), 125));
  }

  test("click paths overlap 125ms replies with animations and do not reauthorize each tile", async () => {
    vi.useFakeTimers();
    const { controller, updates } = straightController();
    try {
      vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(step(1));
      for (let x = 1; x <= 3; x++) vi.mocked(movement.completePlayerStep).mockImplementationOnce(() => delayedCompletion(x, x < 3 ? x + 1 : undefined));
      controller.requestMoveTo(3, 0);
      await vi.advanceTimersByTimeAsync(0);
      expect(updates.map(update => update.x)).toEqual([1]);
      expect(movement.completePlayerStep).toHaveBeenCalledWith("issued-1", expect.any(AbortSignal), "RIGHT");
      for (let x = 1; x <= 3; x++) {
        await vi.advanceTimersByTimeAsync(200);
        controller.onStepComplete(1, x, 0, "RIGHT");
        await vi.advanceTimersByTimeAsync(0);
      }
      expect(updates.map(update => update.x)).toEqual([1, 2, 3]);
      expect(updates.map(update => update.at - updates[0].at)).toEqual([0, 200, 400]);
      expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
      expect(movement.completePlayerStep).toHaveBeenCalledTimes(3);
      expect(controller.getIsMoving()).toBe(false);
    } finally { controller.clear(); vi.useRealTimers(); }
  });

  test("held keys continue smoothly, release cancels one unused token, and repeated focus stops retain cancellation ownership", async () => {
    vi.useFakeTimers();
    const { controller, updates } = straightController();
    let held: "RIGHT" | null = "RIGHT";
    controller.setHeldKeyboardDirectionProvider(() => held);
    let cancel!: (value: Awaited<ReturnType<typeof movement.cancelPlayerStep>>) => void;
    vi.mocked(movement.cancelPlayerStep).mockImplementationOnce(() => new Promise(resolve => { cancel = resolve; }));
    try {
      vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(step(1));
      for (let x = 1; x <= 3; x++) vi.mocked(movement.completePlayerStep).mockImplementationOnce(() => delayedCompletion(x, x + 1));
      controller.handleKeyboardMove("RIGHT");
      await vi.advanceTimersByTimeAsync(0);
      for (let x = 1; x <= 3; x++) {
        await vi.advanceTimersByTimeAsync(200);
        if (x === 3) held = null;
        controller.onStepComplete(1, x, 0, "RIGHT");
        await vi.advanceTimersByTimeAsync(0);
      }
      expect(updates.map(update => update.x)).toEqual([1, 2, 3]);
      expect(updates.map(update => update.at - updates[0].at)).toEqual([0, 200, 400]);
      expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
      expect(movement.cancelPlayerStep).toHaveBeenCalledWith("issued-4", expect.any(AbortSignal));
      const signal = vi.mocked(movement.cancelPlayerStep).mock.calls[0][1]!;
      controller.stopMovement(); controller.stopMovement();
      expect(signal.aborted).toBe(false);
      expect(controller.getIsMoving()).toBe(true);
      expect(controller.handleKeyboardMove("LEFT")).toBe(false);
      cancel({ success: true, cancelled: true, requestId: "cancel", mapId: 9999, x: 3, y: 0, direction: "RIGHT" });
      await vi.advanceTimersByTimeAsync(0);
      expect(controller.getIsMoving()).toBe(false);
      expect(movement.completePlayerStep).toHaveBeenCalledTimes(3);
    } finally { controller.clear(); vi.useRealTimers(); }
  });

  test("changing held direction keeps cancellation owned when a panel discards the new intent", async () => {
    const { controller, updates } = straightController();
    let held: "RIGHT" | "LEFT" = "RIGHT";
    controller.setHeldKeyboardDirectionProvider(() => held);
    vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(step(1));
    vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "one", mapId: 9999, x: 1, y: 0, direction: "RIGHT", nextStep: step(2) });
    let cancel!: (value: Awaited<ReturnType<typeof movement.cancelPlayerStep>>) => void;
    vi.mocked(movement.cancelPlayerStep).mockImplementationOnce(() => new Promise(resolve => { cancel = resolve; }));
    controller.handleKeyboardMove("RIGHT");
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    held = "LEFT";
    controller.onStepComplete(1, 1, 0, "RIGHT");
    await vi.waitFor(() => expect(movement.cancelPlayerStep).toHaveBeenCalledTimes(1));
    const signal = vi.mocked(movement.cancelPlayerStep).mock.calls[0][1]!;
    controller.stopMovement(); controller.stopMovement();
    expect(signal.aborted).toBe(false);
    expect(controller.getIsMoving()).toBe(true);
    cancel({ success: true, cancelled: true, requestId: "cancel", mapId: 9999, x: 1, y: 0, direction: "RIGHT" });
    await vi.waitFor(() => expect(controller.getIsMoving()).toBe(false));
    expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
    expect(updates).toHaveLength(1);
    controller.clear();
  });

  test("uncertain cancellation keeps later movement locked instead of committing the unused token", async () => {
    const { controller, updates } = straightController();
    let held: "RIGHT" | null = "RIGHT";
    controller.setHeldKeyboardDirectionProvider(() => held);
    vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(step(1));
    vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "one", mapId: 9999, x: 1, y: 0, direction: "RIGHT", nextStep: step(2) });
    vi.mocked(movement.cancelPlayerStep).mockRejectedValueOnce(new Error("second cancellation timeout"));
    const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
    controller.handleKeyboardMove("RIGHT");
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    held = null;
    controller.onStepComplete(1, 1, 0, "RIGHT");
    await vi.waitFor(() => expect(log).toHaveBeenCalled());
    controller.stopMovement();
    expect(controller.handleKeyboardMove("LEFT")).toBe(false);
    expect(controller.requestMoveTo(3, 0)).toBe(false);
    expect(movement.completePlayerStep).toHaveBeenCalledTimes(1);
    controller.clear(); log.mockRestore();
  });

  test("a prepared token never continues a server-stopped step", async () => {
    const { controller, updates } = straightController();
    vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(step(1));
    vi.mocked(movement.completePlayerStep).mockResolvedValueOnce({ success: true, requestId: "stop", mapId: 9999, x: 1, y: 0, direction: "RIGHT", stopPath: true });
    controller.requestMoveTo(3, 0);
    await vi.waitFor(() => expect(updates).toHaveLength(1));
    controller.onStepComplete(1, 1, 0, "RIGHT");
    await vi.waitFor(() => expect(controller.getIsMoving()).toBe(false));
    expect(updates).toHaveLength(1);
    expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
    controller.clear();
  });
});
