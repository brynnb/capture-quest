import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";
import type { OwnedPlayerPositionResponse, PlayerStepRequest, PlayerStepResponse, PlayerStepError } from "../../src/net/generated/protocol";

type AtomicMovement = PlayerStepRequest & { commandToken: string; movementStreamId: string; sequence: number };
type AtomicReceipt = PlayerStepResponse & { committed: true; replayed?: boolean };

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

test("a lost final commit reply recovers current position and a retired stream cannot rewind character re-entry", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Reply dropping requires the WebSocket transport");
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const commands: AtomicMovement[] = [];
  const receipts: Array<AtomicReceipt | PlayerStepError> = [];
  const ownedRequests: Array<{ requestId: string; resetMovementStream?: boolean; stepToken?: string }> = [];
  const ownedPositions: OwnedPlayerPositionResponse[] = [];
  const legacyCompletions: unknown[] = [];
  const dropped: { receipt?: AtomicReceipt } = {};
  let armed = false;
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        const data = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepRequest && data.commandToken) commands.push(data);
        if (opcode === OpCodes.PlayerStepCompleteRequest) legacyCompletions.push(data);
        if (opcode === OpCodes.OwnedPlayerPositionRequest) ownedRequests.push(data);
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        const result = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepResponse) {
          if (!dropped.receipt && result.success && result.committed) {
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
    const recoveryRequests = () => ownedRequests.filter((request) => request.resetMovementStream !== true);
    await expect.poll(() => recoveryRequests().some((request) => ownedPositions.some((position) => position.requestId === request.requestId)), { timeout: 20_000 }).toBe(true);
    await assertRenderedPose(page, 5, 7);
    expect(commands, "A lost final reply uses a current read, never resending its historical command").toHaveLength(1);
    expect(dropped.receipt).toMatchObject({ success: true, committed: true, stepToken: commands[0].commandToken, mapId: 37, x: 5, y: 7 });
    expect(recoveryRequests()).toHaveLength(1);
    expect(ownedPositions.find((position) => position.requestId === recoveryRequests()[0].requestId)).toMatchObject({ mapId: 37, x: 5, y: 7, direction: "LEFT", serverMovementPending: false });
    expect(legacyCompletions, "Atomic movement never falls back to the old completion phase").toEqual([]);
    expect(receipts.some((receipt) => receipt.success && receipt.replayed), "No automatic historical replay occurs").toBe(false);
    const original = commands[0];

    await quitToCharacterSelect(page);
    await enterWorld(page, character);
    await waitForNoMapLoading(page);
    await waitForPlayerIdle(page);
    await placeInHouse(page, 6, 7);
    // The old stream belongs to the retired movement owner. Even a durable
    // command token cannot authorize movement through that old stream again.
    await page.evaluate(async (command) => {
      const bridgePath = "/src/net/NetworkBridge.ts";
      const opcodePath = "/src/net/generated/opcodes.ts";
      const { NetworkBridge } = await import(bridgePath);
      const opcodes = await import(opcodePath);
      NetworkBridge.send({ ...command, requestId: "retired-stream-after-reentry" }, opcodes.PlayerStepRequest);
    }, original);
    await expect.poll(() => receipts.find((receipt) => receipt.requestId === "retired-stream-after-reentry")?.success).toBe(false);
    expect(receipts.find((receipt) => receipt.requestId === "retired-stream-after-reentry")).toMatchObject({ success: false, mapId: 37, x: 6, y: 7 });
    await assertRenderedPose(page, 6, 7);

    // Read the durable historical receipt explicitly, alongside the current
    // position. Its old successful tile is evidence, never a position setter.
    await page.evaluate(async (stepToken) => {
      const bridgePath = "/src/net/NetworkBridge.ts";
      const opcodePath = "/src/net/generated/opcodes.ts";
      const { NetworkBridge } = await import(bridgePath);
      const opcodes = await import(opcodePath);
      NetworkBridge.send({ stepToken, requestId: "historical-step-after-reentry" }, opcodes.OwnedPlayerPositionRequest);
    }, original.commandToken);
    await expect.poll(() => ownedPositions.find((position) => position.requestId === "historical-step-after-reentry")?.committedStep).toBeDefined();
    expect(ownedPositions.find((position) => position.requestId === "historical-step-after-reentry")).toMatchObject({
      mapId: 37, x: 6, y: 7,
      committedStep: { stepToken: original.commandToken, mapId: 37, x: 5, y: 7, direction: "LEFT" },
    });
    await assertRenderedPose(page, 6, 7);
    await page.screenshot({ path: test.info().outputPath("historical-receipt-preserves-current-position.png") });
    await quitToCharacterSelect(page);
    errors.assertNoSevereErrors();
  } finally {
    await saveEvidence("lost-final-step-recovery.json", { dropped, commands, receipts, ownedRequests, ownedPositions, legacyCompletions });
  }
});

