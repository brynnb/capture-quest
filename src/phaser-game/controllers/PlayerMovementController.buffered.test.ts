import { beforeEach, afterEach, describe, expect, test, vi } from "vitest";
import type { Scene } from "phaser";
import type { MapRenderer } from "../renderers/MapRenderer";
import type { PhaserTile } from "@/net/generated/world_api";
import type { PlayerStepResponse, PlayerStepCompleteResponse } from "@/net/generated/protocol";
import useGameScreenStore from "@/stores/GameScreenStore";
import { PlayerMovementController } from "./PlayerMovementController";
import * as movement from "../services/PlayerMovementService";

vi.mock("../services/PlayerMovementService", () => ({
  requestPlayerStep: vi.fn(), completePlayerStep: vi.fn(), cancelPlayerStep: vi.fn(),
  requestPlayerFacing: vi.fn(), readOwnedPlayerPosition: vi.fn(),
  readWarpHomeRevision: vi.fn(), requestWarpHome: vi.fn(), requestElevator: vi.fn(),
  requestCut: vi.fn(), requestStateField: vi.fn(), requestSurf: vi.fn(),
  requestFishing: vi.fn(), requestEscapeRope: vi.fn(), requestBicycleState: vi.fn(),
}));

function step(x: number, y = 1, direction = "RIGHT"): PlayerStepResponse {
  return { success: true, requestId: "issue", stepToken: "token-" + x + "-" + y,
    mapId: 9999, x, y, direction, ledgeJump: false };
}
function result(x: number, continuationSteps: PlayerStepResponse[] = [], y = 1): PlayerStepCompleteResponse {
  return { success: true, requestId: "complete", mapId: 9999, x, y,
    direction: y === 1 ? "RIGHT" : "DOWN", continuationSteps };
}
function grant(from = 1, to = 8): PlayerStepResponse {
  return { ...step(from), buffered: true,
    continuationSteps: Array.from({ length: to - from }, (_, i) => step(from + i + 1)) };
}
async function flush(): Promise<void> {
  await Promise.resolve(); await Promise.resolve(); await Promise.resolve();
}
function fixture() {
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
  controller.setPlayer(1, 0, 1, 9999, renderer as unknown as MapRenderer);
  const completions = new Map<string, (reply: PlayerStepCompleteResponse) => void>();
  vi.mocked(movement.completePlayerStep).mockImplementation(token => new Promise(resolve => completions.set(token, resolve)));
  vi.mocked(movement.cancelPlayerStep).mockResolvedValue({ ...result(12), cancelled: true });
  vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(grant());
  function complete(x: number, reply = result(x), y = 1) {
    const resolve = completions.get(step(x, y).stepToken);
    expect(resolve, "completion for " + x + "," + y).toBeDefined();
    resolve!(reply);
  }
  return { controller, renderer, updates, complete, completions };
}

beforeEach(() => { vi.resetAllMocks(); useGameScreenStore.getState().setScreen("game"); });
afterEach(() => vi.clearAllMocks());

