import { expect, type Page } from "@playwright/test";
import type {
  CaptureQuestTestActor,
  CaptureQuestTestState,
} from "../../../src/testing/capturequestTestBridge";

type MapMatcher = number | string | RegExp;

export async function waitForTestBridge(page: Page) {
  await expect
    .poll(
      async () =>
        page.evaluate(() => Boolean(window.__capturequestTest?.getState)),
      { timeout: 30_000 },
    )
    .toBe(true);
}

export async function getGameState(page: Page): Promise<CaptureQuestTestState> {
  await waitForTestBridge(page);
  return page.evaluate(() => {
    const bridge = window.__capturequestTest;
    if (!bridge) {
      throw new Error("CaptureQuest test bridge is not available");
    }
    return bridge.getState();
  });
}

export async function waitForMap(page: Page, map: MapMatcher) {
  if (typeof map === "number") {
    await expect
      .poll(
        async () => {
          const state = await getGameState(page);
          return state.map.id;
        },
        { timeout: 30_000 },
      )
      .toBe(map);
    return;
  }

  if (typeof map === "string") {
    await expect
      .poll(
        async () => {
          const state = await getGameState(page);
          return state.map.name ?? "";
        },
        { timeout: 30_000 },
      )
      .toBe(map);
    return;
  }

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.map.name ?? "";
      },
      { timeout: 30_000 },
    )
    .toMatch(map);
}

export async function waitForPlayerTile(
  page: Page,
  x: number,
  y: number,
  timeout = 20_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return `${state.player.x},${state.player.y}`;
      },
      { timeout },
    )
    .toBe(`${x},${y}`);
}

export async function waitForPlayerIdle(page: Page, timeout = 20_000) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.player.isMoving || state.worldInput.frozen;
      },
      { timeout },
    )
    .toBe(false);
}

export async function waitForMapChange(
  page: Page,
  previousMapId: number | null,
  timeout = 30_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.map.id;
      },
      { timeout },
    )
    .not.toBe(previousMapId);

  await waitForNoMapLoading(page);
}

export async function waitForActorVisible(
  page: Page,
  predicate: (actor: CaptureQuestTestActor) => boolean,
  timeout = 20_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.visibleActors.some(predicate);
      },
      { timeout },
    )
    .toBe(true);
}

export async function waitForActorAbsent(
  page: Page,
  predicate: (actor: CaptureQuestTestActor) => boolean,
  timeout = 20_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.visibleActors.some(predicate);
      },
      { timeout },
    )
    .toBe(false);
}

export async function waitForNoMapLoading(page: Page) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.map.isLoading;
      },
      { timeout: 30_000 },
    )
    .toBe(false);
}

export async function waitForWarps(page: Page, minimumCount = 1) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.warps.length;
      },
      { timeout: 30_000 },
    )
    .toBeGreaterThanOrEqual(minimumCount);
}

export async function waitForMessage(
  page: Page,
  matcher: string | RegExp,
  timeout = 15_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.messages.some((message) =>
          typeof matcher === "string"
            ? message.text.includes(matcher)
            : matcher.test(message.text),
        );
      },
      { timeout },
    )
    .toBe(true);
}

export async function waitForInventoryItem(
  page: Page,
  matcher: string | number | RegExp,
  timeout = 15_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.inventory.items.some((item) => {
          if (typeof matcher === "number") return item.itemId === matcher;
          if (typeof matcher === "string") {
            return item.name === matcher || item.shortName === matcher;
          }
          return matcher.test(item.name) || matcher.test(item.shortName);
        });
      },
      { timeout },
    )
    .toBe(true);
}

export async function waitForPartyPokemon(
  page: Page,
  matcher: string | number | RegExp,
  timeout = 15_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.pokemon.party.some((pokemon) => {
          if (typeof matcher === "number") return pokemon.id === matcher;
          return typeof matcher === "string"
            ? pokemon.name === matcher
            : matcher.test(pokemon.name);
        });
      },
      { timeout },
    )
    .toBe(true);
}

export async function waitForPCPokemon(
  page: Page,
  matcher: string | number | RegExp,
  timeout = 15_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.pokemon.pc.boxPokemon.some((pokemon) => {
          if (typeof matcher === "number") return pokemon.id === matcher;
          return typeof matcher === "string"
            ? pokemon.name === matcher
            : matcher.test(pokemon.name);
        });
      },
      { timeout },
    )
    .toBe(true);
}

export async function waitForInventoryOpen(page: Page, open = true) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.ui.isInventoryOpen;
      },
      { timeout: 10_000 },
    )
    .toBe(open);
}

export async function waitForWarpMode(page: Page, enabled: boolean) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.ui.isWarpMode;
      },
      { timeout: 10_000 },
    )
    .toBe(enabled);
}

export async function waitForAudioRequest(
  page: Page,
  matcher: string | RegExp,
  timeout = 20_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        const requested = state.audio.requestedMusicTrack ?? "";
        return typeof matcher === "string"
          ? requested.includes(matcher)
          : matcher.test(requested);
      },
      { timeout },
    )
    .toBe(true);
}

export async function waitForLastSFX(
  page: Page,
  matcher: string | RegExp,
  timeout = 15_000,
) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        const candidates = [
          state.audio.lastSFXTrack,
          state.audio.lastGeneratedSFXName,
        ].filter((value): value is string => Boolean(value));
        return candidates.some((last) =>
          typeof matcher === "string"
            ? last.includes(matcher)
            : matcher.test(last),
        );
      },
      { timeout },
    )
    .toBe(true);
}

export async function tileToViewport(page: Page, x: number, y: number) {
  await waitForTestBridge(page);
  const point = await page.evaluate(
    ([tileX, tileY]) => {
      const bridge = window.__capturequestTest;
      if (!bridge) return null;
      return bridge.tileToViewport(tileX, tileY);
    },
    [x, y],
  );

  if (!point) {
    throw new Error(`Tile ${x},${y} is not available in viewport diagnostics`);
  }

  return point;
}

export async function centerTileInView(page: Page, x: number, y: number) {
  await waitForTestBridge(page);
  await page.evaluate(
    ([tileX, tileY]) => {
      window.__capturequestTest?.centerTileInView(tileX, tileY);
    },
    [x, y],
  );
}
