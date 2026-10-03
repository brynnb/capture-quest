import { beforeEach, expect, test, vi } from "vitest";
import type { GameplayStateResponse, PokemonDTO } from "@/net/generated/world_api";
const transport = vi.hoisted(() => ({ send: vi.fn() }));
vi.mock("@/net", () => ({ WorldSocket: { sendJsonMessage: transport.send }, OpCodes: { PokeBattleCloseRequest: 89 } }));
import usePokeBattleStore from "./PokeBattleStore";
const pokemon: PokemonDTO = { id: 25, name: "PIKACHU", level: 5, type1: "ELECTRIC", type2: "", curHp: 1, maxHp: 20, attack: 10, defense: 10, speed: 10, special: 10, exp: 125, expToNextLevel: 91, status: "", isWild: false, boxSlot: 0, moves: [] };
const empty = (): GameplayStateResponse => ({ success: true, requestId: "read", position: { success: true, requestId: "read", mapId: 50, x: 7, y: 8, direction: "UP", serverMovementPending: false }, battle: null, safari: null, trainer: null, cutscene: null });
beforeEach(() => { usePokeBattleStore.getState().restoreGameplay(empty()); transport.send.mockClear(); });

test("ordinary recovery replaces stale Safari and preserves faint-switch authority", () => {
  const safari = empty(); safari.safari = { active: true, ballsLeft: 7, stepsLeft: 93, pokemon: { id: 129, name: "MAGIKARP", level: 5, hp: 10, maxHp: 10 } };
  usePokeBattleStore.getState().restoreGameplay(safari); expect(usePokeBattleStore.getState().isSafari).toBe(true);
  const ordinary = empty(); ordinary.battle = { battleId: "battle", revision: 2, phase: "faint_switch", turnNumber: 3, playerPokemon: pokemon, enemyPokemon: { ...pokemon, id: 129, name: "MAGIKARP" }, playerParty: [pokemon], playerActive: 0, battleType: "trainer", allowedActions: [], guaranteedCatch: false, trainerClass: "TEST", trainerName: "Trainer" };
  usePokeBattleStore.getState().restoreGameplay(ordinary);
  const current = usePokeBattleStore.getState();
  expect(current.isSafari).toBe(false); expect(current.phase).toBe("faint_switch"); expect(current.events).toEqual([]); expect(current.turnNumber).toBe(3);
  expect(current.faintSwitchParty).toEqual([pokemon]); expect(transport.send).not.toHaveBeenCalled();
});

test("move-learn recovery restores the exact pending choice; absence clears it without closing a server battle", () => {
  const choice = empty(); choice.battle = { battleId: "battle", revision: 4, phase: "move_learn_prompt", turnNumber: 4, playerPokemon: pokemon, enemyPokemon: pokemon, playerParty: [pokemon], playerActive: 0, battleType: "wild", allowedActions: [], guaranteedCatch: false, trainerClass: "", trainerName: "", pendingMove: { moveId: 150, moveName: "SPLASH", pokemonIndex: 0 } };
  usePokeBattleStore.getState().restoreGameplay(choice);
  expect(usePokeBattleStore.getState().pendingMoveLearn).toEqual({ moveId: 150, moveName: "SPLASH" });
  usePokeBattleStore.getState().restoreGameplay(empty());
  const current = usePokeBattleStore.getState();
  expect(current.isInBattle).toBe(false); expect(current.pendingMoveLearn).toBeNull(); expect(current.phase).toBe("none"); expect(current.enemyPokemon).toBeNull();
  expect(transport.send).not.toHaveBeenCalled();
});
