import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { waitForNoMapLoading } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

for (const lostReply of [false, true]) test(`trainer preference persists through fresh entry${lostReply ? " after lost reply" : ""}`, async ({ page }) => {
  test.setTimeout(90000);
  let dropped = false;
  let mutations = 0;
  let reads = 0;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.SetOption) {
        const request = JSON.parse(message.subarray(6).toString());
        if (request.current) reads++; else mutations++;
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (lostReply && !dropped && Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.SetOption) {
        const reply = JSON.parse(message.subarray(6).toString());
        if (reply.success) { dropped = true; return; }
      }
      socket.send(message);
    });
  });
  const errors = collectPageErrors(page);
  const character = await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);
  await page.getByRole("button", { name: "Options", exact: true }).click();
  const card = page.getByText("Trainer Re-battles", { exact: true }).locator("..");
  await card.getByRole("button", { name: "OFF", exact: true }).click();
  await expect(card.getByRole("button", { name: "ON", exact: true })).toBeVisible({ timeout: 20000 });
  await page.getByRole("button", { name: "Options", exact: true }).click();
  await quitToCharacterSelect(page); await enterWorld(page, character);
  await waitForNoMapLoading(page);
  await page.getByRole("button", { name: "Options", exact: true }).click();
  await expect(card.getByRole("button", { name: "ON", exact: true })).toBeVisible();
  expect(mutations).toBe(1);
  expect(reads).toBe(lostReply ? 1 : 0);
  expect(dropped).toBe(lostReply);
  errors.assertNoSevereErrors();
});
