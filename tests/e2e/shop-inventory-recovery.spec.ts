import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

test("a shop purchase restores its committed bag without an independent inventory notification", async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  let purchases = 0;
  let duplicate: (() => void) | undefined;
  let committed: { inventory: { items: Array<{ instance: { id: number; quantity: number } }>; money: number } } | undefined;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.CQMerchantBuyRequest) purchases++;
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && purchases > 0) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.CQInventoryResponse) return;
        if (opcode === OpCodes.CQMerchantBuyResponse) {
          const reply = JSON.parse(message.subarray(6).toString());
          if (reply.success) { committed = reply; duplicate = () => socket.send(message); }
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  // One fixture supplies both currency and post-parcel clerk eligibility.
  await jumpToScenario(page, "debug_shop_inventory_publication");
  // Open the existing merchant transport for this owned map; purchase uses the UI.
  await page.evaluate(async () => {
    const path = "/src/phaser-game/services/PhaserNetworkService.ts";
    const net = await import(path);
    const mapId = window.__capturequestTest!.getState().map.id;
    if (!mapId) throw new Error("Missing owned merchant map");
    net.sendCQMerchantOpenByMap(mapId);
  });
  await expect(page.getByRole("button", { name: "BUY", exact: true })).toBeEnabled();
  const before = await getGameState(page);
  expect(before.inventory.items.filter(item => item.shortName === "POKE_BALL").map(item => item.quantity)).toEqual([95]);
  for (let i = 0; i < 9; i++) await page.getByRole("button", { name: "+", exact: true }).click();
  await page.getByRole("button", { name: "BUY", exact: true }).click();
  await expect.poll(() => !!committed).toBe(true);
  const expected = committed!.inventory;
  expect(expected.money).toBe(before.inventory.money - 2000);
  expect(expected.items.map(item => item.instance.quantity).sort((a, b) => a - b)).toEqual([6, 99]);
  const bag = expected.items.map(item => ({ id: item.instance.id, quantity: item.instance.quantity }));
  await expect.poll(async () => (await getGameState(page)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity }))).toEqual(bag);
  await expect.poll(async () => (await getGameState(page)).inventory.money).toBe(expected.money);
  await expect(page.getByText(`¥${expected.money.toLocaleString()}`, { exact: true }).first()).toBeVisible();
  duplicate!();
  await page.getByRole("button", { name: "EXIT", exact: true }).click();
  await quitToCharacterSelect(page);
  await enterWorld(page, character);
  await waitForNoMapLoading(page);
  expect((await getGameState(page)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity }))).toEqual(bag);
  expect((await getGameState(page)).inventory.money).toBe(expected.money);
  expect(purchases).toBe(1);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
