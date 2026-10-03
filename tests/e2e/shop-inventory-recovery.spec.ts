import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";
import { pressMovement, pressSpace } from "./helpers/input";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import type { CQMerchantBuyResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";

for (const action of ["buy", "sell"] as const) {
for (const loseReply of [false, true]) {
test(`a duplicated shop ${action} request with lost reply=${loseReply} settles once and recovers its committed bag`, async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  const requestOpcode=action==="buy"?OpCodes.CQMerchantBuyRequest:OpCodes.CQMerchantSellRequest;
  const responseOpcode=action==="buy"?OpCodes.CQMerchantBuyResponse:OpCodes.CQMerchantSellResponse;
  const faults = await inventoryCommandFaults<CQMerchantBuyResponse>(page, requestOpcode, responseOpcode, loseReply);
  const character = await createGuestCharacterAndEnterWorld(page);
  // One fixture supplies both currency and post-parcel clerk eligibility.
  await jumpToScenario(page, "debug_shop_inventory_publication");
  // Exercise the rendered source clerk interaction, including its script fallback.
  await pressMovement(page, "left");
  await expect.poll(async () => (await getGameState(page)).player.direction).toBe("LEFT");
  await pressSpace(page);
  await expect(page.getByRole("button", { name: "BUY", exact: true })).toBeEnabled();
  const before = await getGameState(page);
  expect(before.inventory.items.filter(item => item.shortName === "POKE_BALL").map(item => item.quantity)).toEqual([95]);
  if(action==="buy") {
    for (let i = 0; i < 9; i++) await page.getByRole("button", { name: "+", exact: true }).click();
    await page.getByRole("button", { name: "BUY", exact: true }).click();
  } else {
    // The product currently exposes no Sell button. Exercise its existing
    // coordinator over the real transport and verify the rendered money view.
    await page.evaluate(async instanceId => {
      const path="/src/phaser-game/services/PhaserNetworkService.ts";
      const net=await import(path); void net.sendCQMerchantSell(instanceId);
    },before.inventory.items[0].instanceId);
  }
  await expect.poll(() => faults.replies.length > 0).toBe(true);
  if (loseReply) await expect(page.getByRole("button", { name: "BUY", exact: true })).toBeDisabled();
  const expected = faults.replies[0].inventory;
  expect(expected.commandRevision).toBe(1);
  expect(expected.money).toBe(before.inventory.money + (action==="buy"?-2000:9500));
  expect(expected.items.map(item => item.instance.quantity).sort((a, b) => a - b)).toEqual(action==="buy"?[6,99]:[]);
  const bag = expected.items.map(item => ({ id: item.instance.id, quantity: item.instance.quantity }));
  await expect.poll(async () => (await getGameState(page)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity })),{timeout:20000}).toEqual(bag);
  await expect.poll(async () => (await getGameState(page)).inventory.money).toBe(expected.money);
  await expect(page.getByText(`¥${expected.money.toLocaleString()}`, { exact: true }).first()).toBeVisible();
  faults.deliverReply(0);
  await page.getByRole("button", { name: "EXIT", exact: true }).click();
  await quitToCharacterSelect(page);
  await enterWorld(page, character);
  await waitForNoMapLoading(page);
  expect((await getGameState(page)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity }))).toEqual(bag);
  expect((await getGameState(page)).inventory.money).toBe(expected.money);
  expect(faults.requests).toBe(1); expect(faults.successes).toBe(1); expect(faults.rejections).toBe(1);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
}
}

