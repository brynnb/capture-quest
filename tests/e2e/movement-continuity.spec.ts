import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createCharacter, enterWorld, loginAsGuest, quitToCharacterSelect, uniqueTrainerName } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

const TILE_SIZE = 16;
const WALK_DURATION_MS = 200;
const PIXELS_PER_MS = TILE_SIZE / WALK_DURATION_MS;
// Each direction has latency, so the added round trip is 200–300 ms.
// Varying latency may bunch packets, but must never reorder a WebSocket stream.
const ONE_WAY_LATENCY_MS = [100, 140, 110, 150, 120, 130];
const TRANSPORT = process.env.CQ_E2E_TRANSPORT === "native" ? "native" : "websocket";

type PositionSample = { t: number; wallTime: number; delta: number; x: number; y: number };
type Packet = { direction: string; opcode: number; delayMs: number; data: unknown };
type Run = { name: string; samples: PositionSample[]; metrics: ReturnType<typeof movementMetrics> };
type Evidence = {
  runs: Run[];
  packets: Packet[];
  transportChecks: Array<{ stage: string; connected: boolean; websocket: boolean; native: boolean }>;
  retargetAuthorization?: {
    boundaryTile: { x: number; y: number };
    slowFrameGroups: number;
    slowFrames: Array<{ index: number; atBoundary: boolean; speedRatio: number }>;
    authorizationGapMs: number;
    maximumAuthorizationGapMs: number;
    before: ReturnType<typeof movementMetrics>;
    after: ReturnType<typeof movementMetrics>;
  };
};

async function installHarness(page: Page, evidence: Evidence) {
  let delayEnabled = false;
  const delayed = new Set<NodeJS.Timeout>();
  const movementOpcodes = new Set<number>([
    OpCodes.PlayerStepRequest,
    OpCodes.PlayerStepResponse,
    OpCodes.PlayerStepCompleteRequest,
    OpCodes.PlayerStepCompleteResponse,
  ]);
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    const queues = {
      sent: { lastDelivery: 0, index: 0 },
      received: { lastDelivery: 0, index: 3 },
    };
    const forward = (
      direction: keyof typeof queues,
      message: string | Buffer,
      send: (message: string | Buffer) => void,
    ) => {
      const queue = queues[direction];
      const now = performance.now();
      const delivery = delayEnabled
        ? Math.max(now + ONE_WAY_LATENCY_MS[queue.index++ % ONE_WAY_LATENCY_MS.length], queue.lastDelivery + 1)
        : now;
      queue.lastDelivery = delivery;
      const delayMs = delivery - now;
      const opcode = Buffer.isBuffer(message) && message.length >= 6 ? message.readUInt16LE(4) : -1;
      if (movementOpcodes.has(opcode)) {
        evidence.packets.push({
          direction,
          opcode,
          delayMs,
          data: JSON.parse((message as Buffer).subarray(6).toString()),
        });
      }
      if (!delayEnabled) {
        send(message);
        return;
      }
      const timer = setTimeout(() => {
        delayed.delete(timer);
        send(message);
      }, delayMs);
      delayed.add(timer);
    };
    socket.onMessage((message) => forward("sent", message, (data) => server.send(data)));
    server.onMessage((message) => forward("received", message, (data) => socket.send(data)));
  });
  await page.addInitScript(() => {
    (window as any).__movementContinuity = {};
  });
  return {
    enableLatency: () => { delayEnabled = TRANSPORT === "websocket"; },
    dispose: () => {
      for (const timer of delayed) clearTimeout(timer);
      delayed.clear();
    },
  };
}

