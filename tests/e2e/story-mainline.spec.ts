import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  assertStoryCheckpointState,
  jumpToStoryCheckpoint,
  orderedStoryCheckpoints,
} from "./helpers/story";

const mainlineCheckpoints = orderedStoryCheckpoints(
  (checkpoint) =>
    checkpoint.entry.kind === "mainline" &&
    checkpoint.entry.e2eMode !== "scriptOnly",
);

test("canonical Charmander storyline checkpoints apply from intro through Hall of Fame", async ({
  page,
}) => {
  test.setTimeout(20 * 60_000);
  const errors = collectPageErrors(page);

  expect(mainlineCheckpoints.length).toBeGreaterThan(50);
  expect(mainlineCheckpoints[0]?.name).toBe("pallet_town_oak_stops_player");
  expect(mainlineCheckpoints.at(-1)?.name).toBe("hall_of_fame_oak_congratulations");

  await createGuestCharacterAndEnterWorld(page);

  for (const checkpoint of mainlineCheckpoints) {
    await test.step(checkpoint.name, async () => {
      await jumpToStoryCheckpoint(page, checkpoint);
      await assertStoryCheckpointState(page, checkpoint);
    });
  }

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
