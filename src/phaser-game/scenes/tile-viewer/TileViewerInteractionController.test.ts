import { afterEach, describe, expect, it, vi } from "vitest";

import useGameStatusStore from "@/stores/GameStatusStore";
import { TileViewerInteractionController } from "./TileViewerInteractionController";

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
