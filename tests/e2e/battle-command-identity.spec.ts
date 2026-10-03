import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, waitForBattleOpen, endWildBattleIfOpen } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

test("the real battle menu binds a turn to one durable revision despite duplicated WebSocket delivery", async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  let command: { battle: { battleId: string; revision: number } } | undefined;
  const replies: Array<{ success: boolean; battleId?: string; revision?: number; error?: string }> = [];
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      server.send(message);
      if (!command && Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PokeBattleActionRequest) {
        command = JSON.parse(message.subarray(6).toString());
        server.send(message);
      }
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PokeBattleActionResponse) {
        replies.push(JSON.parse(message.subarray(6).toString()));
      }
      socket.send(message);
    });
  });
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_wild");
  await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
  await page.getByTestId("battle-action-fight").click();
  await page.getByTestId("battle-move-0").click();
  await expect.poll(() => replies.length).toBe(2);
  expect(command?.battle.battleId).toBeTruthy(); expect(command?.battle.revision).toBe(1);
  expect(replies.filter(reply => reply.success)).toEqual([expect.objectContaining({ battleId: command?.battle.battleId, revision: 2 })]);
  expect(replies.filter(reply => !reply.success)).toEqual([expect.objectContaining({ error: expect.stringContaining("Battle changed") })]);
  expect((await getGameState(page)).battle.turnNumber).toBe(1);
  await endWildBattleIfOpen(page); await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});