async function assertTransport(page: Page, evidence: Evidence, stage: "ready" | "finished") {
  const actual = await page.evaluate(async () => {
    const path = "/src/net/index.ts";
    const { WorldSocket } = await import(path) as typeof import("../../src/net/index");
    const socket = WorldSocket as unknown as { useWebSocket: boolean; webtransport: unknown };
    return { connected: WorldSocket.isConnected, websocket: socket.useWebSocket, native: Boolean(socket.webtransport) };
  });
  evidence.transportChecks.push({ stage, ...actual });
  if (TRANSPORT === "native") {
    expect(actual, "Exercise native WebTransport without falling back to WebSocket").toEqual({ connected: true, websocket: false, native: true });
    expect(evidence.packets, "Native coverage must not silently use the WebSocket fallback").toHaveLength(0);
  } else {
    expect(actual.connected).toBe(true);
    expect(actual.websocket, "Latency coverage requires the intercepted WebSocket transport").toBe(true);
    if (stage === "finished") {
      for (const direction of ["sent", "received"]) {
        expect(
          evidence.packets.filter((packet) => packet.direction === direction && packet.delayMs >= 100).length,
          `Movement packets must really receive the injected ${direction} latency`,
        ).toBeGreaterThan(1);
      }
    }
  }
}

async function enterQuietHouse(page: Page) {
  await loginAsGuest(page);
  const characterName = uniqueTrainerName();
  await createCharacter(page, characterName);
  await page.evaluate(async () => {
    const path = "/src/phaser-game/renderers/MapRenderer.ts";
    const { MapRenderer } = await import(path);
    const render = MapRenderer.prototype.renderMap;
    MapRenderer.prototype.renderMap = function (...args: any[]) {
      (window as any).__movementContinuity.renderer = this;
      return render.apply(this, args as any);
    };
  });
  await enterWorld(page, characterName);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await placePlayer(page, 7, 6);
}

async function placePlayer(page: Page, x: number, y: number) {
  await page.evaluate(async ({ x, y }) => {
    await window.__capturequestTest!.warpToMap(37, x, y, "LEFT");
  }, { x, y });
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await waitForPlayerTile(page, x, y);
  await waitForPlayerIdle(page);
}

async function startSampling(page: Page) {
  const playerId = (await getGameState(page)).player.id;
  await page.evaluate((id) => {
    const state = (window as any).__movementContinuity;
    const sprite = state.renderer?.getActorSprite(id);
    if (!sprite) throw new Error("The rendered player sprite is missing");
    state.sprite = sprite;
    state.samples = [{ t: 0, wallTime: performance.now(), delta: 0, x: sprite.x, y: sprite.y }];
    state.elapsed = 0;
    state.recording = true;
    state.sample = (_time: number, delta: number) => {
      state.elapsed += delta;
      state.samples.push({ t: state.elapsed, wallTime: performance.now(), delta, x: sprite.x, y: sprite.y });
    };
    // Phaser has finished movement for this frame. An independent RAF sampler
    // can run before Phaser, assigning a displacement to the wrong frame delta.
    state.renderer.scene.events.on("postupdate", state.sample);
  }, playerId);
}

async function stopSampling(page: Page): Promise<PositionSample[]> {
  return page.evaluate(() => {
    const state = (window as any).__movementContinuity;
    state.renderer.scene.events.off("postupdate", state.sample);
    state.recording = false;
    return state.samples;
  });
}

function movementMetrics(samples: PositionSample[]) {
  const frames = samples.slice(1).map((sample, index) => {
    const previous = samples[index];
    const distance = Math.abs(sample.x - previous.x) + Math.abs(sample.y - previous.y);
    return { ...sample, distance, expected: sample.delta * PIXELS_PER_MS };
  });
  const first = frames.findIndex((frame) => frame.distance > 0.01);
  let last = frames.length - 1;
  while (last >= 0 && frames[last].distance <= 0.01) last -= 1;
  // Initial authorization and the partial final frame are outside steady travel.
  // Every intermediate tile boundary remains in this window.
  const interior = first < 0 ? [] : frames.slice(first + 1, last).filter((frame) => frame.delta > 0);
  let progressError = 0;
  let maxProgressError = 0;
  for (const frame of interior) {
    progressError += frame.distance - frame.expected;
    maxProgressError = Math.max(maxProgressError, Math.abs(progressError));
  }
  const ratios = interior.map((frame) => frame.distance / frame.expected);
  return {
    motionFrames: first < 0 ? 0 : last - first + 1,
    stationaryFrames: interior.filter((frame) => frame.distance < 0.01).length,
    minSpeedRatio: ratios.length ? Math.min(...ratios) : 0,
    maxSpeedRatio: ratios.length ? Math.max(...ratios) : 0,
    maxProgressError,
    distance: frames.reduce((total, frame) => total + frame.distance, 0),
    interiorFrames: interior.length,
    maxFrameDelta: interior.length ? Math.max(...interior.map((frame) => frame.delta)) : 0,
  };
}

