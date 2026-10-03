import { afterEach, beforeEach, expect, test, vi } from "vitest";
const requests = vi.hoisted(() => ({ complete: vi.fn(), read: vi.fn() }));
vi.mock("./PlayerMovementService", () => ({ completeCutscene: requests.complete, readOwnedPlayerPosition: requests.read }));
vi.mock("./PhaserNetworkService", () => ({ isConnected: () => true }));
vi.mock("./DialogueService", () => ({ fetchDialogue: vi.fn(), parseDialogueText: vi.fn() }));
vi.mock("@/services/audio/AudioManager", () => ({ default: { playSFX: vi.fn(), playMusic: vi.fn() } }));
import { CorrelatedResponseError } from "./CorrelatedRequest";
import { cancelActiveCutscene, getLastCompletedCutsceneScriptLabel, handleCutsceneStart, isCutscenePlaying, registerCutsceneCallbacks, unregisterCutsceneCallbacks } from "./CutsceneService";
function deferred<T>() { let resolve!: (value: T) => void; let reject!: (error: Error) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject }; }
const owned = { serverMovementPending: false, mapId: 50, x: 7, y: 8, direction: "UP" };
const event = (label: string) => ({ scriptLabel: label, completionToken: `token:${label}`, mapName: "ROOM", actions: [{ type: "lockInput" }, { type: "unlockInput" }] });
let lock: ReturnType<typeof vi.fn>, reconcile: ReturnType<typeof vi.fn>, cancel: ReturnType<typeof vi.fn>;
beforeEach(() => {
  requests.complete.mockReset(); requests.read.mockReset();
  lock = vi.fn(); reconcile = vi.fn(async () => { }); cancel = vi.fn();
  registerCutsceneCallbacks({ onMove: vi.fn(async () => { }), onShowActor: vi.fn(), onHideActor: vi.fn(), onFace: vi.fn(), onInputLock: lock, onReconcile: reconcile, onCancelPlayback: cancel });
  vi.spyOn(console, "warn").mockImplementation(() => { });
});
afterEach(() => { unregisterCutsceneCallbacks(); vi.restoreAllMocks() });
test("input remains locked through commit response and projection", async () => {
  const completion = deferred<unknown>(), projection = deferred<void>();
  requests.complete.mockReturnValue(completion.promise); reconcile.mockReturnValue(projection.promise);
  const run = handleCutsceneStart(event("Confirmed"));
  await vi.waitFor(() => expect(requests.complete).toHaveBeenCalledOnce());
  expect(lock).not.toHaveBeenCalledWith(false); expect(isCutscenePlaying()).toBe(true);
  completion.resolve({ ...owned, success: true, requestId: "id", completed: true });
  await vi.waitFor(() => expect(reconcile).toHaveBeenCalledWith(expect.objectContaining(owned)));
  expect(lock).not.toHaveBeenCalledWith(false);
  projection.resolve(); await run;
  expect(getLastCompletedCutsceneScriptLabel()).toBe("Confirmed"); expect(isCutscenePlaying()).toBe(false); expect(lock).toHaveBeenLastCalledWith(false);
});
test("late failure reconciles owned source without claiming completion", async () => {
  requests.complete.mockRejectedValue(new CorrelatedResponseError({ ...owned, success: false, requestId: "id", error: "rolled back" }));
  await handleCutsceneStart(event("Failed"));
  expect(reconcile).toHaveBeenCalledWith(expect.objectContaining(owned)); expect(requests.read).not.toHaveBeenCalled(); expect(getLastCompletedCutsceneScriptLabel()).not.toBe("Failed"); expect(lock).toHaveBeenLastCalledWith(false);
});
test("unknown commit outcome reads owned position before unlocking", async () => {
  const recovery = deferred<unknown>(); requests.complete.mockRejectedValue(new Error("Timeout")); requests.read.mockReturnValue(recovery.promise);
  const run = handleCutsceneStart(event("Unknown"));
  await vi.waitFor(() => expect(requests.read).toHaveBeenCalledOnce()); expect(lock).not.toHaveBeenCalledWith(false);
  recovery.resolve({ ...owned, x: 9, success: true, requestId: "read" }); await run;
  expect(reconcile).toHaveBeenCalledWith(expect.objectContaining({ x: 9 })); expect(lock).toHaveBeenLastCalledWith(false);
});
test("failed recovery keeps input locked until explicit retirement", async () => {
  requests.complete.mockRejectedValue(new Error("Timeout")); requests.read.mockRejectedValue(new Error("disconnected"));
  await handleCutsceneStart(event("Uncertain")); expect(isCutscenePlaying()).toBe(true); expect(lock).not.toHaveBeenCalledWith(false);
  cancelActiveCutscene("quit"); expect(isCutscenePlaying()).toBe(false); expect(lock).toHaveBeenLastCalledWith(false);
});
test("retirement aborts the listener and ignores a late commit response", async () => {
  const completion = deferred<unknown>(); requests.complete.mockReturnValue(completion.promise);
  const run = handleCutsceneStart(event("Retired")); await vi.waitFor(() => expect(requests.complete).toHaveBeenCalledOnce());
  const signal = requests.complete.mock.calls[0][2] as AbortSignal;
  unregisterCutsceneCallbacks(); expect(signal.aborted).toBe(true); expect(cancel).toHaveBeenCalled();
  completion.resolve({ ...owned, success: true, requestId: "id", completed: true }); await run;
  expect(reconcile).not.toHaveBeenCalled(); expect(requests.read).not.toHaveBeenCalled(); expect(getLastCompletedCutsceneScriptLabel()).not.toBe("Retired");
});
test("next issued script published before acknowledgement runs once afterward", async () => {
  const completion = deferred<unknown>(); requests.complete.mockReturnValueOnce(completion.promise).mockResolvedValue({ ...owned, success: true, requestId: "next", completed: true });
  const run = handleCutsceneStart(event("First")); await vi.waitFor(() => expect(requests.complete).toHaveBeenCalledOnce());
  await handleCutsceneStart(event("First")); await handleCutsceneStart(event("Next")); await handleCutsceneStart(event("Next"));
  expect(requests.complete).toHaveBeenCalledOnce(); completion.resolve({ ...owned, success: true, requestId: "first", completed: true }); await run;
  await vi.waitFor(() => expect(getLastCompletedCutsceneScriptLabel()).toBe("Next")); expect(requests.complete).toHaveBeenCalledTimes(2);
});

