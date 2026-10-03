import * as PhaserNet from "./PhaserNetworkService";

interface CorrelatedErrorResponse { success: false; requestId: string; error: string }
export class CorrelatedResponseError extends Error {
  constructor(public readonly response: CorrelatedErrorResponse) { super(response.error); }
}
const REQUEST_TIMEOUT_MS = 10000;

let requestSequence = 0;

// Correlation and one settlement boundary cover reads, arrival and movement.
// A local cancellation cannot undo a server commit; a later load reads owned state.
export function correlatedRequest<T extends { success: true; requestId: string }>(
  subscribe: (receive: (response: T | CorrelatedErrorResponse) => void) => () => void,
  send: (requestId: string) => void,
  signal?: AbortSignal,
): Promise<T> {
  if (!PhaserNet.isConnected()) return Promise.reject(new Error("Not connected to server - please log in first"));
  if (signal?.aborted) return Promise.reject(new DOMException("Request cancelled", "AbortError"));
  const requestId = `map:${Date.now()}:${++requestSequence}`;
  return new Promise<T>((resolve, reject) => {
    let settled = false;
    let unsubscribe = () => {};
    let timeout: ReturnType<typeof setTimeout> | undefined;
    const finish = (error?: Error, response?: T) => {
      if (settled) return;
      settled = true;
      unsubscribe();
      if (timeout !== undefined) clearTimeout(timeout);
      signal?.removeEventListener("abort", abort);
      if (error) reject(error);
      else resolve(response!);
    };
    const abort = () => finish(new DOMException("Request cancelled", "AbortError"));
    unsubscribe = subscribe((response) => {
      if (!response || response.requestId !== requestId) return;
      if (response.success === false) finish(new CorrelatedResponseError(response));
      else if (response.success === true) finish(undefined, response);
      else finish(new Error("Invalid correlated response"));
    });
    // Subscription APIs do not emit on registration, but handle that boundary
    // explicitly so cleanup also remains correct if a consumer changes them.
    if (settled) { unsubscribe(); return; }
    signal?.addEventListener("abort", abort, { once: true });
    if (signal?.aborted) { abort(); return; }
    timeout = setTimeout(() => finish(new Error("Timeout waiting for server response")), REQUEST_TIMEOUT_MS);
    try { send(requestId); } catch (error) { finish(error instanceof Error ? error : new Error(String(error))); }
  });
}

