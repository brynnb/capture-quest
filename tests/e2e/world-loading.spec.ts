import { expect, test, type Page } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createCharacter, loginAsGuest, quitToCharacterSelect, uniqueTrainerName } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle } from "./helpers/state";

const terrainPattern = "**/phaser/tile_images/**";

function isMapTexture(url: string) {
  const match = new URL(url).pathname.match(/\/tile_(\d+)\.png$/);
  // TileViewer preloads 1..10 before its scene/test bridge exists. Hold an
  // actual map texture so this test also observes the owned map-input guard.
  return match !== null && Number(match[1]) > 10;
}

async function startWorldEntry(page: Page, name: string) {
  await page.getByRole("button", { name, exact: true }).click();
  const enter = page.getByRole("button", { name: /Enter World/i }).filter({ visible: true });
  await expect(enter).toBeEnabled();
  await enter.click();
}

async function watchLoadingContinuity(page: Page) {
  await page.evaluate(() => {
    const witness = { active: true, gameFrames: 0, uncoveredFrames: 0 };
    (window as any).__worldLoadingWitness = witness;
    const frame = () => {
      if (!witness.active) return;
      const game = document.querySelector('[data-testid="game-screen"]');
      const canvas = document.querySelector("#phaser-game-container canvas");
      const bounds = canvas?.getBoundingClientRect();
      // game-screen uses display:contents; its canvas owns the visible box.
      if (game && bounds && bounds.width > 0 && bounds.height > 0) {
        witness.gameFrames++;
        const overlay = document.querySelector('[data-testid="world-loading-screen"]');
        if (!overlay || getComputedStyle(overlay).display === "none" || overlay.getBoundingClientRect().height === 0) {
          witness.uncoveredFrames++;
        }
      }
      requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  });
}

async function finishLoadingWitness(page: Page) {
  return page.evaluate(() => {
    const witness = (window as any).__worldLoadingWitness;
    witness.active = false;
    return { gameFrames: witness.gameFrames, uncoveredFrames: witness.uncoveredFrames };
  });
}

async function expectOpaqueWorldCover(page: Page) {
  const overlay = page.getByTestId("world-loading-screen");
  await expect(overlay).toBeVisible();
  const cover = await overlay.evaluate(element => {
    const box = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    const color = style.backgroundColor.match(/[\d.]+/g)?.map(Number) ?? [];
    const alpha = color.length === 4 ? color[3] : color.length === 3 ? 1 : 0;
    const game = document.querySelector("#main")!.getBoundingClientRect();
    // The decorative desktop frame sits outside the playable main region.
    const points = [
      [game.left + 4, game.top + 4], [game.right - 5, game.top + 4],
      [game.left + 4, game.bottom - 5], [game.right - 5, game.bottom - 5],
      [game.left + game.width / 2, game.top + game.height / 2],
    ];
    return {
      coversGame: box.left <= game.left + 1 && box.top <= game.top + 1
        && box.right >= game.right - 1 && box.bottom >= game.bottom - 1,
      opaque: alpha === 1 && Number(style.opacity) === 1,
      receivesPointer: points.every(([x, y]) => {
        const target = document.elementFromPoint(x, y);
        return target === element || element.contains(target);
      }),
    };
  });
  expect(cover).toEqual({ coversGame: true, opaque: true, receivesPointer: true });
}

async function attemptMovementWhileLoading(page: Page, sentOpcodes: number[]) {
  const movement = () => sentOpcodes.filter(opcode =>
    opcode === OpCodes.PlayerStepRequest || opcode === OpCodes.PlayerStepCompleteRequest,
  ).length;
  const before = movement();
  const initial = await getGameState(page);
  expect(initial.map.isLoading).toBe(true);
  expect(initial.worldInput.frozen).toBe(true);
  // Exercise the real pointer and keyboard handlers while their visible world
  // is covered, instead of calling the controller directly.
  const viewport = page.viewportSize()!;
  await page.mouse.click(viewport.width / 2, viewport.height / 2);
  await page.keyboard.down("d");
  await page.waitForTimeout(250);
  await page.keyboard.up("d");
  expect(movement()).toBe(before);
  const after = await getGameState(page);
  expect(after.worldInput.frozen).toBe(true);
  expect({ x: after.player.x, y: after.player.y }).toEqual({
    x: initial.player.x, y: initial.player.y,
  });
}

async function expectRenderedTerrain(page: Page) {
  await expect(page.getByTestId("world-loading-screen")).toBeHidden({ timeout: 30_000 });
  await waitForNoMapLoading(page);
  await waitForPlayerIdle(page);
  await expect.poll(async () => {
    // A screenshot samples the real composed WebGL frame. Decode it with the
    // browser's image API so this assertion needs no external imaging package.
    const screenshot = await page.locator("canvas").first().screenshot();
    return page.evaluate(async base64 => {
      const image = new Image();
      image.src = "data:image/png;base64," + base64;
      await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = image.width;
      canvas.height = image.height;
      const context = canvas.getContext("2d")!;
      context.drawImage(image, 0, 0);
      // Keep away from the canvas loading label and HUD. A few character
      // sprites over black cannot fill this central region with terrain.
      const x = Math.floor(canvas.width * 0.30);
      const y = Math.floor(canvas.height * 0.20);
      const width = Math.max(1, Math.floor(canvas.width * 0.40));
      const height = Math.max(1, Math.floor(canvas.height * 0.35));
      const pixels = context.getImageData(x, y, width, height).data;
      let terrainPixels = 0;
      for (let index = 0; index < pixels.length; index += 4) {
        if (Math.max(pixels[index], pixels[index + 1], pixels[index + 2]) > 30
          && pixels[index] + pixels[index + 1] + pixels[index + 2] > 90) terrainPixels++;
      }
      return terrainPixels / (width * height);
    }, screenshot.toString("base64"));
  }, {
    timeout: 15_000,
    intervals: [100, 250, 500],
    message: "The uncovered game must contain rendered terrain, not characters over a black map",
  }).toBeGreaterThan(0.05);
}

test("world entry stays covered and input stays frozen until its terrain is ready", async ({ page }) => {
  test.setTimeout(90_000);
  const errors = collectPageErrors(page);
  let release!: () => void;
  const terrainGate = new Promise<void>(resolve => { release = resolve; });
  const held = new Set<string>();
  await page.route(terrainPattern, async route => {
    if (isMapTexture(route.request().url())) {
      held.add(route.request().url());
      await terrainGate;
    }
    await route.continue();
  });
  const name = uniqueTrainerName();
  await loginAsGuest(page);
  await createCharacter(page, name);
  await watchLoadingContinuity(page);
  try {
    await startWorldEntry(page, name);
    await expect.poll(() => held.size, { timeout: 20_000 }).toBeGreaterThan(0);
    const overlay = page.getByTestId("world-loading-screen");
    await expect(overlay.getByRole("heading", { name: "Loading the world" })).toBeVisible();
    await expect(overlay).toHaveAttribute("aria-busy", "true");
    await expect(overlay.getByRole("progressbar", { name: "Loading progress" })).toBeVisible();
    await expectOpaqueWorldCover(page);
    await attemptMovementWhileLoading(page, errors.sentOpcodes);
    await page.screenshot({ path: test.info().outputPath("world-loading-desktop.png") });

    await page.setViewportSize({ width: 390, height: 844 });
    await expectOpaqueWorldCover(page);
    await expect(overlay.getByRole("heading", { name: "Loading the world" })).toBeInViewport();
    await page.screenshot({ path: test.info().outputPath("world-loading-mobile.png") });
    await page.setViewportSize({ width: 1280, height: 720 });

    const witness = await finishLoadingWitness(page);
    expect(witness.gameFrames).toBeGreaterThan(0);
    expect(witness.uncoveredFrames, "Entry ACK must not expose a black world before map initialization").toBe(0);
    release();
    await expectRenderedTerrain(page);
    await page.screenshot({ path: test.info().outputPath("world-loaded.png") });
    // The mobile viewport deliberately collapses the menu; restoring desktop
    // width preserves that user-facing state until the menu is opened again.
    const openMenu = page.getByRole("button", { name: "Open game menu", exact: true });
    if (await openMenu.isVisible()) await openMenu.click();
    await quitToCharacterSelect(page);
    errors.assertNoSevereErrors();
  } finally {
    release();
  }
});

test("a failed terrain texture keeps a retryable cover and retry renders the world", async ({ page }) => {
  test.setTimeout(90_000);
  const errors = collectPageErrors(page);
  let failedUrl: string | undefined;
  await page.route(terrainPattern, async route => {
    if (!failedUrl && isMapTexture(route.request().url())) {
      failedUrl = route.request().url();
      await route.fulfill({ status: 503, contentType: "image/png", body: "Injected terrain failure" });
      return;
    }
    await route.continue();
  });
  const name = uniqueTrainerName();
  await loginAsGuest(page);
  await createCharacter(page, name);
  await startWorldEntry(page, name);
  await expect.poll(() => Boolean(failedUrl), { timeout: 20_000 }).toBe(true);
  const overlay = page.getByTestId("world-loading-screen");
  await expect(overlay.getByRole("heading", { name: "Loading interrupted" })).toBeVisible({ timeout: 20_000 });
  await expect(overlay).toHaveAttribute("aria-busy", "false");
  await expect(overlay.getByText("The world could not be loaded. Please try again.", { exact: true })).toBeVisible();
  await expectOpaqueWorldCover(page);
  await attemptMovementWhileLoading(page, errors.sentOpcodes);
  await page.screenshot({ path: test.info().outputPath("world-loading-retry.png") });
  await overlay.getByRole("button", { name: "Retry", exact: true }).click();
  await expectRenderedTerrain(page);
  await page.screenshot({ path: test.info().outputPath("world-loading-recovered.png") });
  await quitToCharacterSelect(page);

  expect(errors.pageErrors).toEqual([]);
  expect(errors.retiredPositionPackets).toEqual([]);
  expect(errors.networkErrors).toEqual(["503 " + failedUrl]);
  expect(errors.consoleErrors.filter(message => !message.startsWith("Error loading map data:"))).toEqual([]);
});
