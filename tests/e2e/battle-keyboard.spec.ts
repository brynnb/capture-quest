import { expect, test } from "@playwright/test";
import {
  advanceBattleTextToPhase,
  endWildBattleIfOpen,
  waitForBattleOpen,
  waitForBattlePhase,
} from "./helpers/battle";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState } from "./helpers/state";

test("battle action and move menus can be driven with keyboard input", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_wild");
  await waitForBattleOpen(page);
  await advanceBattleTextToPhase(page, "action_select");

  await expect(page.getByTestId("battle-action-menu")).toBeVisible();
  await expect(page.getByTestId("battle-action-fight")).toHaveAttribute(
    "data-selected",
    "true",
  );

  await page.keyboard.press("ArrowRight");
  await expect(page.getByTestId("battle-action-item")).toHaveAttribute(
    "data-selected",
    "true",
  );
  expect((await getGameState(page)).battle.selectedAction).toBe("item");

  await page.keyboard.press("ArrowDown");
  await expect(page.getByTestId("battle-action-run")).toHaveAttribute(
    "data-selected",
    "true",
  );
  expect((await getGameState(page)).battle.selectedAction).toBe("run");

  await page.keyboard.press("ArrowLeft");
  await expect(page.getByTestId("battle-action-switch")).toHaveAttribute(
    "data-selected",
    "true",
  );
  expect((await getGameState(page)).battle.selectedAction).toBe("switch");

  await page.keyboard.press("ArrowUp");
  await expect(page.getByTestId("battle-action-fight")).toHaveAttribute(
    "data-selected",
    "true",
  );
  expect((await getGameState(page)).battle.selectedAction).toBe("fight");

  await pressSpace(page);
  await waitForBattlePhase(page, "move_select");
  await expect(page.getByTestId("battle-move-menu")).toBeVisible();

  const moveButtons = page.locator(
    'button[data-testid^="battle-move-"]:not([data-testid="battle-move-back"])',
  );
  await expect(moveButtons.first()).toBeVisible();
  await expect(page.getByTestId("battle-move-0")).toHaveAttribute(
    "data-selected",
    "true",
  );

  if ((await moveButtons.count()) > 1) {
    await page.keyboard.press("ArrowRight");
    await expect(page.getByTestId("battle-move-1")).toHaveAttribute(
      "data-selected",
      "true",
    );
    await page.keyboard.press("ArrowLeft");
    await expect(page.getByTestId("battle-move-0")).toHaveAttribute(
      "data-selected",
      "true",
    );
  }

  await pressSpace(page);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.battle.turnNumber;
      },
      { timeout: 30_000 },
    )
    .toBeGreaterThan(0);

  await endWildBattleIfOpen(page);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
