import { expect, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";

const guestTokenKey = "capturequest_guest_token";
const safeNameAlphabet = "lmnpqrstvwxyz";

const tokenToLetters = (token: string) =>
  token
    .replace(/-/g, "")
    .slice(0, 10)
    .split("")
    .map((char) => {
      const value = Number.parseInt(char, 16);
      return safeNameAlphabet[value % safeNameAlphabet.length] ?? "m";
    })
    .join("");

export const uniqueTrainerName = () =>
  `Qa${tokenToLetters(randomUUID())}`.slice(0, 12);

export async function loginAsGuest(page: Page, guestToken = randomUUID()) {
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

export async function createCharacter(page: Page, characterName: string) {
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

export async function enterWorld(page: Page, characterName: string) {
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

export async function createGuestCharacterAndEnterWorld(page: Page) {
  const characterName = uniqueTrainerName();
  await loginAsGuest(page);
  await createCharacter(page, characterName);
  await enterWorld(page, characterName);
  return characterName;
}

export async function quitToCharacterSelect(page: Page) {
  await page.getByRole("button", { name: "Quit" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({
    timeout: 15_000,
  });
}
