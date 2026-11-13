import { expect, test } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import type { PlayerStepCompleteRequest, PlayerStepCompleteResponse, PlayerStepError, ServerPlayerMovementNotify } from "../../src/net/generated/protocol";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

test("ordinary walking onto an imported arrow starts its server-owned route", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "This assertion classifies actual WebSocket command bodies");
  const errors = collectPageErrors(page);
  const requests: PlayerStepCompleteRequest[] = [];
  const responses: Array<PlayerStepCompleteResponse | PlayerStepError> = [];
  const serverPoints: ServerPlayerMovementNotify[] = [];
  const wire: Array<{ direction: string; opcode: number; at: number; data: unknown }> = [];
  let armed = false;
  page.on("websocket", (socket) => {
    if (new URL(socket.url()).pathname !== "/ws") return;
    socket.on("framesent", ({ payload }) => {
      if (!armed || typeof payload === "string" || payload.length < 6) return;
      const opcode = payload.readUInt16LE(4);
      if (opcode !== OpCodes.PlayerStepCompleteRequest) return;
      const data = JSON.parse(payload.subarray(6).toString()) as PlayerStepCompleteRequest;
      requests.push(data);
      wire.push({ direction: "sent", opcode, at: Date.now(), data });
    });
    socket.on("framereceived", ({ payload }) => {
      if (!armed || typeof payload === "string" || payload.length < 6) return;
      const opcode = payload.readUInt16LE(4);
      if (opcode !== OpCodes.PlayerStepCompleteResponse && opcode !== OpCodes.ServerPlayerMovementNotify) return;
      const data = JSON.parse(payload.subarray(6).toString());
      if (opcode === OpCodes.PlayerStepCompleteResponse) responses.push(data);
      else serverPoints.push(data);
      wire.push({ direction: "received", opcode, at: Date.now(), data });
    });
  });
  try {
    await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "debug_spin_route_ready");
    await waitForNoMapLoading(page);
    await waitForPlayerTile(page, 5, 9);
    armed = true;
    await pressMovement(page, "left");
    await waitForPlayerTile(page, 2, 9);
    await waitForPlayerIdle(page);
    // Only the user step is committed by the client. A separately issued,
    // unused continuation may be retired while the stopPath receipt arrives;
    // opcode 189 carries both operations, so classify their actual bodies.
    const commits = requests.filter((request) => request.cancel !== true);
    const cancellations = requests.filter((request) => request.cancel === true);
    expect(commits, "Only the ordinary step onto the arrow is client-committed").toHaveLength(1);
    expect(cancellations.length, "At most one unused continuation needs retirement").toBeLessThanOrEqual(1);
    await expect.poll(() => responses.find((response) => response.requestId === commits[0].requestId)?.success).toBe(true);
    for (const cancellation of cancellations) {
      expect(cancellation.stepToken, "Cancellation retires a different, unused authorization").not.toBe(commits[0].stepToken);
      expect(cancellation.nextDirection).toBeUndefined();
      expect(cancellation.lookAhead).toBeUndefined();
      await expect.poll(() => responses.find((response) => response.requestId === cancellation.requestId)).toMatchObject({ success: true, cancelled: true });
    }
    expect(serverPoints.some((point) => point.mapId === 200 && point.x === 3 && point.y === 9), "The server publishes the intermediate forced-route point").toBe(true);
    expect(serverPoints.some((point) => point.mapId === 200 && point.x === 2 && point.y === 9 && point.pathFinished), "The server finishes its own forced route").toBe(true);
    expect((await getGameState(page)).map.id).toBe(200);
    errors.assertNoSevereErrors();
    console.log(JSON.stringify({ forcedMovementWire: { ordinaryCommits: commits.length, cancellations: cancellations.length, cancellationAcknowledged: cancellations.every((request) => responses.some((response) => response.requestId === request.requestId && response.success && response.cancelled)), serverPoints: serverPoints.map(({ x, y, pathFinished }) => ({ x, y, pathFinished })) } }));
    await page.screenshot({ path: test.info().outputPath("forced-arrow-arrived.png") });
  } finally {
    const path = test.info().outputPath("forced-movement-wire.json");
    await writeFile(path, JSON.stringify({ requests, responses, serverPoints, wire }, null, 2));
    await test.info().attach("forced-movement-wire.json", { path, contentType: "application/json" });
  }
});


for (const boundary of [4, 3]) {
  test(`forced route resumes after process death at committed x=${boundary}`, async ({ page, context }) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the recorded private server crash runner");
    test.setTimeout(120_000);
    const { sql, crash, freeze, record } = await isolatedCrashRuntime();
    let frozen: Promise<number> | undefined;
    let errors = collectPageErrors(page);
    await page.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => server.send(message));
      server.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.ServerPlayerMovementNotify) {
          const point = JSON.parse(message.subarray(6).toString());
          if (!frozen && point.mapId === 200 && point.x === boundary && point.y === 9 && !point.pathFinished) {
            frozen = freeze();
            return;
          }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "debug_spin_route_ready");
    await waitForNoMapLoading(page);
    await waitForPlayerTile(page, 5, 9);
    const charId = (await getGameState(page)).player.internalId!;
    expect(Number.isSafeInteger(charId) && charId > 0).toBe(true);
    await pressMovement(page, "left");
    await expect.poll(() => Boolean(frozen)).toBe(true);
    const pid = await frozen!;
    let receipt: Awaited<ReturnType<typeof crash>>;
    try {
      expect(await sql(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=${charId}`)).toBe(`200|${boundary}|9`);
      expect(await sql(`SELECT x,json_array_length(path_json::json->'path') FROM character_movement_routes WHERE character_id=${charId}`)).toBe(`${boundary}|${boundary - 2}`);
      errors.assertNoSevereErrors();
    } finally {
      // Even a failed boundary assertion must release this stopped private child.
      await page.close();
      receipt = await crash();
    }
    expect(receipt.oldPid).toBe(pid);
    await record({ family: "forced-route", charId, boundary, remaining: boundary - 2, receipt });
    page = await context.newPage();
    await page.goto("/");
    await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
    await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
    errors = collectPageErrors(page);
    await enterWorld(page, character);
    await waitForNoMapLoading(page);
    await waitForPlayerTile(page, 2, 9);
    await waitForPlayerIdle(page);
    expect(await sql(`SELECT COUNT(*) FROM character_movement_routes WHERE character_id=${charId}`)).toBe("0");
    expect(await sql(`SELECT CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=${charId}`)).toBe("2|9");
    expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest)).toEqual([]);
    errors.assertNoSevereErrors();
  });
}
