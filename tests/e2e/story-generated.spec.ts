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

function envBool(name: string, fallback = false) {
  const value = process.env[name];
  if (!value) return fallback;
  return ["1", "true", "yes", "on"].includes(value.toLowerCase());
}

function envNumber(name: string, fallback: number) {
  const value = process.env[name];
  if (!value) return fallback;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function selectedBrowserCheckpoints() {
  if (envBool("STORY_GENERATED_FULL")) {
    return browserCheckpoints;
  }

  const perChapter = envNumber("STORY_GENERATED_PER_CHAPTER", 2);
  const grouped = new Map<string, typeof browserCheckpoints>();
  for (const checkpoint of browserCheckpoints) {
    const current = grouped.get(checkpoint.entry.chapter) ?? [];
    current.push(checkpoint);
    grouped.set(checkpoint.entry.chapter, current);
  }

  return [...grouped.values()].flatMap((checkpoints) =>
    checkpoints
      .sort((a, b) => {
        if (a.entry.e2eMode !== b.entry.e2eMode) {
          return a.entry.e2eMode === "interactive" ? -1 : 1;
        }
        return a.entry.storyOrder - b.entry.storyOrder;
      })
      .slice(0, perChapter),
  );
}

test("browser-facing non-mainline story checkpoint sample can be applied", async ({
  page,
}) => {
  const checkpoints = selectedBrowserCheckpoints();
  test.setTimeout(Math.max(180_000, checkpoints.length * 12_000));
  const errors = collectPageErrors(page);

  expect(browserCheckpoints.length).toBeGreaterThan(10);
  expect(checkpoints.length).toBeGreaterThan(0);

  await createGuestCharacterAndEnterWorld(page);

  for (const checkpoint of checkpoints) {
    await test.step(checkpoint.name, async () => {
      await jumpToStoryCheckpoint(page, checkpoint);
      await assertStoryCheckpointState(page, checkpoint);
    });
  }

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
