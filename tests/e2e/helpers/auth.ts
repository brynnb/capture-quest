import { expect, type Locator, type Page } from "@playwright/test";
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
  await expect(
    page.getByRole("heading", { name: "SELECT A CHARACTER" }),
  ).toBeVisible({
    timeout: 30_000,
  });
}

interface CharacterCreationOptions {
  forceClicks?: boolean;
}

export async function createCharacter(
  page: Page,
  characterName: string,
  options: CharacterCreationOptions = {},
) {
  const click = async (locator: Locator) => {
    if (options.forceClicks) {
      await locator.evaluate((element: HTMLElement) => element.click());
      return;
    }
    await locator.click();
  };
  const nextCreationStep = () =>
    page
      .getByTestId("character-creation-navigation")
      .getByRole("button", { name: /^Next(?: Step: .+)?$/ });

  await click(
    page.getByRole("button", { name: "CREATE NEW CHARACTER" }).first(),
  );

  const factionTitle = page
    .getByText("Choose Your Faction")
    .filter({ visible: true });
  await expect(factionTitle).toBeVisible();
  await page.getByPlaceholder("Enter character name").fill(characterName);
  await page.getByPlaceholder("Enter rival name").fill("Blue");
  await expect(page.getByText("Name is available!")).toBeVisible({
    timeout: 15_000,
  });

  await click(factionTitle.locator("..").getByRole("button").first());
  await click(nextCreationStep());

  const classTitle = page
    .getByText("Choose Your Class")
    .filter({ visible: true });
  await expect(classTitle).toBeVisible();
  await click(classTitle.locator("..").getByRole("button").first());
  await click(nextCreationStep());

  await expect(
    page.getByText("Choose Your Home City").filter({ visible: true }),
  ).toBeVisible();
  await click(nextCreationStep());

  await click(page.getByRole("button", { name: "Create" }));
  await expect(
    page.getByRole("heading", { name: "SELECT A CHARACTER" }),
  ).toBeVisible({
    timeout: 30_000,
  });
  await expect(
    page.getByText(characterName, { exact: true }).filter({ visible: true }),
  ).toBeVisible();
}

export async function enterWorld(
  page: Page,
  characterName: string,
  options: CharacterCreationOptions = {},
) {
  const mobilePreview = page.getByAltText(`${characterName} trainer preview`);
  if (!(await mobilePreview.isVisible())) {
    const characterButton = page.getByRole("button", { name: characterName });
    if (options.forceClicks) {
      await characterButton.evaluate((element: HTMLElement) => element.click());
    } else {
      await characterButton.click();
    }
  }
  const enterWorldButton = page
    .getByRole("button", { name: /Enter World/i })
    .filter({ visible: true });
  await expect(enterWorldButton).toBeEnabled({ timeout: 15_000 });
  if (options.forceClicks) {
    await enterWorldButton.evaluate((element: HTMLElement) => element.click());
  } else {
    await enterWorldButton.click();
  }

  await expect(page.getByTestId("game-screen")).toBeVisible({
    timeout: 30_000,
  });
  await expect(
    page.getByRole("button", {
      name: options.forceClicks ? "Open game menu" : "View Map",
    }),
  ).toBeVisible();
  if (!options.forceClicks) {
    await expect(page.getByText("Welcome to CaptureQuest!")).toBeVisible();
  }
}

export async function createGuestCharacterAndEnterWorld(
  page: Page,
  options: CharacterCreationOptions = {},
) {
  const characterName = uniqueTrainerName();
  await loginAsGuest(page);
  await createCharacter(page, characterName, options);
  await enterWorld(page, characterName, options);
  return characterName;
}

export async function quitToCharacterSelect(page: Page) {
  await page.getByRole("button", { name: "Quit" }).click();
  await expect(
    page.getByRole("heading", { name: "SELECT A CHARACTER" }),
  ).toBeVisible({
    timeout: 15_000,
  });
}
