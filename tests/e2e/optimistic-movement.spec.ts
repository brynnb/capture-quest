import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import * as OpCodes from "../../src/net/generated/opcodes";
import type { MapRenderer as MapRendererType } from "../../src/phaser-game/renderers/MapRenderer";
import type { PlayerStepRequest, PlayerStepResponse, PlayerStepCompleteRequest, PlayerStepCompleteResponse, OwnedPlayerPositionRequest, OwnedPlayerPositionResponse, ServerPlayerMovementNotify } from "../../src/net/generated/protocol";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

const ONE_WAY_LATENCY_MS = [120, 170, 140, 160, 130, 150];
const SPEED = 16 / 200;
type Frame = { frame: number; t: number; wall: number; epoch: number; delta: number; x: number; y: number };
type Input = { type: string; key?: string; repeat?: boolean; frame: number; wall: number; epoch: number };
type MovementWirePayload = Partial<
  PlayerStepRequest &
  Omit<PlayerStepResponse, "success"> &
  PlayerStepCompleteRequest &
  Omit<PlayerStepCompleteResponse, "success"> &
  OwnedPlayerPositionRequest &
  Omit<OwnedPlayerPositionResponse, "success"> &
  ServerPlayerMovementNotify
> & { success?: boolean };
type SampleHandler = (time: number, delta: number) => void;
type RendererWitness = Pick<MapRendererType, "getActorSprite"> & {
  scene: { events: { on: (event: string, handler: SampleHandler) => unknown; off: (event: string, handler: SampleHandler) => unknown } };
};
type OptimisticWitness = {
  renderer?: RendererWitness;
  sprite?: { x: number; y: number };
  recording: boolean;
  frame: number;
  elapsed: number;
  frames: Frame[];
  inputs: Input[];
  sample?: SampleHandler;
};
type MovementWitnessWindow = Window & { __optimisticMovement: OptimisticWitness };
type Wire = { direction: "sent" | "received"; opcode: number; observedAt: number; forwardedAt?: number; delayMs: number; data: MovementWirePayload; originalData?: MovementWirePayload };
type Run = { name: string; packetStart: number; frames: Frame[]; inputs: Input[]; metrics: ReturnType<typeof movementMetrics>; start?: { renderFrames: number; wallMs: number; inputAt: number; firstMotionAt: number; responseAt: number; bootstrapResponseAt?: number } };

function movementMetrics(samples: Frame[]) {
  const frames = samples.slice(1).map((sample, index) => {
    const previous = samples[index];
    return { ...sample, distance: Math.abs(sample.x - previous.x) + Math.abs(sample.y - previous.y), wallDelta: sample.wall - previous.wall };
  });
  const first = frames.findIndex((frame) => frame.distance > 0.01);
  let last = frames.length - 1;
  while (last >= 0 && frames[last].distance <= 0.01) last -= 1;
  const interior = first < 0 ? [] : frames.slice(first + 1, last).filter((frame) => frame.delta > 0);
  let error = 0;
  let maxProgressError = 0;
  for (const frame of interior) {
    error += frame.distance - frame.delta * SPEED;
    maxProgressError = Math.max(maxProgressError, Math.abs(error));
  }
  const ratios = interior.map((frame) => frame.distance / (frame.delta * SPEED));
  const wallMs = interior.reduce((sum, frame) => sum + frame.wallDelta, 0);
  const distance = interior.reduce((sum, frame) => sum + frame.distance, 0);
  const wallGaps = interior.map((frame) => frame.wallDelta).sort((a, b) => a - b);
  return {
    totalDistance: frames.reduce((sum, frame) => sum + frame.distance, 0),
    interiorFrames: interior.length,
    stationaryFrames: interior.filter((frame) => frame.distance < 0.01).length,
    minSpeedRatio: ratios.length ? Math.min(...ratios) : 0,
    maxSpeedRatio: ratios.length ? Math.max(...ratios) : 0,
    maxProgressError,
    wallMs,
    wallSpeed: wallMs ? distance / wallMs * 1000 : 0,
    maxWallFrameMs: wallGaps.length ? wallGaps[wallGaps.length - 1] : 0,
    p95WallFrameMs: wallGaps.length ? wallGaps[Math.ceil(wallGaps.length * 0.95) - 1] : 0,
  };
}

