import { expect, type Page } from "@playwright/test";
import { pressSpace } from "./input";
import { getGameState } from "./state";

export async function waitForBattleOpen(page: Page) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.battle.isOpen;
      },
      { timeout: 30_000 },
    )
    .toBe(true);
}

export async function waitForBattleClosed(page: Page) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.battle.isOpen;
      },
      { timeout: 30_000 },
    )
    .toBe(false);
}

export async function waitForBattlePhase(page: Page, phase: string) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.battle.phase;
      },
      { timeout: 30_000 },
    )
    .toBe(phase);
}

export async function advanceBattleTextToPhase(
  page: Page,
  targetPhase: string,
  maxPresses = 16,
) {
  for (let i = 0; i < maxPresses; i += 1) {
    const state = await getGameState(page);
    if (!state.battle.isOpen) break;
    if (state.battle.phase === targetPhase) return;
    if (state.battle.phase !== "animating" && state.battle.phase !== "battle_end") {
      break;
    }
    await pressSpace(page);
    await page.waitForTimeout(100);
  }

  await waitForBattlePhase(page, targetPhase);
}

export async function endWildBattleIfOpen(page: Page) {
  const state = await getGameState(page);
  if (!state.battle.isOpen) return;

  if (state.battle.phase === "animating") {
    await advanceBattleTextToPhase(page, "action_select", 24).catch(() => undefined);
  }

  const current = await getGameState(page);
  if (!current.battle.isOpen) return;
  if (current.battle.phase === "battle_end") {
    await pressSpace(page);
    await waitForBattleClosed(page);
    return;
  }
  if (current.battle.phase !== "action_select") return;

  await expect(page.getByTestId("battle-action-fight")).toHaveAttribute(
    "data-selected",
    "true",
  );
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("ArrowRight");
  await expect(page.getByTestId("battle-action-run")).toHaveAttribute(
    "data-selected",
    "true",
  );
  await pressSpace(page);

  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.battle.phase;
      },
      { timeout: 30_000 },
    )
    .toMatch(/animating|battle_end|none/);

  for (let i = 0; i < 12; i += 1) {
    const next = await getGameState(page);
    if (!next.battle.isOpen) return;
    await pressSpace(page);
    await page.waitForTimeout(100);
  }
}
