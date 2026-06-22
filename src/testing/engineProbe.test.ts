import { describe, expect, test } from "vitest";
import {
  ENGINE_PROBE_EVENT,
  NodeEngineProbeAdapter,
  buildEngineProbeSnapshot,
  engineProbeInternals,
  type NodeEngineProbeState,
} from "@/testing/engineProbe";
import {
  makeNodeProbeState,
  probeActor,
  probeTile,
  probeWarp,
  runNodeProbeSequence,
} from "@/testing/engineProbeFixtures";
import type { PhaserActor, PhaserTile, PhaserWarp } from "@/net/generated/world_api";

function tile(
  x: number,
  y: number,
  collisionType = 1,
  overrides: Partial<PhaserTile> = {},
): PhaserTile {
  return {
    id: y * 100 + x,
    x,
    y,
    tileImageId: 1000 + y * 100 + x,
    mapId: 9999,
    collisionType,
    talkOverTile: false,
    ...overrides,
  };
}

function actor(overrides: Partial<PhaserActor>): PhaserActor {
  return {
    id: overrides.id ?? 1,
    mapId: overrides.mapId ?? 9999,
    objectType: overrides.objectType ?? "npc",
    moveSpeed: 200,
    ...overrides,
  };
}

function baseState(overrides: Partial<NodeEngineProbeState> = {}): NodeEngineProbeState {
  return {
    map: { id: 9999, name: "Kanto", isLoading: false },
    player: {
      id: 1,
      internalId: 1,
      name: "Tester",
      x: 0,
      y: 0,
      direction: "DOWN",
      isMoving: false,
      isSurfing: false,
      isCycling: false,
    },
    tiles: [tile(0, 0)],
    actors: [],
    warps: [],
    ...overrides,
  };
}

function snapshotTile(state: NodeEngineProbeState, x: number, y: number) {
  const snapshot = buildEngineProbeSnapshot(state, {
    bounds: { radius: 3, center: { x, y } },
  });
  const found = snapshot.tiles.find((entry) => entry.x === x && entry.y === y);
  if (!found) throw new Error(`Missing snapshot tile ${x},${y}`);
  return found;
}

describe("engine probe snapshots", () => {
  test("radius bounds keep overworld snapshots compact", () => {
    const tiles = Array.from({ length: 100 }, (_, index) =>
      tile(index, 0, 1),
    );
    const snapshot = buildEngineProbeSnapshot(
      makeNodeProbeState({
        player: { ...baseState().player, x: 50, y: 0 },
        tiles,
      }),
      { bounds: { radius: 2 } },
    );

    expect(snapshot.bounds).toMatchObject({
      radius: 2,
      minX: 48,
      maxX: 52,
    });
    expect(snapshot.tiles).toHaveLength(5);
    expect(snapshot.tiles.length).toBeLessThan(tiles.length);
  });

  test("radius bounds are capped for large requests", () => {
    const snapshot = buildEngineProbeSnapshot(
      makeNodeProbeState({
        player: { ...baseState().player, x: 50, y: 50 },
        tiles: [tile(50, 50)],
      }),
      { bounds: { radius: 999 } },
    );

    expect(snapshot.bounds).toMatchObject({
      radius: engineProbeInternals.MAX_RADIUS,
      minX: 30,
      maxX: 70,
      minY: 30,
      maxY: 70,
    });
  });

  test("tile tags include water, cut trees, warps, and actor occupancy", () => {
    const warps: PhaserWarp[] = [
      {
        id: 10,
        sourceMapId: 9999,
        x: 1,
        y: 1,
        destinationMapId: 40,
        destinationX: 5,
        destinationY: 11,
        warpType: "door",
      },
    ];
    const state = baseState({
      tiles: [
        tile(0, 0),
        tile(1, 0, 2),
        tile(0, 1, 1, {
          rawFootTileId: engineProbeInternals.CUT_TREE_RAW_FOOT_TILE_ID,
        }),
        tile(1, 1, 2),
      ],
      actors: [actor({ id: 99, x: 0, y: 0, spriteName: "SPRITE_BOULDER" })],
      warps,
    });

    expect(snapshotTile(state, 0, 0)).toMatchObject({
      walkable: false,
      occupiedByActorId: 99,
    });
    expect(snapshotTile(state, 1, 0)).toMatchObject({
      isWater: true,
      walkable: false,
    });
    expect(snapshotTile(state, 0, 1)).toMatchObject({
      isCutTree: true,
    });
    expect(snapshotTile(state, 1, 1)).toMatchObject({
      isWarp: true,
      isWater: false,
    });
  });
});