function assertContinuous(run: Run, tiles: number, minimumFrames = 6) {
  const m = run.metrics;
  expect(m.totalDistance, run.name + ": exact route distance with no rollback or stale suffix").toBeCloseTo(tiles * 16, 3);
  expect(m.interiorFrames, run.name + ": observe enough real animation frames").toBeGreaterThan(minimumFrames);
  expect(m.stationaryFrames, run.name + ": no tile, authorization, or direction-change dwell").toBe(0);
  expect(m.minSpeedRatio, run.name + ": no boundary slowdown").toBeGreaterThanOrEqual(0.92);
  expect(m.maxSpeedRatio, run.name + ": no catch-up acceleration or snap").toBeLessThanOrEqual(1.08);
  expect(m.maxProgressError, run.name + ": carry elapsed time through every turn").toBeLessThanOrEqual(0.75);
  expect(m.maxWallFrameMs, run.name + ": normalized scene delta must not hide a rendering stall").toBeLessThanOrEqual(50);
  expect(m.wallSpeed, run.name + ": real wall-clock speed stays near 80 px/s").toBeGreaterThanOrEqual(74);
  expect(m.wallSpeed, run.name + ": real wall-clock speed does not accelerate").toBeLessThanOrEqual(86);
}

async function createHarness(page: Page) {
  const wire: Wire[] = [];
  const runs: Run[] = [];
  const delayed = new Set<NodeJS.Timeout>();
  let enabled = false;
  let rejectSource = false;
  let active: { name: string; packetStart: number } | null = null;
  const watched = new Set<number>([
    OpCodes.PlayerStepRequest, OpCodes.PlayerStepResponse,
    OpCodes.PlayerStepCompleteRequest, OpCodes.PlayerStepCompleteResponse,
    OpCodes.OwnedPlayerPositionRequest, OpCodes.OwnedPlayerPositionResponse,
    OpCodes.ServerPlayerMovementNotify,
  ]);
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    const queues = { sent: { until: 0, index: 0 }, received: { until: 0, index: 3 } };
    const forward = (direction: "sent" | "received", originalMessage: string | Buffer, send: (message: string | Buffer) => void) => {
      let message = originalMessage;
      const opcode = Buffer.isBuffer(message) && message.length >= 6 ? message.readUInt16LE(4) : -1;
      let data: MovementWirePayload | undefined = watched.has(opcode) ? JSON.parse((message as Buffer).subarray(6).toString()) as MovementWirePayload : undefined;
      let originalData: MovementWirePayload | undefined;
      if (enabled && rejectSource && direction === "sent" && opcode === OpCodes.PlayerStepRequest && typeof data?.commandToken === "string") {
        rejectSource = false;
        originalData = data;
        data = { ...data, fromX: data.fromX! + 1 };
        const json = Buffer.from(JSON.stringify(data));
        const changed = Buffer.alloc(6 + json.length);
        changed.writeUInt32LE(changed.length - 4, 0);
        changed.writeUInt16LE(opcode, 4);
        json.copy(changed, 6);
        message = changed;
      }
      const queue = queues[direction];
      const now = performance.now();
      const delivery = enabled ? Math.max(now + ONE_WAY_LATENCY_MS[queue.index++ % ONE_WAY_LATENCY_MS.length], queue.until + 1) : now;
      queue.until = delivery;
      const record: Wire | undefined = data === undefined ? undefined : { direction, opcode, observedAt: Date.now(), delayMs: delivery - now, data, ...(originalData ? { originalData } : {}) };
      if (record) wire.push(record);
      const deliver = () => {
        if (record) record.forwardedAt = Date.now();
        send(message);
      };
      if (!enabled) { deliver(); return; }
      const timer = setTimeout(() => { delayed.delete(timer); deliver(); }, delivery - now);
      delayed.add(timer);
    };
    socket.onMessage((message) => forward("sent", message, (data) => server.send(data)));
    server.onMessage((message) => forward("received", message, (data) => socket.send(data)));
  });
  return {
    wire, runs,
    enable: () => { enabled = true; },
    rejectNextSource: () => { rejectSource = true; },
    async start(name: string) {
      if (active) throw new Error("A movement witness is already active");
      active = { name, packetStart: wire.length };
      await page.evaluate(() => {
        const state = (window as unknown as MovementWitnessWindow).__optimisticMovement;
        const id = window.__capturequestTest!.getState().player.id;
        const sprite = state.renderer!.getActorSprite(id!)!;
        state.sprite = sprite;
        state.frame = 0;
        state.elapsed = 0;
        state.inputs = [];
        state.frames = [{ frame: 0, t: 0, wall: performance.now(), epoch: Date.now(), delta: 0, x: sprite.x, y: sprite.y }];
        state.recording = true;
        state.sample = (_time: number, delta: number) => {
          state.elapsed += delta;
          state.frames.push({ frame: ++state.frame, t: state.elapsed, wall: performance.now(), epoch: Date.now(), delta, x: sprite.x, y: sprite.y });
        };
        state.renderer!.scene.events.on("postupdate", state.sample);
      });
    },
    activePackets() { return wire.slice(active?.packetStart ?? wire.length); },
    async finish(): Promise<Run> {
      if (!active) throw new Error("No active movement witness");
      const values = await page.evaluate(() => {
        const state = (window as unknown as MovementWitnessWindow).__optimisticMovement;
        state.renderer!.scene.events.off("postupdate", state.sample!);
        state.recording = false;
        return { frames: state.frames as Frame[], inputs: state.inputs as Input[] };
      });
      const run = { ...active, ...values, metrics: movementMetrics(values.frames) };
      runs.push(run);
      active = null;
      console.log(JSON.stringify({ optimisticMovement: run.name, ...run.metrics }));
      return run;
    },
    async save() {
      if (active) await this.finish().catch(() => undefined);
      const path = test.info().outputPath("optimistic-movement.json");
      await writeFile(path, JSON.stringify({ transport: "websocket", oneWayLatencyMs: ONE_WAY_LATENCY_MS, addedRoundTripRangeMs: [240, 340], runs, wire }, null, 2));
      await test.info().attach("optimistic-movement.json", { path, contentType: "application/json" });
    },
    dispose() { for (const timer of delayed) clearTimeout(timer); delayed.clear(); },
  };
}
type Harness = Awaited<ReturnType<typeof createHarness>>;

