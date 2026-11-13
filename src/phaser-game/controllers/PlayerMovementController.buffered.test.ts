import { beforeEach, afterEach, describe, expect, test, vi } from "vitest";
import type { Scene } from "phaser";
import type { MapRenderer } from "../renderers/MapRenderer";
import type { PhaserTile } from "@/net/generated/world_api";
import type { OwnedPlayerPositionResponse, PlayerStepResponse } from "@/net/generated/protocol";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import { PlayerMovementController } from "./PlayerMovementController";
import { CorrelatedResponseError } from "../services/CorrelatedRequest";
import * as movement from "../services/PlayerMovementService";
import * as recovery from "../services/GameplayRecoveryService";

vi.mock("../services/PlayerMovementService", () => ({
  submitPredictedPlayerStep: vi.fn(), requestPlayerStep: vi.fn(), completePlayerStep: vi.fn(), cancelPlayerStep: vi.fn(),
  requestPlayerFacing: vi.fn(), readOwnedPlayerPosition: vi.fn(),
  readWarpHomeRevision: vi.fn(), requestWarpHome: vi.fn(), requestElevator: vi.fn(),
  requestCut: vi.fn(), requestStateField: vi.fn(), requestSurf: vi.fn(),
  requestFishing: vi.fn(), requestEscapeRope: vi.fn(), requestBicycleState: vi.fn(),
}));
vi.mock("../services/GameplayRecoveryService", () => ({
  readCurrentGameplayState: vi.fn(), applyGameplayResourceSnapshot: vi.fn(), applyGameplaySnapshot: vi.fn(),
}));
vi.mock("../services/BattleCommandService", () => ({
  suspendBattleCommandForEmergency: vi.fn(() => () => {}),
  recoverBattlePublication: vi.fn(async () => true), recoverFieldPublication: vi.fn(async () => true),
}));

function owned(x = 0, y = 1): OwnedPlayerPositionResponse {
  return { success: true, requestId: "owned", mapId: 9999, x, y, direction: "RIGHT",
    serverMovementPending: false, movementStreamId: "a".repeat(32), movementSequence: 0 };
}
function receipt(x: number, y = 1, direction = "RIGHT"): PlayerStepResponse {
  return { success: true, committed: true, requestId: "move", stepToken: "command", mapId: 9999, x, y, direction, ledgeJump: false };
}
async function flush() { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); }
function fixture(startX = 0, startY = 1) {
  const updates: unknown[][] = [];
  const renderer = {
    updateActorPosition: vi.fn((...args: unknown[]) => updates.push(args)),
    snapActorPosition: vi.fn(),
    getMovementController: () => ({ handleDirectionUpdate: vi.fn() }),
  };
  const controller = new PlayerMovementController({ events: { emit: vi.fn(), once: vi.fn() } } as unknown as Scene);
  const tiles: PhaserTile[] = [];
  for (let y = 0; y <= 3; y++) for (let x = 0; x <= 16; x++) {
    tiles.push({ id: tiles.length + 1, x, y, mapId: 9999, tileImageId: 1, collisionType: 1,
      talkOverTile: false, isNativeGameData: true, coordinateOrigin: "native", contentOrigin: "native" });
  }
  controller.buildCollisionMap(tiles);
  controller.setPlayer(1, startX, startY, 9999, renderer as unknown as MapRenderer);
  const completions = new Map<number, (reply: PlayerStepResponse) => void>();
  vi.mocked(movement.readOwnedPlayerPosition).mockResolvedValue(owned(startX, startY));
  vi.mocked(movement.submitPredictedPlayerStep).mockImplementation(command => new Promise(resolve => completions.set(command.sequence, resolve)));
  vi.mocked(movement.requestPlayerFacing).mockResolvedValue({
    success: true, requestId: "face", mapId: 9999, x: startX, y: startY, direction: "DOWN", serverMovementPending: false,
  });
  function complete(sequence: number, reply: PlayerStepResponse) {
    expect(completions.has(sequence), "completion for " + sequence).toBe(true);
    completions.get(sequence)!(reply);
  }
  return { controller, renderer, updates, complete };
}

beforeEach(() => { vi.resetAllMocks(); useGameScreenStore.getState().setScreen("game"); });
afterEach(() => vi.clearAllMocks());

