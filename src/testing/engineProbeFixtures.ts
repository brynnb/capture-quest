import type {
  PhaserActor,
  PhaserTile,
  PhaserWarp,
} from "@/net/generated/world_api";
import {
  NodeEngineProbeAdapter,
  type EngineProbeCommand,
  type EngineProbeSnapshot,
  type NodeEngineProbeState,
} from "./engineProbe";

export function probeTile(
  x: number,
  y: number,
  collisionType = 1,
  overrides: Partial<PhaserTile> = {},
): PhaserTile {
  return {
    id: y * 1000 + x,
    x,
    y,
    tileImageId: 1000 + y * 1000 + x,
    mapId: 9999,
    collisionType,
    talkOverTile: false,
    ...overrides,
  };
}

export function probeActor(overrides: Partial<PhaserActor>): PhaserActor {
  return {
    id: overrides.id ?? 1,
    mapId: overrides.mapId ?? 9999,
    objectType: overrides.objectType ?? "npc",
    moveSpeed: 200,
    ...overrides,
  };
}

export function probeWarp(overrides: Partial<PhaserWarp>): PhaserWarp {
  return {
    id: overrides.id ?? 1,
    sourceMapId: overrides.sourceMapId ?? 9999,
    x: overrides.x ?? 0,
    y: overrides.y ?? 0,
    destinationMapId: overrides.destinationMapId ?? 1,
    destinationX: overrides.destinationX ?? 0,
    destinationY: overrides.destinationY ?? 0,
    warpType: overrides.warpType ?? "door",
    ...overrides,
  };
}

export function makeNodeProbeState(
  overrides: Partial<NodeEngineProbeState> = {},
): NodeEngineProbeState {
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
    tiles: [probeTile(0, 0)],
    actors: [],
    warps: [],
    ...overrides,
  };
}

export async function runNodeProbeSequence(
  state: NodeEngineProbeState,
  commands: EngineProbeCommand[],
): Promise<EngineProbeSnapshot> {
  const probe = new NodeEngineProbeAdapter(state);
  for (const command of commands) {
    const result = await probe.sendCommand(command);
    if (!result.ok) {
      throw new Error(
        `Engine probe command failed: ${JSON.stringify(command)} ${result.error ?? ""}`,
      );
    }
  }
  return probe.getSnapshot();
}