async function installWitness(page: Page) {
  await page.evaluate(async () => {
    const path = "/src/phaser-game/renderers/MapRenderer.ts";
    const { MapRenderer } = await import(path) as typeof import("../../src/phaser-game/renderers/MapRenderer");
    const state: OptimisticWitness = { recording: false, frame: 0, elapsed: 0, inputs: [], frames: [] };
    (window as unknown as MovementWitnessWindow).__optimisticMovement = state;
    const render = MapRenderer.prototype.renderMap;
    MapRenderer.prototype.renderMap = function (this: MapRendererType, ...args: Parameters<typeof render>) {
      state.renderer = this as unknown as RendererWitness;
      return render.apply(this, args);
    };
    for (const type of ["keydown", "keyup", "pointerup"]) {
      document.addEventListener(type, (event) => {
        if (!state.recording) return;
        const keyEvent = event as KeyboardEvent;
        state.inputs.push({ type, key: keyEvent.key, repeat: keyEvent.repeat, frame: state.frame, wall: performance.now(), epoch: Date.now() });
      }, true);
    }
  });
}

async function prepare(page: Page, x = 7, y = 6, scenario = "debug_warp_reds_house_1f_exit_mat") {
  await createGuestCharacterAndEnterWorld(page);
  await installWitness(page);
  await jumpToScenario(page, scenario);
  await waitForNoMapLoading(page);
  if (scenario === "debug_warp_reds_house_1f_exit_mat") await place(page, x, y);
  await page.evaluate(() => { if (document.activeElement instanceof HTMLElement) document.activeElement.blur(); });
  const transport = await page.evaluate(async () => {
    const path = "/src/net/index.ts";
    const { WorldSocket } = await import(path) as typeof import("../../src/net/index");
    return { connected: WorldSocket.isConnected, websocket: (WorldSocket as unknown as { useWebSocket: boolean }).useWebSocket };
  });
  expect(transport, "The latency witness must use the intercepted transport").toEqual({ connected: true, websocket: true });
}

