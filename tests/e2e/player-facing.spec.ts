import { expect, test } from "@playwright/test";
import {
  createGuestCharacterAndEnterWorld,
  quitToCharacterSelect,
} from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import {
  getGameState,
  waitForMap,
  waitForPlayerIdle,
  waitForPlayerTile,
} from "./helpers/state";

test("player keeps the completed movement direction while idle", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await waitForMap(page, "REDS_HOUSE_2F");
  await waitForPlayerTile(page, 3, 6);

  await pressMovement(page, "right");
  await waitForPlayerTile(page, 4, 6);
  await waitForPlayerIdle(page);

  await expect
    .poll(
      async () => {
        const player = (await getGameState(page)).player;
        return {
          direction: player.direction,
          frame: player.spriteFrame,
          flipX: player.spriteFlipX,
        };
      },
      { timeout: 5_000 },
    )
    .toEqual({ direction: "RIGHT", frame: 2, flipX: true });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