function assertConstantSpeed(run: Run, minimumTiles: number, minimumFrames = 12) {
  const metrics = run.metrics;
  expect(metrics.interiorFrames, `${run.name}: sample real animated frames`).toBeGreaterThan(minimumFrames);
  expect(metrics.distance, `${run.name}: traverse the intended route`).toBeGreaterThanOrEqual(minimumTiles * TILE_SIZE - 0.01);
  expect(metrics.stationaryFrames, `${run.name}: never dwell at an intermediate tile`).toBe(0);
  expect(metrics.minSpeedRatio, `${run.name}: no slow frame at a tile boundary`).toBeGreaterThanOrEqual(0.92);
  expect(metrics.maxSpeedRatio, `${run.name}: no catch-up acceleration or position snap`).toBeLessThanOrEqual(1.08);
  expect(metrics.maxProgressError, `${run.name}: preserve frame time across every tile boundary`).toBeLessThanOrEqual(0.75);
}

function assertRetarget(run: Run, evidence: Evidence) {
  const { samples } = run;
  const turn = { x: 5 * TILE_SIZE + TILE_SIZE / 2, y: 6 * TILE_SIZE + TILE_SIZE / 2 };
  const atTurn = (point: PositionSample) => Math.abs(point.x - turn.x) < 0.01 && Math.abs(point.y - turn.y) < 0.01;
  const firstTurn = samples.findIndex(atTurn);
  let lastTurn = samples.length - 1;
  while (lastTurn >= 0 && !atTurn(samples[lastTurn])) lastTurn -= 1;
  const firstDown = samples.findIndex((sample) => sample.y > turn.y + 0.01);
  const beforeSamples = samples.slice(0, (firstTurn >= 0 ? firstTurn : firstDown - 1) + 1);
  const afterSamples = samples.slice(lastTurn >= 0 ? lastTurn : Math.max(0, firstDown - 1));
  const before = { name: "retarget: active horizontal segment", samples: beforeSamples, metrics: movementMetrics(beforeSamples) };
  const after = { name: "retarget: optimistic downward segment", samples: afterSamples, metrics: movementMetrics(afterSamples) };
  const frames = samples.slice(1).map((sample, index) => {
    const previous = samples[index];
    const distance = Math.abs(sample.x - previous.x) + Math.abs(sample.y - previous.y);
    return { index, delta: sample.delta, distance, atBoundary: atTurn(previous) || atTurn(sample), speedRatio: distance / (sample.delta * PIXELS_PER_MS) };
  });
  const first = frames.findIndex((frame) => frame.distance > 0.01);
  let last = frames.length - 1;
  while (last >= 0 && frames[last].distance <= 0.01) last -= 1;
  const interior = frames.slice(first + 1, last).filter((frame) => frame.delta > 0);
  const slowFrames = interior.filter((frame) => frame.speedRatio < 0.92);
  const slowFrameGroups = slowFrames.reduce((groups, frame, index) => groups + (index === 0 || frame.index !== slowFrames[index - 1].index + 1 ? 1 : 0), 0);
  const authorizationGapMs = Math.max(0, interior.reduce((lost, frame) => lost + frame.delta - frame.distance / PIXELS_PER_MS, 0));
  // Locally walkable retargets continue optimistically through the new turn.
  // Only floating-point error is tolerated; no authorization frame is allowed.
  const maximumAuthorizationGapMs = 0.001;
  evidence.retargetAuthorization = {
    boundaryTile: { x: 5, y: 6 },
    slowFrameGroups,
    slowFrames: slowFrames.map(({ index, atBoundary, speedRatio }) => ({ index, atBoundary, speedRatio })),
    authorizationGapMs,
    maximumAuthorizationGapMs,
    before: before.metrics,
    after: after.metrics,
  };
  console.log(JSON.stringify({ movementRetarget: evidence.retargetAuthorization }));
  for (let index = 0; index < samples.length; index += 1) {
    const sample = samples[index];
    const onHorizontal = Math.abs(sample.y - turn.y) < 0.01 && sample.x >= turn.x - 0.01 && sample.x <= 7 * TILE_SIZE + TILE_SIZE / 2 + 0.01;
    const onVertical = Math.abs(sample.x - turn.x) < 0.01 && sample.y >= turn.y - 0.01 && sample.y <= 7 * TILE_SIZE + TILE_SIZE / 2 + 0.01;
    expect(onHorizontal || onVertical, "Retarget follows only the active step and latest destination; no stale suffix").toBe(true);
    if (index > 0) {
      expect(sample.x - samples[index - 1].x, "Retarget never backtracks or corrects an obsolete step").toBeLessThanOrEqual(0.01);
      expect(sample.y - samples[index - 1].y, "Retarget never snaps away from the new destination").toBeGreaterThanOrEqual(-0.01);
    }
  }
  expect(run.metrics.distance, "Retarget travels exactly two tiles left and one down").toBeCloseTo(3 * TILE_SIZE, 3);
  expect(run.metrics.maxSpeedRatio, "Retarget never accelerates to conceal authorization delay").toBeLessThanOrEqual(1.08);
  expect(slowFrameGroups, "Changing a walkable destination must not introduce an authorization pause").toBe(0);
  expect(authorizationGapMs, "The new direction starts continuously before its server grant arrives").toBeLessThanOrEqual(maximumAuthorizationGapMs);
  assertConstantSpeed(run, 3);
  assertConstantSpeed(before, 1, 3);
  assertConstantSpeed(after, 1, 3);
}