async function place(page: Page, x: number, y: number) {
  await page.evaluate(async ({ x, y }) => { await window.__capturequestTest!.warpToMap(37, x, y, "LEFT"); }, { x, y });
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await assertPose(page, x, y);
}

async function assertPose(page: Page, x: number, y: number) {
  await waitForPlayerTile(page, x, y);
  await waitForPlayerIdle(page);
  const point = await page.evaluate(() => {
    const id = window.__capturequestTest!.getState().player.id;
    const sprite = (window as unknown as MovementWitnessWindow).__optimisticMovement.renderer!.getActorSprite(id!)!;
    return { x: sprite.x, y: sprite.y };
  });
  expect(point.x).toBeCloseTo(x * 16 + 8, 3);
  expect(point.y).toBeCloseTo(y * 16 + 8, 3);
}

async function confirmTiles(page: Page, harness: Harness, tiles: number, x: number, y: number, mapId = 37) {
  const commands = () => harness.activePackets().filter((packet) => packet.direction === "sent" && packet.opcode === OpCodes.PlayerStepRequest && typeof packet.data.commandToken === "string");
  await expect.poll(() => commands().length, { timeout: 20_000 }).toBe(tiles);
  await expect.poll(() => commands().every((request) => harness.wire.some((reply) => reply.direction === "received" && reply.opcode === OpCodes.PlayerStepResponse && reply.data.requestId === request.data.requestId && reply.data.success && reply.data.committed && reply.forwardedAt !== undefined)), { timeout: 20_000 }).toBe(true);
  expect(new Set(commands().map((packet) => packet.data.commandToken)).size, "Each started visual tile commits once").toBe(tiles);
  expect(new Set(commands().map((packet) => packet.data.movementStreamId)).size, "A healthy route retains one ordered movement stream").toBe(1);
  const firstSequence = commands()[0].data.sequence;
  expect(Number.isSafeInteger(firstSequence) && firstSequence! > 0).toBe(true);
  expect(commands().map((packet) => packet.data.sequence), "Every visual tile has exactly the next stream sequence").toEqual(Array.from({ length: tiles }, (_, index) => firstSequence! + index));
  expect(harness.activePackets().filter((packet) => packet.direction === "sent" && packet.opcode === OpCodes.PlayerStepCompleteRequest), "Atomic movement never enters the old two-phase completion path").toEqual([]);
  const last = commands()[tiles - 1];
  expect(harness.wire.find((reply) => reply.direction === "received" && reply.opcode === OpCodes.PlayerStepResponse && reply.data.requestId === last.data.requestId)?.data).toMatchObject({ success: true, committed: true, stepToken: last.data.commandToken, mapId, x, y });
  await assertPose(page, x, y);
}

