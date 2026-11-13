import { afterEach, expect, test, vi } from "vitest";
import type { OwnedPlayerPositionResponse, PlayerStepResponse, PlayerStepCompleteResponse } from "@/net/generated/protocol";
import { cancelPlayerStep, completeCutscene, completePlayerStep, readOwnedPlayerPosition, submitPredictedPlayerStep } from "./PlayerMovementService";
import { CorrelatedRequestTimeoutError, CorrelatedResponseError } from "./CorrelatedRequest";

type StepReply = PlayerStepCompleteResponse | PlayerStepResponse | OwnedPlayerPositionResponse | { success: false; requestId: string; error: string };

const net = vi.hoisted(() => ({
  connected: true,
  listeners: new Set<(data: StepReply) => void>(),
  complete: vi.fn(),
  request: vi.fn(),
  read: vi.fn(),
}));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => net.connected,
  onPlayerStepComplete: (receive: (data: StepReply) => void) => {
    net.listeners.add(receive); return () => net.listeners.delete(receive);
  },
  onPlayerStep: (receive: (data: StepReply) => void) => { net.listeners.add(receive); return () => net.listeners.delete(receive); },
  requestPlayerStep: net.request,
  completePlayerStep: net.complete,
  completeCutscene: net.complete,
  onCutsceneEnd: (receive: (data: StepReply) => void) => { net.listeners.add(receive); return () => net.listeners.delete(receive); },
  onOwnedPlayerPosition: (receive: (data: StepReply) => void) => {
    net.listeners.add(receive); return () => net.listeners.delete(receive);
  },
  requestOwnedPlayerPosition: net.read,
}));
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); net.listeners.clear(); net.connected = true; });

test("a timed-out completion retries once with its original token and a fresh correlation", async () => {
  vi.useFakeTimers();
  const result = completePlayerStep("issued");
  const first = net.complete.mock.calls[0][0];
  await vi.advanceTimersByTimeAsync(10000);
  expect(net.complete).toHaveBeenCalledTimes(2);
  const second = net.complete.mock.calls[1][0];
  expect(second.stepToken).toBe(first.stepToken);
  expect(second.requestId).not.toBe(first.requestId);
  // The late first reply cannot settle the new request.
  for (const receive of net.listeners) receive({ success: true, requestId: first.requestId, mapId: 50, x: 8, y: 8, direction: "RIGHT" });
  expect(net.listeners.size).toBe(1);
  for (const receive of net.listeners) receive({ success: true, requestId: second.requestId, replayed: true, mapId: 50, x: 8, y: 8, direction: "RIGHT" });
  await expect(result).resolves.toMatchObject({ replayed: true });
  expect(net.listeners.size).toBe(0);
});

test("a second timeout is terminal and cancellation or rejection never retries", async () => {
  vi.useFakeTimers();
  const result = completePlayerStep("issued");
  const rejected = expect(result).rejects.toBeInstanceOf(CorrelatedRequestTimeoutError);
  await vi.advanceTimersByTimeAsync(20000); await rejected;
  expect(net.complete).toHaveBeenCalledTimes(2);
  expect(net.listeners.size).toBe(0);
  net.complete.mockClear();
  const abort = new AbortController();
  const cancelled = completePlayerStep("issued", abort.signal);
  abort.abort(); await expect(cancelled).rejects.toMatchObject({ name: "AbortError" });
  expect(net.complete).toHaveBeenCalledTimes(1);
  net.complete.mockClear();
  const failed = completePlayerStep("issued");
  const request = net.complete.mock.calls[0][0];
  for (const receive of net.listeners) receive({ success: false, requestId: request.requestId, error: "rejected" });
  await expect(failed).rejects.toBeInstanceOf(CorrelatedResponseError);
  expect(net.complete).toHaveBeenCalledTimes(1);
});

test("owned position reads can ask for a receipt without resending a mutation", async () => {
  const result = readOwnedPlayerPosition(undefined, "issued");
  expect(net.read).toHaveBeenCalledWith({ requestId: expect.any(String), stepToken: "issued" });
  const request = net.read.mock.calls[0][0];
  for (const receive of net.listeners) receive({ success: true, requestId: request.requestId, mapId: 50, x: 7, y: 8, direction: "LEFT" });
  await expect(result).resolves.toMatchObject({ x: 7 });
  expect(net.complete).not.toHaveBeenCalled();
});


test("cutscene timeout retries its issued token once with a fresh correlation", async () => {
  vi.useFakeTimers();
  const result = completeCutscene("Reward", "issued");
  const first = net.complete.mock.calls[0][0];
  await vi.advanceTimersByTimeAsync(10000);
  const second = net.complete.mock.calls[1][0];
  expect(net.complete).toHaveBeenCalledTimes(2);
  expect(second).toMatchObject({ scriptLabel: "Reward", completionToken: "issued" });
  expect(second.requestId).not.toBe(first.requestId);
  for (const receive of net.listeners) receive({ success: true, requestId: second.requestId, replayed: true, mapId: 50, x: 7, y: 8, direction: "LEFT" });
  await expect(result).resolves.toMatchObject({ replayed: true, x: 7 });
  expect(net.listeners.size).toBe(0);
});

