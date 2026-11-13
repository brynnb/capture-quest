import { expect, test } from "@playwright/test";
import type { OwnedPlayerPositionResponse, PlayerStepCompleteRequest, PlayerStepCompleteResponse, PlayerStepError, PlayerStepRequest, PlayerStepResponse } from "../../src/net/generated/protocol";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

test("issued and committed walking survive SIGKILL without replaying Safari steps or rewinding", async ({ page, context }) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash recovery mode");
  test.setTimeout(240000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  const errors = collectPageErrors(page);
  const intents: PlayerStepRequest[] = [];
  const completions: PlayerStepCompleteRequest[] = [];
  const replies: (PlayerStepCompleteResponse | PlayerStepError)[] = [];
  const positions: OwnedPlayerPositionResponse[] = [];
  let issued: PlayerStepResponse | undefined;
  let committed: PlayerStepCompleteResponse | undefined;
  await context.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PlayerStepRequest) intents.push(JSON.parse(message.subarray(6).toString()));
        if (opcode === OpCodes.PlayerStepCompleteRequest) completions.push(JSON.parse(message.subarray(6).toString()));
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PlayerStepResponse && !issued) {
          const reply: PlayerStepResponse = JSON.parse(message.subarray(6).toString());
          if (reply.success) { issued = reply; return; }
        }
        if (opcode === OpCodes.PlayerStepCompleteResponse) {
          const reply: PlayerStepCompleteResponse | PlayerStepError = JSON.parse(message.subarray(6).toString());
          if (reply.success && !committed) { committed = reply; return; }
          replies.push(reply);
        }
        if (opcode === OpCodes.OwnedPlayerPositionResponse) positions.push(JSON.parse(message.subarray(6).toString()));
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_step_ongoing_session");
  await waitForMap(page, "SAFARI_ZONE_CENTER"); await waitForPlayerTile(page, 14, 24);
  const characterId = (await getGameState(page)).player.internalId;
  expect(Number.isSafeInteger(characterId) && characterId! > 0).toBe(true);
  const readDurable = async () => JSON.parse(await sql(`SELECT json_build_object(
    'position',(SELECT json_build_object('mapId',map_id,'x',x,'y',y) FROM character_data WHERE id=${characterId}),
    'safari',(SELECT state_json::json FROM character_safari_state WHERE character_id=${characterId}),
    'receipt',(SELECT json_build_object('token',step_token,'result',result_json::json,'committedAt',committed_at) FROM character_movement_receipts WHERE character_id=${characterId}),
    'pokemon',(SELECT json_agg(p ORDER BY p.id) FROM character_pokemon p WHERE character_id=${characterId}))`));
  // This existing fixture uses floor tiles beside the entrance, not encounter
  // grass. Require the live imported catalog to agree rather than disabling RNG.
  expect(await sql("SELECT COUNT(*) FROM phaser_tiles WHERE map_id=220 AND x=14 AND y IN (23,24) AND encounter_area_id IS NULL")).toBe("2");
  const original = await readDurable();
  expect(original.safari.visit.stepsLeft).toBe(3); expect(original.receipt).toBeNull();
  await pressMovement(page, "up"); await expect.poll(() => !!issued).toBe(true);
  expect(issued).toMatchObject({ mapId: 220, x: 14, y: 23 });
  expect(intents).toHaveLength(1); expect(completions).toHaveLength(0);
  expect(await readDurable()).toEqual(original); errors.assertNoSevereErrors();
  const issuedCrash = await crash(); expect(await readDurable()).toEqual(original);
  await page.close();
  const fresh = await context.newPage();
  const freshErrors = collectPageErrors(fresh);
  await fresh.goto("/"); await fresh.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(fresh.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
  await enterWorld(fresh, character); await waitForMap(fresh, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(fresh);
  await waitForPlayerIdle(fresh); await waitForPlayerTile(fresh, 14, 24);
  expect(await readDurable()).toEqual(original); expect(intents).toHaveLength(1); expect(completions).toHaveLength(0);
  const sendCompletion = async (target: typeof fresh, stepToken: string, requestId: string) => {
    await target.evaluate(async ({ stepToken, requestId }) => {
      const bridgePath = "/src/net/NetworkBridge.ts", opcodePath = "/src/net/generated/opcodes.ts";
      const { NetworkBridge } = await import(bridgePath); const opcodes = await import(opcodePath);
      NetworkBridge.send({ stepToken, requestId }, opcodes.PlayerStepCompleteRequest);
    }, { stepToken, requestId });
  };
  await sendCompletion(fresh, issued!.stepToken, "retired-issued-token");
  await expect.poll(() => replies.find(reply => reply.requestId === "retired-issued-token")?.success).toBe(false);
  expect(await readDurable()).toEqual(original);
  await pressMovement(fresh, "up"); await expect.poll(() => !!committed).toBe(true);
  expect(committed).toMatchObject({ success: true, mapId: 220, x: 14, y: 23 });
  expect(intents).toHaveLength(2); expect(completions).toHaveLength(2);
  const completedToken = completions[1].stepToken;
  expect(completedToken).not.toBe(issued!.stepToken);
  const settled = await readDurable();
  expect(settled.position).toEqual({ mapId: 220, x: 14, y: 23 });
  expect(settled.safari.visit.stepsLeft).toBe(2); expect(settled.safari.visit.battle).toBeUndefined();
  expect(settled.receipt).toMatchObject({ token: completedToken, result: { version: 1, result: { stepToken: completedToken, mapId: 220, x: 14, y: 23 } } });
  expect(settled.pokemon).toEqual(original.pokemon); freshErrors.assertNoSevereErrors();
  const committedCrash = await crash(); expect(await readDurable()).toEqual(settled);
  await fresh.close();
  const final = await context.newPage();
  const finalErrors = collectPageErrors(final);
  await final.goto("/"); await final.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(final.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
  await enterWorld(final, character); await waitForMap(final, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(final);
  await waitForPlayerIdle(final); await waitForPlayerTile(final, 14, 23);
  expect(await readDurable()).toEqual(settled); expect(intents).toHaveLength(2); expect(completions).toHaveLength(2);
  await sendCompletion(final, completedToken, "recover-committed-token");
  await expect.poll(() => replies.find(reply => reply.requestId === "recover-committed-token")?.success).toBe(true);
  expect(replies.find(reply => reply.requestId === "recover-committed-token")).toMatchObject({ replayed: true, x: 14, y: 23 });
  expect(await readDurable()).toEqual(settled);
  await final.evaluate(() => window.__capturequestTest!.warpToMap(220, 14, 24, "UP"));
  await waitForPlayerTile(final, 14, 24); await waitForPlayerIdle(final);
  const moved = await readDurable();
  expect(moved.safari).toEqual(settled.safari); expect(moved.receipt).toEqual(settled.receipt);
  await sendCompletion(final, completedToken, "historical-after-warp");
  await expect.poll(() => replies.find(reply => reply.requestId === "historical-after-warp")?.success).toBe(true);
  expect(replies.find(reply => reply.requestId === "historical-after-warp")).toMatchObject({ replayed: true, x: 14, y: 23 });
  await final.evaluate(async stepToken => {
    const bridgePath = "/src/net/NetworkBridge.ts", opcodePath = "/src/net/generated/opcodes.ts";
    const { NetworkBridge } = await import(bridgePath); const opcodes = await import(opcodePath);
    NetworkBridge.send({ stepToken, requestId: "current-with-history" }, opcodes.OwnedPlayerPositionRequest);
  }, completedToken);
  await expect.poll(() => positions.find(reply => reply.requestId === "current-with-history")?.x).toBe(14);
  expect(positions.find(reply => reply.requestId === "current-with-history")).toMatchObject({ y: 24, committedStep: { stepToken: completedToken, y: 23 } });
  await waitForPlayerTile(final, 14, 24); expect(await readDurable()).toEqual(moved);
  await pressMovement(final, "up"); await waitForPlayerIdle(final); await waitForPlayerTile(final, 14, 23);
  const latest = await readDurable();
  expect(latest.safari.visit.stepsLeft).toBe(1); expect(latest.receipt.token).not.toBe(completedToken);
  await sendCompletion(final, completedToken, "superseded-receipt");
  await expect.poll(() => replies.find(reply => reply.requestId === "superseded-receipt")?.success).toBe(false);
  expect(await readDurable()).toEqual(latest); expect(intents).toHaveLength(3); expect(completions).toHaveLength(6);
  await final.screenshot({ path: test.info().outputPath("process-movement-recovered.png") });
  finalErrors.assertNoSevereErrors(); await quitToCharacterSelect(final); await final.close();
  await record({ outcome: "ordinary-walking", characterId, issued, committed, issuedCrash, committedCrash, original, settled, moved, latest, intents, completions, replies, positions });
});