async function assertOptimisticStart(harness: Harness, run: Run, accepted = true, requireBootstrap = false) {
  const input = run.inputs.find((event) => (event.type === "keydown" && !event.repeat) || event.type === "pointerup");
  expect(input, "Record the actual DOM input event").toBeDefined();
  const first = run.frames.find((sample, index) => index > 0 && Math.abs(sample.x - run.frames[index - 1].x) + Math.abs(sample.y - run.frames[index - 1].y) > 0.01);
  expect(first, "Observe actual sprite movement").toBeDefined();
  const initial = harness.wire.slice(run.packetStart).find((packet) => packet.direction === "sent" && packet.opcode === OpCodes.PlayerStepRequest && typeof packet.data.commandToken === "string");
  expect(initial).toBeDefined();
  const response = () => harness.wire.find((packet) => packet.direction === "received" && packet.opcode === OpCodes.PlayerStepResponse && packet.data.requestId === initial!.data.requestId);
  await expect.poll(() => response()?.forwardedAt, { timeout: 10_000 }).toBeGreaterThan(0);
  run.start = { renderFrames: first!.frame - input!.frame, wallMs: first!.wall - input!.wall, inputAt: input!.epoch, firstMotionAt: first!.epoch, responseAt: response()!.forwardedAt! };
  const bootstrap = harness.wire.slice(run.packetStart).find((packet) => packet.direction === "sent" && packet.opcode === OpCodes.OwnedPlayerPositionRequest && packet.data.resetMovementStream === true);
  if (requireBootstrap) expect(bootstrap, "Exercise a real fresh-stream bootstrap on the wire").toBeDefined();
  if (bootstrap) {
    const bootstrapResponse = harness.wire.find((packet) => packet.direction === "received" && packet.opcode === OpCodes.OwnedPlayerPositionResponse && packet.data.requestId === bootstrap.data.requestId);
    expect(bootstrapResponse?.forwardedAt).toBeDefined();
    expect(bootstrapResponse!.data.movementStreamId).toMatch(/^[a-f0-9]{32}$/i);
    expect(bootstrapResponse!.data.movementSequence, "The actual zero-sequence bootstrap omits movementSequence; the client must still start sequence 1").toBeUndefined();
    expect(initial!.data.sequence).toBe(1);
    run.start.bootstrapResponseAt = bootstrapResponse!.forwardedAt;
    expect(run.start.firstMotionAt, run.name + ": prediction must start before the delayed stream bootstrap returns").toBeLessThan(run.start.bootstrapResponseAt!);
  }
  expect(run.start.renderFrames, run.name + ": start within two rendered frames of input").toBeLessThanOrEqual(2);
  expect(run.start.wallMs, run.name + ": initial response cannot impose input latency").toBeLessThanOrEqual(50);
  expect(run.start.firstMotionAt, run.name + ": prediction must visibly start before its delayed commit receipt").toBeLessThan(run.start.responseAt);
  expect(response()!.data.success).toBe(accepted);
  if (accepted) expect(response()!.data.committed).toBe(true);
  expect(initial!.delayMs).toBeGreaterThanOrEqual(120);
  expect(response()!.delayMs).toBeGreaterThanOrEqual(120);
  console.log(JSON.stringify({ optimisticStart: run.name, ...run.start }));
}

async function beyond(page: Page, axis: "x" | "y", comparison: "less" | "greater", value: number) {
  await page.waitForFunction(({ axis, comparison, value }) => {
    const position = (window as unknown as MovementWitnessWindow).__optimisticMovement.sprite![axis];
    return comparison === "less" ? position < value : position > value;
  }, { axis, comparison, value });
}

async function releaseKeys(page: Page) {
  for (const key of ["w", "a", "s", "d"]) await page.keyboard.up(key).catch(() => undefined);
}

test("input starts before authorization, short taps finish one tile, and early click retargets never pause", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "This test deliberately injects WebSocket latency");
  test.setTimeout(180_000);
  const errors = collectPageErrors(page);
  const harness = await createHarness(page);
  try {
    await prepare(page);
    harness.enable();
    await harness.start("tap released before its initial commit receipt");
    await page.keyboard.down("a");
    await page.waitForTimeout(35);
    await page.keyboard.up("a");
    await confirmTiles(page, harness, 1, 6, 6);
    const tap = await harness.finish();
    await assertOptimisticStart(harness, tap, true, true);
    expect(tap.inputs.find((input) => input.type === "keyup" && input.key === "a")!.epoch, "Release occurs before its commit receipt arrives").toBeLessThan(tap.start!.responseAt);
    assertContinuous(tap, 1);
    await page.waitForTimeout(650);
    await assertPose(page, 6, 6);

    await harness.start("optimistic ten-tile click path with corners");
    await clickTile(page, 0, 2);
    await confirmTiles(page, harness, 10, 0, 2);
    const click = await harness.finish();
    await assertOptimisticStart(harness, click);
    assertContinuous(click, 10, 20);

    await place(page, 7, 6);
    await harness.start("retarget during the first unconfirmed visual tile");
    await clickTile(page, 0, 6);
    await beyond(page, "x", "less", 7 * 16 + 5);
    await clickTile(page, 6, 7);
    await confirmTiles(page, harness, 2, 6, 7);
    const retarget = await harness.finish();
    await assertOptimisticStart(harness, retarget, true, true);
    expect(retarget.inputs.filter((input) => input.type === "pointerup")[1].epoch, "Replace the destination before the original commit receipt arrives").toBeLessThan(retarget.start!.responseAt);
    assertContinuous(retarget, 2);
    for (const point of retarget.frames) {
      const across = Math.abs(point.y - 104) < 0.01 && point.x >= 104 - 0.01 && point.x <= 120 + 0.01;
      const down = Math.abs(point.x - 104) < 0.01 && point.y >= 104 - 0.01 && point.y <= 120 + 0.01;
      expect(across || down, "Only the active tile and latest destination are traversed").toBe(true);
    }
    await page.waitForTimeout(650);
    await assertPose(page, 6, 7);
    errors.assertNoSevereErrors();
    await page.screenshot({ path: test.info().outputPath("optimistic-click-arrived.png") });
    await quitToCharacterSelect(page);
  } finally {
    await releaseKeys(page);
    await harness.save();
    harness.dispose();
  }
});