async function finishRun(page: Page, evidence: Evidence, name: string) {
  const samples = await stopSampling(page);
  const run = { name, samples, metrics: movementMetrics(samples) };
  evidence.runs.push(run);
  console.log(JSON.stringify({ movementRun: name, ...run.metrics }));
  return run;
}

async function assertDestination(page: Page, x: number, y: number) {
  await waitForPlayerTile(page, x, y);
  await waitForPlayerIdle(page);
  const point = await page.evaluate(() => {
    const state = (window as any).__movementContinuity;
    const id = window.__capturequestTest!.getState().player.id;
    const sprite = state.renderer.getActorSprite(id);
    return { x: sprite.x, y: sprite.y };
  });
  expect(point.x, "The real sprite reaches the clicked tile center").toBeCloseTo(x * TILE_SIZE + TILE_SIZE / 2, 3);
  expect(point.y, "The real sprite reaches the clicked tile center").toBeCloseTo(y * TILE_SIZE + TILE_SIZE / 2, 3);
}

async function saveEvidence(page: Page, evidence: Evidence) {
  const unfinished = await page.evaluate(() => {
    const state = (window as any).__movementContinuity;
    if (!state?.recording) return null;
    state.renderer.scene.events.off("postupdate", state.sample);
    state.recording = false;
    return state.samples as PositionSample[];
  }).catch(() => null);
  if (unfinished) evidence.runs.push({ name: "unfinished movement at failure", samples: unfinished, metrics: movementMetrics(unfinished) });
  const evidencePath = test.info().outputPath("movement-continuity.json");
  await writeFile(evidencePath, JSON.stringify({ transport: TRANSPORT, addedLatencyMs: TRANSPORT === "native" ? 0 : { oneWay: ONE_WAY_LATENCY_MS, roundTripRange: [200, 300] }, walkDurationMs: WALK_DURATION_MS, ...evidence }, null, 2));
  await test.info().attach("movement-continuity.json", { path: evidencePath, contentType: "application/json" });
  await page.screenshot({ path: test.info().outputPath("continuous-movement.png") }).catch(() => undefined);
}

