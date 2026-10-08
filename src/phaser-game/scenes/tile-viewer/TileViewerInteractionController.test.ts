import { afterEach, describe, expect, it, vi } from "vitest";

import useGameStatusStore from "@/stores/GameStatusStore";
import { TileViewerInteractionController } from "./TileViewerInteractionController";
import type { PhaserInstantWarpResponse } from "@/net/generated/protocol";

describe("TileViewerInteractionController editor gesture handoff", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    useGameStatusStore.setState({
      isTileManagerOpen: false,
      isWarpMode: false,
      isMapLoading: false,
      pendingInstantWarpTarget: null,
    });
  });

  it("leaves the camera gesture marker for TileViewer while the editor is open", () => {
    const consumePointerGesture = vi.fn(() => true);
    const controller = new TileViewerInteractionController({
      cameraController: () => ({ consumePointerGesture }),
    } as never);

    useGameStatusStore.setState({ isTileManagerOpen: true });
    (
      controller as unknown as { handlePointerUp: (pointer: unknown) => void }
    ).handlePointerUp({ id: 4, getDistance: () => 30 });

    expect(consumePointerGesture).not.toHaveBeenCalled();
  });
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

describe("Instant Warp committed response presentation", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    useGameStatusStore.setState({ isWarpMode: false });
  });

  function pendingActivation() {
    const response = deferred<PhaserInstantWarpResponse>();
    const instantWarp = vi.fn(() => response.promise);
    const active = { value: true };
    const stopMovement = vi.fn();
    const controller = new TileViewerInteractionController({
      scene: { sys: { isActive: () => active.value } },
      getPlayerActor: () => null,
      playerMovementController: () => ({ stopMovement, getCurrentDirection: () => "LEFT", getCurrentMapId: () => 38 }),
      mapDataService: () => ({ instantWarp }),
    } as never);
    const activate = (controller as unknown as {
      commitInstantWarp: (target: { mapId: number; x: number; y: number }) => Promise<void>;
    }).commitInstantWarp.bind(controller);
    return { controller, response, instantWarp, active, stopMovement, activate };
  }

  it("publishes only the committed result and rejects concurrent activation", async () => {
    const dispatch = vi.spyOn(window, "dispatchEvent");
    const { controller, response, instantWarp, activate } = pendingActivation();
    useGameStatusStore.setState({ isWarpMode: true });
    const pending = activate({ mapId: 50, x: 3, y: 4 });
    await activate({ mapId: 60, x: 5, y: 6 });
    expect(instantWarp).toHaveBeenCalledOnce();
    expect(controller.isInstantWarpPending()).toBe(true);
    expect(useGameStatusStore.getState().isWarpMode).toBe(true);
    expect(dispatch).not.toHaveBeenCalled();
    const result = { success: true as const, requestId: "owned", mapId: 50, x: 3, y: 4, direction: "LEFT" };
    response.resolve(result);
    await pending;
    expect(controller.isInstantWarpPending()).toBe(false);
    expect(useGameStatusStore.getState().isWarpMode).toBe(false);
    expect((dispatch.mock.calls[0][0] as CustomEvent).detail).toEqual({ ...result, serverCommitted: true });
  });

  it("does not present a response after its scene has retired", async () => {
    const dispatch = vi.spyOn(window, "dispatchEvent");
    const { controller, response, active, activate } = pendingActivation();
    const pending = activate({ mapId: 50, x: 3, y: 4 });
    active.value = false;
    response.resolve({ success: true, requestId: "retired", mapId: 50, x: 3, y: 4, direction: "LEFT" });
    await pending;
    expect(dispatch).not.toHaveBeenCalled();
    expect(controller.isInstantWarpPending()).toBe(false);
  });
});

function createInstantWarpController(
  ensureDisplayedTileAvailable: (x: number, y: number) => Promise<boolean>,
) {
  const setLoadingText = vi.fn();
  const hideLoadingText = vi.fn();
  const errorTimer = { destroy: vi.fn() };
  const delayedCall = vi.fn(() => errorTimer);
  const controller = new TileViewerInteractionController({
    scene: { time: { delayedCall } },
    uiManager: () => ({ setLoadingText, hideLoadingText }),
    getDisplayedMapId: () => 9999,
    getWorldInputFreezeReason: () => "map_view",
    ensureDisplayedTileAvailable,
  } as never);
  const commitInstantWarp = vi.fn();
  (
    controller as unknown as {
      commitInstantWarp: typeof commitInstantWarp;
    }
  ).commitInstantWarp = commitInstantWarp;
  return {
    controller,
    commitInstantWarp,
    delayedCall,
    errorTimer,
    hideLoadingText,
    setLoadingText,
  };
}

function useCompactTouchLayout(): void {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({ matches: true })),
  );
}

