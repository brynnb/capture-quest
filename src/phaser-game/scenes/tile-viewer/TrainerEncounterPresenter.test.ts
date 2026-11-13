import { afterEach, expect, test, vi } from "vitest";
import { TrainerEncounterPresenter } from "./TrainerEncounterPresenter";
import type { TrainerEncounterNotifyPayload } from "@/net/generated/protocol";

const net = vi.hoisted(() => ({ ready: vi.fn() }));
vi.mock("../../services/PhaserNetworkService", () => ({ sendTrainerEncounterReady: net.ready }));
afterEach(() => vi.clearAllMocks());

const plan: TrainerEncounterNotifyPayload = { encounterToken: "issued", trainerActorId: 17, trainerX: 3, trainerY: 1, playerX: 3, playerY: 3, approachToX: 3, approachToY: 2, walkToX: 3, walkToY: 3, trainerClass: "TEST", trainerName: "Trainer" };

function fixture() {
  let delay!: () => void;
  const remove = vi.fn();
  const animate = vi.fn(async (): Promise<void> => undefined);
  const lock = vi.fn();
  const presenter = new TrainerEncounterPresenter({
    scene: { time: { delayedCall: (_ms: number, callback: () => void) => { delay = callback; return { remove }; } } },
    mapRenderer: () => ({ getActorSprite: () => null, getActorTilePosition: () => null, animateActorLocalPath: animate }),
    setInputLocked: lock,
  } as never);
  return { presenter, animate, lock, remove, delay: () => delay() };
}

test("successful approach sends the exact issued token once and ignores an active duplicate", async () => {
  const f = fixture();
  const done = f.presenter.handleEncounter(plan);
  await f.presenter.handleEncounter(plan);
  expect(net.ready).not.toHaveBeenCalled();
  f.delay(); await done;
  expect(f.animate).toHaveBeenCalledWith(17, [{ x: 3, y: 2 }], "DOWN");
  expect(net.ready).toHaveBeenCalledTimes(1);
  expect(net.ready).toHaveBeenCalledWith(17, "issued");
  expect(f.lock.mock.calls).toEqual([[true], [false]]);
});

test("cleanup settles a pending delay and prevents readiness from a retired scene", async () => {
  const f = fixture();
  const done = f.presenter.handleEncounter(plan);
  f.presenter.cleanup(); await done;
  expect(f.remove).toHaveBeenCalledWith(false);
  expect(f.animate).not.toHaveBeenCalled();
  expect(net.ready).not.toHaveBeenCalled();
});

test("late animation completion cannot acknowledge after retirement or unlock a newer plan", async () => {
  const f = fixture();
  let finish!: () => void;
  f.animate.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const old = f.presenter.handleEncounter(plan);
  f.delay(); await Promise.resolve();
  f.presenter.cleanup();
  const next = f.presenter.handleEncounter({ ...plan, encounterToken: "new" });
  finish(); await old;
  expect(net.ready).not.toHaveBeenCalled();
  expect(f.lock.mock.calls.at(-1)).toEqual([true]);
  f.delay(); await next;
  expect(net.ready).toHaveBeenCalledTimes(1);
  expect(net.ready).toHaveBeenCalledWith(17, "new");
});

test("animation failure leaves the durable plan available instead of claiming readiness", async () => {
  const f = fixture();
  const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
  f.animate.mockRejectedValueOnce(new Error("animation failed"));
  const done = f.presenter.handleEncounter(plan);
  f.delay(); await done;
  expect(net.ready).not.toHaveBeenCalled();
  expect(f.lock.mock.calls.at(-1)).toEqual([false]);
  expect(warn).toHaveBeenCalled(); warn.mockRestore();
});
