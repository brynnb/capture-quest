import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";
import { pressMovement, pressSpace } from "./helpers/input";
import * as OpCodes from "../../src/net/generated/opcodes";

for (const loseReply of [false, true]) {
test(`a duplicated shop request with lost reply=${loseReply} settles once and recovers its committed bag`, async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  let purchases = 0, successes = 0, rejections = 0;
  let duplicate: (() => void) | undefined;
  let committed: { inventory: { items: Array<{ instance: { id: number; quantity: number } }>; money: number; shopRevision: number } } | undefined;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.CQMerchantBuyRequest) {
        purchases++;
        // Deliver the same authenticated command twice, preserving its identity.
        server.send(message);
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && purchases > 0) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.CQInventoryResponse) return;
        if (opcode === OpCodes.CQMerchantBuyResponse) {
          const reply = JSON.parse(message.subarray(6).toString());
          if (reply.success) {
            successes++; committed = reply; duplicate = () => socket.send(message);
            if (loseReply) return;
          } else { rejections++; return; }
        }
      }
      socket.send(message);
    });
  });
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
  for (let i = 0; i < 9; i++) await page.getByRole("button", { name: "+", exact: true }).click();
  await page.getByRole("button", { name: "BUY", exact: true }).click();
  await expect.poll(() => !!committed).toBe(true);
  if (loseReply) await expect(page.getByRole("button", { name: "BUY", exact: true })).toBeDisabled();
  const expected = committed!.inventory;
  expect(expected.shopRevision).toBe(1);
  expect(expected.money).toBe(before.inventory.money - 2000);
  expect(expected.items.map(item => item.instance.quantity).sort((a, b) => a - b)).toEqual([6, 99]);
  const bag = expected.items.map(item => ({ id: item.instance.id, quantity: item.instance.quantity }));
  await expect.poll(async () => (await getGameState(page)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity })),{timeout:20000}).toEqual(bag);
  await expect.poll(async () => (await getGameState(page)).inventory.money).toBe(expected.money);
  await expect(page.getByText(`¥${expected.money.toLocaleString()}`, { exact: true }).first()).toBeVisible();
  duplicate!();
  await page.getByRole("button", { name: "EXIT", exact: true }).click();
  await quitToCharacterSelect(page);
  await enterWorld(page, character);
  await waitForNoMapLoading(page);
  expect((await getGameState(page)).inventory.items.map(item => ({ id: item.instanceId, quantity: item.quantity }))).toEqual(bag);
  expect((await getGameState(page)).inventory.money).toBe(expected.money);
  expect(purchases).toBe(1); expect(successes).toBe(1); expect(rejections).toBe(1);
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
    const stop=net.onShopCommand(95,()=>{ target.shopLateMenuReceived=true; stop(); });
  });
  release!();
  await expect.poll(()=>page.evaluate(()=> (window as typeof window & { shopLateMenuReceived?: boolean }).shopLateMenuReceived)).toBe(true);
  await expect.poll(async()=> (await getGameState(page)).inventory.money).toBe(10000);
  await expect(page.getByRole("button",{name:"BUY",exact:true})).toHaveCount(0);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