test("click paths keep constant rendered speed, reach nearby tiles, and accept a new destination while walking", async ({ page }) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);
  const evidence: Evidence = { runs: [], packets: [], transportChecks: [] };
  const network = await installHarness(page, evidence);
  try {
    await enterQuietHouse(page);
    await assertTransport(page, evidence, "ready");
    network.enableLatency();

    await startSampling(page);
    // The house has an immobile NPC at (5,4) and exit warps at (2,7)/(3,7).
    // This eleven-tile route uses ordinary floor and necessarily includes turns.
    await clickTile(page, 0, 2);
    await assertDestination(page, 0, 2);
    const corners = await finishRun(page, evidence, "long click path with corners");

    // A succession of very nearby, plainly walkable targets must each be honored.
    for (const [x, y] of [[1, 2], [2, 2], [3, 2], [3, 3], [2, 3], [1, 3]]) {
      await clickTile(page, x, y);
      await assertDestination(page, x, y);
    }

    await placePlayer(page, 7, 6);
    await startSampling(page);
    await clickTile(page, 0, 6);
    await page.waitForFunction(() => (window as any).__movementContinuity.sprite.x < 6 * 16 + 7);
    // Replace a long walk with a destination beside the currently active step.
    // Previously handleTileClick silently discarded all clicks during stepAbort.
    await clickTile(page, 5, 7);
    await assertDestination(page, 5, 7);
    const retarget = await finishRun(page, evidence, "nearby destination replaces an active click path");
    await page.waitForTimeout(650);
    await assertDestination(page, 5, 7);

    assertConstantSpeed(corners, 11);
    assertRetarget(retarget, evidence);
    await assertTransport(page, evidence, "finished");
    errors.assertNoSevereErrors();
    await page.screenshot({ path: test.info().outputPath("movement-arrived.png") });
    await quitToCharacterSelect(page);
  } finally {
    await saveEvidence(page, evidence);
    network.dispose();
  }
});

test("held WASD stays at constant speed and releasing a key stops on the current tile", async ({ page }) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);
  const evidence: Evidence = { runs: [], packets: [], transportChecks: [] };
  const network = await installHarness(page, evidence);
  try {
    await enterQuietHouse(page);
    await assertTransport(page, evidence, "ready");
    network.enableLatency();
    const legs = [
      { key: "a", axis: "x", beyond: "less", threshold: 2 * 16 + 7, x: 1, y: 6, tiles: 6 },
      { key: "w", axis: "y", beyond: "less", threshold: 3 * 16 + 7, x: 1, y: 2, tiles: 4 },
      { key: "d", axis: "x", beyond: "greater", threshold: 5 * 16 + 9, x: 6, y: 2, tiles: 5 },
      { key: "s", axis: "y", beyond: "greater", threshold: 4 * 16 + 9, x: 6, y: 5, tiles: 3 },
    ];
    for (const leg of legs) {
      await startSampling(page);
      await page.evaluate(() => {
        if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
      });
      await page.keyboard.down(leg.key);
      try {
        await page.waitForFunction(({ axis, beyond, threshold }) => {
          const value = (window as any).__movementContinuity.sprite[axis];
          return beyond === "less" ? value < threshold : value > threshold;
        }, leg);
      } finally {
        await page.keyboard.up(leg.key);
      }
      await assertDestination(page, leg.x, leg.y);
      const run = await finishRun(page, evidence, `held ${leg.key.toUpperCase()}`);
      // Let every delayed completion/cancellation arrive: no extra step or snap.
      await page.waitForTimeout(650);
      await assertDestination(page, leg.x, leg.y);
      assertConstantSpeed(run, leg.tiles);
    }
    await expect(page.getByRole("button", { name: "Bag", exact: true })).toBeEnabled();
    await page.getByRole("button", { name: "Bag", exact: true }).click();
    await expect.poll(async () => (await getGameState(page)).ui.isInventoryOpen).toBe(true);
    await page.getByRole("button", { name: "Bag", exact: true }).click();
    await assertTransport(page, evidence, "finished");
    errors.assertNoSevereErrors();
    await page.screenshot({ path: test.info().outputPath("movement-arrived.png") });
    await quitToCharacterSelect(page);
  } finally {
    await saveEvidence(page, evidence);
    network.dispose();
  }
});
