import {WorldSocket} from "@/net/index";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import * as PhaserNet from "./PhaserNetworkService";

interface CorrelatedErrorResponse { success: false; requestId: string; error: string }
export class CorrelatedResponseError extends Error {
  constructor(public readonly response: CorrelatedErrorResponse) { super(response.error); }
}
export class CorrelatedRequestTimeoutError extends Error {
  constructor() { super("Timeout waiting for server response"); }
}
const REQUEST_TIMEOUT_MS = 10000;

let requestSequence = 0;

// Correlation and one settlement boundary cover reads, arrival and movement.
// A local cancellation cannot undo a server commit; a later load reads owned state.
export function correlatedRequest<T extends { success: true; requestId: string }>(
  subscribe: (receive: (response: T | CorrelatedErrorResponse) => void) => () => void,
  send: (requestId: string) => void | Promise<void>,
  signal?: AbortSignal,
  timeoutMs: number = REQUEST_TIMEOUT_MS,
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
    timeout = setTimeout(() => finish(new CorrelatedRequestTimeoutError()), timeoutMs);
    try { Promise.resolve(send(requestId)).catch((error) => finish(error instanceof Error ? error : new Error(String(error)))); } catch (error) { finish(error instanceof Error ? error : new Error(String(error))); }
  });
}



// Reads owned by the active character share one retirement boundary. Resource
// caches and source-specific presentation remain their domain callers' policy.
export async function readForCurrentCharacter<T>(read:(characterId:number,signal:AbortSignal)=>Promise<T>,signal?:AbortSignal):Promise<T>{
 const characterId=usePlayerCharacterStore.getState().characterProfile.id;
 if(!characterId || useGameScreenStore.getState().currentScreen!=="game")throw new Error("Read requires an active character and game screen");
 const generation=WorldSocket.sessionGeneration;
 const controller=new AbortController();const abort=()=>controller.abort();
 signal?.addEventListener("abort",abort,{once:true});if(signal?.aborted)abort();
 const stops=[WorldSocket.subscribeSessionRetirement(abort),usePlayerCharacterStore.subscribe(state=>{if(state.characterProfile.id!==characterId)abort();}),useGameScreenStore.subscribe(state=>{if(state.currentScreen!=="game")abort();})];
 try {
 if(controller.signal.aborted)throw new DOMException("Request cancelled","AbortError");
 const result=await read(characterId,controller.signal);
 if(controller.signal.aborted || WorldSocket.sessionGeneration!==generation)throw new DOMException("Request cancelled","AbortError");
 return result;
 } finally {signal?.removeEventListener("abort",abort);stops.forEach(stop=>stop());}
}
