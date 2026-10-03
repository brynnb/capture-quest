import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { BattleCommandResponse, BattleCommandError, GameplayBattleState, GameplayStateResponse, PokemonDTO } from "@/net/generated/world_api";
import type { OwnedPlayerPositionResponse } from "@/net/generated/protocol";
const net = vi.hoisted(() => ({
  commands: new Map<number, Set<(data: BattleCommandResponse | BattleCommandError) => void>>(),
  positions: new Set<(data: OwnedPlayerPositionResponse) => void>(),
  gameplay: new Set<(data: GameplayStateResponse) => void>(),
  send: vi.fn(), positionRequests: vi.fn(), gameplayRequests: vi.fn(),
}));
vi.mock("@/net", () => ({ WorldSocket: { sendStreamJsonMessage: net.send }, OpCodes: { PokeBattleActionRequest: 70, PokeBattleActionResponse: 71, PokeBattleSwitchRequest: 72, PokeBattleSwitchResponse: 73, PokeMoveLearnRequest: 87, PokeMoveLearnResponse: 88, PokeBattleCloseRequest: 89, PokeBattleCloseResponse: 199 } }));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onBattleCommand: (opcode: number, receive: (data: BattleCommandResponse | BattleCommandError) => void) => {
    if (!net.commands.has(opcode)) net.commands.set(opcode, new Set());
    const listeners = net.commands.get(opcode)!; listeners.add(receive); return () => listeners.delete(receive);
  },
  onOwnedPlayerPosition: (receive: (data: OwnedPlayerPositionResponse) => void) => { net.positions.add(receive); return () => net.positions.delete(receive); },
  requestOwnedPlayerPosition: net.positionRequests,
  onGameplayState: (receive: (data: GameplayStateResponse) => void) => { net.gameplay.add(receive); return () => net.gameplay.delete(receive); },
  requestGameplayState: net.gameplayRequests,
  dispatchPhaserResponse: vi.fn(),
}));
vi.mock("./CutsceneService", () => ({ handleCutsceneStart: vi.fn() }));
vi.mock("@/services/audio/AudioManager", () => ({ default: { playMusic: vi.fn(), playSFX: vi.fn() } }));
import usePokeBattleStore from "@/stores/PokeBattleStore";
import { bindBattleScene, sendBattleAction, sendBattleSwitch, sendMoveLearningChoice, closeOrdinaryBattle } from "./BattleCommandService";

const pokemon: PokemonDTO = { id: 25, name: "PIKACHU", level: 5, type1: "ELECTRIC", type2: "", curHp: 1, maxHp: 20, attack: 10, defense: 10, speed: 10, special: 10, exp: 125, expToNextLevel: 91, status: "", isWild: false, boxSlot: 0, moves: [] };
const battle = (revision = 2): GameplayBattleState => ({ battleId: "battle", revision, phase: "action_select", turnNumber: revision - 1, playerPokemon: pokemon, enemyPokemon: pokemon, playerParty: [pokemon], playerActive: 0, battleType: "wild", allowedActions: [], guaranteedCatch: false, trainerClass: "", trainerName: "" });
const position = (requestId: string): OwnedPlayerPositionResponse => ({ success: true, requestId, mapId: 50, x: 7, y: 8, direction: "UP", serverMovementPending: false });
const reply = (requestId: string, revision = 3): BattleCommandResponse => ({ success: true, requestId, position: position(requestId), battle: battle(revision), events: [] });
const emit = (opcode: number, data: BattleCommandResponse | BattleCommandError) => net.commands.get(opcode)?.forEach(receive => receive(data));
const sentID = () => net.send.mock.calls.at(-1)![1].requestId as string;
const project = vi.fn(async () => {});
let retireScene: () => void;
beforeEach(() => {
  vi.useFakeTimers(); net.commands.clear(); net.positions.clear(); net.gameplay.clear();
  net.send.mockReset().mockResolvedValue(undefined); net.positionRequests.mockReset(); net.gameplayRequests.mockReset(); project.mockClear();
  usePokeBattleStore.getState().startBattle(battle());
  retireScene = bindBattleScene(project);
});
afterEach(async () => { retireScene(); await Promise.resolve(); usePokeBattleStore.getState().retireBattle(); vi.useRealTimers(); });

async function recoverRead(currentBattle: GameplayBattleState | null) {
  await vi.advanceTimersByTimeAsync(0);
  const request = net.gameplayRequests.mock.calls.at(-1)![0];
  expect(request).toEqual({ current: true, requestId: expect.any(String) });
  expect(net.positionRequests).not.toHaveBeenCalled();
  net.gameplay.forEach(receive => receive({ success: true, requestId: request.requestId, position: position(request.requestId), battle: currentBattle, safari: null, trainer: null, cutscene: null }));
}

