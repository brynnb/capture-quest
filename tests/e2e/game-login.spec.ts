import { expect, test } from "@playwright/test";
import {
  createCharacter,
  enterWorld,
  loginAsGuest,
  quitToCharacterSelect,
  uniqueTrainerName,
} from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  getGameState,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerTile,
  waitForTestBridge,
} from "./helpers/state";

test("guest can create a character and enter the world through the real UI", async ({
  page,
}) => {
  test.setTimeout(90_000);
  const errors = collectPageErrors(page);

  const characterName = uniqueTrainerName();
  await loginAsGuest(page);
  await createCharacter(page, characterName);
  await enterWorld(page, characterName);
  await waitForTestBridge(page);
  await waitForNoMapLoading(page);
  await waitForMap(page, "REDS_HOUSE_2F");
  await waitForPlayerTile(page, 3, 6);

  const state = await getGameState(page);
  expect(state.screen).toBe("game");
  expect(state.connected).toBe(true);
  expect(state.player.name).toBe(characterName);
  expect(state.player.x).not.toBeNull();
  expect(state.player.y).not.toBeNull();
  expect(state.map.id).not.toBeNull();

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
