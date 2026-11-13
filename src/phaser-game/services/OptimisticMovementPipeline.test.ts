import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { OwnedPlayerPositionResponse, PlayerStepResponse } from "@/net/generated/protocol";
import { OptimisticMovementPipeline, type PredictedPlayerStep } from "./OptimisticMovementPipeline";
import * as movement from "./PlayerMovementService";

vi.mock("./PlayerMovementService", () => ({ readOwnedPlayerPosition: vi.fn(), submitPredictedPlayerStep: vi.fn() }));

const streamId = "a".repeat(32);
function owned(x = 0, y = 0): OwnedPlayerPositionResponse {
  return { success: true, requestId: "owned", mapId: 50, x, y, direction: "RIGHT",
    serverMovementPending: false, movementStreamId: streamId, movementSequence: 0 };
}
function receipt(x: number, y = 0, direction = "RIGHT"): PlayerStepResponse {
  return { success: true, committed: true, requestId: "receipt", stepToken: "command",
    mapId: 50, x, y, direction, ledgeJump: false };
}
function prediction(sourceX: number, x: number): Omit<PredictedPlayerStep, "sequence"> {
  return { mapId: 50, sourceX, sourceY: 0, x, y: 0, direction: x > sourceX ? "RIGHT" : "LEFT" };
}
async function flush() { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); }
function fixture() {
  const callbacks = { confirmed: vi.fn(), boundary: vi.fn(), failed: vi.fn(), changed: vi.fn() };
  vi.mocked(movement.readOwnedPlayerPosition).mockResolvedValue(owned());
  return { pipeline: new OptimisticMovementPipeline(callbacks), callbacks };
}
beforeEach(() => vi.resetAllMocks());
afterEach(() => vi.useRealTimers());

test("starts a prediction synchronously before stream initialization and commits exactly one released tap", async () => {
  const { pipeline, callbacks } = fixture();
  let initialize!: (value: OwnedPlayerPositionResponse) => void;
  let commit!: (value: PlayerStepResponse) => void;
  vi.mocked(movement.readOwnedPlayerPosition).mockImplementation(() => new Promise(resolve => { initialize = resolve; }));
  vi.mocked(movement.submitPredictedPlayerStep).mockImplementation(() => new Promise(resolve => { commit = resolve; }));
  expect(pipeline.start(prediction(0, 1))).toMatchObject({ sequence: 1, x: 1 });
  expect(movement.submitPredictedPlayerStep).not.toHaveBeenCalled();
  expect(movement.readOwnedPlayerPosition).toHaveBeenCalledWith(expect.any(AbortSignal), undefined, true);
  pipeline.finishIntent();
  initialize(owned()); await flush();
  expect(movement.submitPredictedPlayerStep).toHaveBeenCalledOnce();
  expect(movement.submitPredictedPlayerStep).toHaveBeenCalledWith({
    movementStreamId: streamId, sequence: 1, commandToken: expect.stringMatching(/^[a-f0-9]{32}$/),
    mapId: 50, fromX: 0, fromY: 0, direction: "RIGHT",
  }, expect.any(AbortSignal));
  expect(pipeline.isBusy()).toBe(true);
  commit(receipt(1)); await flush();
  expect(pipeline.isBusy()).toBe(false);
  expect(callbacks.failed).not.toHaveBeenCalled();
});

test("a fresh stream accepts the wire format with an omitted zero sequence", async () => {
  const { pipeline, callbacks } = fixture();
  const snapshot = owned();
  delete snapshot.movementSequence;
  vi.mocked(movement.readOwnedPlayerPosition).mockResolvedValue(snapshot);
  vi.mocked(movement.submitPredictedPlayerStep).mockResolvedValue(receipt(1));
  pipeline.start(prediction(0, 1));
  await flush();
  expect(movement.submitPredictedPlayerStep).toHaveBeenCalledWith(expect.objectContaining({ sequence: 1 }), expect.any(AbortSignal));
  expect(callbacks.failed).not.toHaveBeenCalled();
  expect(pipeline.isBusy()).toBe(false);
});

test("turns are submitted immediately without waiting for preceding receipts", async () => {
  const { pipeline } = fixture();
  vi.mocked(movement.submitPredictedPlayerStep).mockImplementation(() => new Promise(() => {}));
  pipeline.start(prediction(0, 1)); await flush();
  expect(pipeline.start(prediction(1, 0))).toMatchObject({ sequence: 2, direction: "LEFT" });
  expect(movement.submitPredictedPlayerStep).toHaveBeenCalledTimes(2);
  expect(movement.submitPredictedPlayerStep).toHaveBeenLastCalledWith(
    expect.objectContaining({ fromX: 1, direction: "LEFT", sequence: 2 }), expect.any(AbortSignal));
  pipeline.dispose();
});

test("the fixed uncertainty bound blocks a ninth prediction until an authoritative child confirms", async () => {
  const { pipeline } = fixture();
  const replies: Array<(value: PlayerStepResponse) => void> = [];
  vi.mocked(movement.submitPredictedPlayerStep).mockImplementation(() => new Promise(resolve => replies.push(resolve)));
  for (let x = 1; x <= 8; x++) expect(pipeline.start(prediction(x - 1, x))).not.toBeNull();
  expect(pipeline.start(prediction(8, 9))).toBeNull();
  await flush();
  expect(replies).toHaveLength(8);
  replies[7](receipt(8)); await flush();
  expect(pipeline.start(prediction(8, 9))).not.toBeNull();
  expect(vi.mocked(movement.submitPredictedPlayerStep).mock.calls[0][1]?.aborted).toBe(true);
  expect(movement.submitPredictedPlayerStep).toHaveBeenCalledTimes(9);
  pipeline.dispose();
});

