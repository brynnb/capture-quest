import { expect, type Page } from "@playwright/test";
import type {
  EngineProbeActorSnapshot,
  EngineProbeBattleSnapshot,
  EngineProbeCommand,
  EngineProbeInventorySnapshot,
  EngineProbeSnapshot,
  EngineProbeSnapshotOptions,
  EngineProbeTileSnapshot,
  EngineProbeWarpSnapshot,
} from "../../../src/testing/engineProbe";
import { getEngineSnapshot, waitForTestBridge } from "./state";

type SnapshotPredicate<T> = (entry: T) => boolean;

export async function sendProbeCommand(
  page: Page,
  command: EngineProbeCommand,
  options?: EngineProbeSnapshotOptions,
) {
  const attempts = "scenario" in command ? 3 : 1;
  let lastError: unknown;

  for (let attempt = 1; attempt <= attempts; attempt++) {
    await waitForTestBridge(page);
    try {
      const result = await page.evaluate(
        ({ command: nextCommand, options: snapshotOptions }) =>
          window.__capturequestTest?.sendCommand(nextCommand, snapshotOptions),
        { command, options },
      );
      if (!result) {
        throw new Error("CaptureQuest engine probe command bridge is not available");
      }
      return result;
    } catch (err) {
      lastError = err;
      if (attempt >= attempts || !isExecutionContextReset(err)) {
        throw err;
      }
      await page.waitForLoadState("domcontentloaded").catch(() => undefined);
    }
  }

  throw lastError;
}

function isExecutionContextReset(err: unknown) {
  const message = err instanceof Error ? err.message : String(err);
  return (
    message.includes("Execution context was destroyed") ||
    message.includes("Cannot find context with specified id")
  );
}

export async function runProbeSequence(
  page: Page,
  commands: EngineProbeCommand[],
  options?: EngineProbeSnapshotOptions,
): Promise<EngineProbeSnapshot> {
  let lastSnapshot = await getEngineSnapshot(page, options);
  for (const command of commands) {
    const result = await sendProbeCommand(page, command, options);
    lastSnapshot = result.after;
    expect(
      result.ok,
      formatProbeFailure(`command ${JSON.stringify(command)} failed`, result.after),
    ).toBe(true);
  }
  return lastSnapshot;
}

export function formatProbeFailure(message: string, snapshot: EngineProbeSnapshot) {
  return [
    message,
    `map=${snapshot.map.name ?? snapshot.map.id}`,
    `player=${snapshot.player.x},${snapshot.player.y} ${snapshot.player.direction}`,
    `moving=${snapshot.player.isMoving}`,
    `frozen=${snapshot.worldInput.frozen}:${snapshot.worldInput.reason ?? ""}`,
    `dialogue=${snapshot.dialogue.isOpen ? snapshot.dialogue.text : "closed"}`,
    `battle=${snapshot.battle.isOpen ? `${snapshot.battle.phase}/${snapshot.battle.selectedAction ?? ""}` : "closed"}`,
    `events=${snapshot.lastEvents.map((event) => event.type).join(",")}`,
  ].join(" | ");
}

export function expectPlayer(
  snapshot: EngineProbeSnapshot,
  expected: Partial<EngineProbeSnapshot["player"]>,
) {
  expect(snapshot.player, formatProbeFailure("unexpected player snapshot", snapshot)).toMatchObject(
    expected,
  );
}

export function expectTile(
  snapshot: EngineProbeSnapshot,
  x: number,
  y: number,
  expected?: Partial<EngineProbeTileSnapshot>,
) {
  const tile = snapshot.tiles.find((entry) => entry.x === x && entry.y === y);
  expect(tile, formatProbeFailure(`missing tile ${x},${y}`, snapshot)).toBeTruthy();
  if (expected) {
    expect(tile).toMatchObject(expected);
  }
  return tile!;
}

export function expectActor(
  snapshot: EngineProbeSnapshot,
  predicate: SnapshotPredicate<EngineProbeActorSnapshot>,
  expected?: Partial<EngineProbeActorSnapshot>,
) {
  const actor = snapshot.actors.find(predicate);
  expect(actor, formatProbeFailure("missing actor", snapshot)).toBeTruthy();
  if (expected) {
    expect(actor).toMatchObject(expected);
  }
  return actor!;
}

export function expectWarp(
  snapshot: EngineProbeSnapshot,
  predicate: SnapshotPredicate<EngineProbeWarpSnapshot>,
  expected?: Partial<EngineProbeWarpSnapshot>,
) {
  const warp = snapshot.warps.find(predicate);
  expect(warp, formatProbeFailure("missing warp", snapshot)).toBeTruthy();
  if (expected) {
    expect(warp).toMatchObject(expected);
  }
  return warp!;
}

export function expectDialogue(
  snapshot: EngineProbeSnapshot,
  expected: Partial<Omit<EngineProbeSnapshot["dialogue"], "text">> & {
    text?: string | RegExp;
  },
) {
  const { text, ...rest } = expected;
  expect(snapshot.dialogue).toMatchObject(rest);
  if (text !== undefined) {
    const matches =
      typeof text === "string"
        ? snapshot.dialogue.text.includes(text)
        : text.test(snapshot.dialogue.text);
    expect(matches, formatProbeFailure(`dialogue did not match ${text}`, snapshot)).toBe(true);
  }
}

export function expectBattle(
  snapshot: EngineProbeSnapshot,
  expected: Partial<EngineProbeBattleSnapshot>,
) {
  expect(snapshot.battle, formatProbeFailure("unexpected battle snapshot", snapshot)).toMatchObject(
    expected,
  );
}

export function expectInventory(
  snapshot: EngineProbeSnapshot,
  expected: Partial<EngineProbeInventorySnapshot> & {
    contains?: string | number | RegExp;
  },
) {
  const { contains, ...rest } = expected;
  expect(snapshot.inventory).toMatchObject(rest);
  if (contains !== undefined) {
    const found = snapshot.inventory.items.some((item) => {
      if (typeof contains === "number") return item.itemId === contains;
      if (typeof contains === "string") {
        return item.name === contains || item.shortName === contains;
      }
      return contains.test(item.name) || contains.test(item.shortName);
    });
    expect(found, formatProbeFailure(`missing inventory item ${contains}`, snapshot)).toBe(true);
  }
}

export function expectParty(
  snapshot: EngineProbeSnapshot,
  predicate: (pokemon: unknown) => boolean,
) {
  expect(
    snapshot.party.some(predicate),
    formatProbeFailure("missing party Pokemon", snapshot),
  ).toBe(true);
}

export async function waitForProbeIdle(page: Page, timeoutMs = 20_000) {
  const result = await sendProbeCommand(page, { waitForIdle: true }, {
    bounds: { radius: 3 },
  });
  expect(result.ok, formatProbeFailure("probe did not become idle", result.after)).toBe(true);
  await expect
    .poll(
      async () => {
        const snapshot = await getEngineSnapshot(page, { bounds: { radius: 3 } });
        return snapshot.player.isMoving || snapshot.worldInput.frozen || snapshot.map.isLoading;
      },
      { timeout: timeoutMs },
    )
    .toBe(false);
}
