import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { BattleCommandResponse, SafariBattleActionResponse, SafariRecoveryState, BattleCommandError, GameplayBattleState, GameplayStateResponse, PokemonDTO } from "@/net/generated/world_api";
import type { OwnedPlayerPositionResponse } from "@/net/generated/protocol";
const net = vi.hoisted(() => ({
  commands: new Map<number, Set<(data: BattleCommandResponse | SafariBattleActionResponse | BattleCommandError) => void>>(),
  positions: new Set<(data: OwnedPlayerPositionResponse) => void>(),
  gameplay: new Set<(data: GameplayStateResponse) => void>(),
  send: vi.fn(), positionRequests: vi.fn(), gameplayRequests: vi.fn(),
}));
vi.mock("@/net", () => ({ WorldSocket: { sendStreamJsonMessage: net.send }, OpCodes: { SafariBattleActionRequest: 129, SafariBattleActionResponse: 130, PokeBattleActionRequest: 70, PokeBattleActionResponse: 71, PokeBattleSwitchRequest: 72, PokeBattleSwitchResponse: 73, PokeMoveLearnRequest: 87, PokeMoveLearnResponse: 88, PokeBattleCloseRequest: 89, PokeBattleCloseResponse: 199 } }));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onBattleCommand: (opcode: number, receive: (data: BattleCommandResponse | SafariBattleActionResponse | BattleCommandError) => void) => {
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
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import { bindBattleScene, sendBattleAction, sendBattleSwitch, sendMoveLearningChoice, closeOrdinaryBattle, sendSafariAction, closeSafariBattle } from "./BattleCommandService";

const pokemon: PokemonDTO = { id: 25, name: "PIKACHU", level: 5, type1: "ELECTRIC", type2: "", curHp: 1, maxHp: 20, attack: 10, defense: 10, speed: 10, special: 10, exp: 125, expToNextLevel: 91, status: "", isWild: false, boxSlot: 0, moves: [] };
const battle = (revision = 2): GameplayBattleState => ({ battleId: "battle", revision, phase: "action_select", turnNumber: revision - 1, playerPokemon: pokemon, enemyPokemon: pokemon, playerParty: [pokemon], playerActive: 0, battleType: "wild", allowedActions: [], guaranteedCatch: false, trainerClass: "", trainerName: "" });
const position = (requestId: string): OwnedPlayerPositionResponse => ({ success: true, requestId, mapId: 50, x: 7, y: 8, direction: "UP", serverMovementPending: false });
const reply = (requestId: string, revision = 3): BattleCommandResponse => ({ success: true, requestId, position: position(requestId), battle: battle(revision), events: [] });
const emit = (opcode: number, data: BattleCommandResponse | SafariBattleActionResponse | BattleCommandError) => net.commands.get(opcode)?.forEach(receive => receive(data));
const sentID = () => net.send.mock.calls.at(-1)![1].requestId as string;
const project = vi.fn(async () => {});
const safari = (revision = 1): SafariRecoveryState => ({ active: true, ballsLeft: 30, stepsLeft: 499, battleId: "safari", revision, pokemon: { id: 129, name: "MAGIKARP", level: 5, hp: 20, maxHp: 20 }, playerParty: [pokemon] });
const safariReply = (requestId: string, revision = 2): SafariBattleActionResponse => ({ success: true, requestId, battleId: "safari", revision, position: position(requestId), events: [], ballsLeft: 29, stepsLeft: 499, isOver: false, caught: false, fled: false });
let retireScene: () => void;
beforeEach(() => {
  vi.useFakeTimers(); net.commands.clear(); net.positions.clear(); net.gameplay.clear();
  net.send.mockReset().mockResolvedValue(undefined); net.positionRequests.mockReset(); net.gameplayRequests.mockReset(); project.mockClear();
  usePokeBattleStore.getState().startBattle(battle());
  retireScene = bindBattleScene(project);
});
afterEach(async () => { vi.restoreAllMocks(); retireScene(); await Promise.resolve(); usePokeBattleStore.getState().retireBattle(); vi.useRealTimers(); });

async function recoverRead(currentBattle: GameplayBattleState | null, safari: SafariRecoveryState | null = null) {
  await vi.advanceTimersByTimeAsync(0);
  const request = net.gameplayRequests.mock.calls.at(-1)![0];
  expect(request).toEqual({ current: true, requestId: expect.any(String) });
  expect(net.positionRequests).not.toHaveBeenCalled();
  net.gameplay.forEach(receive => receive({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: currentBattle?.playerParty ?? safari?.playerParty ?? [], eventFlags: [], success: true, requestId: request.requestId, position: position(request.requestId), battle: currentBattle, safari, trainer: null, cutscene: null }));
}

test("Safari uses the same single-flight coordinator and correlated durable revision", async () => {
  usePokeBattleStore.getState().restoreGameplay({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: [], eventFlags: [], success: true, requestId: "start", position: position("start"), battle: null, safari: safari(), trainer: null, cutscene: null });
  const action = sendSafariAction("ball");
  await sendSafariAction("ball"); await sendBattleAction({ action: "fight" });
  expect(net.send).toHaveBeenCalledTimes(1);
  expect(net.send.mock.calls[0]).toEqual([129, { action: "ball", battle: { battleId: "safari", revision: 1 }, requestId: expect.any(String) }]);
  emit(130, safariReply("unrelated", 8));
  expect(usePokeBattleStore.getState().revision).toBe(1);
  const id = sentID(); emit(130, safariReply(id)); await action;
  emit(130, safariReply(id, 3));
  expect(usePokeBattleStore.getState()).toMatchObject({ isSafari: true, revision: 2, safariBallsLeft: 29, phase: "action_select", battleCommandPending: false });
  expect(net.commands.get(130)?.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
});

test("lost Safari catch reply recovers its PC summary and party; dismissal waits for acknowledgement", async () => {
  usePokeBattleStore.getState().startSafariBattle({ ...safari(), pokemon: safari().pokemon! });
  const action = sendSafariAction("ball"); const oldID = sentID();
  await vi.advanceTimersByTimeAsync(10000);
  const party = [{ ...pokemon, curHp: 20 }];
  await recoverRead(null, { ...safari(2), ballsLeft: 29, isOver: true, caught: true, sentToPC: true, pcBox: 4, playerParty: party }); await action;
  expect(usePokeBattleStore.getState()).toMatchObject({ phase: "battle_end", battleResult: "caught", sentToPCBox: 4, eventQueue: [], battleCommandPending: false });
  expect(usePokemonPartyStore.getState().party).toEqual(party);
  emit(130, { ...safariReply(oldID), events: [{ type: "catch_success", message: "Old catch" }] });
  expect(usePokeBattleStore.getState().eventQueue).toEqual([]);
  const closing = closeSafariBattle();
  expect(usePokeBattleStore.getState().isInBattle).toBe(true);
  emit(130, { ...safariReply(sentID(), 3), closed: true, isOver: true, caught: true }); await closing;
  expect(usePokeBattleStore.getState().isInBattle).toBe(false);
  expect(net.send.mock.calls.map(call => call[1].action)).toEqual(["ball", "close"]);
});

test("lost Safari close acknowledgement restores absence without resending dismissal", async () => {
  usePokeBattleStore.getState().startSafariBattle({ ...safari(2), pokemon: safari().pokemon! });
  const closing = closeSafariBattle();
  await vi.advanceTimersByTimeAsync(10000);
  await recoverRead(null, { active: true, ballsLeft: 29, stepsLeft: 499 }); await closing;
  expect(usePokeBattleStore.getState().isInBattle).toBe(false);
  expect(net.send).toHaveBeenCalledTimes(1); expect(net.commands.get(130)?.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
});

test.each([false, true])("expired Safari dismissal presents the server message after projection; lost reply=%s", async lost => {
  const terminal = { ...safari(2), active: false, ballsLeft: 0, isOver: true, exitMessage: "Server expiry announcement" };
  usePokeBattleStore.getState().restoreGameplay({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: [], eventFlags: [], success: true, requestId: "start", position: position("start"), battle: null, safari: terminal, trainer: null, cutscene: null });
  const announce = vi.spyOn(usePokemonDialogueStore.getState(), "openDialogue").mockImplementation(() => {});
  project.mockImplementationOnce(async () => { expect(announce).not.toHaveBeenCalled(); });
  const closing = closeSafariBattle(); const id = sentID();
  if (lost) { await vi.advanceTimersByTimeAsync(10000); await recoverRead(null); }
  else emit(130, { ...safariReply(id, 3), closed: true, safariOver: true, isOver: true, ballsLeft: 0, exitMessage: terminal.exitMessage });
  await closing;
  expect(announce).toHaveBeenCalledTimes(1); expect(announce).toHaveBeenCalledWith([terminal.exitMessage], null);
  expect(usePokeBattleStore.getState()).toMatchObject({ isInBattle: false, safariExitMessage: null });
  emit(130, { ...safariReply(id, 3), closed: true, safariOver: true, exitMessage: terminal.exitMessage });
  expect(announce).toHaveBeenCalledTimes(1); expect(net.send).toHaveBeenCalledTimes(1);
});

test("failed expired Safari dismissal retains the terminal encounter without announcing or retrying", async () => {
  const terminal = { ...safari(2), active: false, ballsLeft: 0, isOver: true, exitMessage: "Server expiry announcement" };
  usePokeBattleStore.getState().restoreGameplay({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: [], eventFlags: [], success: true, requestId: "start", position: position("start"), battle: null, safari: terminal, trainer: null, cutscene: null });
  const announce = vi.spyOn(usePokemonDialogueStore.getState(), "openDialogue").mockImplementation(() => {});
  const closing = closeSafariBattle(); emit(130, { success: false, requestId: sentID(), error: "Commit failed" });
  await recoverRead(null, terminal); await closing;
  expect(usePokeBattleStore.getState()).toMatchObject({ isInBattle: true, safariExitMessage: terminal.exitMessage, commandError: expect.stringContaining("reconnect") });
  expect(announce).not.toHaveBeenCalled(); expect(net.send).toHaveBeenCalledTimes(1);
});

test("scene retirement during recovered Safari dismissal projection suppresses the old announcement", async () => {
  const terminal = { ...safari(2), active: false, ballsLeft: 0, isOver: true, exitMessage: "Old scene announcement" };
  usePokeBattleStore.getState().restoreGameplay({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: [], eventFlags: [], success: true, requestId: "start", position: position("start"), battle: null, safari: terminal, trainer: null, cutscene: null });
  const announce = vi.spyOn(usePokemonDialogueStore.getState(), "openDialogue").mockImplementation(() => {});
  let finish!: () => void;
  project.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const closing = closeSafariBattle(); await vi.advanceTimersByTimeAsync(10000); await recoverRead(null);
  await vi.advanceTimersByTimeAsync(0); expect(project).toHaveBeenCalledTimes(1);
  retireScene(); finish(); await closing;
  expect(announce).not.toHaveBeenCalled(); expect(net.send).toHaveBeenCalledTimes(1);
  expect(net.gameplay.size).toBe(0); expect(net.commands.get(130)?.size).toBe(0);
});

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

test.each([0, -1])("lost pending-choice and learning replies recover without resending slot %s", async (forgetSlot) => {
  const turn = sendBattleAction({ action: "fight", moveSlot: 0 });
  const turnID = sentID();
  await vi.advanceTimersByTimeAsync(10000);
  const pending = { ...battle(3), phase: "move_learn_prompt", pendingMove: { moveId: 150, moveName: "SPLASH", pokemonIndex: 0 } };
  await recoverRead(pending); await turn;
  expect(usePokeBattleStore.getState()).toMatchObject({ phase: "move_learn_prompt", pendingMoveLearn: { moveId: 150, moveName: "SPLASH" }, recoveredDismissal: false, battleCommandPending: false });
  expect(net.send).toHaveBeenCalledTimes(1);

  const learning = sendMoveLearningChoice({ forgetSlot });
  const choiceID = sentID();
  expect(net.send.mock.calls.at(-1)).toEqual([87, { forgetSlot, battle: { battleId: "battle", revision: 3 }, requestId: choiceID }]);
  await vi.advanceTimersByTimeAsync(10000);
  const settledPokemon = forgetSlot === -1 ? pokemon : { ...pokemon, moves: [{ id: 150, name: "SPLASH", pp: 40, maxPp: 40, type: "NORMAL", power: 0, accuracy: 0 }] };
  const settled = { ...battle(4), phase: "battle_end", needsDismissal: true, playerPokemon: settledPokemon, playerParty: [settledPokemon] };
  await recoverRead(settled); await learning;
  expect(usePokeBattleStore.getState()).toMatchObject({ phase: "battle_end", revision: 4, pendingMoveLearn: null, recoveredDismissal: true, battleCommandPending: false, eventQueue: [] });
  expect(usePokemonPartyStore.getState().party).toEqual([settledPokemon]);

  // Retired correlation listeners cannot replay the prompt or learning/reward
  // text. Recovery displays current authority without inventing a lost outcome.
  emit(71, { ...reply(turnID), battle: pending });
  emit(88, { ...reply(choiceID, 4), battle: settled, learning: { skipped: forgetSlot === -1, message: "Old choice result", postEvents: [{ type: "message", message: "Old reward", targetHp: 0, targetMaxHp: 0 }] } });
  expect(usePokeBattleStore.getState()).toMatchObject({ revision: 4, pendingMoveLearn: null, eventQueue: [] });
  expect(net.send).toHaveBeenCalledTimes(2);
  const close = closeOrdinaryBattle();
  emit(199, { ...reply(sentID(), 4), battle: null }); await close;
  expect(usePokeBattleStore.getState().isInBattle).toBe(false);
  expect(net.send.mock.calls.map(call => call[0])).toEqual([70, 87, 89]);
  expect(net.gameplay.size).toBe(0); expect(net.commands.get(88)?.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
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

test("a failed terminal dismissal recovers authority without starting an automatic close retry", async () => {
  const work = closeOrdinaryBattle();
  emit(199, { success: false, requestId: sentID(), error: "plan issuance failed" });
  await recoverRead({ ...battle(2), phase: "battle_end", needsDismissal: true }); await work;
  await vi.advanceTimersByTimeAsync(30000);
  expect(net.send).toHaveBeenCalledTimes(1);
  expect(usePokeBattleStore.getState()).toMatchObject({ commandError: "Could not restore your battle. Please reconnect.", battleCommandPending: false, phase: "animating" });
});

test("restored terminal presentation stays pending until the old command releases scene projection", async () => {
  let release!: () => void;
  project.mockImplementationOnce(() => new Promise<void>(resolve => { release = resolve; }));
  const work = sendBattleAction({ action: "fight" });
  await vi.advanceTimersByTimeAsync(10000);
  await recoverRead({ ...battle(3), phase: "battle_end", needsDismissal: true });
  await vi.advanceTimersByTimeAsync(0);
  expect(usePokeBattleStore.getState()).toMatchObject({ recoveredDismissal: true, phase: "battle_end", battleCommandPending: true });
  release(); await work;
  expect(usePokeBattleStore.getState().battleCommandPending).toBe(false);
  const close = closeOrdinaryBattle();
  expect(net.send).toHaveBeenCalledTimes(2);
  emit(199, { ...reply(sentID()), battle: null }); await close;
  expect(usePokeBattleStore.getState().isInBattle).toBe(false);
});

test("current-state battle recovery refreshes the shared party view with the same authoritative party", async () => {
  usePokemonPartyStore.getState().setParty([pokemon]);
  const work = sendBattleAction({ action: "fight" });
  await vi.advanceTimersByTimeAsync(10000);
  const fresh = { ...pokemon, maxHp: 21, defense: 12, exp: 130, moves: [{ id: 150, name: "Splash", pp: 39, maxPp: 40, type: "NORMAL", power: 0, accuracy: 0 }] };
  await recoverRead({ ...battle(3), playerPokemon: fresh, playerParty: [fresh] }); await work;
  expect(usePokemonPartyStore.getState().party).toEqual([fresh]);
  expect(usePokeBattleStore.getState().faintSwitchParty).toEqual([fresh]);
});

test("a replacement scene can dismiss a recovered terminal battle before the retired projection resolves", async () => {
  const releases: ReturnType<typeof vi.fn>[] = [];
  const subscribe = usePokeBattleStore.subscribe;
  const subscribed = vi.spyOn(usePokeBattleStore, "subscribe").mockImplementation(listener => {
    const release = vi.fn(subscribe(listener)); releases.push(release); return release;
  });
  let release!: () => void;
  project.mockImplementationOnce(() => new Promise<void>(resolve => { release = resolve; }));
  const oldWork = sendBattleAction({ action: "fight" });
  await vi.advanceTimersByTimeAsync(10000);
  const terminal = { ...battle(3), phase: "battle_end", needsDismissal: true };
  await recoverRead(terminal); await vi.advanceTimersByTimeAsync(0);
  const cleanupOld = retireScene;
  cleanupOld();
  expect(releases[0]).toHaveBeenCalledTimes(1);
  const destinationProject = vi.fn(async () => {});
  retireScene = bindBattleScene(destinationProject);
  usePokeBattleStore.getState().restoreGameplay({ pc: {currentBox:0,boxCount:12,boxSize:20,box:[],sources:[]}, commandRevision: 0, inventory: [], wallet: { characterId: 42, pokedollars: 0 }, party: [], eventFlags: [], success: true, requestId: "destination", position: position("destination"), battle: terminal, safari: null, trainer: null, cutscene: null });
  cleanupOld(); // A second old cleanup cannot retire the destination binding.
  const close = closeOrdinaryBattle();
  expect(net.send).toHaveBeenCalledTimes(2);
  expect(net.send.mock.calls.at(-1)![0]).toBe(89);
  release(); await oldWork;
  expect(usePokeBattleStore.getState().battleCommandPending).toBe(true);
  emit(199, { ...reply(sentID()), battle: null }); await close;
  expect(destinationProject).toHaveBeenCalledTimes(1);
  subscribed.mockRestore();
  expect(usePokeBattleStore.getState()).toMatchObject({ isInBattle: false, battleCommandPending: false });
  expect(vi.getTimerCount()).toBe(0);
});