describe("node engine probe commands", () => {
  test("movement reports open and blocked steps", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0), probeTile(2, 0, 0)],
      }),
    );

    const open = await probe.sendCommand({ press: "RIGHT" });
    expect(open.ok).toBe(true);
    expect(open.after.player).toMatchObject({ x: 1, y: 0, direction: "RIGHT" });

    const blocked = await probe.sendCommand({ press: "RIGHT" });
    expect(blocked.ok).toBe(false);
    expect(blocked.error).toBe("Blocked");
    expect(blocked.after.player).toMatchObject({ x: 1, y: 0 });
  });

  test("click pathing summarizes the path and final tile", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0), probeTile(2, 0), probeTile(2, 1)],
      }),
    );

    const result = await probe.sendCommand({ clickTile: { x: 2, y: 1 } });

    expect(result.ok).toBe(true);
    expect(result.after.player).toMatchObject({ x: 2, y: 1 });
    expect(result.events.some((event) => event.type === ENGINE_PROBE_EVENT.pathFound)).toBe(true);
  });

  test("boulder pushes update actor and player positions", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0), probeTile(2, 0)],
        actors: [probeActor({ id: 5, x: 1, y: 0, spriteName: "SPRITE_BOULDER" })],
      }),
    );

    const result = await probe.sendCommand({ press: "RIGHT" });

    expect(result.ok).toBe(true);
    expect(result.after.player).toMatchObject({ x: 1, y: 0 });
    expect(result.after.actors.find((entry) => entry.id === 5)).toMatchObject({
      x: 2,
      y: 0,
    });
    expect(result.events.some((event) => event.type === ENGINE_PROBE_EVENT.boulderMoved)).toBe(true);
  });

  test("NPC actors block movement and can be interacted with", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0)],
        actors: [
          probeActor({
            id: 7,
            name: "Guide",
            text: "TEXT_GUIDE",
            x: 1,
            y: 0,
          }),
        ],
      }),
    );

    const blocked = await probe.sendCommand({ press: "RIGHT" });
    expect(blocked.ok).toBe(false);
    expect(blocked.after.player).toMatchObject({ x: 0, y: 0 });

    const interacted = await probe.sendCommand({
      interactActor: { name: "Guide" },
    });
    expect(interacted.ok).toBe(true);
    expect(interacted.after.dialogue).toMatchObject({
      isOpen: true,
      actorId: 7,
      text: "TEXT_GUIDE",
    });
  });

  test("water is blocked until surf state is active", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0, 2)],
      }),
    );

    const blocked = await probe.sendCommand({ press: "RIGHT" });
    expect(blocked.ok).toBe(false);
    expect(blocked.after.player).toMatchObject({ x: 0, y: 0, isSurfing: false });

    const surf = await probe.sendCommand({
      useFieldMove: { moveName: "SURF", targetX: 1, targetY: 0 },
    });
    expect(surf.ok).toBe(true);
    expect(surf.after.player).toMatchObject({ x: 1, y: 0, isSurfing: true });
  });

  test("cut changes the local tree tile into walkable land", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [
          probeTile(0, 0),
          probeTile(1, 0, 0, {
            rawFootTileId: engineProbeInternals.CUT_TREE_RAW_FOOT_TILE_ID,
          }),
        ],
      }),
    );

    const result = await probe.sendCommand({
      useFieldMove: { moveName: "CUT", targetX: 1, targetY: 0 },
    });
    const cutTile = result.after.tiles.find((entry) => entry.x === 1 && entry.y === 0);

    expect(result.ok).toBe(true);
    expect(cutTile).toMatchObject({
      isCutTree: false,
      walkable: true,
    });
  });

  test("surf moves onto adjacent water after explicit field move use", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0, 2)],
      }),
    );

    const result = await probe.sendCommand({
      useFieldMove: { moveName: "SURF", targetX: 1, targetY: 0 },
    });

    expect(result.ok).toBe(true);
    expect(result.after.player).toMatchObject({
      x: 1,
      y: 0,
      isSurfing: true,
      direction: "RIGHT",
    });
  });

  test("dialogue, battle, item, scenario, and warp commands expose stable events", async () => {
    const probe = new NodeEngineProbeAdapter(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0)],
        warps: [
          probeWarp({
            id: 42,
            x: 1,
            y: 0,
            destinationMapId: 37,
            destinationX: 2,
            destinationY: 7,
            warpDirection: "DOWN",
          }),
        ],
        dialogue: {
          isOpen: true,
          isChoicePending: true,
          selectedChoice: "yes",
        },
        battle: {
          isOpen: true,
          phase: "action_select",
          battleType: "wild",
        },
        inventory: {
          money: 100,
          shopOpen: false,
          shopName: "",
          items: [
            {
              instanceId: 1,
              itemId: 17,
              name: "Potion",
              shortName: "POTION",
              quantity: 1,
            },
          ],
        },
      }),
    );

    expect((await probe.sendCommand({ scenario: { name: "debug_case" } })).events[0].type).toBe(
      ENGINE_PROBE_EVENT.scenarioJumped,
    );
    expect(
      (await probe.sendCommand({ dialogueChoice: { choice: "no" } })).after.dialogue,
    ).toMatchObject({ isOpen: false, selectedChoice: "no" });
    expect(
      (await probe.sendCommand({ battleAction: { action: "fight" } })).after.battle,
    ).toMatchObject({ selectedAction: "fight", phase: "move_select" });
    expect((await probe.sendCommand({ battleMove: { index: 0 } })).after.battle).toMatchObject({
      selectedMoveIndex: 0,
      turnNumber: 1,
    });
    expect((await probe.sendCommand({ useItem: { itemId: 17 } })).ok).toBe(true);
    const warp = await probe.sendCommand({ activateWarp: { x: 1, y: 0 } });
    expect(warp.after.map.id).toBe(37);
    expect(warp.after.player).toMatchObject({ x: 2, y: 7, direction: "DOWN" });
  });

  test("runNodeProbeSequence returns the final snapshot for compact scripts", async () => {
    const snapshot = await runNodeProbeSequence(
      makeNodeProbeState({
        tiles: [probeTile(0, 0), probeTile(1, 0), probeTile(1, 1)],
      }),
      [{ press: "RIGHT" }, { press: "DOWN" }],
    );

    expect(snapshot.player).toMatchObject({ x: 1, y: 1, direction: "DOWN" });
  });
});
