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
export async function completeCutscene(scriptLabel: string, completionToken: string, signal?: AbortSignal, cancel = false): Promise<CutsceneEndResponse> {
  const complete = () => correlatedRequest<CutsceneEndResponse>(PhaserNet.onCutsceneEnd, (requestId) => PhaserNet.completeCutscene({ scriptLabel, completionToken, requestId, ...(cancel ? { cancel: true } : {}) }), signal);
  try { return await complete(); }
  catch (error) {
    // The durable script receipt makes one lost-reply retry safe. Explicit
    // rejection and retirement never authorize another attempt.
    if (!(error instanceof CorrelatedRequestTimeoutError) || signal?.aborted) throw error;
    return complete();
  }
}

export function requestBicycleState(
  characterId: number,
  signal: AbortSignal,
  command?: { instanceId: number; wantsRiding: boolean; revision: number },
): Promise<import("@/net/generated/world_api").BicycleStateResponse> {
  return correlatedRequest<import("@/net/generated/world_api").BicycleStateResponse>(
    PhaserNet.onBicycleState,
    requestId => PhaserNet.requestBicycleState({ requestId, characterId, ...command }),
    signal,
  );
}
