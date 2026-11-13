import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle } from "./helpers/state";

test("View Map stays covered until every visible overview chunk is rendered", async ({ page }) => {
  test.setTimeout(90_000);
  const errors = collectPageErrors(page);
  await page.setViewportSize({ width: 1280, height: 720 });
  await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);
  await expect(page.getByTestId("world-loading-screen")).toBeHidden();
  await waitForPlayerIdle(page);
  const original = await getGameState(page);

  // Observe the real renderer's requested visible footprint. This does not
  // change its requests, readiness promise, textures, or scene behavior.
  await page.evaluate(async () => {
    const modulePath = "/src/phaser-game/renderers/OverworldOverviewLayer.ts";
    const { OverworldOverviewLayer } = await import(modulePath);
    const witness: any = { layer: null, required: [] };
    (window as any).__overviewLoadingWitness = witness;
    const sync = OverworldOverviewLayer.prototype.sync;
    OverworldOverviewLayer.prototype.sync = function (...args: any[]) {
      const required = args[3] as ReadonlySet<string> | undefined;
      if (required?.size && witness.required.length === 0) {
        witness.layer = this;
        witness.required = [...required];
      }
      return sync.apply(this, args as any);
    };
  });

  let release!: () => void;
  const heldResponse = new Promise<void>(resolve => { release = resolve; });
  let heldKey: string | undefined;
  const delivered = new Set<string>();
  await page.route("**/api/overworld/overview?**", async route => {
    const url = new URL(route.request().url());
    const key = "overworld:" + url.searchParams.get("chunkX") + ":" + url.searchParams.get("chunkY");
    const required = await page.evaluate(() => (window as any).__overviewLoadingWitness.required as string[]);
    if (!heldKey && required.includes(key)) {
      heldKey = key;
      await heldResponse;
    }
    await route.continue();
  });
  page.on("response", response => {
    const url = new URL(response.url());
    if (url.pathname === "/api/overworld/overview" && response.ok()) {
      delivered.add("overworld:" + url.searchParams.get("chunkX") + ":" + url.searchParams.get("chunkY"));
    }
  });

  try {
    await page.getByRole("button", { name: "View Map", exact: true }).click();
    await expect.poll(() => heldKey, { timeout: 20_000 }).toBeTruthy();
    const footprint = await page.evaluate(() => (window as any).__overviewLoadingWitness.required as string[]);
    expect(footprint.length, "Use a real wide overview with multiple visible chunks").toBeGreaterThan(1);
    expect(footprint).toContain(heldKey);
    await expect.poll(async () => page.evaluate(key => {
      const witness = (window as any).__overviewLoadingWitness;
      return witness.required.filter((candidate: string) => candidate !== key
        && !witness.layer.entries.has(candidate)).length;
    }, heldKey!), { timeout: 10_000 }).toBe(0);
    expect(delivered.size).toBeGreaterThan(0);
    expect(delivered.has(heldKey!)).toBe(false);

    // Give the former first-chunk readiness path time to finish its 200ms fade.
    // The final required response is still deliberately withheld throughout.
    await page.waitForTimeout(350);
    const overlay = page.getByTestId("world-loading-screen");
    await expect(overlay).toBeVisible();
    await expect(overlay.getByRole("heading", { name: "Loading the world" })).toBeVisible();
    await expect(overlay).toHaveAttribute("aria-busy", "true");
    const covered = await overlay.evaluate(element => {
      const game = document.querySelector("#main")!.getBoundingClientRect();
      const box = element.getBoundingClientRect();
      const color = getComputedStyle(element).backgroundColor.match(/[\d.]+/g)?.map(Number) ?? [];
      const alpha = color.length === 4 ? color[3] : color.length === 3 ? 1 : 0;
      const target = document.elementFromPoint(game.left + game.width / 2, game.top + game.height / 2);
      return alpha === 1 && Number(getComputedStyle(element).opacity) === 1
        && box.left <= game.left + 1 && box.top <= game.top + 1
        && box.right >= game.right - 1 && box.bottom >= game.bottom - 1
        && (target === element || element.contains(target));
    });
    expect(covered).toBe(true);
    const loading = await getGameState(page);
    expect(loading.map.isLoading).toBe(true);
    expect(loading.worldInput.frozen).toBe(true);
    await page.screenshot({ path: test.info().outputPath("overworld-last-visible-chunk-loading.png") });

    release();
    await expect(overlay).toBeHidden({ timeout: 20_000 });
    await waitForNoMapLoading(page);
    const loaded = await page.evaluate(() => {
      const witness = (window as any).__overviewLoadingWitness;
      return witness.required.every((key: string) => {
        const entry = witness.layer.entries.get(key);
        return entry?.image.visible && witness.layer.scene.textures.exists(entry.textureKey);
      });
    });
    expect(loaded, "Every visible overview chunk must own a rendered image").toBe(true);
    await expect.poll(async () => {
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
        const width = Math.floor(canvas.width * 0.6);
        const height = Math.floor(canvas.height * 0.5);
        const pixels = context.getImageData(Math.floor(canvas.width * 0.2), Math.floor(canvas.height * 0.15), width, height).data;
        let terrain = 0;
        for (let i = 0; i < pixels.length; i += 4) {
          if (Math.max(pixels[i], pixels[i + 1], pixels[i + 2]) > 30
            && pixels[i] + pixels[i + 1] + pixels[i + 2] > 90) terrain++;
        }
        return terrain / (width * height);
      }, screenshot.toString("base64"));
    }, { timeout: 10_000, message: "The uncovered overview must contain visible terrain pixels" }).toBeGreaterThan(0.02);
    await page.screenshot({ path: test.info().outputPath("overworld-loaded.png") });

    // Desktop uses the same View Map toggle to return to the player.
    await page.getByRole("button", { name: "View Map", exact: true }).click();
    await waitForNoMapLoading(page);
    await expect(overlay).toBeHidden();
    const restored = await getGameState(page);
    expect(restored.map.id).toBe(original.map.id);
    expect({ x: restored.player.x, y: restored.player.y }).toEqual({ x: original.player.x, y: original.player.y });
    await quitToCharacterSelect(page);
    errors.assertNoSevereErrors();
  } finally {
    release();
  }
});
