import { devices, expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  getGameState,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerTile,
} from "./helpers/state";

test.use({ ...devices["Pixel 5"] });

test("mobile nipple joystick maps analog input to cardinal tile movement", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page, { forceClicks: true });
  await waitForMap(page, "REDS_HOUSE_2F");
  await waitForNoMapLoading(page);
  await waitForPlayerTile(page, 3, 6);

  const joystick = page.getByRole("application", {
    name: "Movement joystick",
  });
  await expect(joystick).toBeVisible();

  const zoneBox = await joystick.boundingBox();
  const backBox = await joystick.locator(".back").boundingBox();
  const frontBox = await joystick.locator(".front").boundingBox();
  expect(zoneBox).toMatchObject({ width: 132, height: 132 });
  expect(backBox).toMatchObject({ width: 112, height: 112 });
  expect(frontBox).toMatchObject({ width: 56, height: 56 });

  const center = {
    x: zoneBox!.x + zoneBox!.width / 2,
    y: zoneBox!.y + zoneBox!.height / 2,
  };
  await page.mouse.move(center.x, center.y);
  await page.mouse.down();
  await page.mouse.move(center.x + 44, center.y, { steps: 3 });
  await page.mouse.up();

  await waitForPlayerTile(page, 4, 6);
  await page.waitForTimeout(250);
  expect((await getGameState(page)).player).toMatchObject({ x: 4, y: 6 });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
