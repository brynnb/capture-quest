import { afterEach, expect, test, vi } from "vitest";
import { NetworkBridge } from "./NetworkBridge";
import { WorldSocket } from "./index";
import { PokeSurfingResponse } from "./generated/opcodes";

afterEach(() => vi.restoreAllMocks());

test.each([false, true])("committed Surf result with blackout=%s gives presentation to the correct outcome", (blackout) => {
  NetworkBridge.initialize();
  const dispatch = vi.spyOn(window, "dispatchEvent");
  WorldSocket.onJson?.(PokeSurfingResponse, {
    success: true, encounter: blackout, blackout,
    mapId: 50, x: 8, y: 8, direction: "RIGHT",
  });
  const animations = dispatch.mock.calls.filter(([event]) => event.type === "pokeSurfingSuccess");
  if (blackout) {
    expect(animations).toHaveLength(0);
  } else {
    expect(animations).toHaveLength(1);
    expect((animations[0][0] as CustomEvent).detail).toEqual({ mapId: 50, x: 8, y: 8, direction: "RIGHT" });
  }
});