describe("TileViewerInteractionController streamed instant warp", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    useGameStatusStore.setState({
      isWarpMode: false,
      isMapLoading: false,
      pendingInstantWarpTarget: null,
    });
  });

  it("does not commit a tile fetch that finishes after warp mode closes", async () => {
    const available = deferred<boolean>();
    const { controller, commitInstantWarp } = createInstantWarpController(
      () => available.promise,
    );
    useGameStatusStore.setState({ isWarpMode: true, isMapLoading: false });

    const pending = (
      controller as unknown as {
        handleWarpModeClick: (x: number, y: number) => Promise<void>;
      }
    ).handleWarpModeClick(16, 32);
    useGameStatusStore.getState().setWarpMode(false);
    available.resolve(true);
    await pending;

    expect(commitInstantWarp).not.toHaveBeenCalled();
  });

  it("lets only the newest asynchronous tile selection commit", async () => {
    const first = deferred<boolean>();
    const second = deferred<boolean>();
    const { controller, commitInstantWarp } = createInstantWarpController(
      (x) => (x === 1 ? first.promise : second.promise),
    );
    useGameStatusStore.setState({ isWarpMode: true, isMapLoading: false });
    const select = (
      controller as unknown as {
        handleWarpModeClick: (x: number, y: number) => Promise<void>;
      }
    ).handleWarpModeClick.bind(controller);

    const firstSelection = select(16, 16);
    const secondSelection = select(32, 16);
    second.resolve(true);
    await secondSelection;
    first.resolve(true);
    await firstSelection;

    expect(commitInstantWarp).toHaveBeenCalledOnce();
    expect(commitInstantWarp).toHaveBeenCalledWith({ mapId: 9999, x: 2, y: 1 });
  });

  it("does not clear a newer pending mobile target after confirmation races", async () => {
    const available = deferred<boolean>();
    const { controller, commitInstantWarp } = createInstantWarpController(
      () => available.promise,
    );
    const original = { mapId: 9999, x: 1, y: 1 };
    const replacement = { mapId: 9999, x: 2, y: 2 };
    useGameStatusStore.setState({
      isWarpMode: true,
      isMapLoading: false,
      pendingInstantWarpTarget: original,
    });

    const confirmation = (
      controller as unknown as {
        confirmPendingInstantWarp: () => Promise<void>;
      }
    ).confirmPendingInstantWarp();
    useGameStatusStore.getState().setPendingInstantWarpTarget(replacement);
    available.resolve(true);
    await confirmation;

    expect(commitInstantWarp).not.toHaveBeenCalled();
    expect(useGameStatusStore.getState().pendingInstantWarpTarget).toBe(
      replacement,
    );
  });

  it("clears the previous mobile target when a new selection is not a tile", async () => {
    useCompactTouchLayout();
    const { controller, commitInstantWarp } = createInstantWarpController(
      async () => false,
    );
    useGameStatusStore.setState({
      isWarpMode: true,
      isMapLoading: false,
      pendingInstantWarpTarget: { mapId: 9999, x: 1, y: 1 },
    });

    await (
      controller as unknown as {
        handleWarpModeClick: (x: number, y: number) => Promise<void>;
      }
    ).handleWarpModeClick(32, 32);

    expect(useGameStatusStore.getState().pendingInstantWarpTarget).toBeNull();
    expect(commitInstantWarp).not.toHaveBeenCalled();
  });

  it("turns a rejected mobile selection into a visible retry status", async () => {
    useCompactTouchLayout();
    const { controller, commitInstantWarp, delayedCall, setLoadingText } =
      createInstantWarpController(async () => {
        throw new Error("tile request timed out");
      });
    useGameStatusStore.setState({
      isWarpMode: true,
      isMapLoading: false,
      pendingInstantWarpTarget: { mapId: 9999, x: 1, y: 1 },
    });

    await expect(
      (
        controller as unknown as {
          handleWarpModeClick: (x: number, y: number) => Promise<void>;
        }
      ).handleWarpModeClick(32, 32),
    ).resolves.toBeUndefined();

    expect(useGameStatusStore.getState().pendingInstantWarpTarget).toBeNull();
    expect(commitInstantWarp).not.toHaveBeenCalled();
    expect(setLoadingText).toHaveBeenCalledWith(
      "Couldn't load that tile. Tap a tile again to retry.",
    );
    expect(delayedCall).toHaveBeenCalledWith(4000, expect.any(Function));
  });

  it("keeps a pending target retryable when confirmation verification rejects", async () => {
    useCompactTouchLayout();
    const target = { mapId: 9999, x: 4, y: 5 };
    const { controller, commitInstantWarp, setLoadingText } =
      createInstantWarpController(async () => {
        throw new Error("tile request timed out");
      });
    useGameStatusStore.setState({
      isWarpMode: true,
      isMapLoading: false,
      pendingInstantWarpTarget: target,
    });

    await expect(
      (
        controller as unknown as {
          confirmPendingInstantWarp: () => Promise<void>;
        }
      ).confirmPendingInstantWarp(),
    ).resolves.toBeUndefined();

    expect(useGameStatusStore.getState().pendingInstantWarpTarget).toBe(target);
    expect(commitInstantWarp).not.toHaveBeenCalled();
    expect(setLoadingText).toHaveBeenCalledWith(
      "Couldn't verify that tile. Tap Confirm Warp to retry.",
    );
  });
});


