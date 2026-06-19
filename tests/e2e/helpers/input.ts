import type { Page } from "@playwright/test";
import { expect } from "@playwright/test";
import { getGameState, tileToViewport } from "./state";

const movementKeys = {
  up: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  right: "ArrowRight",
} as const;

export type MovementDirection = keyof typeof movementKeys;

async function blurActiveElement(page: Page) {
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
  });
}

export async function pressMovement(
  page: Page,
  direction: MovementDirection,
  count = 1,
) {
  for (let i = 0; i < count; i += 1) {
    await blurActiveElement(page);
    await page.keyboard.down(movementKeys[direction]);
    await page.waitForTimeout(35);
    await page.keyboard.up(movementKeys[direction]);
    await page.waitForTimeout(60);
  }
}

export async function pressSpace(page: Page, count = 1) {
  for (let i = 0; i < count; i += 1) {
    await blurActiveElement(page);
    await page.keyboard.down("Space");
    await page.waitForTimeout(90);
    await page.keyboard.up("Space");
    await page.waitForTimeout(40);
  }
}

export async function dismissDialogue(page: Page, maxPresses = 8) {
  for (let i = 0; i < maxPresses; i += 1) {
    const state = await getGameState(page);
    if (!state.dialogue.isOpen) return;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen;
      },
      { timeout: 5_000 },
    )
    .toBe(false);
}

export async function waitForDialogueText(
  page: Page,
  matcher: string | RegExp,
  maxPresses = 12,
) {
  for (let i = 0; i < maxPresses; i += 1) {
    const state = await getGameState(page);
    const text = state.dialogue.text;
    const matches =
      typeof matcher === "string" ? text.includes(matcher) : matcher.test(text);
    if (state.dialogue.isOpen && matches) return;

    await pressSpace(page);
    await page.waitForTimeout(150);
  }

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        const text = state.dialogue.text;
        return typeof matcher === "string" ? text.includes(matcher) : matcher.test(text);
      },
      { timeout: 5_000 },
    )
    .toBe(true);
}

export async function clickTile(page: Page, x: number, y: number) {
  const point = await tileToViewport(page, x, y);
  await page.mouse.click(point.x, point.y);
}
