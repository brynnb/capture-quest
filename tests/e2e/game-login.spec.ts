import { expect, type Page, test } from "@playwright/test";
import { randomUUID } from "node:crypto";

const guestTokenKey = "capturequest_guest_token";

const tokenToLetters = (token: string) =>
  token
    .replace(/-/g, "")
    .slice(0, 10)
    .replace(/\d/g, (digit) => "abcdefghij"[Number(digit)] ?? "a");

const uniqueTrainerName = () => `Qa${tokenToLetters(randomUUID())}`.slice(0, 12);

async function loginAsGuest(page: Page, guestToken: string) {
  await page.addInitScript(
    ([key, token]) => localStorage.setItem(key, token),
    [guestTokenKey, guestToken],
  );

  await page.goto("/");
  await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({
    timeout: 30_000,
  });
}

async function createCharacter(page: Page, characterName: string) {
  await page.getByRole("button", { name: "CREATE NEW CHARACTER" }).first().click();

  await expect(page.getByText("Choose Your Faction")).toBeVisible();
  await page.getByPlaceholder("Enter character name").fill(characterName);
  await page.getByPlaceholder("Enter rival name").fill("Blue");
  await expect(page.getByText("Name is available!")).toBeVisible({
    timeout: 15_000,
  });

  await page
    .getByText("Choose Your Faction")
    .locator("..")
    .getByRole("button")
    .first()
    .click();
  await page.getByRole("button", { name: "Next Step: Class" }).click();

  await expect(page.getByText("Choose Your Class")).toBeVisible();
  await page
    .getByText("Choose Your Class")
    .locator("..")
    .getByRole("button")
    .first()
    .click();
  await page.getByRole("button", { name: "Next Step: Home City" }).click();

  await expect(page.getByText("Choose Your Home City")).toBeVisible();
  await page.getByRole("button", { name: "Next Step: Confirm" }).click();

  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByRole("button", { name: characterName })).toBeVisible();
}

async function enterWorld(page: Page, characterName: string) {
  await page.getByRole("button", { name: characterName }).click();
  const enterWorldButton = page.getByRole("button", { name: "ENTER WORLD" });
  await expect(enterWorldButton).toBeEnabled({ timeout: 15_000 });
  await enterWorldButton.click();

  await expect(page.getByTestId("game-screen")).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByRole("button", { name: "View Map" })).toBeVisible();
  await expect(page.getByText("Welcome to CaptureQuest!")).toBeVisible();
}

async function quitToCharacterSelect(page: Page) {
  await page.getByRole("button", { name: "Quit" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({
    timeout: 15_000,
  });
}

test("guest can create a character and enter the world through the real UI", async ({
  page,
}) => {
  test.setTimeout(90_000);

  const characterName = uniqueTrainerName();
  await loginAsGuest(page, randomUUID());
  await createCharacter(page, characterName);
  await enterWorld(page, characterName);
  await quitToCharacterSelect(page);
});
