import { expect, test, type Locator, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

async function fullyInViewport(page: Page, locator: Locator) {
  // Visibility alone permits a clipped element. Check both ancestor clipping
  // through IntersectionObserver and every edge against the actual viewport.
  await expect(locator).toBeInViewport({ ratio: 1 });
  const box = await locator.boundingBox();
  const viewport = page.viewportSize();
  expect(box).not.toBeNull();
  expect(viewport).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(viewport!.width);
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport!.height);
}

async function wheelCabinet(page: Page, distance: number) {
  const box = await page.getByTestId("slot-machine-overlay").boundingBox();
  expect(box).not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.wheel(0, distance);
}

test("slot cabinet keeps its balance and controls reachable through desktop and phone resizing", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await page.setViewportSize({ width: 1280, height: 720 });
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_game_corner_slots_ready");
  await waitForPlayerTile(page, 17, 15);
  await waitForPlayerIdle(page);
  await clickTile(page, 18, 15);
  const cabinet = page.getByTestId("slot-machine");
  const overlay = page.getByTestId("slot-machine-overlay");
  const spin = page.getByTestId("slot-machine-spin");
  const quit = page.getByTestId("slot-machine-quit");
  await expect(cabinet).toBeVisible();
  const measurements: unknown[] = [];

  for (const viewport of [
    { width: 1280, height: 720 },
    { width: 390, height: 844 },
    { width: 844, height: 390 },
    { width: 1280, height: 900 },
  ]) {
    await page.setViewportSize(viewport);
    await waitForNoMapLoading(page);
    // A resize may retire a source view if its scene is replaced. Reopen only
    // by interacting with the real adjacent machine, never a test-store setter.
    if (!(await cabinet.isVisible())) {
      await waitForPlayerTile(page, 17, 15);
      await waitForPlayerIdle(page);
      await clickTile(page, 18, 15);
    }
    await expect(cabinet).toBeVisible();
    await expect(page.getByTestId("slot-machine-coins")).toHaveText("COINS: 10");
    await wheelCabinet(page, -10_000);
    await expect.poll(() => overlay.evaluate(element => element.scrollTop)).toBe(0);
    await fullyInViewport(page, cabinet.getByRole("heading", { name: "GAME CORNER", exact: true }));
    await fullyInViewport(page, page.getByTestId("slot-machine-coins"));
    await fullyInViewport(page, page.getByTestId("slot-machine-payout"));
    await fullyInViewport(page, cabinet.getByTitle("Close (Esc)"));
    const top = await cabinet.boundingBox();
    expect(top!.x).toBeGreaterThanOrEqual(0);
    expect(top!.x + top!.width).toBeLessThanOrEqual(viewport.width);
    const name = `${viewport.width}x${viewport.height}`;
    await page.screenshot({ path: testInfo.outputPath(`slot-${name}-top.png`) });

    // Scroll through the actual wheel input when the controls need more room.
    const controlsBox = await quit.boundingBox();
    const frame = await overlay.boundingBox();
    const needed = controlsBox!.y + controlsBox!.height - (frame!.y + frame!.height - 16);
    if (needed > 0) await wheelCabinet(page, needed + 16);
    await fullyInViewport(page, spin);
    await fullyInViewport(page, quit);
    await expect(spin).toBeEnabled();
    await expect(quit).toBeEnabled();
    if (viewport.width === 1280 && viewport.height === 900) {
      await fullyInViewport(page, cabinet.getByTitle("Pull to spin!"));
    }
    measurements.push({
      viewport,
      cabinet: await cabinet.boundingBox(),
      coins: await page.getByTestId("slot-machine-coins").boundingBox(),
      spin: await spin.boundingBox(),
      quit: await quit.boundingBox(),
      scroll: await overlay.evaluate(element => ({ top: element.scrollTop, height: element.clientHeight, content: element.scrollHeight })),
    });
    await page.screenshot({ path: testInfo.outputPath(`slot-${name}-controls.png`) });
  }

  // The final reachable controls still operate the ordinary game interaction.
  await spin.click();
  await expect(spin).toBeDisabled();
  await expect(spin).toBeEnabled({ timeout: 15_000 });
  const coins = (await getGameState(page)).inventory.coins;
  await expect(page.getByTestId("slot-machine-coins")).toHaveText(`COINS: ${coins}`);
  await quit.click();
  await expect(cabinet).toBeHidden();
  await testInfo.attach("slot-viewport-measurements", { body: JSON.stringify(measurements, null, 2), contentType: "application/json" });
  const openMenu = page.getByRole("button", { name: "Open game menu", exact: true });
  if (await openMenu.isVisible()) await openMenu.click();
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
