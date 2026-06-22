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

function envNumber(name: string, fallback: number) {
  const value = process.env[name];
  if (!value) return fallback;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function selectedMainlineCheckpoints() {
  const offset = envNumber("STORY_MAINLINE_OFFSET", 0);
  const limit = envNumber("STORY_MAINLINE_LIMIT", 0);
  if (limit > 0) {
    return mainlineCheckpoints.slice(offset, offset + limit);
  }
  return mainlineCheckpoints;
}

test("canonical Charmander storyline checkpoints apply from intro through Hall of Fame", async ({
  page,
}) => {
  const checkpoints = selectedMainlineCheckpoints();
  test.setTimeout(Math.max(20 * 60_000, checkpoints.length * 12_000));
  const errors = collectPageErrors(page);

  expect(mainlineCheckpoints.length).toBeGreaterThan(50);
  expect(checkpoints.length).toBeGreaterThan(0);
  expect(mainlineCheckpoints[0]?.name).toBe("pallet_town_oak_stops_player");
  expect(mainlineCheckpoints.at(-1)?.name).toBe("hall_of_fame_oak_congratulations");

  await createGuestCharacterAndEnterWorld(page);

  for (const [index, checkpoint] of checkpoints.entries()) {
    await test.step(`${index + 1}/${checkpoints.length} ${checkpoint.name}`, async () => {
      console.log(
        `[story-mainline] ${index + 1}/${checkpoints.length} ${checkpoint.name}`,
      );
      await jumpToStoryCheckpoint(page, checkpoint);
      await assertStoryCheckpointState(page, checkpoint);
    });
  }

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
