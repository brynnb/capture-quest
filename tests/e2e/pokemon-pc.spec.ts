import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, dismissDialogue, pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForMap,
  waitForPartyPokemon,
  waitForPlayerIdle,
  waitForPlayerTile,
} from "./helpers/state";

function waitForPartyNames(page: Parameters<typeof getGameState>[0]) {
  return expect.poll(async () => {
    const state = await getGameState(page);
    return state.pokemon.party.map((pokemon) => pokemon.name.toUpperCase());
  });
}

function waitForPCNames(page: Parameters<typeof getGameState>[0]) {
  return expect.poll(async () => {
    const state = await getGameState(page);
    return state.pokemon.pc.boxPokemon.map((pokemon) =>
      pokemon.name.toUpperCase(),
    );
  });
}

async function expectPCOverlayInsideGameFrame(
  page: Parameters<typeof getGameState>[0],
) {
  const boxes = await page.evaluate(() => {
    const main = document.querySelector("#main")?.getBoundingClientRect();
    const overlay = document
      .querySelector('[data-testid="pokemon-pc-overlay"]')
      ?.getBoundingClientRect();
    if (!main || !overlay) return null;
    return {
      main: {
        x: main.x,
        y: main.y,
        width: main.width,
        height: main.height,
      },
      overlay: {
        x: overlay.x,
        y: overlay.y,
        width: overlay.width,
        height: overlay.height,
      },
    };
  });

  expect(boxes).not.toBeNull();
  expect(boxes!.overlay.x).toBeCloseTo(boxes!.main.x, 1);
  expect(boxes!.overlay.y).toBeCloseTo(boxes!.main.y, 1);
  expect(boxes!.overlay.width).toBeCloseTo(boxes!.main.width, 1);
  expect(boxes!.overlay.height).toBeCloseTo(boxes!.main.height, 1);
}

test("Pokemon Center PC opens from the terminal and stores Pokemon", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  const characterName = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_pokemon_center_pc_ready");
  await waitForMap(page, "VIRIDIAN_POKECENTER");
  await waitForPlayerTile(page, 12, 4);
  await waitForPartyPokemon(page, /BULBASAUR/i);
  await waitForPartyPokemon(page, /CHARMANDER/i);

  await clickTile(page, 13, 4);
  await waitForPlayerTile(page, 13, 4);
  await expect(page.getByTestId("pokemon-pc-main-menu")).toBeVisible({
    timeout: 15_000,
  });
  await expectPCOverlayInsideGameFrame(page);
  await expect(page.getByTestId("pokemon-pc-player-pc")).toContainText(
    `${characterName}'s PC`,
  );

  await page.getByTestId("pokemon-pc-player-pc").click();
  await expect(page.getByTestId("pokemon-pc-player-menu")).toBeVisible();
  await expect(page.getByText(`${characterName}'s PC`)).toBeVisible();
  await expect(page.getByTestId("pokemon-pc-player-withdraw")).toBeDisabled();
  await page.getByTestId("pokemon-pc-back").click();

  await page.getByTestId("pokemon-pc-oak-pc").click();
  await expect(page.getByTestId("pokemon-pc-oak-menu")).toBeVisible();
  await expect(page.getByTestId("pokemon-pc-message")).toContainText(
    /rating your POKEDEX/i,
  );
  await page.getByTestId("pokemon-pc-back").click();

  await page.getByTestId("pokemon-pc-bills-pc").click();
  await expect(page.getByTestId("pokemon-pc-storage")).toBeVisible();
  await expect(page.getByTestId("pokemon-pc-current-box")).toContainText(
    "BOX 1",
  );
  await expect(page.getByTestId("pokemon-pc-box-empty")).toBeVisible();

  await page.getByTestId("pokemon-pc-party-slot-0").click();
  await expect(page.getByTestId("pokemon-pc-deposit")).toBeEnabled();
  await page.getByTestId("pokemon-pc-deposit").click();

  await waitForPCNames(page).toContain("BULBASAUR");
  await waitForPartyNames(page).toEqual(["CHARMANDER"]);

  await page.getByTestId("pokemon-pc-box-slot-0").click();
  await expect(page.getByTestId("pokemon-pc-withdraw")).toBeEnabled();
  await page.getByTestId("pokemon-pc-withdraw").click();

  await waitForPartyNames(page).toEqual(["CHARMANDER", "BULBASAUR"]);
  await expect(page.getByTestId("pokemon-pc-box-empty")).toBeVisible({
    timeout: 15_000,
  });

  await page.getByTestId("pokemon-pc-next-box").click();
  await expect(page.getByTestId("pokemon-pc-current-box")).toContainText(
    "BOX 2",
    { timeout: 15_000 },
  );
  await expect(page.getByTestId("pokemon-pc-box-empty")).toBeVisible();
  await page.getByTestId("pokemon-pc-prev-box").click();
  await expect(page.getByTestId("pokemon-pc-current-box")).toContainText(
    "BOX 1",
    { timeout: 15_000 },
  );

  await page.getByTestId("pokemon-pc-close").click();
  await expect(page.getByTestId("pokemon-pc-window")).toBeHidden({
    timeout: 10_000,
  });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("clicking Viridian Pokemon Center NPC 4 does not lock movement", async ({
  page,
}) => {
  test.setTimeout(90_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_pokemon_center_pc_ready");
  await waitForMap(page, "VIRIDIAN_POKECENTER");
  await waitForPlayerTile(page, 12, 4);

  await clickTile(page, 11, 2);
  await waitForPlayerIdle(page, 30_000);

  let state = await getGameState(page);
  if (state.dialogue.isOpen) {
    await dismissDialogue(page);
    await waitForPlayerIdle(page);
    state = await getGameState(page);
  }

  expect(state.worldInput.frozen).toBe(false);
  if (state.player.x == null || state.player.y == null) {
    throw new Error("Player tile was not available after NPC interaction");
  }
  const start = { x: state.player.x, y: state.player.y };
  await pressMovement(page, "down");
  await waitForPlayerTile(page, start.x, start.y + 1);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
