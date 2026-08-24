import { afterEach, describe, expect, it, vi } from "vitest";

import useGameStatusStore from "@/stores/GameStatusStore";
import { TileViewerInteractionController } from "./TileViewerInteractionController";

describe("TileViewerInteractionController editor gesture handoff", () => {
  afterEach(() => {
    useGameStatusStore.setState({ isTileManagerOpen: false });
  });

  it("leaves the camera gesture marker for TileViewer while the editor is open", () => {
    const consumePointerGesture = vi.fn(() => true);
    const controller = new TileViewerInteractionController({
      cameraController: () => ({ consumePointerGesture }),
    } as never);

    useGameStatusStore.setState({ isTileManagerOpen: true });
    (controller as unknown as { handlePointerUp: (pointer: unknown) => void })
      .handlePointerUp({ id: 4, getDistance: () => 30 });

    expect(consumePointerGesture).not.toHaveBeenCalled();
  });
});
