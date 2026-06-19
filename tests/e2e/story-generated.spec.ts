import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  assertStoryCheckpointState,
  jumpToStoryCheckpoint,
  orderedStoryCheckpoints,
} from "./helpers/story";

const browserCheckpoints = orderedStoryCheckpoints(
  (checkpoint) =>
    checkpoint.entry.kind !== "mainline" &&
    checkpoint.entry.e2eMode !== "scriptOnly",
);

test("browser-facing non-mainline story checkpoints can be applied", async ({
  page,
}) => {
  test.setTimeout(10 * 60_000);
  const errors = collectPageErrors(page);

  expect(browserCheckpoints.length).toBeGreaterThan(10);

  await createGuestCharacterAndEnterWorld(page);

  for (const checkpoint of browserCheckpoints) {
    await test.step(checkpoint.name, async () => {
      await jumpToStoryCheckpoint(page, checkpoint);
      await assertStoryCheckpointState(page, checkpoint);
    });
  }

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
