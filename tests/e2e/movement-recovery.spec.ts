import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";
import type { OwnedPlayerPositionResponse, PlayerStepCompleteRequest, PlayerStepCompleteResponse, PlayerStepRequest } from "../../src/net/generated/protocol";

type Frame = { t: number; delta: number; x: number; y: number };

async function installRenderedWitness(page: Page) {
  await page.evaluate(async () => {
    const path = "/src/phaser-game/renderers/MapRenderer.ts";
    const { MapRenderer } = await import(path);
    (window as any).__movementRecovery = {};
    const render = MapRenderer.prototype.renderMap;
    MapRenderer.prototype.renderMap = function (...args: any[]) {
      (window as any).__movementRecovery.renderer = this;
      return render.apply(this, args as any);
    };
  });
}

async function assertWebSocket(page: Page) {
  const actual = await page.evaluate(async () => {
    const path = "/src/net/index.ts";
    const { WorldSocket } = await import(path) as typeof import("../../src/net/index");
    return { connected: WorldSocket.isConnected, websocket: (WorldSocket as unknown as { useWebSocket: boolean }).useWebSocket };
  });
  expect(actual, "The lost-reply regression must exercise the intercepted transport").toEqual({ connected: true, websocket: true });
}

async function placeInHouse(page: Page, x: number, y: number) {
  await page.evaluate(async ({ x, y }) => {
    await window.__capturequestTest!.warpToMap(37, x, y, "LEFT");
  }, { x, y });
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await assertRenderedPose(page, x, y);
}

async function assertRenderedPose(page: Page, x: number, y: number) {
  await waitForPlayerTile(page, x, y);
  await waitForPlayerIdle(page);
  const point = await page.evaluate(() => {
    const id = window.__capturequestTest!.getState().player.id;
    const sprite = (window as any).__movementRecovery.renderer.getActorSprite(id);
    return { x: sprite.x, y: sprite.y };
  });
  expect(point).toEqual({ x: x * 16 + 8, y: y * 16 + 8 });
}

async function startFrames(page: Page) {
  await page.evaluate(() => {
    const witness = (window as any).__movementRecovery;
    const id = window.__capturequestTest!.getState().player.id;
    const sprite = witness.renderer.getActorSprite(id);
    witness.elapsed = 0;
    witness.frames = [{ t: 0, delta: 0, x: sprite.x, y: sprite.y }];
    witness.sample = (_time: number, delta: number) => {
      witness.elapsed += delta;
      witness.frames.push({ t: witness.elapsed, delta, x: sprite.x, y: sprite.y });
    };
    witness.renderer.scene.events.on("postupdate", witness.sample);
  });
}

async function stopFrames(page: Page): Promise<Frame[]> {
  return page.evaluate(() => {
    const witness = (window as any).__movementRecovery;
    if (witness.sample) witness.renderer.scene.events.off("postupdate", witness.sample);
    return witness.frames ?? [];
  });
}

function frameMetrics(samples: Frame[]) {
  const frames = samples.slice(1).map((sample, index) => ({
    ...sample,
    distance: Math.abs(sample.x - samples[index].x) + Math.abs(sample.y - samples[index].y),
  }));
  const first = frames.findIndex((frame) => frame.distance > 0.01);
  let last = frames.length - 1;
  while (last >= 0 && frames[last].distance <= 0.01) last -= 1;
  const interior = first < 0 ? [] : frames.slice(first + 1, last).filter((frame) => frame.delta > 0);
  let error = 0;
  let maxProgressError = 0;
  for (const frame of interior) {
    error += frame.distance - frame.delta * 0.08;
    maxProgressError = Math.max(maxProgressError, Math.abs(error));
  }
  const ratios = interior.map((frame) => frame.distance / (frame.delta * 0.08));
  return {
    distance: frames.reduce((total, frame) => total + frame.distance, 0),
    interiorFrames: interior.length,
    stationaryFrames: interior.filter((frame) => frame.distance < 0.01).length,
    minSpeedRatio: ratios.length ? Math.min(...ratios) : 0,
    maxSpeedRatio: ratios.length ? Math.max(...ratios) : 0,
    maxProgressError,
  };
}

async function saveEvidence(name: string, evidence: unknown) {
  const path = test.info().outputPath(name);
  await writeFile(path, JSON.stringify(evidence, null, 2));
  await test.info().attach(name, { path, contentType: "application/json" });
}