describe("actor dialogue demand ownership",()=>{
 afterEach(()=>vi.restoreAllMocks());
 it("superseded dialogue and moved sources cannot open a stale box",async()=>{
 const dialogue=await import("../../services/DialogueService");
 const net=await import("../../services/PhaserNetworkService");
 const store=(await import("@/stores/PokemonDialogueStore")).default;
 vi.spyOn(net,"tryScriptedEventInteraction").mockResolvedValue(false);
 const pending=deferred<import("../../services/DialogueService").DialogueResult>();
 const fetch=vi.spyOn(dialogue,"fetchDialogueWithBranching").mockReturnValue(pending.promise);
 const open=vi.spyOn(store.getState(),"openDialogue");
 const actor={id:7,x:1,y:0,mapId:1,text:"TEXT",objectType:"sign"};
 const controller=new TileViewerInteractionController({isWorldInputFrozen:()=>false,currentActorById:()=>actor,getDisplayedMapId:()=>1,playerMovementController:()=>({getCurrentPosition:()=>({x:0,y:0})})} as never);
 const internal=controller as unknown as {ensureActorInteractionReachable:()=>Promise<boolean>;handleActorClicked:(actor:unknown)=>Promise<void>};
 vi.spyOn(internal,"ensureActorInteractionReachable").mockResolvedValue(true);
 const first=internal.handleActorClicked(actor);
 await vi.waitFor(()=>expect(fetch).toHaveBeenCalledOnce());
 const signal=fetch.mock.calls[0][1]!;
 actor.x=3;
 pending.resolve({lines:["Old"],hasBranching:false,branchingPrompt:null});await first;
 expect(open).not.toHaveBeenCalled();expect(signal.aborted).toBe(false);
 const held=deferred<import("../../services/DialogueService").DialogueResult>();fetch.mockReturnValueOnce(held.promise).mockResolvedValue({lines:[],hasBranching:false,branchingPrompt:null});
 const old=internal.handleActorClicked(actor);await vi.waitFor(()=>expect(fetch).toHaveBeenCalledTimes(2));
 const oldSignal=fetch.mock.calls[1][1]!;
 await internal.handleActorClicked(actor);expect(oldSignal.aborted).toBe(true);
 held.resolve({lines:["Old"],hasBranching:false,branchingPrompt:null});await old;expect(open).not.toHaveBeenCalled();
 });
});

describe("trainer dialogue source ownership",()=>{
 afterEach(()=>vi.restoreAllMocks());
 it("rejects late moved-source replies and fences the later battle callback",async()=>{
 const trainer=await import("../../services/TrainerInteractionService");
 const net=await import("../../services/PhaserNetworkService");
 const store=(await import("@/stores/PokemonDialogueStore")).default;
 vi.spyOn(net,"tryScriptedEventInteraction").mockResolvedValue(false);
 const send=vi.spyOn(net,"sendTrainerBattleStart").mockImplementation(()=>{});
 const open=vi.spyOn(store.getState(),"openDialogue").mockImplementation(()=>{});
 const actor={id:7,x:1,y:0,mapId:1,text:"TEXT",objectType:"npc",trainerClass:"YOUNGSTER",trainerPartyIndex:1};
 const reply={success:true as const,requestId:"owned",characterId:42,trainerActorId:7,trainerName:"Trainer",trainerClass:"YOUNGSTER",dialogue:"Battle!",shouldBattle:true,defeated:false};
 const pending=deferred<typeof reply>();const read=vi.spyOn(trainer,"readTrainerInteraction").mockReturnValueOnce(pending.promise).mockResolvedValue(reply);
 const controller=new TileViewerInteractionController({isWorldInputFrozen:()=>false,currentActorById:()=>actor,getDisplayedMapId:()=>1,playerMovementController:()=>({getCurrentPosition:()=>({x:0,y:0})})} as never);
 const internal=controller as unknown as {ensureActorInteractionReachable:()=>Promise<boolean>;handleActorClicked:(actor:unknown)=>Promise<void>};
 vi.spyOn(internal,"ensureActorInteractionReachable").mockResolvedValue(true);
 const first=internal.handleActorClicked(actor);await vi.waitFor(()=>expect(read).toHaveBeenCalledOnce());actor.x=3;pending.resolve(reply);await first;expect(open).not.toHaveBeenCalled();
 actor.x=1;await internal.handleActorClicked(actor);const stale=open.mock.calls.at(-1)![3]!;actor.x=3;stale();expect(send).not.toHaveBeenCalled();
 actor.x=1;await internal.handleActorClicked(actor);const current=open.mock.calls.at(-1)![3]!;current();expect(send).toHaveBeenCalledTimes(1);expect(send).toHaveBeenCalledWith(7);
 await internal.handleActorClicked(actor);current();expect(send).toHaveBeenCalledTimes(1);
 });
});
