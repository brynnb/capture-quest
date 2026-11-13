import { afterEach, expect, test, vi } from "vitest";
import { NetworkBridge } from "./NetworkBridge";
import { WorldSocket } from "./index";
import { PokeSurfingResponse } from "./generated/opcodes";
import { onSurf } from "@/phaser-game/services/PhaserNetworkService";

afterEach(() => vi.restoreAllMocks());

test.each([false, true])("SURF blackout=%s delivers only to its scoped subscribers", async (blackout) => {
  NetworkBridge.initialize();
  const receive=vi.fn(); const stop=onSurf(receive);
  const dispatch=vi.spyOn(window,"dispatchEvent");
  const reply={success:true,requestId:"surf-owned",characterId:42,encounter:blackout,blackout};
  WorldSocket.onJson?.(PokeSurfingResponse,reply);
  await vi.waitFor(()=>expect(receive).toHaveBeenCalledWith(reply));
  expect(dispatch.mock.calls.filter(([event])=>event.type==="pokeSurfingSuccess")).toHaveLength(0);
  stop(); receive.mockClear();
  WorldSocket.onJson?.(PokeSurfingResponse,reply);
  await Promise.resolve(); await Promise.resolve();
  expect(receive).not.toHaveBeenCalled();
});
