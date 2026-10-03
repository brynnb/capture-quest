import { create } from "zustand";
import type { BattleCommandIdentity, GameplayStateResponse, PokemonDTO } from "@/net/generated/world_api";
import { closeOrdinaryBattle, closeSafariBattle } from "@/phaser-game/services/BattleCommandService";
import useAudioActivityStore from "@/stores/AudioActivityStore";

export type BattlePhase =
  | "none"
  | "action_select"
  | "move_select"
  | "item_select"
  | "pokemon_select"
  | "faint_switch"
  | "move_learn_prompt"
  | "battle_end"
  | "animating";

import type { BattleEvent } from "@/net/generated/battle_events";
export type { BattleEvent } from "@/net/generated/battle_events";

interface PokeBattleState extends BattleCommandIdentity {
  retireBattle: () => void;
  presentationGeneration: number;
  battleCommandPending: boolean;
  commandError: string | null;
  recoveredDismissal: boolean;
  restoreGameplay: (snapshot: GameplayStateResponse) => void;
  isInBattle: boolean;
  phase: BattlePhase;
  pendingPhase: BattlePhase;
  turnNumber: number;
  playerPokemon: PokemonDTO | null;
  enemyPokemon: PokemonDTO | null;
  // Deferred final state — applied after all events are shown
  pendingPlayerPokemon: PokemonDTO | null;
  pendingEnemyPokemon: PokemonDTO | null;
  events: BattleEvent[];
  eventQueue: BattleEvent[];
  currentEventIndex: number;
  battleResult: "win" | "lose" | "fled" | "caught" | null;
  // Queued end-of-battle result (applied after events finish)
  pendingBattleEnd: { playerWon: boolean } | null;
  lossMessage: string | null;
  // Trainer class constant for sprite lookup (null for wild battles)
  trainerClass: string | null;
  // Blackout warp coordinates (from server, for loss/blackout)
  blackoutWarp: { mapId: number; x: number; y: number } | null;
  // Faint switch state — party snapshot + active index sent by server
  faintSwitchParty: PokemonDTO[];
  faintSwitchActive: number;
  battleType: "wild" | "trainer" | "safari" | null;
  allowedActions: string[];
  guaranteedCatch: boolean;
  // Pending move learn (set when move_learn_prompt event is received)
  pendingMoveLearn: { moveId: number; moveName: string } | null;
  // Safari Zone battle state
  isSafari: boolean;
  safariBallsLeft: number;
  // Caught Pokémon sent to Bill's PC (party was full)
  sentToPC: boolean;
  sentToPCBox: number | null;

  startBattle: (data: BattleCommandIdentity & {
    playerPokemon: PokemonDTO;
    enemyPokemon: PokemonDTO;
    phase: string;
    turnNumber: number;
    events?: BattleEvent[];
    trainerClass?: string;
    playerParty?: PokemonDTO[];
    playerActive?: number;
    battleType?: string;
    allowedActions?: string[];
    guaranteedCatch?: boolean;
  }) => void;

  updateBattleState: (data: BattleCommandIdentity & {
    playerPokemon: PokemonDTO;
    enemyPokemon: PokemonDTO;
    phase: string;
    turnNumber: number;
    events: BattleEvent[];
    playerParty?: PokemonDTO[];
    playerActive?: number;
    battleType?: string;
    allowedActions?: string[];
    guaranteedCatch?: boolean;
  }) => void;

  endBattle: (playerWon: boolean, blackoutWarp?: { mapId: number; x: number; y: number }, sentToPC?: boolean, sentToPCBox?: number, lossMessage?: string) => void;
  closeBattle: () => void;
  setPhase: (phase: BattlePhase) => void;
  advanceEvent: () => void;
  startSafariBattle: (data: { battleId?: string; revision?: number; pokemon: { id: number; name: string; level: number; hp: number; maxHp: number }; ballsLeft: number; stepsLeft: number }) => void;
  updateSafariState: (data: { events: BattleEvent[]; ballsLeft: number; stepsLeft: number; isOver: boolean; caught: boolean; fled: boolean; caughtPokemon?: { name: string }; sentToPC?: boolean; pcBox?: number }) => void;
}

type BattlePresentationState = Omit<PokeBattleState,
  "retireBattle" | "restoreGameplay" | "startBattle" | "updateBattleState" | "endBattle" | "closeBattle" | "setPhase" | "advanceEvent" | "startSafariBattle" | "updateSafariState">;

const initialBattleState: BattlePresentationState = {
  presentationGeneration: 0,
  battleCommandPending: false,
  commandError: null,
  recoveredDismissal: false,
  battleId: "",
  revision: 0,
  isInBattle: false,
  phase: "none",
  pendingPhase: "none",
  turnNumber: 0,
  playerPokemon: null,
  enemyPokemon: null,
  pendingPlayerPokemon: null,
  pendingEnemyPokemon: null,
  events: [],
  eventQueue: [],
  currentEventIndex: 0,
  battleResult: null,
  pendingBattleEnd: null,
  lossMessage: null,
  trainerClass: null,
  blackoutWarp: null,
  faintSwitchParty: [],
  faintSwitchActive: 0,
  battleType: null,
  allowedActions: [],
  guaranteedCatch: false,
  pendingMoveLearn: null,
  isSafari: false,
  safariBallsLeft: 0,
  sentToPC: false,
  sentToPCBox: null,
};

