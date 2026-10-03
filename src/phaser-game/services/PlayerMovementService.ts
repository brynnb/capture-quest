import type { CutsceneEndResponse, OwnedPlayerPositionResponse, PlayerFacingRequest, PlayerFacingResponse, PlayerStepRequest, PlayerStepResponse, PlayerStepCompleteResponse } from "@/net/generated/protocol";
import * as PhaserNet from "./PhaserNetworkService";
import { correlatedRequest, CorrelatedRequestTimeoutError } from "./CorrelatedRequest";

export function requestPlayerStep(request: Omit<PlayerStepRequest, "requestId">, signal?: AbortSignal): Promise<PlayerStepResponse> {
  return correlatedRequest<PlayerStepResponse>(PhaserNet.onPlayerStep, (requestId) => PhaserNet.requestPlayerStep({ ...request, requestId }), signal);
}

export async function completePlayerStep(stepToken: string, signal?: AbortSignal): Promise<PlayerStepCompleteResponse> {
  const complete = () => correlatedRequest<PlayerStepCompleteResponse>(PhaserNet.onPlayerStepComplete, (requestId) => PhaserNet.completePlayerStep({ stepToken, requestId }), signal);
  try { return await complete(); }
  catch (error) {
    // Only a lost reply retries, once, with the same server-issued authorization.
    // The durable receipt makes this safe even if the first command committed.
    if (!(error instanceof CorrelatedRequestTimeoutError) || signal?.aborted) throw error;
    return complete();
  }
}

export function requestPlayerFacing(request: Omit<PlayerFacingRequest, "requestId">, signal?: AbortSignal): Promise<PlayerFacingResponse> {
  return correlatedRequest<PlayerFacingResponse>(PhaserNet.onPlayerFacing, (requestId) => PhaserNet.requestPlayerFacing({ ...request, requestId }), signal);
}

// Position recovery is a read serialized after any preceding accepted command.
export function readOwnedPlayerPosition(signal?: AbortSignal, stepToken?: string): Promise<OwnedPlayerPositionResponse> {
  return correlatedRequest<OwnedPlayerPositionResponse>(PhaserNet.onOwnedPlayerPosition, (requestId) => PhaserNet.requestOwnedPlayerPosition({ requestId, ...(stepToken ? { stepToken } : {}) }), signal);
}
export function completeCutscene(scriptLabel: string, completionToken: string, signal?: AbortSignal): Promise<CutsceneEndResponse> {
  return correlatedRequest<CutsceneEndResponse>(PhaserNet.onCutsceneEnd, (requestId) => PhaserNet.completeCutscene({ scriptLabel, completionToken, requestId }), signal);
}
