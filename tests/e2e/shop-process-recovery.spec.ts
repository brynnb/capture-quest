import { expect, test, type Page } from "@playwright/test";
import type { CQMerchantBuyRequest, CQMerchantSellRequest, CQMerchantBuyResponse, CQMerchantSellResponse, ShopCommandError } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement, pressSpace } from "./helpers/input";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";

test("committed shop buy and sale survive two SIGKILLs without replaying effects", async ({ page, context }) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash mode");
  test.setTimeout(240000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  const errors = collectPageErrors(page);
  const buys: CQMerchantBuyRequest[] = [], sales: CQMerchantSellRequest[] = [];
  const rejected: ShopCommandError[] = [];
  let purchase: CQMerchantBuyResponse | undefined, sale: CQMerchantSellResponse | undefined;
  let purchasePacket: Buffer | undefined;
  let withhold = false;
  let deliver: ((message: Buffer) => void) | undefined;
  await context.routeWebSocket("**/ws", socket => {
    deliver = message => socket.send(message);
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.CQMerchantBuyRequest) buys.push(JSON.parse(message.subarray(6).toString()));
        if (opcode === OpCodes.CQMerchantSellRequest) sales.push(JSON.parse(message.subarray(6).toString()));
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (withhold && opcode === OpCodes.CQInventoryResponse) return;
        if (opcode === OpCodes.CQMerchantBuyResponse || opcode === OpCodes.CQMerchantSellResponse) {
          const reply = JSON.parse(message.subarray(6).toString());
          if (!reply.success) rejected.push(reply);
          else if (withhold) {
            if (opcode === OpCodes.CQMerchantBuyResponse) { purchase = reply; purchasePacket = message; }
            else sale = reply;
            return;
          }
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_shop_inventory_publication");
  const characterId = (await getGameState(page)).player.internalId;
  expect(Number.isSafeInteger(characterId) && characterId! > 0).toBe(true);
  const readDurable = async () => JSON.parse(await sql(`SELECT json_build_object(
    'money',(SELECT pokedollars FROM character_wallet WHERE character_id=${characterId}),
    'revision',COALESCE((SELECT revision FROM character_shop_state WHERE character_id=${characterId}),0),
    'items',COALESCE((SELECT json_agg(json_build_object('id',ii.id,'itemId',ii.item_id,'quantity',ii.quantity,'ownerId',ii.owner_id,'ownerType',ii.owner_type) ORDER BY ii.id)
      FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=${characterId}),'[]'::json),
    'stock',(SELECT quantity FROM cq_merchant_items WHERE merchant_id=1 AND item_id=4))`));
  const open = async (target: Page) => {
    await pressMovement(target, "left");
    await expect.poll(async () => (await getGameState(target)).player.direction).toBe("LEFT");
    await pressSpace(target);
    await expect(target.getByRole("button", { name: "BUY", exact: true })).toBeEnabled();
    const actorId = await target.evaluate(async () => {
      const path = "/src/stores/CQInventoryStore.ts";
      const { default: store } = await import(path);
      return store.getState().shopActorId as number;
    });
    expect(Number.isSafeInteger(actorId) && actorId > 0).toBe(true);
    return actorId;
  };
  const reopen = async () => {
    const fresh = await context.newPage();
    await fresh.goto("/"); await fresh.getByRole("button", { name: "PLAY AS GUEST" }).click();
    await expect(fresh.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
    await enterWorld(fresh, character); await waitForNoMapLoading(fresh); await waitForPlayerTile(fresh, 2, 5);
    return fresh;
  };
  const original = await readDurable();
  expect(original).toMatchObject({ money: 10000, revision: 0, items: [{ itemId: 4, quantity: 95, ownerId: characterId, ownerType: 0 }] });
  expect(original.items).toHaveLength(1);
  await open(page);
  withhold = true;
  for (let i = 0; i < 9; i++) await page.getByRole("button", { name: "+", exact: true }).click();
  await page.getByRole("button", { name: "BUY", exact: true }).click();
  await expect.poll(() => !!purchase).toBe(true);
  expect(buys).toHaveLength(1);
  const bought = await readDurable();
  expect(bought.money).toBe(8000); expect(bought.revision).toBe(1);
  expect(bought.items.map((item: { quantity: number }) => item.quantity)).toEqual([99, 6]);
  expect(bought.items[0].id).toBe(original.items[0].id); expect(bought.stock).toBe(original.stock);
  expect((await getGameState(page)).inventory.money).toBe(10000);
  await expect(page.getByRole("button", { name: "BUY", exact: true })).toBeDisabled();
  errors.assertNoSevereErrors();
  const buyCrash = await crash(); expect(await readDurable()).toEqual(bought);
  await page.close(); withhold = false;
  const fresh = await reopen(), freshErrors = collectPageErrors(fresh);
  expect(await readDurable()).toEqual(bought);
  expect((await getGameState(fresh)).inventory.money).toBe(8000);
  const actorId = await open(fresh);
  const send = async (target: Page, opcode: number, payload: unknown) => target.evaluate(async ({ opcode, payload }) => {
    const path = "/src/net/NetworkBridge.ts";
    const { NetworkBridge } = await import(path); NetworkBridge.send(payload, opcode);
  }, { opcode, payload });
  // A new runtime actor selector keeps authorization valid so this specifically
  // tests the persisted revision, rather than an obsolete registry ID.
  await send(fresh, OpCodes.CQMerchantBuyRequest, { ...buys[0], actorId, requestId: "stale-buy-after-crash" });
  await expect.poll(() => rejected.find(reply => reply.requestId === "stale-buy-after-crash")?.success).toBe(false);
  expect(rejected.find(reply => reply.requestId === "stale-buy-after-crash")?.error).toBe("Could not buy this item. Read current inventory before trying again.");
  expect(await readDurable()).toEqual(bought);
  withhold = true;
  await fresh.evaluate(async instanceId => {
    const path = "/src/phaser-game/services/PhaserNetworkService.ts";
    const net = await import(path); void net.sendCQMerchantSell(instanceId);
  }, bought.items[1].id);
  await expect.poll(() => !!sale).toBe(true);
  expect(sales).toHaveLength(1);
  expect(sale).toMatchObject({ sellPrice: 600, money: 8600, inventory: { shopRevision: 2 } });
  const sold = await readDurable();
  expect(sold).toEqual({ ...bought, money: 8600, revision: 2, items: [bought.items[0]] });
  expect((await getGameState(fresh)).inventory.money).toBe(8000);
  freshErrors.assertNoSevereErrors();
  const saleCrash = await crash(); expect(await readDurable()).toEqual(sold);
  await fresh.close(); withhold = false;
  const final = await reopen(), finalErrors = collectPageErrors(final);
  const finalActor = await open(final);
  expect((await getGameState(final)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity }))).toEqual([{ id: bought.items[0].id, quantity: 99 }]);
  // Target an existing owned stack: absence of the sold instance must not mask
  // a missing revision guard after restart.
  await send(final, OpCodes.CQMerchantSellRequest, { ...sales[0], actorId: finalActor, instanceId: bought.items[0].id, requestId: "stale-sale-after-crash" });
  await expect.poll(() => rejected.find(reply => reply.requestId === "stale-sale-after-crash")?.success).toBe(false);
  expect(rejected.find(reply => reply.requestId === "stale-sale-after-crash")?.error).toBe("Could not sell this item. Read current inventory before trying again.");
  expect(await readDurable()).toEqual(sold);
  await final.evaluate(async opcode => {
    const path = "/src/phaser-game/services/PhaserNetworkService.ts";
    const net = await import(path);
    const target = window as typeof window & { recoveredShopLateReply?: boolean };
    target.recoveredShopLateReply = false;
    const stop = net.onShopCommand(opcode, () => { target.recoveredShopLateReply = true; stop(); });
  }, OpCodes.CQMerchantBuyResponse);
  deliver!(purchasePacket!);
  await expect.poll(() => final.evaluate(() => (window as typeof window & { recoveredShopLateReply?: boolean }).recoveredShopLateReply)).toBe(true);
  expect((await getGameState(final)).inventory.money).toBe(8600);
  await expect(final.getByText("¥8,600", { exact: true }).first()).toBeVisible();
  expect(await readDurable()).toEqual(sold); expect(buys).toHaveLength(2); expect(sales).toHaveLength(2);
  await final.screenshot({ path: test.info().outputPath("shop-after-two-crashes.png") });
  finalErrors.assertNoSevereErrors();
  await final.getByRole("button", { name: "EXIT", exact: true }).click();
  await quitToCharacterSelect(final); await final.close();
  await record({ outcome: "shop-buy-sale", characterId, buyCrash, saleCrash, original, bought, sold, buys, sales, purchase, sale, rejected });
});
