import { WorldSocket } from "@/net/index";
import { NetworkBridge } from "@/net/NetworkBridge";
import * as OpCodes from "@/net/generated/opcodes";
import type {
  TileProperty,
  TileEditorBroadcastPayload,
} from "@/net/generated/world_api";
import useTileEditorStore from "@/stores/TileEditorStore";

// --- Send functions ---

export function requestTileProperties(): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({}, OpCodes.TilePropertiesRequest);
}

export function sendTilePlace(tiles: { x: number; y: number; tileImageId: number }[], mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ tiles, mapId }, OpCodes.TileEditorPlaceRequest);
}

export function sendTileErase(tiles: { x: number; y: number }[], mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ tiles, mapId }, OpCodes.TileEditorEraseRequest);
}

export function sendTileFill(x: number, y: number, tileImageId: number, mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ x, y, tileImageId, mapId }, OpCodes.TileEditorFillRequest);
}

export function sendTileUndo(tiles: { x: number; y: number; tileImageId: number }[], mapId: number): void {
  if (!WorldSocket.isConnected) return;
  NetworkBridge.send({ tiles, mapId }, OpCodes.TileEditorUndoRequest);
}

export function sendTilePropertyUpdate(
  tileImageId: number,
  name?: string,
  collisionType?: number,
): void {
  if (!WorldSocket.isConnected) return;
  const payload: Record<string, unknown> = { tileImageId };
  if (name !== undefined) payload.name = name;
  if (collisionType !== undefined) payload.collisionType = collisionType;
  NetworkBridge.send(payload, OpCodes.TilePropertyUpdateRequest);
}

// --- Response dispatch (called from NetworkBridge) ---

export function dispatchTileEditorResponse(opcode: number, data: unknown): boolean {
  switch (opcode) {
    case OpCodes.TilePropertiesResponse: {
      const props = data as TileProperty[];
      if (Array.isArray(props)) {
        useTileEditorStore.getState().setTileProperties(props);
      }
      return true;
    }
    case OpCodes.TileEditorBroadcast: {
      const payload = data as TileEditorBroadcastPayload;
      if (payload && Array.isArray(payload.tiles)) {
        // Dispatch a custom event that TileViewer listens for
        window.dispatchEvent(
          new CustomEvent("tileEditorBroadcast", { detail: payload }),
        );
      }
      return true;
    }
    case OpCodes.TileEditorPlaceResponse:
    case OpCodes.TileEditorEraseResponse:
    case OpCodes.TileEditorFillResponse:
    case OpCodes.TileEditorUndoResponse:
    case OpCodes.TilePropertyUpdateResponse:
      // Acknowledgements — tile rendering comes via the broadcast
      return true;
    default:
      return false;
  }
}
