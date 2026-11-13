import { expect, test, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createCharacter, enterWorld, loginAsGuest, quitToCharacterSelect, uniqueTrainerName } from "./helpers/auth";
import { waitForMap, waitForPlayerIdle } from "./helpers/state";

function frameOpcode(message: string | Buffer) {
  return Buffer.isBuffer(message) && message.length >= 6 ? message.readUInt16LE(4) : undefined;
}

async function rosterState(page: Page) {
  return page.evaluate(async () => {
    const path = "/src/stores/CharacterSelectStore.ts";
    const { default: store } = await import(path);
    const state = store.getState();
    return {
      characters: state.characters.map((character: { id: number; name: string }) => ({ id: character.id, name: character.name })),
      pending: state.accountOperation,
      ready: state.rosterReady,
    };
  });
}

async function openDelete(page: Page) {
  await page.getByRole("button", { name: /^DELETE$/i }).filter({ visible: true }).click();
  const dialog = page.getByRole("dialog", { name: "Delete Character?" });
  await expect(dialog).toBeVisible();
  return dialog;
}

test("a lost delete acknowledgement keeps the row pending and recovers the current roster once", async ({ page }, testInfo) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Uses WebSocket application-response interception");
  test.setTimeout(60_000);
  let deleteRequests = 0;
  let rosterRequests = 0;
  let heldResponse: (() => void) | undefined;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (frameOpcode(message) === OpCodes.DeleteCharacter) deleteRequests++;
      if (frameOpcode(message) === OpCodes.CharacterListRequest) rosterRequests++;
      server.send(message);
    });
    server.onMessage(message => {
      if (frameOpcode(message) === OpCodes.CharacterDeleteResponse) {
        heldResponse = () => socket.send(message);
        return;
      }
      socket.send(message);
    });
  });
  const name = uniqueTrainerName();
  await loginAsGuest(page);
  await createCharacter(page, name);
  const original = await rosterState(page);
  const rosterReadsBeforeDelete = rosterRequests;
  const dialog = await openDelete(page);
  // Two real DOM clicks in the same turn exercise the store's shared guard.
  await dialog.getByRole("button", { name: /^DELETE$/ }).evaluate((button: HTMLButtonElement) => { button.click(); button.click(); });
  await expect.poll(() => Boolean(heldResponse)).toBe(true);
  expect(deleteRequests).toBe(1);
  await expect(dialog.getByRole("button", { name: /DELETING/i })).toBeDisabled();
  await expect(dialog.getByRole("button", { name: "CANCEL" })).toBeDisabled();
  await expect(page.getByRole("button", { name: /ENTER WORLD/i }).filter({ visible: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "CREATE NEW CHARACTER" }).filter({ visible: true }).first()).toBeDisabled();
  expect((await rosterState(page)).characters).toEqual(original.characters);
  await page.screenshot({ path: testInfo.outputPath("delete-awaiting-confirmation.png") });

  await expect(dialog).not.toBeVisible({ timeout: 20_000 });
  await expect.poll(() => rosterState(page)).toMatchObject({ characters: [], pending: null, ready: true });
  expect(deleteRequests).toBe(1);
  expect(rosterRequests).toBe(rosterReadsBeforeDelete + 1);
  // A late historical acknowledgement must not alter a newly created same-name row.
  await createCharacter(page, name);
  const recreated = await rosterState(page);
  expect(recreated.characters).toHaveLength(1);
  expect(recreated.characters[0].id).not.toBe(original.characters[0].id);
  heldResponse!();
  await expect.poll(() => rosterState(page)).toEqual(recreated);
  await enterWorld(page, name);
  await waitForMap(page, 38);
  await waitForPlayerIdle(page);
  await quitToCharacterSelect(page);
});