const usePokeBattleStore = create<PokeBattleState>((set, get) => ({
  ...initialBattleState,

  retireBattle: () => {
    useAudioActivityStore.getState().setBattleVictoryTrack(null);
    set({ ...initialBattleState, presentationGeneration: get().presentationGeneration + 1 });
  },

  restoreGameplay: (snapshot) => {
    useAudioActivityStore.getState().setBattleVictoryTrack(null);
    if (snapshot.battle) {
      const battle = snapshot.battle;
      get().startBattle({ ...battle, events: [] });
      set({ recoveredDismissal: battle.needsDismissal === true });
      if (battle.caught) {
        set({ battleResult: "caught", sentToPC: battle.capture?.sentToPC === true, sentToPCBox: battle.capture?.sentToPC ? battle.capture.pcBox : null });
      }
      if (battle.pendingMove) {
        set({ pendingMoveLearn: { moveId: battle.pendingMove.moveId, moveName: battle.pendingMove.moveName } });
      }
    } else if (snapshot.safari?.pokemon) {
      get().startSafariBattle({ ...snapshot.safari, pokemon: snapshot.safari.pokemon });
      if (snapshot.safari.isOver) get().updateSafariState({ ...snapshot.safari, isOver: true, caught: snapshot.safari.caught === true, fled: snapshot.safari.fled === true, events: [] });
    } else {
      // Absence retires local presentation without sending a CloseBattle command.
      get().retireBattle();
    }
  },

  startBattle: (data) => {
    const hasEvents = data.events && data.events.length > 0;
    set({
      ...initialBattleState,
      presentationGeneration: get().presentationGeneration + 1,
      isInBattle: true,
      battleId: data.battleId,
      revision: data.revision,
      phase: hasEvents ? "animating" : (data.phase as BattlePhase),
      pendingPhase: data.phase as BattlePhase,
      turnNumber: data.turnNumber,
      playerPokemon: data.playerPokemon,
      enemyPokemon: data.enemyPokemon,
      pendingPlayerPokemon: null,
      pendingEnemyPokemon: null,
      events: data.events || [],
      eventQueue: data.events || [],
      currentEventIndex: 0,
      battleResult: null,
      pendingBattleEnd: null,
      lossMessage: null,
      trainerClass: data.trainerClass || null,
      faintSwitchParty: data.playerParty || [data.playerPokemon],
      faintSwitchActive: data.playerActive ?? 0,
      battleType: (data.battleType as "wild" | "trainer") || null,
      allowedActions: data.allowedActions || [],
      guaranteedCatch: data.guaranteedCatch || false,
    });
  },

  updateBattleState: (data) => {
    const serverPhase = data.phase as BattlePhase;
    const hasEvents = data.events && data.events.length > 0;
    // Store faint switch data if present
    const faintUpdate: Partial<PokeBattleState> = { battleId: data.battleId, revision: data.revision };
    if (data.playerParty) faintUpdate.faintSwitchParty = data.playerParty;
    if (data.playerActive !== undefined) faintUpdate.faintSwitchActive = data.playerActive;
    if (data.battleType) faintUpdate.battleType = data.battleType as "wild" | "trainer";
    if (data.allowedActions) faintUpdate.allowedActions = data.allowedActions;
    if (data.guaranteedCatch !== undefined) faintUpdate.guaranteedCatch = data.guaranteedCatch;

    if (hasEvents) {
      // Defer the final Pokémon state — keep current HP on screen
      // while events animate. Apply after all events are shown.
      set({
        pendingPlayerPokemon: data.playerPokemon,
        pendingEnemyPokemon: data.enemyPokemon,
        turnNumber: data.turnNumber,
        events: data.events,
        eventQueue: data.events,
        currentEventIndex: 0,
        pendingPhase: serverPhase,
        phase: "animating",
        ...faintUpdate,
      });
    } else {
      set({
        playerPokemon: data.playerPokemon,
        enemyPokemon: data.enemyPokemon,
        turnNumber: data.turnNumber,
        events: data.events || [],
        eventQueue: [],
        currentEventIndex: 0,
        pendingPhase: serverPhase,
        phase: serverPhase,
        pendingPlayerPokemon: null,
        pendingEnemyPokemon: null,
        ...faintUpdate,
      });
    }
  },

  endBattle: (playerWon, blackoutWarp, sentToPC, sentToPCBox, lossMessage) => {
    const { phase, eventQueue } = get();
    const fled = eventQueue.some((e) => e.type === "run_success");
    const caught = eventQueue.some((e) => e.type === "catch_success");
    const result = caught ? "caught" : fled ? "fled" : playerWon ? "win" : "lose";

    const pcUpdates = {
      sentToPC: sentToPC || false,
      sentToPCBox: sentToPCBox ?? null,
    };

    if (phase === "animating") {
      // Don't override animating — queue the end for after events finish
      set({ pendingBattleEnd: { playerWon }, battleResult: result, blackoutWarp: blackoutWarp || null, lossMessage: lossMessage || null, ...pcUpdates });
    } else {
      set({
        phase: "battle_end",
        battleResult: result,
        pendingBattleEnd: null,
        blackoutWarp: blackoutWarp || null,
        lossMessage: lossMessage || null,
        ...pcUpdates,
      });
    }
  },

  closeBattle: () => {
    if (get().isSafari) void closeSafariBattle();
    else void closeOrdinaryBattle();
  },

  setPhase: (phase) => set({ phase }),

  startSafariBattle: (data) => {
    // Build a minimal PokemonDTO for the wild Pokémon
    const wildDTO: PokemonDTO = {
      id: data.pokemon.id,
      name: data.pokemon.name,
      level: data.pokemon.level,
      type1: "",
      type2: "",
      curHp: data.pokemon.hp,
      maxHp: data.pokemon.maxHp,
      attack: 0,
      defense: 0,
      speed: 0,
      special: 0,
      exp: 0,
      expToNextLevel: 0,
      boxSlot: 0,
      status: "",
      isWild: true,
      moves: [],
    };
    // Dummy player "pokemon" — we only need the sprite (player back)
    const playerDummy: PokemonDTO = {
      id: 0,
      name: "Player",
      level: 0,
      type1: "",
      type2: "",
      curHp: 1,
      maxHp: 1,
      attack: 0,
      defense: 0,
      speed: 0,
      special: 0,
      exp: 0,
      expToNextLevel: 0,
      boxSlot: 0,
      status: "",
      isWild: false,
      moves: [],
    };
    set({
      ...initialBattleState,
      presentationGeneration: get().presentationGeneration + 1,
      isInBattle: true,
      isSafari: true,
      battleId: data.battleId ?? "", revision: data.revision ?? 0,
      safariBallsLeft: data.ballsLeft,
      phase: "action_select",
      pendingPhase: "action_select",
      turnNumber: 0,
      playerPokemon: playerDummy,
      enemyPokemon: wildDTO,
      pendingPlayerPokemon: null,
      pendingEnemyPokemon: null,
      events: [],
      eventQueue: [],
      currentEventIndex: 0,
      battleResult: null,
      pendingBattleEnd: null,
      trainerClass: null,
      battleType: "safari",
      allowedActions: [],
      guaranteedCatch: false,
    });
  },

  updateSafariState: (data) => {
    const hasEvents = data.events && data.events.length > 0;
    const updates: Partial<PokeBattleState> = {
      safariBallsLeft: data.ballsLeft,
      sentToPC: data.sentToPC || false,
      sentToPCBox: data.pcBox ?? null,
    };

    if (data.isOver) {
      const result = data.caught ? "caught" : data.fled ? "fled" : "fled";
      if (hasEvents) {
        Object.assign(updates, {
          events: data.events,
          eventQueue: data.events,
          currentEventIndex: 0,
          phase: "animating",
          pendingPhase: "battle_end",
          battleResult: result,
          pendingBattleEnd: { playerWon: data.caught },
        });
      } else {
        Object.assign(updates, {
          phase: "battle_end",
          battleResult: result,
        });
      }
    } else if (hasEvents) {
      Object.assign(updates, {
        events: data.events,
        eventQueue: data.events,
        currentEventIndex: 0,
        phase: "animating",
        pendingPhase: "action_select",
      });
    } else {
      Object.assign(updates, {
        phase: "action_select",
      });
    }
    set(updates);
  },

  advanceEvent: () => {
    const {
      eventQueue,
      currentEventIndex,
      pendingPhase,
      pendingPlayerPokemon,
      pendingEnemyPokemon,
      pendingBattleEnd,
    } = get();
    if (currentEventIndex < eventQueue.length - 1) {
      set({ currentEventIndex: currentEventIndex + 1 });
    } else {
      // All events shown — apply deferred Pokémon state
      const updates: Partial<PokeBattleState> = {
        currentEventIndex: 0,
        eventQueue: [],
        pendingPlayerPokemon: null,
        pendingEnemyPokemon: null,
      };
      if (pendingPlayerPokemon) {
        updates.playerPokemon = pendingPlayerPokemon;
      }
      if (pendingEnemyPokemon) {
        updates.enemyPokemon = pendingEnemyPokemon;
      }

      // Check if there's a move learn prompt in the event queue
      const moveLearnEvent = eventQueue.find((e) => e.type === "move_learn_prompt");
      if (moveLearnEvent) {
        updates.phase = "move_learn_prompt";
        updates.pendingMoveLearn = {
          moveId: moveLearnEvent.newMoveId ?? 0,
          moveName: moveLearnEvent.newMoveName ?? "",
        };
      } else if (pendingBattleEnd) {
        // Battle ended — show the end screen now
        updates.phase = "battle_end";
        updates.pendingBattleEnd = null;
      } else {
        updates.phase = pendingPhase;
      }
      set(updates);
    }
  },
}));

export default usePokeBattleStore;
