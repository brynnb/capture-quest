import { expect, test, type Page } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createCharacter, enterWorld, loginAsGuest, quitToCharacterSelect, uniqueTrainerName } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { waitForMap, waitForPlayerIdle } from "./helpers/state";

async function entryState(page: Page) {
  return page.evaluate(async () => {
    const selectPath = "/src/stores/CharacterSelectStore.ts";
    const statusPath = "/src/stores/GameStatusStore.ts";
    const screenPath = "/src/stores/GameScreenStore.ts";
    const [selection, status, screen] = await Promise.all([import(selectPath), import(statusPath), import(screenPath)]);
    return {
      entering: selection.default.getState().isEnteringWorld,
      loading: status.default.getState().isMapLoading,
      screen: screen.default.getState().currentScreen,
    };
  });
}

function responseFrame(value: number) {
  const body = Buffer.from(JSON.stringify({ value }));
  const frame = Buffer.alloc(body.length + 6);
  frame.writeUInt32LE(body.length + 2, 0);
  frame.writeUInt16LE(OpCodes.PostEnterWorld, 4);
  body.copy(frame, 6);
  return frame;
}

test("entry rejection settles the UI and a duplicate click sends one retry", async ({ page }, testInfo) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Uses WebSocket response interception");
  const errors = collectPageErrors(page);
  let requests = 0;
  let release: (() => void) | undefined;
  const alerts: string[] = [];
  page.on("dialog", async dialog => { alerts.push(dialog.message()); await dialog.accept(); });
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.EnterWorld) {
        requests++;
        // A denied request acquires no character. The retry goes through the
        // real server, with only its final acknowledgement withheld.
        if (requests === 1) { socket.send(responseFrame(0)); return; }
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PostEnterWorld && requests === 2) {
        release = () => socket.send(message); return;
      }
      socket.send(message);
    });
  });
  const name = uniqueTrainerName();
  await loginAsGuest(page); await createCharacter(page, name);
  const button = page.getByRole("button", { name: /Enter World/i }).filter({ visible: true });
  await button.click();
  await expect.poll(() => alerts.length).toBe(1);
  await expect(button).toBeEnabled();
  await expect.poll(() => entryState(page)).toEqual({ entering: false, loading: false, screen: "characterSelect" });
  await button.evaluate((element: HTMLElement) => { element.click(); element.click(); });
  await expect.poll(() => Boolean(release)).toBe(true);
  expect(requests).toBe(2);
  await expect(button).toBeDisabled();
  await expect(page.getByTestId("game-screen")).not.toBeVisible();
  await expect.poll(() => entryState(page)).toEqual({ entering: true, loading: true, screen: "characterSelect" });
  await page.screenshot({ path: testInfo.outputPath("entry-awaiting-confirmation.png") });
  release!();
  await expect(page.getByTestId("game-screen")).toBeVisible();
  await expect(page.getByRole("button", { name: "View Map" })).toBeVisible();
  await expect.poll(async () => (await entryState(page)).entering).toBe(false);
  await waitForMap(page, 38); await waitForPlayerIdle(page);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("an unacknowledged entry releases its connection and allows fresh login", async ({ page }, testInfo) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Uses WebSocket response interception");
  test.setTimeout(75_000);
  let requests = 0;
  let heldSuccess = false;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.EnterWorld) requests++;
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PostEnterWorld && requests === 1) {
        const reply = JSON.parse(message.subarray(6).toString()) as { value: number };
        heldSuccess = reply.value === 1;
        return;
      }
      socket.send(message);
    });
  });
  const name = uniqueTrainerName();
  await loginAsGuest(page); await createCharacter(page, name);
  await page.getByRole("button", { name: /Enter World/i }).filter({ visible: true }).click();
  await expect.poll(() => heldSuccess).toBe(true);
  await expect(page.getByTestId("game-screen")).not.toBeVisible();
  await expect(page.getByRole("button", { name: "PLAY AS GUEST" })).toBeVisible({ timeout: 20_000 });
  await expect.poll(() => entryState(page)).toEqual({ entering: false, loading: false, screen: "title" });
  await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
  await enterWorld(page, name);
  await waitForMap(page, 38); await waitForPlayerIdle(page);
  expect(requests).toBe(2);
  await page.screenshot({ path: testInfo.outputPath("entry-after-unacknowledged-attempt.png") });
  await quitToCharacterSelect(page);
});
