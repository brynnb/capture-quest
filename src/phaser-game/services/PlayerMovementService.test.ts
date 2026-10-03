import { afterEach, expect, test, vi } from "vitest";
import type { PlayerStepCompleteResponse } from "@/net/generated/protocol";
import { completePlayerStep, readOwnedPlayerPosition } from "./PlayerMovementService";
import { CorrelatedRequestTimeoutError, CorrelatedResponseError } from "./CorrelatedRequest";

type StepReply = PlayerStepCompleteResponse | { success: false; requestId: string; error: string };

const net = vi.hoisted(() => ({
  connected: true,
  listeners: new Set<(data: StepReply) => void>(),
  complete: vi.fn(),
  read: vi.fn(),
}));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => net.connected,
  onPlayerStepComplete: (receive: (data: StepReply) => void) => {
    net.listeners.add(receive); return () => net.listeners.delete(receive);
  },
  completePlayerStep: net.complete,
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
