import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { waitForNoMapLoading } from "./helpers/state";

test("desired trainer preference persists through fresh character entry", async ({ page }) => {
  const errors = collectPageErrors(page);
  const character = await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);
  await page.getByRole("button", { name: "Options", exact: true }).click();
  const card = page.getByText("Trainer Re-battles", { exact: true }).locator("..");
  await card.getByRole("button", { name: "OFF", exact: true }).click();
  await expect(card.getByRole("button", { name: "ON", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Options", exact: true }).click();
  await quitToCharacterSelect(page); await enterWorld(page, character);
  await waitForNoMapLoading(page);
  await page.getByRole("button", { name: "Options", exact: true }).click();
  await expect(card.getByRole("button", { name: "ON", exact: true })).toBeVisible();
  errors.assertNoSevereErrors();
});
