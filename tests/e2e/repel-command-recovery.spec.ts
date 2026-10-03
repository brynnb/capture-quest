import { expect, test } from "@playwright/test";
import type { RepelUseResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForInventoryOpen, waitForNoMapLoading } from "./helpers/state";

for (const loseReply of [false, true]) {
test(`Repel duplicate and lost reply=${loseReply} reconcile once across closure and reentry`, async ({page}, testInfo) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the disposable database and owned crash runner");
  test.setTimeout(120000);
  const {sql, crash, record} = await isolatedCrashRuntime();
  let errors = collectPageErrors(page);
  const faults = await inventoryCommandFaults<RepelUseResponse>(page, OpCodes.RepelUseRequest, OpCodes.RepelUseResponse, loseReply);
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "repel_use_consumes_item");
  const initial = await getGameState(page);
  const characterId = initial.player.internalId!;
  expect(Number.isSafeInteger(characterId) && characterId > 0).toBe(true);
  const instanceId = initial.inventory.items.find(item => item.itemId === 30)!.instanceId;
  const quantity = async () => (await getGameState(page)).inventory.items.find(item => item.instanceId === instanceId)?.quantity ?? 0;
  const row = page.locator(`[data-testid="inventory-item-repel"][data-instance-id="${instanceId}"]`);
  await page.getByRole("button", {name:"Bag",exact:true}).click();
  await waitForInventoryOpen(page, true);
  await row.click();
  await expect.poll(() => faults.replies.length).toBe(1);
  expect(faults.replies[0]).toMatchObject({instanceId,stepsLeft:100,inventory:{commandRevision:1,money:initial.inventory.money}});
  if (loseReply) {
    await page.getByRole("button", {name:"Bag",exact:true}).click();
    await waitForInventoryOpen(page, false);
  }
  await expect.poll(quantity, {timeout:20000}).toBe(1);
  expect(faults.requests).toBe(1);
  expect(await sql(`SELECT steps_left FROM character_repels WHERE character_id=${characterId}`)).toBe("100");
  if (loseReply) {
    // Opening the bag's legacy inventory read is suppressed by the fault helper;
    // the shared command recovery must already have reconciled this quantity.
    await page.getByRole("button", {name:"Bag",exact:true}).click();
    await waitForInventoryOpen(page, true);
  }
  // The inventory intentionally omits the quantity label for a single item.
  await expect(row).toHaveText("REPEL");

  // Set up an expired effect in the verified private database. Actual step
  // decrement/expiry is covered by the durable-effect tests, not this fixture.
  await sql(`DELETE FROM character_repels WHERE character_id=${characterId}`);
  await row.click();
  await expect.poll(() => faults.replies.length).toBe(2);
  expect(faults.replies[1].inventory.commandRevision).toBe(2);
  expect(faults.replies[1].inventory.items.some(item => item.instance.id === instanceId)).toBe(false);
  await expect.poll(() => faults.rejections).toBe(2);
  if (loseReply) {
    // Kill the exact owned server after the second commit, before delivery or
    // timeout reconciliation. Reentry must recover the committed empty stack.
    expect(await quantity()).toBe(1);
    errors.assertNoSevereErrors();
    const receipt = await crash();
    await record({family:"repel",characterId,receipt,commands:faults.successes});
    await page.reload();
    await page.getByRole("button", {name:"PLAY AS GUEST"}).click();
    await expect(page.getByRole("heading", {name:"SELECT A CHARACTER"})).toBeVisible();
    errors = collectPageErrors(page);
  } else {
    await expect.poll(quantity).toBe(0);
    await quitToCharacterSelect(page);
  }
  await enterWorld(page, character); await waitForNoMapLoading(page);
  expect(await quantity()).toBe(0);
  expect((await getGameState(page)).inventory.money).toBe(initial.inventory.money);
  expect(await sql(`SELECT revision FROM character_shop_state WHERE character_id=${characterId}`)).toBe("2");
  expect(await sql(`SELECT steps_left FROM character_repels WHERE character_id=${characterId}`)).toBe("100");
  expect(faults.requests).toBe(2); expect(faults.successes).toBe(2);
  await page.getByRole("button", {name:"Bag",exact:true}).click();
  await waitForInventoryOpen(page, true);
  await expect(row).toHaveCount(0);
  await page.screenshot({path:testInfo.outputPath("repel-recovered-after-reentry.png")});
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
}