describe("buffered player movement", () => {
  test("starts the next authorized tile synchronously without waiting for a network receipt", async () => {
    const { controller, updates, complete } = fixture();
    controller.handleTileClick(12 * 16 + 8, 1 * 16 + 8);
    await flush();
    expect(updates).toHaveLength(1);
    controller.onStepComplete(1, 1, 1, "RIGHT");
    // No microtask or animation frame has elapsed at this assertion.
    expect(updates).toHaveLength(2);
    expect(updates[1].slice(1, 5)).toEqual([1, 1, 2, 1]);
    expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
    complete(2);
    await flush();
    controller.onStepComplete(1, 2, 1, "RIGHT");
    expect(updates).toHaveLength(3);
    controller.stopMovement(true);
  });

  test("bounds unconfirmed movement and resumes when a later child confirms its predecessors", async () => {
    const { controller, updates, complete } = fixture();
    controller.handleTileClick(12 * 16 + 8, 24);
    await flush();
    for (let x = 1; x <= 4; x++) controller.onStepComplete(1, x, 1, "RIGHT");
    expect(updates).toHaveLength(4);
    complete(4);
    await flush();
    expect(updates).toHaveLength(5);
    const oldestSignal = vi.mocked(movement.completePlayerStep).mock.calls[0][1];
    expect(oldestSignal?.aborted).toBe(true);
    // Later acknowledgement cancels the obsolete listener; it never retries token1.
    expect(vi.mocked(movement.completePlayerStep).mock.calls.filter(call => call[0] === step(1).stepToken)).toHaveLength(1);
    controller.stopMovement(true);
  });

  test("recovers a lost refill publication from the next child's window and reaches a long destination", async () => {
    const { controller, updates, complete } = fixture();
    controller.handleTileClick(12 * 16 + 8, 24);
    await flush();
    for (let x = 1; x <= 12; x++) {
      // Drop the refill-bearing fifth receipt completely. Child6 republishes the
      // server's still-pending tail, including grants9..12 from that lost reply.
      if (x !== 5) {
        const continuation = x === 6 ? Array.from({ length: 6 }, (_, i) => step(i + 7)) : [];
        complete(x, result(x, continuation));
        await flush();
      }
      controller.onStepComplete(1, x, 1, "RIGHT");
    }
    expect(updates.map(args => args[3])).toEqual(Array.from({ length: 12 }, (_, i) => i + 1));
    expect(movement.requestPlayerStep).toHaveBeenCalledTimes(1);
    const fifthCall = vi.mocked(movement.completePlayerStep).mock.calls[4];
    expect(fifthCall[3]).toMatchObject({ afterStepToken: step(8).stepToken, lookAhead: ["RIGHT", "RIGHT", "RIGHT", "RIGHT"], retryOnTimeout: false });
    expect(fifthCall[1]?.aborted).toBe(true);
    expect(controller.getIsMoving()).toBe(false);
  });

  test.each(["tile click", "requestMoveTo", "approach callback"])("retargets %s from the current issued endpoint without replaying its destination", async mode => {
    const { controller, renderer, updates, complete } = fixture();
    controller.handleTileClick(8 * 16 + 8, 24);
    await flush();
    let accept!: (reply: PlayerStepResponse) => void;
    vi.mocked(movement.requestPlayerStep).mockImplementationOnce(() => new Promise(resolve => { accept = resolve; }));
    const onReach = vi.fn();
    if (mode === "tile click") controller.handleTileClick(24, 40);
    else if (mode === "requestMoveTo") controller.requestMoveTo(1, 2);
    else controller.requestPathToTile(1, 2, onReach);
    expect(movement.requestPlayerStep).toHaveBeenLastCalledWith(
      { mapId: 9999, fromX: 1, fromY: 1, direction: "DOWN", lookAhead: [] }, expect.any(AbortSignal));
    // Finish the original segment; wait only for the changed route's admission.
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(1);
    complete(1, result(1, [step(2), step(3)]));
    await flush();
    accept({ ...step(1, 2, "DOWN"), buffered: true });
    await flush();
    expect(updates).toHaveLength(2);
    expect(updates[1].slice(1, 5)).toEqual([1, 1, 1, 2]);
    complete(1, result(1, [], 2), 2);
    await flush();
    controller.onStepComplete(1, 1, 2, "DOWN");
    expect(controller.getIsMoving()).toBe(false);
    expect(renderer.snapActorPosition).not.toHaveBeenCalled();
    if (mode === "approach callback") expect(onReach).toHaveBeenCalledOnce();
  });

  test("keeps a click received during the unused-window cancellation round trip", async () => {
    const { controller, updates, complete } = fixture();
    let held: "RIGHT" | null = "RIGHT";
    controller.setHeldKeyboardDirectionProvider(() => held);
    let finishCancel!: (reply: PlayerStepCompleteResponse) => void;
    vi.mocked(movement.cancelPlayerStep).mockImplementationOnce(() => new Promise(resolve => { finishCancel = resolve; }));
    controller.handleKeyboardMove("RIGHT");
    await flush();
    held = null;
    complete(1);
    await flush();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(movement.cancelPlayerStep).toHaveBeenCalledWith(step(2).stepToken, expect.any(AbortSignal));
    vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce(grant(2, 3));
    controller.handleTileClick(3 * 16 + 8, 24);
    finishCancel({ ...result(1), cancelled: true });
    await flush();
    expect(updates).toHaveLength(2);
    expect(movement.requestPlayerStep).toHaveBeenLastCalledWith(
      { mapId: 9999, fromX: 1, fromY: 1, direction: "RIGHT", lookAhead: ["RIGHT"] }, expect.any(AbortSignal));
    complete(2); await flush(); controller.onStepComplete(1, 2, 1, "RIGHT");
    complete(3); await flush(); controller.onStepComplete(1, 3, 1, "RIGHT");
    expect(controller.getIsMoving()).toBe(false);
  });



  test("retargets an NPC approach beside the previous source from the issued endpoint", async () => {
    const { controller, updates, complete } = fixture();
    controller.handleTileClick(8 * 16 + 8, 24);
    await flush();
    const onReach = vi.fn();
    vi.mocked(movement.requestPlayerStep).mockResolvedValueOnce({ ...step(0, 1, "LEFT"), buffered: true });
    vi.mocked(movement.requestPlayerFacing).mockResolvedValue({
      success: true, requestId: "face", mapId: 9999, x: 0, y: 1, direction: "UP", serverMovementPending: false,
    });
    expect(controller.requestInteractionPath(0, 0, onReach)).toBe(true);
    expect(onReach).not.toHaveBeenCalled();
    expect(movement.requestPlayerStep).toHaveBeenLastCalledWith(
      { mapId: 9999, fromX: 1, fromY: 1, direction: "LEFT", lookAhead: [] }, expect.any(AbortSignal));
    await flush();
    complete(1); await flush(); controller.onStepComplete(1, 1, 1, "RIGHT");
    complete(0, { ...result(0), direction: "LEFT" }); await flush();
    controller.onStepComplete(1, 0, 1, "LEFT");
    expect(updates.map(args => args[3])).toEqual([1, 0]);
    expect(onReach).toHaveBeenCalledOnce();
    expect(controller.getIsMoving()).toBe(false);
  });

  test.each(["retarget active endpoint", "stop for input focus"])("retires a late staged grant after %s", async reason => {
    const { controller, updates, complete } = fixture();
    controller.handleTileClick(8 * 16 + 8, 24);
    await flush();
    let accept!: (reply: PlayerStepResponse) => void;
    vi.mocked(movement.requestPlayerStep).mockImplementationOnce(() => new Promise(resolve => { accept = resolve; }));
    let finishCancel!: (reply: PlayerStepCompleteResponse) => void;
    vi.mocked(movement.cancelPlayerStep).mockImplementationOnce(() => new Promise(resolve => { finishCancel = resolve; }));
    controller.handleTileClick(24, 40);
    const stagedSignal = vi.mocked(movement.requestPlayerStep).mock.calls[1][1];
    if (reason === "retarget active endpoint") controller.handleTileClick(24, 24);
    else controller.stopMovement();
    // Do not abandon the only listener capable of discovering the issued token.
    expect(stagedSignal?.aborted).toBe(false);
    complete(1); await flush();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(1);
    expect(controller.getIsMoving()).toBe(true);
    accept({ ...step(1, 2, "DOWN"), buffered: true });
    await flush();
    expect(movement.cancelPlayerStep).toHaveBeenLastCalledWith(step(1, 2).stepToken, stagedSignal);
    finishCancel({ ...result(1), cancelled: true });
    await flush();
    expect(controller.getIsMoving()).toBe(false);
    expect(updates).toHaveLength(1);
  });

  test("a delayed committed effect boundary retires an already-started visual child", async () => {
    const { controller, renderer, updates, complete } = fixture();
    controller.handleTileClick(8 * 16 + 8, 24);
    await flush();
    controller.onStepComplete(1, 1, 1, "RIGHT");
    expect(updates).toHaveLength(2);
    complete(1, { ...result(1), stopPath: true });
    await flush();
    expect(renderer.snapActorPosition).toHaveBeenCalledWith(1, 1, 1, "RIGHT");
    expect(controller.getIsMoving()).toBe(false);
    expect(vi.mocked(movement.completePlayerStep).mock.calls[1][1]?.aborted).toBe(true);
    expect(updates).toHaveLength(2);
  });
});