test("retiring during initialization ignores its late stream and never submits old predictions", async () => {
  const { pipeline, callbacks } = fixture();
  let initialize!: (value: OwnedPlayerPositionResponse) => void;
  vi.mocked(movement.readOwnedPlayerPosition).mockImplementation(() => new Promise(resolve => { initialize = resolve; }));
  pipeline.start(prediction(0, 1));
  pipeline.dispose();
  initialize(owned()); await flush();
  expect(vi.mocked(movement.readOwnedPlayerPosition).mock.calls[0][0]?.aborted).toBe(true);
  expect(movement.submitPredictedPlayerStep).not.toHaveBeenCalled();
  expect(callbacks.confirmed).not.toHaveBeenCalled();
});

test("a stream initialized at a different owned source corrects prediction without sending stale commands", async () => {
  const { pipeline, callbacks } = fixture();
  vi.mocked(movement.readOwnedPlayerPosition).mockResolvedValue(owned(4, 3));
  pipeline.start(prediction(0, 1)); await flush();
  expect(callbacks.boundary).toHaveBeenCalledWith(expect.objectContaining({ sequence: 1 }), expect.objectContaining({ x: 4, y: 3 }));
  expect(movement.submitPredictedPlayerStep).not.toHaveBeenCalled();
});

test("a committed effect boundary retires already-predicted children and publishes one correction", async () => {
  const { pipeline, callbacks } = fixture();
  const replies: Array<(value: PlayerStepResponse) => void> = [];
  vi.mocked(movement.submitPredictedPlayerStep).mockImplementation(() => new Promise(resolve => replies.push(resolve)));
  pipeline.start(prediction(0, 1)); await flush();
  pipeline.start(prediction(1, 2));
  replies[0]({ ...receipt(1), stopPath: true }); await flush();
  expect(callbacks.boundary).toHaveBeenCalledOnce();
  expect(pipeline.start(prediction(2, 3))).toBeNull();
  expect(vi.mocked(movement.submitPredictedPlayerStep).mock.calls[1][1]?.aborted).toBe(true);
});

test("sustains forty one-tile turns at 200ms cadence under 340ms RTT within the fixed prediction bound", async () => {
  vi.useFakeTimers();
  vi.setSystemTime(0);
  const latency = 170;
  let serverTail = Promise.resolve();
  let serverX = 0, serverY = 0, serverSequence = 0, lastCommitStart: number | null = null;
  let confirmed = 0, maxDebt = 0;
  const failures: unknown[] = [];
  const callbacks = {
    confirmed: (step: PredictedPlayerStep) => { confirmed = step.sequence; },
    boundary: vi.fn(), failed: (error: unknown) => failures.push(error), changed: vi.fn(),
  };
  const pipeline = new OptimisticMovementPipeline(callbacks);
  const delta = (direction: string) => direction === "UP" ? [0, -1] : direction === "DOWN" ? [0, 1] : direction === "LEFT" ? [-1, 0] : [1, 0];
  function overWire<T>(operation: () => Promise<T> | T): Promise<T> {
    return new Promise((resolve, reject) => {
      setTimeout(() => {
        serverTail = serverTail.then(async () => {
          try { const response = await operation(); setTimeout(() => resolve(response), latency); }
          catch (error) { setTimeout(() => reject(error), latency); }
        });
      }, latency);
    });
  }
  vi.mocked(movement.readOwnedPlayerPosition).mockImplementation(() => overWire(() => owned(serverX, serverY)));
  vi.mocked(movement.submitPredictedPlayerStep).mockImplementation(command => overWire(async () => {
    if (command.movementStreamId !== streamId || command.sequence !== serverSequence + 1
      || command.fromX !== serverX || command.fromY !== serverY) throw new Error("Unordered prediction");
    const deadline = lastCommitStart === null ? Date.now() + 200 : lastCommitStart + 200;
    if (deadline > Date.now()) await new Promise<void>(resolve => setTimeout(resolve, deadline - Date.now()));
    lastCommitStart = Date.now();
    const [dx, dy] = delta(command.direction); serverX += dx; serverY += dy; serverSequence++;
    return { ...receipt(serverX, serverY, command.direction), stepToken: command.commandToken };
  }));
  let visualX = 0, visualY = 0;
  const directions = ["UP", "LEFT", "DOWN", "RIGHT"];
  for (let index = 0; index < 40; index++) {
    const direction = directions[index % directions.length];
    const [dx, dy] = delta(direction);
    const started = pipeline.start({ mapId: 50, sourceX: visualX, sourceY: visualY,
      x: visualX + dx, y: visualY + dy, direction });
    expect(started, "prediction stalled at turn " + (index + 1)).not.toBeNull();
    visualX += dx; visualY += dy;
    maxDebt = Math.max(maxDebt, index + 1 - confirmed);
    await vi.advanceTimersByTimeAsync(200);
    expect(failures).toEqual([]);
  }
  pipeline.finishIntent();
  await vi.advanceTimersByTimeAsync(2000);
  expect(failures).toEqual([]);
  expect(callbacks.boundary).not.toHaveBeenCalled();
  expect(serverSequence).toBe(40);
  expect([serverX, serverY]).toEqual([visualX, visualY]);
  expect(maxDebt).toBeLessThanOrEqual(5);
  expect(pipeline.isBusy()).toBe(false);
  expect(new Set(vi.mocked(movement.submitPredictedPlayerStep).mock.calls.map(([command]) => command.commandToken)).size).toBe(40);
});
