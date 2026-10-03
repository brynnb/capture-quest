import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const network = vi.hoisted(() => ({
  info: new Set<(data: unknown) => void>(),
  warp: new Set<(data: unknown) => void>(),
  warpRequests: [] as Array<{ warpId: number; requestId: string }>,
  load: new Set<(data: unknown) => void>(),
  infoRequests: [] as Array<{ mapId: number; requestId: string }>,
  loadRequests: [] as Array<{ mapId: number; requestId: string }>,
}));

vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onMapInfo: (receive: (data: unknown) => void) => {
    network.info.add(receive);
    return () => network.info.delete(receive);
  },
  onMapLoad: (receive: (data: unknown) => void) => {
    network.load.add(receive);
    return () => network.load.delete(receive);
  },
  onWarpActivation: (receive: (data: unknown) => void) => {
    network.warp.add(receive);
    return () => network.warp.delete(receive);
  },
  requestWarpActivation: (request: { warpId: number; requestId: string }) => network.warpRequests.push(request),
  requestMapInfo: (request: { mapId: number; requestId: string }) => network.infoRequests.push(request),
  requestMapLoad: (request: { mapId: number; requestId: string }) => network.loadRequests.push(request),
}));

vi.mock("./RuntimeAssetCompatibility", () => ({
  ensureRuntimeTileCatalogCurrent: vi.fn(async () => undefined),
}));

import { MapDataService } from "./MapDataService";

describe("correlated map read and load requests", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    network.warp.clear();
    network.warpRequests.length = 0;
    network.info.clear();
    network.load.clear();
    network.infoRequests.length = 0;
    network.loadRequests.length = 0;
  });
  afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });

  const cases = [
    { name: "metadata", handlers: network.info, requests: network.infoRequests,
      start: (service: MapDataService, signal?: AbortSignal) => service.fetchMapInfo(38, signal) },
    { name: "warp activation", handlers: network.warp, requests: network.warpRequests,
      start: (service: MapDataService, signal?: AbortSignal) => service.activateWarp(9, "DOWN", "click", signal) },
    { name: "arrival", handlers: network.load, requests: network.loadRequests,
      start: (service: MapDataService, signal?: AbortSignal) => service.prepareMapLoad(38, 3, 7, signal) },
  ];
  for (const tc of cases) {
    const reply = (data: unknown) => tc.handlers.forEach((receive) => receive(data));
    const success = (requestId: string) => ({ success: true, requestId, id: 38, mapId: 38, x: 3, y: 7, name: "ROOM", width: 8, height: 8, isOverworld: 0 });

    it(`${tc.name}: concurrent same-map requests settle only their own response`, async () => {
      const first = tc.start(new MapDataService());
      const second = tc.start(new MapDataService());
      const [a, b] = tc.requests;
      expect(a.requestId).not.toBe(b.requestId);
      reply(success(b.requestId));
      await second;
      expect(tc.handlers.size).toBe(1);
      reply(success("unrelated"));
      expect(tc.handlers.size).toBe(1);
      reply(success(a.requestId));
      await first;
      expect(tc.handlers.size).toBe(0);
      expect(vi.getTimerCount()).toBe(0);
    });

    it(`${tc.name}: errors and timeout remove listeners and allow a clean retry`, async () => {
      const failed = tc.start(new MapDataService());
      const failure = expect(failed).rejects.toThrow("catalog failure");
      const obsolete = tc.requests[0].requestId;
      reply({ success: false, requestId: obsolete, error: "catalog failure" });
      await failure;
      expect(tc.handlers.size).toBe(0);
      expect(vi.getTimerCount()).toBe(0);

      const timedOut = tc.start(new MapDataService());
      const timeout = expect(timedOut).rejects.toThrow("Timeout fetching map response");
      await vi.advanceTimersByTimeAsync(10_000);
      await timeout;
      expect(tc.handlers.size).toBe(0);

      const retry = tc.start(new MapDataService());
      reply(success(obsolete));
      expect(tc.handlers.size).toBe(1);
      reply(success(tc.requests[2].requestId));
      await retry;
      expect(tc.handlers.size).toBe(0);
      expect(vi.getTimerCount()).toBe(0);
    });

    it(`${tc.name}: a synchronous send failure releases all request resources`, async () => {
      vi.spyOn(tc.requests, "push").mockImplementation(() => { throw new Error("transport closed"); });
      await expect(tc.start(new MapDataService())).rejects.toThrow("transport closed");
      expect(tc.handlers.size).toBe(0);
      expect(vi.getTimerCount()).toBe(0);
    });

    it(`${tc.name}: cancellation prevents admission or cleans up an in-flight request`, async () => {
      const aborted = new AbortController();
      aborted.abort();
      await expect(tc.start(new MapDataService(), aborted.signal)).rejects.toMatchObject({ name: "AbortError" });
      expect(tc.requests).toHaveLength(0);
      const controller = new AbortController();
      const pending = tc.start(new MapDataService(), controller.signal);
      const cancellation = expect(pending).rejects.toMatchObject({ name: "AbortError" });
      controller.abort();
      await cancellation;
      reply(success(tc.requests[0].requestId));
      expect(tc.handlers.size).toBe(0);
      expect(vi.getTimerCount()).toBe(0);
    });
  }

  it("rejects a correlated metadata response with a conflicting map identity", async () => {
    const pending = new MapDataService().fetchMapInfo(38);
    const rejection = expect(pending).rejects.toThrow("identity disagrees");
    network.info.forEach((receive) => receive({ success: true, requestId: network.infoRequests[0].requestId, id: 60 }));
    await rejection;
    expect(network.info.size).toBe(0);
  });
});