for (const completion of ["delayed reply", "timeout recovery"] as const) {
test(`closing a committed purchase preserves ${completion} and the next purchase`, async ({page}, testInfo) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  const faults = await inventoryCommandFaults<CQMerchantBuyResponse>(page, OpCodes.CQMerchantBuyRequest, OpCodes.CQMerchantBuyResponse, true);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_shop_inventory_publication");
  await pressMovement(page, "left");
  await expect.poll(async () => (await getGameState(page)).player.direction).toBe("LEFT");
  await pressSpace(page);
  await expect(page.getByRole("button", {name:"BUY",exact:true})).toBeEnabled();
  const before = (await getGameState(page)).inventory;
  const commandState = () => page.evaluate(async () => {
    const path = "/src/stores/CQInventoryStore.ts";
    const {default:store} = await import(path);
    const state = store.getState();
    return {revision:state.commandRevision,pending:state.inventoryCommandPending};
  });
  await page.getByRole("button", {name:"BUY",exact:true}).click();
  // The real server has committed; keep its reply outside the browser until
  // after the actual EXIT/Escape interaction has closed the shop.
  await expect.poll(() => faults.replies.length).toBe(1);
  const committed = faults.replies[0].inventory;
  expect(committed.commandRevision).toBe(1);
  expect(committed.money).toBe(before.money - 200);
  expect(committed.items.map(item => item.instance.quantity)).toEqual([96]);
  if (completion === "delayed reply") await page.getByRole("button", {name:"EXIT",exact:true}).click();
  else await page.keyboard.press("Escape");
  await expect(page.getByRole("button", {name:"BUY",exact:true})).toHaveCount(0);
  expect(await commandState()).toEqual({revision:0,pending:true});
  if (completion === "delayed reply") faults.deliverReply(0);
  await expect.poll(commandState, {timeout:20000}).toEqual({revision:1,pending:false});
  const recovered = (await getGameState(page)).inventory;
  expect(recovered.shopOpen).toBe(false);
  expect(recovered.money).toBe(committed.money);
  expect(recovered.items.map(item => ({id:item.instanceId,quantity:item.quantity}))).toEqual(
    committed.items.map(item => ({id:item.instance.id,quantity:item.instance.quantity})),
  );
  expect(faults.requests).toBe(1);
  await expect(page.getByRole("button", {name:"BUY",exact:true})).toHaveCount(0);

  // Reopen through the clerk and make a distinct purchase. A stale local
  // revision would cause the server to reject it instead of returning success.
  await pressSpace(page);
  await expect(page.getByRole("button", {name:"BUY",exact:true})).toBeEnabled();
  await expect(page.getByText(`¥${committed.money.toLocaleString()}`, {exact:true}).first()).toBeVisible();
  await page.getByRole("button", {name:"BUY",exact:true}).click();
  await expect.poll(() => faults.replies.length).toBe(2);
  expect(faults.replies[1].inventory.commandRevision).toBe(2);
  faults.deliverReply(1);
  await expect.poll(commandState).toEqual({revision:2,pending:false});
  await expect.poll(async () => (await getGameState(page)).inventory.items.map(item => item.quantity)).toEqual([97]);
  await expect(page.getByText(`¥${(before.money-400).toLocaleString()}`, {exact:true}).first()).toBeVisible();
  await page.screenshot({path:testInfo.outputPath("shop-reopened-after-reconciliation.png")});
  expect(faults.requests).toBe(2); expect(faults.successes).toBe(2);
  await expect.poll(() => faults.rejections).toBe(2);
  await page.getByRole("button", {name:"EXIT",exact:true}).click();
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
}

test("a delayed merchant menu cannot reopen a retired scene", async ({page}) => {
  test.setTimeout(120000);
  const errors=collectPageErrors(page);
  let release:(()=>void)|undefined;
  await page.routeWebSocket("**/ws",socket=>{
    const server=socket.connectToServer();
    socket.onMessage(message=>server.send(message));
    server.onMessage(message=>{
      if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.CQMerchantOpenResponse) {
        const reply=JSON.parse(message.subarray(6).toString());
        if(reply.success) { release=()=>socket.send(message); return; }
      }
      socket.send(message);
    });
  });
  const character=await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page,"debug_shop_inventory_publication");
  await pressMovement(page, "left");
  await expect.poll(async () => (await getGameState(page)).player.direction).toBe("LEFT");
  await pressSpace(page);
  await expect.poll(()=>!!release).toBe(true);
  await expect(page.getByRole("button",{name:"BUY",exact:true})).toHaveCount(0);
  await quitToCharacterSelect(page);
  await enterWorld(page,character); await waitForNoMapLoading(page);
  await page.evaluate(async () => {
    const path="/src/phaser-game/services/PhaserNetworkService.ts";
    const net=await import(path);
    const target=window as typeof window & { shopLateMenuReceived?: boolean };
    target.shopLateMenuReceived=false;
    const stop=net.onInventoryCommand(95,()=>{ target.shopLateMenuReceived=true; stop(); });
  });
  release!();
  await expect.poll(()=>page.evaluate(()=> (window as typeof window & { shopLateMenuReceived?: boolean }).shopLateMenuReceived)).toBe(true);
  await expect.poll(async()=> (await getGameState(page)).inventory.money).toBe(10000);
  await expect(page.getByRole("button",{name:"BUY",exact:true})).toHaveCount(0);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