test("a lost final step reply recovers from owned position without retrying and its historical receipt cannot rewind re-entry", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Reply dropping requires the WebSocket transport");
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const completions: PlayerStepCompleteRequest[] = [];
  const receipts: PlayerStepCompleteResponse[] = [];
  const ownedRequests: Array<{ requestId: string }> = [];
  const ownedPositions: OwnedPlayerPositionResponse[] = [];
  const dropped: { receipt?: PlayerStepCompleteResponse } = {};
  let armed = false;
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PlayerStepCompleteRequest) completions.push(JSON.parse(message.subarray(6).toString()));
        if (opcode === OpCodes.OwnedPlayerPositionRequest) ownedRequests.push(JSON.parse(message.subarray(6).toString()));
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        const result = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepCompleteResponse && result.success) {
          const request = completions.find((completion) => completion.requestId === result.requestId);
          if (!dropped.receipt && request && !request.cancel) {
            dropped.receipt = result;
            return;
          }
          receipts.push(result);
        }
        if (opcode === OpCodes.OwnedPlayerPositionResponse && result.success) ownedPositions.push(result);
      }
      socket.send(message);
    });
  });
  try {
    const character = await createGuestCharacterAndEnterWorld(page);
    await installRenderedWitness(page);
    await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
    await placeInHouse(page, 6, 7);
    await assertWebSocket(page);
    armed = true;
    await pressMovement(page, "left");
    await expect.poll(() => ownedPositions.length, { timeout: 20_000 }).toBe(1);
    await assertRenderedPose(page, 5, 7);
    const commits = completions.filter((completion) => !completion.cancel);
    expect(commits, "A lost final reply uses a current read, never a historical-token retry").toHaveLength(1);
    expect(dropped.receipt).toMatchObject({ success: true, mapId: 37, x: 5, y: 7 });
    expect(ownedRequests).toHaveLength(1);
    expect(ownedPositions[0]).toMatchObject({ requestId: ownedRequests[0].requestId, mapId: 37, x: 5, y: 7, direction: "LEFT", serverMovementPending: false });
    expect(receipts.some((receipt) => receipt.replayed), "No automatic historical replay occurs").toBe(false);
    expect(errors.sentOpcodes).toContain(OpCodes.OwnedPlayerPositionRequest);

    // A deliberately replayed durable receipt after re-entry remains historical,
    // even when the current owned position differs from its old successful step.
    await quitToCharacterSelect(page);
    await enterWorld(page, character);
    await waitForNoMapLoading(page);
    await waitForPlayerIdle(page);
    await placeInHouse(page, 6, 7);
    await page.evaluate(async (stepToken) => {
      const bridgePath = "/src/net/NetworkBridge.ts";
      const opcodePath = "/src/net/generated/opcodes.ts";
      const { NetworkBridge } = await import(bridgePath);
      const opcodes = await import(opcodePath);
      NetworkBridge.send({ stepToken, requestId: "after-reentry" }, opcodes.PlayerStepCompleteRequest);
    }, commits[0].stepToken);
    await expect.poll(() => receipts.find((receipt) => receipt.requestId === "after-reentry")?.replayed).toBe(true);
    expect(receipts.find((receipt) => receipt.requestId === "after-reentry")).toMatchObject({ mapId: 37, x: 5, y: 7, replayed: true });
    await assertRenderedPose(page, 6, 7);
    await page.screenshot({ path: test.info().outputPath("historical-receipt-preserves-current-position.png") });
    await quitToCharacterSelect(page);
    errors.assertNoSevereErrors();
  } finally {
    await saveEvidence("lost-final-step-recovery.json", { dropped, completions, receipts, ownedRequests, ownedPositions });
  }
});