test("cutscene second timeout is terminal; cancellation and rejection do not retry", async () => {
  vi.useFakeTimers();
  const result = completeCutscene("Reward", "issued");
  const rejected = expect(result).rejects.toBeInstanceOf(CorrelatedRequestTimeoutError);
  await vi.advanceTimersByTimeAsync(20000); await rejected;
  expect(net.complete).toHaveBeenCalledTimes(2);
  net.complete.mockClear();
  const controller = new AbortController();
  const cancelled = completeCutscene("Reward", "issued", controller.signal);
  controller.abort(); await expect(cancelled).rejects.toMatchObject({ name: "AbortError" });
  expect(net.complete).toHaveBeenCalledTimes(1);
  net.complete.mockClear();
  const failed = completeCutscene("Reward", "issued");
  const request = net.complete.mock.calls[0][0];
  for (const receive of net.listeners) receive({ success: false, requestId: request.requestId, error: "rejected" });
  await expect(failed).rejects.toBeInstanceOf(CorrelatedResponseError);
  expect(net.complete).toHaveBeenCalledTimes(1);
});


test("prepared cancellation retries only the same token and remains terminal after a second timeout", async () => {
  vi.useFakeTimers();
  const result = cancelPlayerStep("unused-prepared");
  const rejected = expect(result).rejects.toBeInstanceOf(CorrelatedRequestTimeoutError);
  await vi.advanceTimersByTimeAsync(20000);
  await rejected;
  expect(net.complete).toHaveBeenCalledTimes(2);
  for (const [request] of net.complete.mock.calls) expect(request).toEqual({ requestId: expect.any(String), stepToken: "unused-prepared", cancel: true });
  expect(net.listeners.size).toBe(0);
});

test("a pipeline receipt retry keeps its original direction intent and accepts only the new correlation", async () => {
  vi.useFakeTimers();
  const result = completePlayerStep("issued", undefined, "RIGHT");
  await vi.advanceTimersByTimeAsync(10000);
  const [first, second] = net.complete.mock.calls.map(call => call[0]);
  expect(first).toMatchObject({ stepToken: "issued", nextDirection: "RIGHT" });
  expect(second).toMatchObject({ stepToken: "issued", nextDirection: "RIGHT" });
  for (const receive of net.listeners) receive({ success: true, requestId: second.requestId, replayed: true, mapId: 50, x: 8, y: 8, direction: "RIGHT" });
  await expect(result).resolves.toMatchObject({ replayed: true });
  expect(net.listeners.size).toBe(0);
});

test("a buffered completion publishes its exact tail and never retries an obsolete receipt on timeout", async () => {
  vi.useFakeTimers();
  const result = completePlayerStep("issued", undefined, undefined, {
    afterStepToken: "tail", lookAhead: ["RIGHT", "DOWN"], retryOnTimeout: false,
  });
  const rejected = expect(result).rejects.toBeInstanceOf(CorrelatedRequestTimeoutError);
  expect(net.complete).toHaveBeenCalledWith({
    stepToken: "issued", requestId: expect.any(String),
    afterStepToken: "tail", lookAhead: ["RIGHT", "DOWN"],
  });
  await vi.advanceTimersByTimeAsync(20000);
  await rejected;
  expect(net.complete).toHaveBeenCalledTimes(1);
  expect(net.listeners.size).toBe(0);
});


test("stream initialization is an explicit owned read and accepts an omitted zero sequence", async () => {
  const result = readOwnedPlayerPosition(undefined, undefined, true);
  expect(net.read).toHaveBeenCalledWith({ requestId: expect.any(String), resetMovementStream: true });
  const request = net.read.mock.calls[0][0];
  for (const receive of net.listeners) receive({ success: true, requestId: request.requestId, mapId: 50, x: 7, y: 8, direction: "LEFT", serverMovementPending: false, movementStreamId: "a".repeat(32) });
  await expect(result).resolves.toMatchObject({ movementStreamId: "a".repeat(32) });
  expect(net.request).not.toHaveBeenCalled();
  expect(net.complete).not.toHaveBeenCalled();
});

test("atomic prediction sends one source and direction command and never retries a timed-out historical move", async () => {
  vi.useFakeTimers();
  const command = { commandToken: "b".repeat(32), movementStreamId: "a".repeat(32), sequence: 1, mapId: 50, fromX: 7, fromY: 8, direction: "LEFT" };
  const result = submitPredictedPlayerStep(command);
  const rejected = expect(result).rejects.toBeInstanceOf(CorrelatedRequestTimeoutError);
  expect(net.request).toHaveBeenCalledWith({ ...command, requestId: expect.any(String) });
  await vi.advanceTimersByTimeAsync(20000);
  await rejected;
  expect(net.request).toHaveBeenCalledOnce();
  expect(net.complete).not.toHaveBeenCalled();
  expect(net.listeners.size).toBe(0);
});

test("a committed atomic receipt settles only its matching correlation", async () => {
  const command = { commandToken: "b".repeat(32), movementStreamId: "a".repeat(32), sequence: 1, mapId: 50, fromX: 7, fromY: 8, direction: "LEFT" };
  const result = submitPredictedPlayerStep(command);
  const request = net.request.mock.calls[0][0];
  for (const receive of net.listeners) receive({ success: true, requestId: "unrelated", committed: true, stepToken: command.commandToken, mapId: 50, x: 6, y: 8, direction: "LEFT", ledgeJump: false });
  expect(net.listeners.size).toBe(1);
  for (const receive of net.listeners) receive({ success: true, requestId: request.requestId, committed: true, stepToken: command.commandToken, mapId: 50, x: 6, y: 8, direction: "LEFT", ledgeJump: false });
  await expect(result).resolves.toMatchObject({ committed: true, x: 6 });
  expect(net.listeners.size).toBe(0);
});
