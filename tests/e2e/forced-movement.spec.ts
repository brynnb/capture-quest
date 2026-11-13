import { expect, test } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

test("ordinary walking onto an imported arrow starts its server-owned route", async ({ page }) => {
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_spin_route_ready");
  await waitForNoMapLoading(page);
  await waitForPlayerTile(page, 5, 9);
  const before = errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest).length;
  await pressMovement(page, "left");
  await waitForPlayerTile(page, 2, 9);
  await waitForPlayerIdle(page);
  // Only the user step is acknowledged; the two source-data points belong to
  // the server timer and cannot become independent client completion effects.
  expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest)).toHaveLength(before + 1);
  expect((await getGameState(page)).map.id).toBe(200);
  errors.assertNoSevereErrors();
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