test("a later receipt restores a lost movement refill without pauses, stale retries, or stopping before the destination", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Reply dropping requires the WebSocket transport");
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const requests: PlayerStepRequest[] = [];
  const completions: PlayerStepCompleteRequest[] = [];
  const receipts: PlayerStepCompleteResponse[] = [];
  const ownedRequests: unknown[] = [];
  const failures: unknown[] = [];
  const deliveredTokens = new Set<string>();
  const dropped: {
    request?: PlayerStepCompleteRequest;
    receipt?: PlayerStepCompleteResponse;
    at?: number;
    newTokens?: string[];
    restoredBy?: string;
  } = {};
  let armed = false;
  let samples: Frame[] = [];
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        const request = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepRequest) requests.push(request);
        if (opcode === OpCodes.PlayerStepCompleteRequest) completions.push(request);
        if (opcode === OpCodes.OwnedPlayerPositionRequest) ownedRequests.push(request);
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PlayerStepResponse || opcode === OpCodes.PlayerStepCompleteResponse) {
          const result = JSON.parse(message.subarray(6).toString());
          if (!result.success) failures.push(result);
          const request = completions.find((completion) => completion.requestId === result.requestId);
          if (!dropped.receipt && opcode === OpCodes.PlayerStepCompleteResponse && result.success && request?.lookAhead?.length && !request.cancel) {
            const newTokens = (result.continuationSteps ?? []).map((step: { stepToken: string }) => step.stepToken).filter((token: string) => !deliveredTokens.has(token));
            if (newTokens.length) {
              dropped.request = request;
              dropped.receipt = result;
              dropped.newTokens = newTokens;
              dropped.at = Date.now();
              return;
            }
          }
          if (opcode === OpCodes.PlayerStepCompleteResponse && result.success) {
            receipts.push(result);
            if (dropped.newTokens?.some((token) => result.continuationSteps?.some((step: { stepToken: string }) => step.stepToken === token))) {
              dropped.restoredBy ??= result.requestId;
            }
          }
          if (result.stepToken) deliveredTokens.add(result.stepToken);
          for (const step of result.continuationSteps ?? []) deliveredTokens.add(step.stepToken);
        }
      }
      socket.send(message);
    });
  });
  try {
    await createGuestCharacterAndEnterWorld(page);
    await installRenderedWitness(page);
    await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
    await placeInHouse(page, 7, 6);
    await assertWebSocket(page);
    armed = true;
    await startFrames(page);
    await clickTile(page, 0, 2);
    await assertRenderedPose(page, 0, 2);
    samples = await stopFrames(page);
    expect(dropped.receipt, "Drop a real successful refill, not an arbitrary step or cancellation").toBeDefined();
    expect(dropped.request?.lookAhead?.length).toBeGreaterThan(0);
    expect(dropped.restoredBy, "A subsequent successful receipt must restore the missing authorized window").toBeDefined();
    expect(dropped.restoredBy).not.toBe(dropped.request?.requestId);
    expect(completions.some((completion) => dropped.newTokens?.includes(completion.stepToken)), "The restored grants actually carry the player past the old window").toBe(true);

    // CorrelatedRequest has a ten-second deadline. Remain in-world beyond the
    // dropped request's deadline to catch a late historical retry or recovery.
    await page.waitForTimeout(Math.max(0, (dropped.at ?? Date.now()) + 10_250 - Date.now()));
    await assertRenderedPose(page, 0, 2);
    const commits = completions.filter((completion) => !completion.cancel);
    expect(requests, "The original route continues using the restored window").toHaveLength(1);
    expect(commits).toHaveLength(11);
    expect(new Set(commits.map((completion) => completion.stepToken)).size, "Every visual tile is committed once; never retry a historical token").toBe(11);
    expect(receipts.every((receipt) => !receipt.replayed)).toBe(true);
    expect(ownedRequests, "Later confirmation settles the lost reply without interrupting for position recovery").toEqual([]);
    expect(failures).toEqual([]);
    const metrics = frameMetrics(samples);
    console.log(JSON.stringify({ lostRefillMovement: metrics, droppedRequestId: dropped.request?.requestId, restoredBy: dropped.restoredBy }));
    expect(metrics.distance).toBeCloseTo(11 * 16, 3);
    expect(metrics.interiorFrames).toBeGreaterThan(20);
    expect(metrics.stationaryFrames, "A lost refill must not reintroduce per-tile pauses").toBe(0);
    expect(metrics.minSpeedRatio).toBeGreaterThanOrEqual(0.92);
    expect(metrics.maxSpeedRatio).toBeLessThanOrEqual(1.08);
    expect(metrics.maxProgressError).toBeLessThanOrEqual(0.75);
    await page.screenshot({ path: test.info().outputPath("lost-refill-arrived.png") });
    errors.assertNoSevereErrors();
    await quitToCharacterSelect(page);
  } finally {
    if (!samples.length) samples = await stopFrames(page).catch(() => []);
    await saveEvidence("lost-movement-refill.json", { dropped, requests, completions, receipts, ownedRequests, failures, metrics: frameMetrics(samples), samples });
  }
});