test("accepted ineligible completion reconciles without claiming the script completed", async () => {
  requests.complete.mockResolvedValue({ ...owned, success: true, requestId: "id", completed: false });
  await handleCutsceneStart(event("Ineligible")); expect(getLastCompletedCutsceneScriptLabel()).not.toBe("Ineligible"); expect(reconcile).toHaveBeenCalled(); expect(lock).toHaveBeenLastCalledWith(false);
});
test("retiring an active animation settles playback without completing or recovering the old scene", async () => {
  const animation = deferred<void>(); const move = vi.fn(() => animation.promise);
  registerCutsceneCallbacks({ onMove: move, onShowActor: vi.fn(), onHideActor: vi.fn(), onFace: vi.fn(), onInputLock: lock, onReconcile: reconcile, onCancelPlayback: () => { cancel(); animation.resolve() } });
  const run = handleCutsceneStart({ ...event("Animation"), actions: [{ type: "movePlayer", movements: ["RIGHT"] }] });
  await vi.waitFor(() => expect(move).toHaveBeenCalledOnce()); cancelActiveCutscene("quit during animation"); await run;
  expect(cancel).toHaveBeenCalled(); expect(requests.complete).not.toHaveBeenCalled(); expect(requests.read).not.toHaveBeenCalled(); expect(reconcile).not.toHaveBeenCalled();
});


test("failed active animation cancels its issued plan before unlocking; retired scenes do not cancel", async () => {
  const animation = vi.fn(async () => { throw new Error("animation interrupted") });
  const cancellation = deferred<unknown>();
  requests.complete.mockReturnValue(cancellation.promise);
  registerCutsceneCallbacks({ onMove: animation, onShowActor: vi.fn(), onHideActor: vi.fn(), onFace: vi.fn(), onInputLock: lock, onReconcile: reconcile, onCancelPlayback: cancel });
  const log = vi.spyOn(console, "error").mockImplementation(() => {});
  const run = handleCutsceneStart({ ...event("Cancel"), actions: [{ type: "movePlayer", movements: ["RIGHT"] }] });
  await vi.waitFor(() => expect(requests.complete).toHaveBeenCalledWith("Cancel", "token:Cancel", expect.any(AbortSignal), true));
  expect(lock).not.toHaveBeenCalledWith(false);
  cancellation.resolve({ ...owned, success: true, completed: false, replayed: false });
  await run;
  expect(reconcile).toHaveBeenCalled(); expect(lock).toHaveBeenLastCalledWith(false);
  expect(requests.read).not.toHaveBeenCalled(); log.mockRestore();
});