describe("optimistic player movement", () => {
  test("the first tile renders synchronously before even the stream bootstrap response", async () => {
    const { controller, updates } = fixture();
    vi.mocked(movement.readOwnedPlayerPosition).mockImplementationOnce(() => new Promise(() => {}));
    controller.handleKeyboardMove("RIGHT");
    expect(updates).toHaveLength(1);
    expect(updates[0].slice(1, 5)).toEqual([0, 1, 1, 1]);
    expect(movement.submitPredictedPlayerStep).not.toHaveBeenCalled();
    expect(controller.getIsMoving()).toBe(true);
    controller.clear();
  });

  test("a key released before bootstrap finishes exactly its one visual and authoritative tile", async () => {
    const { controller, updates, complete } = fixture();
    let initialize!: (reply: OwnedPlayerPositionResponse) => void;
    vi.mocked(movement.readOwnedPlayerPosition).mockImplementationOnce(() => new Promise(resolve => { initialize = resolve; }));
    let held: "RIGHT" | null = "RIGHT";
    controller.setHeldKeyboardDirectionProvider(() => held);
    controller.handleKeyboardMove("RIGHT");
    held = null;
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(1);
    expect(controller.getIsMoving()).toBe(true);
    initialize(owned()); await flush();
    expect(movement.submitPredictedPlayerStep).toHaveBeenCalledOnce();
    complete(1, receipt(1)); await flush();
    expect(updates).toHaveLength(1);
    expect(controller.getIsMoving()).toBe(false);
  });

  test("held input changes direction at the next boundary without waiting for any server reply", async () => {
    const { controller, updates } = fixture(5, 2);
    vi.mocked(movement.readOwnedPlayerPosition).mockImplementationOnce(() => new Promise(() => {}));
    let held: "UP" | "LEFT" | null = "UP";
    controller.setHeldKeyboardDirectionProvider(() => held);
    controller.handleKeyboardMove("UP");
    held = "LEFT";
    controller.onStepComplete(1, 5, 1, "UP");
    expect(updates.map(args => args.slice(3, 6))).toEqual([[5, 1, "UP"], [4, 1, "LEFT"]]);
    held = null;
    controller.onStepComplete(1, 4, 1, "LEFT");
    expect(updates).toHaveLength(2);
    controller.clear();
  });

  test.each(["tile click", "requestMoveTo", "approach callback"])("%s retargets an unacknowledged first tile without an authorization pause", async mode => {
    const { controller, updates, complete } = fixture();
    let initialize!: (reply: OwnedPlayerPositionResponse) => void;
    vi.mocked(movement.readOwnedPlayerPosition).mockImplementationOnce(() => new Promise(resolve => { initialize = resolve; }));
    controller.handleTileClick(8 * 16 + 8, 24);
    const onReach = vi.fn();
    if (mode === "tile click") controller.handleTileClick(24, 40);
    else if (mode === "requestMoveTo") controller.requestMoveTo(1, 2);
    else controller.requestPathToTile(1, 2, onReach);
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(2);
    expect(updates[1].slice(1, 6)).toEqual([1, 1, 1, 2, "DOWN"]);
    controller.onStepComplete(1, 1, 2, "DOWN");
    expect(onReach).not.toHaveBeenCalled();
    initialize(owned()); await flush();
    expect(vi.mocked(movement.submitPredictedPlayerStep).mock.calls.map(([command]) => [command.fromX, command.fromY, command.direction])).toEqual([[0, 1, "RIGHT"], [1, 1, "DOWN"]]);
    complete(2, receipt(1, 2, "DOWN")); await flush();
    expect(controller.getIsMoving()).toBe(false);
    if (mode === "approach callback") expect(onReach).toHaveBeenCalledOnce();
    expect(updates).toHaveLength(2);
  });

  test("the fixed eight-tile uncertainty bound resumes on a confirmed child without historical retries", async () => {
    const { controller, updates, complete } = fixture();
    controller.handleTileClick(12 * 16 + 8, 24);
    await flush();
    for (let x = 1; x <= 8; x++) controller.onStepComplete(1, x, 1, "RIGHT");
    expect(updates).toHaveLength(8);
    complete(8, receipt(8)); await flush();
    expect(updates).toHaveLength(9);
    expect(vi.mocked(movement.submitPredictedPlayerStep).mock.calls[0][1]?.aborted).toBe(true);
    expect(movement.submitPredictedPlayerStep).toHaveBeenCalledTimes(9);
    controller.clear();
  });

  test("input focus stops future prediction but lets the active segment and its one command finish", async () => {
    const { controller, updates, complete } = fixture();
    controller.requestMoveTo(8, 1); await flush();
    controller.stopMovement();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(1);
    expect(movement.submitPredictedPlayerStep).toHaveBeenCalledOnce();
    complete(1, receipt(1)); await flush();
    expect(controller.getIsMoving()).toBe(false);
  });

  test("retirement ignores a late bootstrap and cannot revive an old route", async () => {
    const { controller, updates } = fixture();
    let initialize!: (reply: OwnedPlayerPositionResponse) => void;
    vi.mocked(movement.readOwnedPlayerPosition).mockImplementationOnce(() => new Promise(resolve => { initialize = resolve; }));
    controller.requestMoveTo(8, 1);
    controller.clear();
    initialize(owned()); await flush();
    expect(updates).toHaveLength(1);
    expect(movement.submitPredictedPlayerStep).not.toHaveBeenCalled();
    expect(controller.getIsMoving()).toBe(false);
  });

  test("an authoritative rejection corrects the visible prediction and allows a fresh valid input after recovery", async () => {
    const { controller, renderer, updates } = fixture();
    const rejection = { success: false as const, requestId: "reject", error: "source changed", mapId: 9999, x: 0, y: 1, direction: "RIGHT", serverMovementPending: false };
    vi.mocked(movement.submitPredictedPlayerStep).mockRejectedValueOnce(new CorrelatedResponseError(rejection));
    controller.handleKeyboardMove("RIGHT");
    expect(updates).toHaveLength(1);
    await flush(); await flush();
    expect(renderer.snapActorPosition).toHaveBeenLastCalledWith(1, 0, 1, "RIGHT");
    expect(movement.readOwnedPlayerPosition).toHaveBeenLastCalledWith(expect.any(AbortSignal));
    expect(controller.getIsMoving()).toBe(false);
    expect(controller.handleKeyboardMove("RIGHT")).toBe(true);
    expect(updates).toHaveLength(2);
    controller.clear();
  });

  test("a delayed effect boundary reconciles an already-started child", async () => {
    const { controller, renderer, updates, complete } = fixture();
    controller.requestMoveTo(8, 1); await flush();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(2);
    complete(1, { ...receipt(1), stopPath: true }); await flush();
    expect(renderer.snapActorPosition).toHaveBeenCalledWith(1, 1, 1, "RIGHT");
    expect(controller.getIsMoving()).toBe(false);
    expect(vi.mocked(movement.submitPredictedPlayerStep).mock.calls[1][1]?.aborted).toBe(true);
  });

  test("an early same-tile effect receipt lets the current segment finish before presentation stops", async () => {
    const { controller, renderer, updates, complete } = fixture();
    controller.requestMoveTo(8, 1); await flush();
    complete(1, { ...receipt(1), stopPath: true }); await flush();
    expect(renderer.snapActorPosition).not.toHaveBeenCalled();
    expect(updates).toHaveLength(1);
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(renderer.snapActorPosition).toHaveBeenCalledWith(1, 1, 1, "RIGHT");
    expect(controller.getIsMoving()).toBe(false);
  });

  test("arrival callbacks attached at an already visible destination wait for authority and can perform real facing", async () => {
    const { controller, complete } = fixture();
    controller.requestMoveTo(1, 1); await flush();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    const onReach = vi.fn(() => controller.faceTile(1, 2));
    controller.requestPathToTile(1, 1, onReach);
    expect(controller.getIsMoving()).toBe(true);
    expect(onReach).not.toHaveBeenCalled();
    complete(1, receipt(1)); await flush();
    expect(onReach).toHaveBeenCalledOnce();
    expect(movement.requestPlayerFacing).toHaveBeenCalledWith(
      { mapId: 9999, fromX: 1, fromY: 1, direction: "DOWN" }, expect.any(AbortSignal));
    expect(controller.getIsMoving()).toBe(false);
  });

  test("ordinary field actions stay locked during prediction, then run from a confirmed arrival callback", async () => {
    const { controller, complete } = fixture();
    usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 });
    controller.setTileReconciler(async () => true);
    vi.mocked(recovery.readCurrentGameplayState, { partial: true }).mockResolvedValue({
      cutScope: "visit", fieldCommands: { cut: 0 }, position: owned(1, 1),
    });
    vi.mocked(movement.requestCut).mockResolvedValue({ success: true, requestId: "cut", characterId: 42, moveName: "CUT", mapId: 9999, cutScope: "visit", targetX: 1, targetY: 2, message: "Used CUT!" });
    controller.requestPathToTile(1, 1, () => { void controller.cut(1, 2, "DOWN"); });
    await flush();
    await controller.cut(1, 2, "DOWN");
    expect(movement.requestCut).not.toHaveBeenCalled();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    complete(1, receipt(1)); await flush(); await flush();
    expect(movement.requestCut).toHaveBeenCalledOnce();
  });
});