test("newest held key wins, release falls back, and forty fresh turns never exhaust prediction throughput", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "This test deliberately injects WebSocket latency");
  test.setTimeout(180_000);
  const errors = collectPageErrors(page);
  const harness = await createHarness(page);
  try {
    await prepare(page, 2, 6);
    harness.enable();
    await harness.start("W held through A and D priority changes");
    await page.keyboard.down("w");
    await beyond(page, "y", "less", 6 * 16 + 5);
    await page.keyboard.down("a");
    await page.keyboard.down("w"); // Repeat while already held must not steal priority.
    await beyond(page, "x", "less", 2 * 16 + 5);
    await page.keyboard.up("a"); // W remains physically held.
    await beyond(page, "y", "less", 5 * 16 + 5);
    await page.keyboard.down("d");
    await beyond(page, "x", "greater", 1 * 16 + 11);
    await page.keyboard.up("d");
    await beyond(page, "y", "less", 4 * 16 + 5);
    await page.keyboard.down("a");
    await beyond(page, "x", "less", 2 * 16 + 5);
    await page.keyboard.up("w");
    await page.keyboard.up("a");
    await confirmTiles(page, harness, 6, 1, 3);
    const priority = await harness.finish();
    await assertOptimisticStart(harness, priority, true, true);
    expect(priority.inputs.some((input) => input.type === "keydown" && input.key === "w" && input.repeat), "The regression actually delivers an OS-style repeat").toBe(true);
    assertContinuous(priority, 6, 20);

    await harness.start("forty one-tile turns under sustained 240–340ms round trips");
    const circuit = [
      { key: "d", axis: "x" as const, comparison: "greater" as const, threshold: 1 * 16 + 11 },
      { key: "w", axis: "y" as const, comparison: "less" as const, threshold: 3 * 16 + 5 },
      { key: "a", axis: "x" as const, comparison: "less" as const, threshold: 2 * 16 + 5 },
      { key: "s", axis: "y" as const, comparison: "greater" as const, threshold: 2 * 16 + 11 },
    ];
    await page.keyboard.down(circuit[0].key);
    for (let step = 0; step < 40; step += 1) {
      const current = circuit[step % 4];
      await beyond(page, current.axis, current.comparison, current.threshold);
      if (step < 39) {
        await page.keyboard.down(circuit[(step + 1) % 4].key);
      }
      await page.keyboard.up(current.key);
    }
    await confirmTiles(page, harness, 40, 1, 3);
    const sustained = await harness.finish();
    await assertOptimisticStart(harness, sustained);
    assertContinuous(sustained, 40, 100);
    expect(sustained.inputs.filter((input) => input.type === "keydown" && !input.repeat)).toHaveLength(40);
    for (const point of sustained.frames) {
      const horizontal = (Math.abs(point.y - 40) < 0.01 || Math.abs(point.y - 56) < 0.01) && point.x >= 24 - 0.01 && point.x <= 40 + 0.01;
      const vertical = (Math.abs(point.x - 24) < 0.01 || Math.abs(point.x - 40) < 0.01) && point.y >= 40 - 0.01 && point.y <= 56 + 0.01;
      expect(horizontal || vertical, "All forty turns follow the clear 2×2 circuit").toBe(true);
    }
    await page.waitForTimeout(650);
    await assertPose(page, 1, 3);
    errors.assertNoSevereErrors();
    await page.screenshot({ path: test.info().outputPath("optimistic-turns-arrived.png") });
    await quitToCharacterSelect(page);
  } finally {
    await releaseKeys(page);
    await harness.save();
    harness.dispose();
  }
});