test("a correlated turn has one in-flight mutation; unrelated and late duplicate replies cannot apply", async () => {
  const work = sendBattleAction({ action: "fight", moveSlot: 0 });
  await sendBattleAction({ action: "run" });
  expect(net.send).toHaveBeenCalledTimes(1);
  expect(net.send.mock.calls[0]).toEqual([70, { action: "fight", moveSlot: 0, battle: { battleId: "battle", revision: 2 }, requestId: expect.any(String) }]);
  emit(71, reply("unrelated", 8)); expect(usePokeBattleStore.getState().revision).toBe(2);
  emit(71, reply(sentID())); await work;
  emit(71, reply(sentID(), 4));
  expect(usePokeBattleStore.getState()).toMatchObject({ revision: 3, phase: "action_select", battleCommandPending: false });
  expect(net.commands.get(71)?.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
});

test("switch, learning and dismissal use their own correlated response; closing waits for acknowledgement", async () => {
  const switching = sendBattleSwitch({ action: "switch", partyIndex: 1 });
  expect(net.send.mock.calls.at(-1)![0]).toBe(72); emit(73, reply(sentID())); await switching;
  const learning = sendMoveLearningChoice({ forgetSlot: -1 });
  expect(net.send.mock.calls.at(-1)![0]).toBe(87);
  const learned = reply(sentID(), 4); learned.battle!.phase = "battle_end";
  learned.learning = { skipped: true, message: "Did not learn Splash." };
  learned.end = { playerWon: true, blackoutMapId: 0, blackoutX: 0, blackoutY: 0 };
  emit(88, learned); await learning;
  expect(usePokeBattleStore.getState().eventQueue[0].message).toBe("Did not learn Splash.");
  const closing = closeOrdinaryBattle(); expect(net.send.mock.calls.at(-1)![0]).toBe(89);
  expect(usePokeBattleStore.getState().isInBattle).toBe(true);
  emit(199, { ...reply(sentID(), 4), battle: null }); await closing;
  expect(usePokeBattleStore.getState().isInBattle).toBe(false); expect(project).toHaveBeenCalledWith(position(sentID()));
});

test("a lost reply reads current authority without resending; a delayed original reply cannot overwrite recovery", async () => {
  const work = sendBattleAction({ action: "item", itemId: 1 }); const oldID = sentID();
  await vi.advanceTimersByTimeAsync(10000);
  expect(net.gameplay.size).toBe(1); expect(net.commands.get(71)?.size).toBe(0);
  emit(71, reply(oldID, 3)); expect(usePokeBattleStore.getState().revision).toBe(2);
  await recoverRead({ ...battle(3), phase: "faint_switch" }); await work;
  emit(71, reply(oldID, 4));
  expect(usePokeBattleStore.getState()).toMatchObject({ revision: 3, phase: "faint_switch", battleCommandPending: false });
  expect(net.send).toHaveBeenCalledTimes(1); expect(project).toHaveBeenCalledTimes(1);
});

test("explicit rejection restores a rolled-back phase without another mutation", async () => {
  const work = sendBattleAction({ action: "fight" });
  emit(71, { success: false, requestId: sentID(), error: "storage failed" });
  await recoverRead(battle(2)); await work;
  expect(net.send).toHaveBeenCalledTimes(1); expect(usePokeBattleStore.getState()).toMatchObject({ revision: 2, phase: "action_select" });
});

test("a replacement presentation aborts the request even with the same durable battle ID", async () => {
  const work = sendBattleAction({ action: "fight" }); const oldID = sentID();
  usePokeBattleStore.getState().startBattle(battle(6)); await work;
  emit(71, reply(oldID)); await vi.advanceTimersByTimeAsync(30000);
  expect(usePokeBattleStore.getState().revision).toBe(6); expect(net.positionRequests).not.toHaveBeenCalled();
  expect(vi.getTimerCount()).toBe(0);
});

test("scene retirement during timeout recovery removes its read listener and never projects a late reply", async () => {
  const work = sendBattleAction({ action: "run" });
  await vi.advanceTimersByTimeAsync(10000); expect(net.gameplay.size).toBe(1);
  retireScene(); await work;
  expect(net.gameplay.size).toBe(0); expect(net.send).toHaveBeenCalledTimes(1); expect(project).not.toHaveBeenCalled();
  expect(vi.getTimerCount()).toBe(0);
});

test("an asynchronous transport rejection is caught and recovered, rather than left as an unhandled promise", async () => {
  net.send.mockRejectedValueOnce(new Error("control stream closed"));
  const work = sendBattleAction({ action: "fight" }); await vi.advanceTimersByTimeAsync(0);
  expect(net.gameplayRequests).toHaveBeenCalledTimes(1);
  await recoverRead(battle(2)); await work;
  expect(vi.getTimerCount()).toBe(0); expect(net.send).toHaveBeenCalledTimes(1);
});

test("if current-state recovery also times out, the panel stays locked with an explicit recovery error", async () => {
  const work = sendBattleAction({ action: "run" });
  await vi.advanceTimersByTimeAsync(20000); await work;
  expect(usePokeBattleStore.getState()).toMatchObject({ phase: "animating", battleCommandPending: false, commandError: "Could not restore your battle. Please reconnect." });
  expect(net.gameplay.size).toBe(0); expect(net.send).toHaveBeenCalledTimes(1);
});

test("missing or malformed identity never sends a mutation", async () => {
  for (const identity of [{ battleId: "", revision: 2 }, { battleId: "battle", revision: 0 }, { battleId: "battle", revision: NaN }]) {
    usePokeBattleStore.setState(identity); await sendBattleAction({ action: "run" });
  }
  expect(net.send).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBe(0);
});
