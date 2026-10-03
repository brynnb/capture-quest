import { OpCodes, WorldSocket } from "@/net";
import type { PokeBattleActionRequest, PokeBattleSwitchRequest, PokeMoveLearnRequest } from "@/net/generated/world_api";
import usePokeBattleStore from "@/stores/PokeBattleStore";

type TurnCommand = Omit<PokeBattleActionRequest, "battle">;
type SwitchCommand = Omit<PokeBattleSwitchRequest, "battle">;
type LearnCommand = Omit<PokeMoveLearnRequest, "battle">;

// Capture the durable identity at send time. A delayed or repeated packet must
// never turn into a command for the next revision or a replacement battle.
function sendBattleCommand(opcode: number, command: TurnCommand | SwitchCommand | LearnCommand): void {
  const { battleId, revision } = usePokeBattleStore.getState();
  if (!battleId || !Number.isSafeInteger(revision) || revision < 1) {
    throw new Error("Battle identity is missing; recover current gameplay before acting");
  }
  WorldSocket.sendJsonMessage(opcode, { ...command, battle: { battleId, revision } });
}

export const sendBattleAction = (command: TurnCommand): void => sendBattleCommand(OpCodes.PokeBattleActionRequest, command);
export const sendBattleSwitch = (command: SwitchCommand): void => sendBattleCommand(OpCodes.PokeBattleSwitchRequest, command);
export const sendMoveLearningChoice = (command: LearnCommand): void => sendBattleCommand(OpCodes.PokeMoveLearnRequest, command);
