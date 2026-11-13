import { expect, test, type Page } from "@playwright/test";
import type { OwnedPlayerPositionResponse, PlayerStepError, PlayerStepRequest, PlayerStepResponse } from "../../src/net/generated/protocol";
import * as OpCodes from "../../src/net/generated/opcodes";
import type { MapRenderer as MapRendererType } from "../../src/phaser-game/renderers/MapRenderer";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

type AtomicMovement = PlayerStepRequest & { commandToken: string; movementStreamId: string; sequence: number };
type AtomicReceipt = PlayerStepResponse & { committed: true; replayed?: boolean };
type ProcessMovementWindow = Window & { __processMovementRenderer: Pick<MapRendererType, "getActorSprite"> };

test("unsubmitted prediction and committed walking survive SIGKILL without replaying Safari steps or rewinding", async ({ page, context }) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash recovery mode");
  test.setTimeout(240_000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  const errors = collectPageErrors(page);
  const commands: AtomicMovement[] = [];
  const legacyCompletions: unknown[] = [];
  const replies: Array<AtomicReceipt | PlayerStepError> = [];
  const positions: OwnedPlayerPositionResponse[] = [];
  let unsubmitted: AtomicMovement | undefined;
  let committed: AtomicReceipt | undefined;
  const requestOpcodes = new Set<number>([OpCodes.PlayerStepRequest, OpCodes.PlayerStepCompleteRequest]);
  const responseOpcodes = new Set<number>([OpCodes.PlayerStepResponse, OpCodes.OwnedPlayerPositionResponse]);
  await context.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6 && requestOpcodes.has(message.readUInt16LE(4))) {
        const opcode = message.readUInt16LE(4);
        const data = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepRequest && data.commandToken) {
          commands.push(data);
          // A one-phase response is already committed. Hold the first command
          // before transport delivery to exercise the genuinely uncommitted case.
          if (!unsubmitted) { unsubmitted = data; return; }
        }
        if (opcode === OpCodes.PlayerStepCompleteRequest) legacyCompletions.push(data);
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6 && responseOpcodes.has(message.readUInt16LE(4))) {
        const opcode = message.readUInt16LE(4);
        const reply = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.PlayerStepResponse) {
          if (reply.success && reply.committed && !committed) { committed = reply; return; }
          replies.push(reply);
        }
        if (opcode === OpCodes.OwnedPlayerPositionResponse && reply.success) positions.push(reply);
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await page.evaluate(async () => {
    const path = "/src/phaser-game/renderers/MapRenderer.ts";
    const { MapRenderer } = await import(path) as typeof import("../../src/phaser-game/renderers/MapRenderer");
    const render = MapRenderer.prototype.renderMap;
    MapRenderer.prototype.renderMap = function (this: MapRendererType, ...args: Parameters<typeof render>) {
      (window as unknown as ProcessMovementWindow).__processMovementRenderer = this;
      return render.apply(this, args);
    };
  });
  await jumpToScenario(page, "safari_step_ongoing_session");
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await waitForPlayerTile(page, 14, 24);
  const characterId = (await getGameState(page)).player.internalId;
  expect(Number.isSafeInteger(characterId) && characterId! > 0).toBe(true);
  const readDurable = async () => JSON.parse(await sql("SELECT json_build_object(" +
    "'position',(SELECT json_build_object('mapId',map_id,'x',x,'y',y) FROM character_data WHERE id=" + characterId + ")," +
    "'safari',(SELECT state_json::json FROM character_safari_state WHERE character_id=" + characterId + ")," +
    "'receipt',(SELECT json_build_object('token',step_token,'result',result_json::json,'committedAt',committed_at) FROM character_movement_receipts WHERE character_id=" + characterId + ")," +
    "'pokemon',(SELECT json_agg(p ORDER BY p.id) FROM character_pokemon p WHERE character_id=" + characterId + "))"));
  expect(await sql("SELECT COUNT(*) FROM phaser_tiles WHERE map_id=220 AND x=14 AND y IN (23,24) AND encounter_area_id IS NULL")).toBe("2");
  const original = await readDurable();
  expect(original.safari.visit.stepsLeft).toBe(3);
  expect(original.receipt).toBeNull();
  await pressMovement(page, "up");
  await expect.poll(() => Boolean(unsubmitted)).toBe(true);
  await page.waitForFunction(() => {
    const id = window.__capturequestTest!.getState().player.id;
    const sprite = (window as unknown as ProcessMovementWindow).__processMovementRenderer.getActorSprite(id!)!;
    return Math.abs(sprite.x - (14 * 16 + 8)) < 0.01 && Math.abs(sprite.y - (23 * 16 + 8)) < 0.01;
  });
  expect(unsubmitted).toMatchObject({ mapId: 220, fromX: 14, fromY: 24, direction: "UP", sequence: 1 });
  expect(commands).toHaveLength(1);
  expect(legacyCompletions).toEqual([]);
  expect(await readDurable(), "Visible prediction does not itself persist position or Safari effects").toEqual(original);
  errors.assertNoSevereErrors();
  const unsubmittedCrash = await crash();
  expect(await readDurable()).toEqual(original);
  await page.close();

  const loginAgain = async () => {
    const target = await context.newPage();
    const pageErrors = collectPageErrors(target);
    await target.goto("/");
    await target.getByRole("button", { name: "PLAY AS GUEST" }).click();
    await expect(target.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30_000 });
    await enterWorld(target, character);
    await waitForMap(target, "SAFARI_ZONE_CENTER");
    await waitForNoMapLoading(target);
    await waitForPlayerIdle(target);
    return { target, pageErrors };
  };
  const sendRetiredCommand = async (target: Page, command: AtomicMovement, requestId: string) => {
    await target.evaluate(async ({ command, requestId }) => {
      const bridgePath = "/src/net/NetworkBridge.ts", opcodePath = "/src/net/generated/opcodes.ts";
      const { NetworkBridge } = await import(bridgePath);
      const opcodes = await import(opcodePath);
      NetworkBridge.send({ ...command, requestId }, opcodes.PlayerStepRequest);
    }, { command, requestId });
    await expect.poll(() => replies.find((reply) => reply.requestId === requestId)?.success).toBe(false);
  };
  const readHistorical = async (target: Page, stepToken: string, requestId: string) => {
    await target.evaluate(async ({ stepToken, requestId }) => {
      const bridgePath = "/src/net/NetworkBridge.ts", opcodePath = "/src/net/generated/opcodes.ts";
      const { NetworkBridge } = await import(bridgePath);
      const opcodes = await import(opcodePath);
      NetworkBridge.send({ stepToken, requestId }, opcodes.OwnedPlayerPositionRequest);
    }, { stepToken, requestId });
    await expect.poll(() => positions.find((position) => position.requestId === requestId)).toBeDefined();
    return positions.find((position) => position.requestId === requestId)!;
  };

  const { target: fresh, pageErrors: freshErrors } = await loginAgain();
  await waitForPlayerTile(fresh, 14, 24);
  expect(await readDurable()).toEqual(original);
  expect(commands).toHaveLength(1);
  await sendRetiredCommand(fresh, unsubmitted!, "retired-unsubmitted-command");
  expect(await readDurable()).toEqual(original);
  await pressMovement(fresh, "up");
  await expect.poll(() => Boolean(committed)).toBe(true);
  expect(committed).toMatchObject({ success: true, committed: true, mapId: 220, x: 14, y: 23 });
  expect(commands).toHaveLength(3);
  const completedCommand = commands.find((command) => command.commandToken === committed!.stepToken)!;
  expect(completedCommand.commandToken).not.toBe(unsubmitted!.commandToken);
  const settled = await readDurable();
  expect(settled.position).toEqual({ mapId: 220, x: 14, y: 23 });
  expect(settled.safari.visit.stepsLeft).toBe(2);
  expect(settled.safari.visit.battle).toBeUndefined();
  expect(settled.receipt).toMatchObject({ token: completedCommand.commandToken, result: { version: 1, result: { stepToken: completedCommand.commandToken, mapId: 220, x: 14, y: 23 } } });
  expect(settled.pokemon).toEqual(original.pokemon);
  freshErrors.assertNoSevereErrors();
  const committedCrash = await crash();
  expect(await readDurable()).toEqual(settled);
  await fresh.close();

  const { target: final, pageErrors: finalErrors } = await loginAgain();
  await waitForPlayerTile(final, 14, 23);
  expect(await readDurable()).toEqual(settled);
  expect(commands).toHaveLength(3);
  await sendRetiredCommand(final, completedCommand, "retired-committed-after-crash");
  expect(await readDurable()).toEqual(settled);
  expect(await readHistorical(final, completedCommand.commandToken, "history-after-crash")).toMatchObject({ mapId: 220, x: 14, y: 23, committedStep: { stepToken: completedCommand.commandToken, x: 14, y: 23 } });

  await final.evaluate(() => window.__capturequestTest!.warpToMap(220, 14, 24, "UP"));
  await waitForNoMapLoading(final);
  await waitForPlayerTile(final, 14, 24);
  await waitForPlayerIdle(final);
  const moved = await readDurable();
  expect(moved.safari).toEqual(settled.safari);
  expect(moved.receipt).toEqual(settled.receipt);
  await sendRetiredCommand(final, completedCommand, "historical-after-warp");
  expect(await readHistorical(final, completedCommand.commandToken, "current-with-history")).toMatchObject({ mapId: 220, x: 14, y: 24, committedStep: { stepToken: completedCommand.commandToken, y: 23 } });
  await waitForPlayerTile(final, 14, 24);
  expect(await readDurable()).toEqual(moved);

  await pressMovement(final, "up");
  await waitForPlayerIdle(final);
  await waitForPlayerTile(final, 14, 23);
  await expect.poll(async () => (await readDurable()).safari.visit.stepsLeft).toBe(1);
  const latest = await readDurable();
  expect(latest.receipt.token).not.toBe(completedCommand.commandToken);
  expect((await readHistorical(final, completedCommand.commandToken, "superseded-history")).committedStep).toBeUndefined();
  expect(await readDurable()).toEqual(latest);
  expect(commands).toHaveLength(6);
  expect(legacyCompletions).toEqual([]);
  await final.screenshot({ path: test.info().outputPath("process-movement-recovered.png") });
  finalErrors.assertNoSevereErrors();
  await quitToCharacterSelect(final);
  await final.close();
  await record({ outcome: "atomic-walking", characterId, unsubmitted, committed, unsubmittedCrash, committedCrash, original, settled, moved, latest, commands, legacyCompletions, replies, positions });
});
