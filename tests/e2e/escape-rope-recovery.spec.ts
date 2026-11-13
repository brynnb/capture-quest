import { expect, test } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import type { EscapeRopeUseResponse } from "../../src/net/generated/world_api";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { pressMovement } from "./helpers/input";
import { collectPageErrors } from "./helpers/errors";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForInventoryItem, waitForInventoryOpen, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";

for (const mode of ["normal", "timeout", "crash"] as const) {
  test(`Escape Rope duplicate and mode=${mode} recover one committed bag and destination`, async ({ page, context }) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated database and exact-process runner");
    test.setTimeout(120_000);
    const { sql, crash, record } = await isolatedCrashRuntime();
    let errors = collectPageErrors(page);
    const faults = await inventoryCommandFaults<EscapeRopeUseResponse>(page, OpCodes.EscapeRopeUseRequest, OpCodes.EscapeRopeUseResponse, mode !== "normal");
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "debug_escape_rope_ready");
    await waitForPlayerTile(page, 9, 9);
    await waitForInventoryItem(page, 29);
    const before = await getGameState(page);
    const charId = before.player.internalId!;
    const mapId = before.map.id!;
    expect(Number.isSafeInteger(charId) && charId > 0).toBe(true);
    expect(Number.isSafeInteger(mapId) && mapId > 0).toBe(true);
    const expected = (await sql(`SELECT CASE WHEN COALESCE(m.is_overworld,0)=1 THEN 9999 ELSE w.destination_map_id END,w.destination_x,w.destination_y FROM phaser_warps w LEFT JOIN phaser_maps m ON m.id=w.destination_map_id WHERE w.source_map_id=${mapId} AND w.destination_map_id IS NOT NULL AND w.destination_x IS NOT NULL AND w.destination_y IS NOT NULL AND COALESCE(w.warp_type,'door') NOT IN ('elevator','inactive') ORDER BY CASE WHEN COALESCE(m.is_overworld,0)=1 OR w.destination_map_id=9999 THEN 0 ELSE 1 END,w.id LIMIT 1`)).split("|").map(Number);
    expect(expected).toHaveLength(3);
    await page.getByRole("button", { name: "Bag", exact: true }).click();
    await waitForInventoryOpen(page, true);
    await page.getByTestId("inventory-item-escape-rope").click();
    await expect.poll(() => faults.successes).toBe(1);
    await expect.poll(() => faults.rejections).toBe(1);
    expect(faults.requests).toBe(1);
    const committed = `${expected[0]}|${expected[1]}|${expected[2]}|1|1`;
    const stateQuery = `SELECT c.map_id,CAST(c.x AS INTEGER),CAST(c.y AS INTEGER),s.revision,i.quantity FROM character_data c JOIN character_shop_state s ON s.character_id=c.id JOIN cq_character_inventory b ON b.character_id=c.id JOIN cq_item_instances i ON i.id=b.item_instance_id WHERE c.id=${charId} AND i.item_id=29`;
    expect(await sql(stateQuery)).toBe(committed);
    if (mode === "crash") {
      errors.assertNoSevereErrors();
      await page.close();
      const receipt = await crash();
      await record({ family: "escape-rope", charId, expected, receipt, revision: 1 });
      page = await context.newPage();
      await page.goto("/");
      await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
      await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
      errors = collectPageErrors(page);
      await enterWorld(page, character);
    }
    await waitForNoMapLoading(page);
    await expect.poll(async () => {
      const state = await getGameState(page);
      return [state.map.id, state.player.x, state.player.y, state.inventory.items.find(i => i.itemId === 29)?.quantity];
    }, { timeout: 25_000 }).toEqual([...expected, 1]);
    if (mode !== "crash") {
      faults.deliverReply(0);
      await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      if ((await getGameState(page)).ui.isInventoryOpen) {
        await page.getByRole("button", { name: "Bag", exact: true }).click();
      }
      await waitForInventoryOpen(page, false);
      await quitToCharacterSelect(page);
      await enterWorld(page, character);
      await waitForNoMapLoading(page);
      expect(await sql(stateQuery)).toBe(committed);
    }
    expect(faults.requests).toBe(1);
    errors.assertNoSevereErrors();
  });
}


test("failed Escape Rope recovery locks the owner until quit and fresh entry", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const warnings: string[] = [];
  page.on("console", message => { if (message.type() === "warning") warnings.push(message.text()); });
  let withholdReads = false;
  const faults = await inventoryCommandFaults<EscapeRopeUseResponse>(page,
    OpCodes.EscapeRopeUseRequest, OpCodes.EscapeRopeUseResponse, false, () => true,
    opcode => withholdReads && opcode === OpCodes.GameplayStateResponse);
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_escape_rope_ready");
  await waitForPlayerTile(page, 9, 9);
  await waitForInventoryItem(page, 29);
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await waitForInventoryOpen(page, true);
  withholdReads = true;
  const rope = page.getByTestId("inventory-item-escape-rope");
  await rope.click();
  await expect.poll(() => warnings.some(w => w.includes("Escape Rope recovery unavailable")), { timeout: 20_000 }).toBe(true);
  expect(faults.requests).toBe(1);
  expect(faults.successes).toBe(1);
  await rope.click();
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await waitForInventoryOpen(page, false);
  const stepRequests = errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepRequest).length;
  await pressMovement(page, "down");
  await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepRequest)).toHaveLength(stepRequests);
  expect(faults.requests).toBe(1);
  await waitForPlayerTile(page, 9, 9);
  await quitToCharacterSelect(page);
  withholdReads = false;
  await enterWorld(page, character);
  await waitForNoMapLoading(page);
  await expect.poll(async () => (await getGameState(page)).inventory.items.find(i => i.itemId === 29)?.quantity).toBe(1);
  expect((await getGameState(page)).map.id).toBe(9999);
  faults.deliverReply(0);
  await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  expect((await getGameState(page)).map.id).toBe(9999);
  errors.assertNoSevereErrors();
});

test("Escape Rope retires a lost step authorization after its deadline", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const lostStep = await inventoryCommandFaults(page, OpCodes.PlayerStepRequest, OpCodes.PlayerStepResponse, true);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_escape_rope_ready");
  await waitForPlayerTile(page, 9, 9);
  await waitForInventoryItem(page, 29);
  await waitForNoMapLoading(page);
  await pressMovement(page, "right");
  await expect.poll(() => lostStep.requests).toBe(1);
  // Issuance is not a commit. Withhold both replacement acknowledgements and
  // let the ordinary client timeout retire prediction without moving a tile.
  await expect.poll(() => errors.consoleErrors.length, { timeout: 20_000 }).toBe(1);
  expect(errors.consoleErrors[0]).toContain("[PlayerMovement] Movement request failed");
  await waitForPlayerTile(page, 9, 9);
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await waitForInventoryOpen(page, true);
  await page.getByTestId("inventory-item-escape-rope").click();
  await expect.poll(async () => {
    const state = await getGameState(page);
    return [state.map.id, state.inventory.items.find(i => i.itemId === 29)?.quantity];
  }, { timeout: 20_000 }).toEqual([9999, 1]);
  lostStep.deliverReply(0);
  await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest)).toEqual([]);
  expect((await getGameState(page)).map.id).toBe(9999);
  expect(errors.consoleErrors).toHaveLength(1);
  expect([...errors.pageErrors, ...errors.networkErrors, ...errors.retiredPositionPackets]).toEqual([]);
});