test("a later committed tile confirms a lost intermediate receipt without pauses, historical retries, or stopping short", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Reply dropping requires the WebSocket transport");
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const commands: AtomicMovement[] = [];
  const receipts: AtomicReceipt[] = [];
  const ownedRequests: Array<{ requestId: string; resetMovementStream?: boolean }> = [];
  const legacyCompletions: unknown[] = [];
  const failures: unknown[] = [];
  const dropped: { request?: AtomicMovement; receipt?: AtomicReceipt; at?: number; confirmedBy?: string } = {};
  let armed = false;
  let samples: Frame[] = [];
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        const request = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepRequest && request.commandToken) commands.push(request);
        if (opcode === OpCodes.PlayerStepCompleteRequest) legacyCompletions.push(request);
        if (opcode === OpCodes.OwnedPlayerPositionRequest) ownedRequests.push(request);
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (armed && Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PlayerStepResponse) {
        const result = JSON.parse(message.subarray(6).toString());
        if (!result.success) failures.push(result);
        const request = commands.find((command) => command.requestId === result.requestId);
        if (!dropped.receipt && result.success && result.committed && request && commands.indexOf(request) === 4) {
          dropped.request = request;
          dropped.receipt = result;
          dropped.at = Date.now();
          return;
        }
        if (result.success && result.committed) {
          receipts.push(result);
          if (dropped.request && request && request.movementStreamId === dropped.request.movementStreamId && request.sequence > dropped.request.sequence) {
            dropped.confirmedBy ??= result.requestId;
          }
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
    expect(dropped.receipt, "Drop a real committed fifth tile, not an admission or bootstrap").toBeDefined();
    expect(dropped.confirmedBy, "A later committed sequence must confirm progress beyond the lost receipt").toBeDefined();
    expect(dropped.confirmedBy).not.toBe(dropped.request?.requestId);

    // Remain in-world past the lost command's correlated-request deadline.
    await page.waitForTimeout(Math.max(0, (dropped.at ?? Date.now()) + 10_250 - Date.now()));
    await assertRenderedPose(page, 0, 2);
    expect(commands, "Every visible tile uses one atomic command").toHaveLength(11);
    expect(new Set(commands.map((command) => command.commandToken)).size, "Never retry a historical command token").toBe(11);
    expect(new Set(commands.map((command) => command.movementStreamId)).size).toBe(1);
    expect(commands.map((command) => command.sequence)).toEqual(Array.from({ length: 11 }, (_, index) => commands[0].sequence + index));
    expect(receipts.every((receipt) => !receipt.replayed)).toBe(true);
    expect(ownedRequests.filter((request) => request.resetMovementStream !== true), "A later sequence settles the lost reply without a recovery read").toEqual([]);
    expect(legacyCompletions).toEqual([]);
    expect(failures).toEqual([]);
    const final = receipts.find((receipt) => receipt.requestId === commands[10].requestId);
    expect(final).toMatchObject({ success: true, committed: true, stepToken: commands[10].commandToken, mapId: 37, x: 0, y: 2 });
    const metrics = frameMetrics(samples);
    console.log(JSON.stringify({ lostIntermediateMovement: metrics, droppedRequestId: dropped.request?.requestId, confirmedBy: dropped.confirmedBy }));
    expect(metrics.distance).toBeCloseTo(11 * 16, 3);
    expect(metrics.interiorFrames).toBeGreaterThan(20);
    expect(metrics.stationaryFrames, "A lost receipt must not reintroduce per-tile pauses").toBe(0);
    expect(metrics.minSpeedRatio).toBeGreaterThanOrEqual(0.92);
    expect(metrics.maxSpeedRatio).toBeLessThanOrEqual(1.08);
    expect(metrics.maxProgressError).toBeLessThanOrEqual(0.75);
    await page.screenshot({ path: test.info().outputPath("lost-intermediate-receipt-arrived.png") });
    errors.assertNoSevereErrors();
    await quitToCharacterSelect(page);
  } finally {
    if (!samples.length) samples = await stopFrames(page).catch(() => []);
    await saveEvidence("lost-intermediate-movement-receipt.json", { dropped, commands, receipts, ownedRequests, legacyCompletions, failures, metrics: frameMetrics(samples), samples });
  }
});
