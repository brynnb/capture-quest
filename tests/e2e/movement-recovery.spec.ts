import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile, waitForWarpMode } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

test("a lost step reply recovers once and its receipt survives character re-entry without rewinding", async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  const completions: Array<{ stepToken: string; requestId: string }> = [];
  const receipts: Array<{ requestId: string; replayed?: boolean; x: number }> = [];
  let dropped = false;
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PlayerStepCompleteRequest) {
        completions.push(JSON.parse(message.subarray(6).toString()));
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PlayerStepCompleteResponse) {
        const result = JSON.parse(message.subarray(6).toString());
        if (result.success) {
          if (!dropped) { dropped = true; return; }
          receipts.push(result);
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await page.getByRole("button", { name: "Instant Warp" }).click();
  await waitForWarpMode(page, true); await clickTile(page, 6, 7);
  await waitForWarpMode(page, false); await waitForPlayerTile(page, 6, 7);
  await pressMovement(page, "left");
  await expect.poll(() => completions.length, { timeout: 20000 }).toBe(2);
  expect(dropped).toBe(true);
  expect(completions[1].stepToken).toBe(completions[0].stepToken);
  expect(completions[1].requestId).not.toBe(completions[0].requestId);
  await expect.poll(() => receipts[0]?.replayed).toBe(true);
  await waitForPlayerIdle(page); await waitForPlayerTile(page, 5, 7);
  expect(errors.sentOpcodes).toContain(OpCodes.OwnedPlayerPositionRequest);

  // Re-entering creates a fresh movement registration. A receipt retry can only
  // acknowledge historical success, even after the current location changes.
  await quitToCharacterSelect(page); await enterWorld(page, character);
  await waitForNoMapLoading(page); await waitForPlayerIdle(page);
  await page.getByRole("button", { name: "Instant Warp" }).click();
  await waitForWarpMode(page, true); await clickTile(page, 6, 7);
  await waitForWarpMode(page, false); await waitForPlayerTile(page, 6, 7);
  await page.evaluate(async (stepToken) => {
    const bridgePath = "/src/net/NetworkBridge.ts";
    const opcodePath = "/src/net/generated/opcodes.ts";
    const { NetworkBridge } = await import(bridgePath);
    const opcodes = await import(opcodePath);
    NetworkBridge.send({ stepToken, requestId: "after-reentry" }, opcodes.PlayerStepCompleteRequest);
  }, completions[0].stepToken);
  await expect.poll(() => receipts.find((r) => r.requestId === "after-reentry")?.replayed).toBe(true);
  await waitForPlayerIdle(page); await waitForPlayerTile(page, 6, 7);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