test("a real delayed server rejection reconciles the predicted sprite and releases the next valid input", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "This test deliberately injects WebSocket latency");
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);
  const harness = await createHarness(page);
  try {
    await prepare(page, 6, 6);
    harness.enable();
    harness.rejectNextSource();
    await harness.start("outdated expected-source rejection");
    await clickTile(page, 6, 2);
    await expect.poll(() => harness.activePackets().some((packet) => packet.direction === "received" && packet.opcode === OpCodes.PlayerStepResponse && packet.data.success === false && packet.forwardedAt !== undefined), { timeout: 20_000 }).toBe(true);
    await assertPose(page, 6, 6);
    const rejected = await harness.finish();
    await assertOptimisticStart(harness, rejected, false, true);
    const packets = harness.wire.slice(rejected.packetStart);
    const requests = packets.filter((packet) => packet.direction === "sent" && packet.opcode === OpCodes.PlayerStepRequest);
    expect(requests.length).toBeGreaterThanOrEqual(1);
    expect(new Set(requests.map((packet) => packet.data.commandToken)).size, "Retired commands are never retried").toBe(requests.length);
    expect(requests[0].originalData).toMatchObject({ fromX: 6, fromY: 6 });
    expect(requests[0].data.fromX).toBe(7);
    expect(packets.find((packet) => packet.direction === "received" && packet.opcode === OpCodes.PlayerStepResponse)?.data).toMatchObject({ success: false, mapId: 37, x: 6, y: 6 });
    expect(packets.filter((packet) => packet.direction === "sent" && packet.opcode === OpCodes.PlayerStepCompleteRequest && packet.data.cancel !== true), "Rejection recovery never uses legacy token completions").toEqual([]);
    await page.waitForTimeout(650);
    await assertPose(page, 6, 6);
    expect(harness.wire.slice(rejected.packetStart).filter((packet) => packet.direction === "received" && packet.opcode === OpCodes.PlayerStepResponse).every((packet) => packet.data.success === false), "Retiring the failed stream prevents every queued prediction from committing").toBe(true);

    await harness.start("valid input after rejected prediction");
    await clickTile(page, 5, 6);
    await confirmTiles(page, harness, 1, 5, 6);
    const recovered = await harness.finish();
    await assertOptimisticStart(harness, recovered);
    assertContinuous(recovered, 1);
    errors.assertNoSevereErrors();
    await page.screenshot({ path: test.info().outputPath("optimistic-rejection-recovered.png") });
    await quitToCharacterSelect(page);
  } finally {
    await releaseKeys(page);
    await harness.save();
    harness.dispose();
  }
});

test("a delayed arrow effect hands prediction to the authoritative forced route", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "This test deliberately injects WebSocket latency");
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);
  const harness = await createHarness(page);
  try {
    await prepare(page, 5, 9, "debug_spin_route_ready");
    await waitForPlayerTile(page, 5, 9);
    harness.enable();
    await harness.start("predicted entry followed by server-owned arrow route");
    await page.keyboard.down("a");
    await page.waitForTimeout(35);
    await page.keyboard.up("a");
    await assertPose(page, 2, 9);
    const run = await harness.finish();
    await assertOptimisticStart(harness, run, true, true);
    const packets = harness.wire.slice(run.packetStart);
    const commits = packets.filter((packet) => packet.direction === "sent" && packet.opcode === OpCodes.PlayerStepRequest && typeof packet.data.commandToken === "string");
    expect(commits, "Only the ordinary entry tile is client-committed").toHaveLength(1);
    const points = packets.filter((packet) => packet.direction === "received" && packet.opcode === OpCodes.ServerPlayerMovementNotify && packet.forwardedAt !== undefined);
    expect(points.some((packet) => packet.data.mapId === 200 && packet.data.x === 3 && packet.data.y === 9)).toBe(true);
    expect(points.some((packet) => packet.data.mapId === 200 && packet.data.x === 2 && packet.data.y === 9 && packet.data.pathFinished)).toBe(true);
    await page.waitForTimeout(650);
    await assertPose(page, 2, 9);
    expect((await getGameState(page)).map.id).toBe(200);
    errors.assertNoSevereErrors();
    await page.screenshot({ path: test.info().outputPath("optimistic-arrow-arrived.png") });
    await quitToCharacterSelect(page);
  } finally {
    await releaseKeys(page);
    await harness.save();
    harness.dispose();
  }
});
