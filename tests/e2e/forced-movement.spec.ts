import { expect, test } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

test("ordinary walking onto an imported arrow starts its server-owned route", async ({ page }) => {
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_spin_route_ready");
  await waitForNoMapLoading(page);
  await waitForPlayerTile(page, 5, 9);
  const before = errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest).length;
  await pressMovement(page, "left");
  await waitForPlayerTile(page, 2, 9);
  await waitForPlayerIdle(page);
  // Only the user step is acknowledged; the two source-data points belong to
  // the server timer and cannot become independent client completion effects.
  expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest)).toHaveLength(before + 1);
  expect((await getGameState(page)).map.id).toBe(200);
  errors.assertNoSevereErrors();
});
