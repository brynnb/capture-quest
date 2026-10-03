import type { GameplayStateResponse } from "@/net/generated/world_api";
import { OpCodes } from "@/net";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import { correlatedRequest } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { handleCutsceneStart } from "./CutsceneService";

// Apply only a correlated, source-matching read owned by this scene. No handler
// applies arbitrary recovery packets globally; retirement aborts the listener.
export async function recoverGameplayState(mapId: number, signal?: AbortSignal): Promise<GameplayStateResponse> {
  for (let attempt = 0; attempt < 2; attempt++) {
    const before = usePokeBattleStore.getState();
    const snapshot = await correlatedRequest<GameplayStateResponse>(PhaserNet.onGameplayState, requestId => PhaserNet.requestGameplayState({ mapId, requestId }), signal);
    if (signal?.aborted) throw new DOMException("Scene retired", "AbortError");
    if (snapshot.position.mapId !== mapId) throw new Error("Gameplay recovery map differs from loaded ownership");
    // A newer battle event/choice can overtake a read while its reply is delayed.
    // Read once more instead of overwriting that new local presentation.
    if (usePokeBattleStore.getState() !== before) continue;
    before.restoreGameplay(snapshot);
    window.dispatchEvent(new CustomEvent("safariZoneEnter", { detail: snapshot.safari
      ? { success: true, ballsLeft: snapshot.safari.ballsLeft, stepsLeft: snapshot.safari.stepsLeft }
      : { success: false } }));
    if (snapshot.trainer) PhaserNet.dispatchPhaserResponse(OpCodes.TrainerEncounterNotify, snapshot.trainer);
    if (snapshot.cutscene) void handleCutsceneStart(snapshot.cutscene);
    return snapshot;
  }
  throw new Error("Gameplay changed during recovery; a new recovery read is required");
}
