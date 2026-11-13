import { beforeEach, afterEach, expect, test, vi } from "vitest";
const network = vi.hoisted(() => ({ listeners: new Set<(reply: unknown) => void>(), send: vi.fn() }));
vi.mock("@/net", () => ({ OpCodes: {}, WorldSocket: {} }));
vi.mock("./PhaserNetworkService", () => ({
  isConnected: () => true,
  onPreferences: (receive: (reply: unknown) => void) => { network.listeners.add(receive); return () => network.listeners.delete(receive); },
  requestPreferences: network.send,
}));
import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import { setTrainerPreference } from "./PreferenceCommandService";
const reply = (requestId: string, revision = 1, enabled = true) => ({ success: true, requestId, characterId: 42, revision, allowTrainerRebattles: enabled, showNetworkStats: true });
const receive = (data: unknown) => network.listeners.forEach(listener => listener(data));
beforeEach(() => { network.send.mockReset(); useGameScreenStore.getState().setScreen("game"); usePlayerCharacterStore.getState().setCharacterProfile({ id: 42 }); useGameStatusStore.setState({ preferenceRevision: 0, allowTrainerRebattles: false, preferenceRecoveryRequired: false, preferenceCommandPending: false }); });
afterEach(() => { vi.useRealTimers(); network.listeners.clear(); });
test("preference changes only after confirmed reply and ignores rapid second command", async () => {
 const run = setTrainerPreference(true); await setTrainerPreference(false);
 expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(false);
 expect(network.send).toHaveBeenCalledTimes(1);
 receive(reply(network.send.mock.calls[0][0].requestId)); await run;
 expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(true);
 expect(network.listeners.size).toBe(0);
});
test("lost reply recovers current state without resending mutation or applying late acknowledgement", async () => {
 vi.useFakeTimers(); const run = setTrainerPreference(true);
 const first = network.send.mock.calls[0][0];
 await vi.advanceTimersByTimeAsync(10000);
 expect(network.send).toHaveBeenCalledTimes(2);
 const read = network.send.mock.calls[1][0]; expect(read.current).toBe(true);
 receive(reply(first.requestId)); expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(false);
 receive(reply(read.requestId)); await run;
 expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(true);
 expect(network.listeners.size).toBe(0);
});
test("quit retires pending work and same-character reentry cannot apply its reply", async () => {
 const run = setTrainerPreference(true); const first = network.send.mock.calls[0][0];
 useGameScreenStore.getState().setScreen("characterSelect");
 useGameScreenStore.getState().setScreen("game");
 receive(reply(first.requestId)); await run;
 expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(false);
 expect(network.listeners.size).toBe(0);
});
test("failed recovery holds mutation admission until a later current read succeeds", async () => {
 vi.useFakeTimers(); const run = setTrainerPreference(true);
 await vi.advanceTimersByTimeAsync(20000); await run;
 expect(useGameStatusStore.getState().preferenceRecoveryRequired).toBe(true);
 const recover = setTrainerPreference(true); const request = network.send.mock.calls[2][0];
 expect(request.current).toBe(true);
 receive(reply(request.requestId, 0, false)); await recover;
 expect(useGameStatusStore.getState().preferenceRecoveryRequired).toBe(false);
 expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(false);
});
test("rejection reads authority and cannot apply an older snapshot over a newer view", async () => {
 const run = setTrainerPreference(true);
 const first = network.send.mock.calls[0][0];
 receive({ success: false, requestId: first.requestId, error: "stale preference" });
 await Promise.resolve(); await Promise.resolve();
 const read = network.send.mock.calls[1][0]; expect(read.current).toBe(true);
 useGameStatusStore.setState({ preferenceRevision: 2, allowTrainerRebattles: true });
 receive(reply(read.requestId, 1, false)); await run;
 expect(useGameStatusStore.getState().preferenceRevision).toBe(2);
 expect(useGameStatusStore.getState().allowTrainerRebattles).toBe(true);
 expect(useGameStatusStore.getState().preferenceRecoveryRequired).toBe(true);
});
