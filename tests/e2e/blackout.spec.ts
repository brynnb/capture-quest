import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { waitForMap, waitForPlayerTile } from "./helpers/state";

test("battle-start blackout presents its committed recovery without an open battle panel", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_blackout_empty_party");
  // Native map 51 is VIRIDIAN_FOREST in the authoritative runtime catalog.
  await page.evaluate(async () => {
    const bridgePath = "/src/net/NetworkBridge.ts";
    const opcodePath = "/src/net/generated/opcodes.ts";
    const { NetworkBridge } = await import(bridgePath);
    const opcodes = await import(opcodePath);
    NetworkBridge.send({ mapId: 51 }, opcodes.PokeBattleStartRequest);
  });
  await waitForMap(page, "VIRIDIAN_POKECENTER");
  await waitForPlayerTile(page, 3, 4);
  await page.screenshot({ path: test.info().outputPath("committed-blackout.png") });
  errors.assertNoSevereErrors();
});

test("test warp probes await the explicit command and preserve position on rejection", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await page.evaluate(() => window.__capturequestTest!.warpToMap(37, 6, 7, "LEFT"));
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForPlayerTile(page, 6, 7);
  await expect(page.evaluate(() => window.__capturequestTest!.warpToMap(37, 300, 300)))
    .rejects.toThrow("Could not warp to that tile");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForPlayerTile(page, 6, 7);
  errors.assertNoSevereErrors();
});