test("deleting from another account session drains the active character before same-name recreation", async ({ page, context }, testInfo) => {
  test.setTimeout(75_000);
  const token = randomUUID();
  const name = uniqueTrainerName();
  await loginAsGuest(page, token);
  await createCharacter(page, name);
  const original = await rosterState(page);
  await enterWorld(page, name);
  await waitForMap(page, 38);
  await waitForPlayerIdle(page);

  const manager = await context.newPage();
  try {
    await loginAsGuest(manager, token);
    await expect(manager.getByRole("button", { name, exact: true })).toBeVisible();
    const dialog = await openDelete(manager);
    await dialog.getByRole("button", { name: /^DELETE$/ }).click();
    await expect(dialog).not.toBeVisible();
    await expect.poll(() => rosterState(manager)).toMatchObject({ characters: [], pending: null, ready: true });
    await expect(page.getByRole("button", { name: "PLAY AS GUEST" })).toBeVisible();
    await expect(page.getByTestId("game-screen")).not.toBeVisible();
    await manager.screenshot({ path: testInfo.outputPath("deleted-after-active-owner-drain.png") });

    await createCharacter(manager, name);
    const recreated = await rosterState(manager);
    expect(recreated.characters).toHaveLength(1);
    expect(recreated.characters[0].id).not.toBe(original.characters[0].id);
    await enterWorld(manager, name);
    await waitForMap(manager, 38);
    await waitForPlayerIdle(manager);
    await quitToCharacterSelect(manager);
  } finally {
    await manager.close();
  }
});

test("a missing post-login roster settles into an explicit retry without another login", async ({ page }, testInfo) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Uses WebSocket application-response interception");
  test.setTimeout(45_000);
  let authRequests = 0;
  let rosterRequests = 0;
  let withheld = false;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (frameOpcode(message) === OpCodes.JWTLogin) authRequests++;
      if (frameOpcode(message) === OpCodes.CharacterListRequest) rosterRequests++;
      server.send(message);
    });
    server.onMessage(message => {
      if (frameOpcode(message) === OpCodes.CharacterListResponse && !withheld) {
        withheld = true;
        return;
      }
      socket.send(message);
    });
  });
  await page.addInitScript(token => localStorage.setItem("capturequest_guest_token", token), randomUUID());
  await page.goto("/");
  await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect.poll(() => withheld).toBe(true);
  const retry = page.getByRole("button", { name: "RETRY CHARACTERS" });
  await expect(retry).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText("Characters could not be loaded. Please try again.")).toBeVisible();
  expect(authRequests).toBe(1);
  expect(rosterRequests).toBe(1);
  await expect(retry).toBeInViewport({ ratio: 1 });
  await page.screenshot({ path: testInfo.outputPath("roster-timeout-retry.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(retry).toBeInViewport({ ratio: 1 });
  await page.screenshot({ path: testInfo.outputPath("roster-timeout-retry-mobile.png") });
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(retry).toBeInViewport({ ratio: 1 });
  await retry.click();
  await expect(retry).not.toBeVisible();
  await expect.poll(() => rosterState(page)).toMatchObject({ characters: [], pending: null, ready: true });
  expect(authRequests).toBe(1);
  expect(rosterRequests).toBe(2);
  await createCharacter(page, uniqueTrainerName());
});

test("character creation owns the creator navigation until its result and current roster arrive", async ({ page }) => {
  test.skip(process.env.CQ_E2E_TRANSPORT === "native", "Uses WebSocket application-response interception");
  let createRequests = 0;
  let release: (() => void) | undefined;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (frameOpcode(message) === OpCodes.CharacterCreate) createRequests++;
      server.send(message);
    });
    server.onMessage(message => {
      if (frameOpcode(message) === OpCodes.CharacterCreateResponse) {
        release = () => socket.send(message);
        return;
      }
      socket.send(message);
    });
  });
  await loginAsGuest(page);
  const name = uniqueTrainerName();
  const creation = createCharacter(page, name);
  await expect.poll(() => Boolean(release)).toBe(true);
  expect(createRequests).toBe(1);
  await expect(page.getByTestId("character-creation-navigation").getByRole("button", { name: /BACK/i })).toBeDisabled();
  expect((await rosterState(page)).pending).toBe("create");
  release!();
  await creation;
  expect(createRequests).toBe(1);
  await expect.poll(() => rosterState(page)).toMatchObject({ pending: null, ready: true });
});
