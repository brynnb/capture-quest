import { beforeEach, expect, test, vi } from "vitest";
const transport = vi.hoisted(() => ({ send: vi.fn() }));
vi.mock("@/net", () => ({ WorldSocket: { sendJsonMessage: transport.send }, OpCodes: { PokeBattleActionRequest: 70, PokeBattleSwitchRequest: 72, PokeMoveLearnRequest: 87 } }));
import usePokeBattleStore from "@/stores/PokeBattleStore";
import { sendBattleAction, sendBattleSwitch, sendMoveLearningChoice } from "./BattleCommandService";

beforeEach(() => { transport.send.mockClear(); usePokeBattleStore.setState({ battleId: "battle", revision: 2 }); });

test("turn, switch and learning use the current durable identity and retain their own opcode", () => {
  sendBattleAction({ action: "fight", moveSlot: 0 });
  sendBattleSwitch({ action: "switch", partyIndex: 1 });
  sendMoveLearningChoice({ forgetSlot: -1 });
  expect(transport.send.mock.calls).toEqual([
    [70, { action: "fight", moveSlot: 0, battle: { battleId: "battle", revision: 2 } }],
    [72, { action: "switch", partyIndex: 1, battle: { battleId: "battle", revision: 2 } }],
    [87, { forgetSlot: -1, battle: { battleId: "battle", revision: 2 } }],
  ]);
  usePokeBattleStore.setState({ revision: 3 });
  sendBattleAction({ action: "run" });
  expect(transport.send.mock.calls[3][1].battle.revision).toBe(3);
  expect(transport.send.mock.calls[0][1].battle.revision).toBe(2);
});

test("missing or malformed identity cannot send a gameplay mutation", () => {
  for (const identity of [{ battleId: "", revision: 2 }, { battleId: "battle", revision: 0 }, { battleId: "battle", revision: NaN }]) {
    usePokeBattleStore.setState(identity);
    expect(() => sendBattleAction({ action: "run" })).toThrow("Battle identity is missing");
  }
  expect(transport.send).not.toHaveBeenCalled();
});
