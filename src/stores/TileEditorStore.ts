import { create } from "zustand";
import type { TileProperty } from "@/net/generated/world_api";

export type TileEditorTool =
  | "single"
  | "brush"
  | "eraser"
  | "fill"
  | "eyedropper"
  | "stamp";

export interface StampDefinition {
  id: number;
  name: string;
  widthTiles: number;
  heightTiles: number;
  tileImageIds: number[][];
}

interface UndoTileEntry {
  x: number;
  y: number;
  tileImageId: number;
}

interface UndoEntry {
  oldTiles: UndoTileEntry[]; // Previous tile states (tileImageId=0 means was empty)
  newTiles: UndoTileEntry[]; // New tile states that were applied
  mapId: number;
}

interface TileEditorStore {
  // Tool state
  selectedTool: TileEditorTool;
  setSelectedTool: (tool: TileEditorTool) => void;
  brushSize: number;
  setBrushSize: (size: number) => void;
  increaseBrushSize: () => void;
  decreaseBrushSize: () => void;

  // Selected tile for painting
  selectedTileImageId: number | null;
  setSelectedTileImageId: (id: number | null) => void;

  // Tile properties (palette data from server)
  tileProperties: TileProperty[];
  setTileProperties: (props: TileProperty[]) => void;
  tilePropertiesLoaded: boolean;

  // Undo stack (max 10 entries)
  undoStack: UndoEntry[];
  pushUndo: (entry: UndoEntry) => void;
  popUndo: () => UndoEntry | undefined;
  clearUndoStack: () => void;

  // Stamp state
  selectedStamp: StampDefinition | null;
  setSelectedStamp: (stamp: StampDefinition | null) => void;
  availableStamps: StampDefinition[];
  setAvailableStamps: (stamps: StampDefinition[]) => void;
  addCapturedStamp: (
    tileImageIds: number[][],
    widthTiles: number,
    heightTiles: number,
  ) => StampDefinition;
  renameCapturedStamp: (id: number, name: string) => void;
  deleteCapturedStamp: (id: number) => void;

  // Drag state for continuous painting
  isDragging: boolean;
  setIsDragging: (dragging: boolean) => void;
  dragBatch: UndoTileEntry[];
  addToDragBatch: (edits: UndoTileEntry[]) => void;
  clearDragBatch: () => void;
}

const MAX_UNDO_ENTRIES = 10;
const MAX_BRUSH_SIZE = 5;
const CAPTURED_STAMPS_STORAGE_KEY = "capturequest.tile-editor-stamps.v1";

function loadCapturedStamps(): StampDefinition[] {
  if (typeof window === "undefined") return [];
  try {
    const parsed = JSON.parse(window.localStorage.getItem(CAPTURED_STAMPS_STORAGE_KEY) ?? "[]");
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((stamp): stamp is StampDefinition =>
      typeof stamp?.id === "number" &&
      stamp.id < 0 &&
      typeof stamp.name === "string" &&
      Number.isInteger(stamp.widthTiles) &&
      Number.isInteger(stamp.heightTiles) &&
      Array.isArray(stamp.tileImageIds),
    );
  } catch {
    return [];
  }
}

function persistCapturedStamps(stamps: StampDefinition[]) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(
      CAPTURED_STAMPS_STORAGE_KEY,
      JSON.stringify(stamps.filter((stamp) => stamp.id < 0)),
    );
  } catch (error) {
    console.warn("[TileEditor] Could not persist captured stamps", error);
  }
}

const useTileEditorStore = create<TileEditorStore>()((set, get) => ({
  selectedTool: "single",
  setSelectedTool: (tool) => set({ selectedTool: tool }),

  brushSize: 1,
  setBrushSize: (size) => set({ brushSize: Math.max(1, Math.min(MAX_BRUSH_SIZE, size)) }),
  increaseBrushSize: () => {
    const { brushSize } = get();
    if (brushSize < MAX_BRUSH_SIZE) set({ brushSize: brushSize + 1 });
  },
  decreaseBrushSize: () => {
    const { brushSize } = get();
    if (brushSize > 1) set({ brushSize: brushSize - 1 });
  },

  selectedTileImageId: null,
  setSelectedTileImageId: (id) => set({ selectedTileImageId: id }),

  tileProperties: [],
  setTileProperties: (props) => set({ tileProperties: props, tilePropertiesLoaded: true }),
  tilePropertiesLoaded: false,

  undoStack: [],
  pushUndo: (entry) => {
    set((state) => {
      const newStack = [...state.undoStack, entry];
      if (newStack.length > MAX_UNDO_ENTRIES) {
        newStack.shift();
      }
      return { undoStack: newStack };
    });
  },
  popUndo: () => {
    const { undoStack } = get();
    if (undoStack.length === 0) return undefined;
    const entry = undoStack[undoStack.length - 1];
    set({ undoStack: undoStack.slice(0, -1) });
    return entry;
  },
  clearUndoStack: () => set({ undoStack: [] }),

  selectedStamp: null,
  setSelectedStamp: (stamp) => set({ selectedStamp: stamp, selectedTool: stamp ? "stamp" : "single" }),
  availableStamps: loadCapturedStamps(),
  setAvailableStamps: (stamps) => set((state) => {
    const captured = state.availableStamps.filter((stamp) => stamp.id < 0);
    const serverStamps = stamps.filter((stamp) => stamp.id >= 0);
    return { availableStamps: [...captured, ...serverStamps] };
  }),
  addCapturedStamp: (tileImageIds, widthTiles, heightTiles) => {
    const capturedStamps = get().availableStamps.filter((stamp) => stamp.id < 0);
    const capturedCount = capturedStamps.length;
    const lowestCapturedId = Math.min(0, ...capturedStamps.map((stamp) => stamp.id));
    const stamp: StampDefinition = {
      id: Math.min(-Date.now(), lowestCapturedId - 1),
      name: `Captured Stamp ${capturedCount + 1}`,
      widthTiles,
      heightTiles,
      tileImageIds,
    };
    const availableStamps = [stamp, ...get().availableStamps];
    persistCapturedStamps(availableStamps);
    set({ availableStamps, selectedStamp: stamp, selectedTool: "stamp" });
    return stamp;
  },
  renameCapturedStamp: (id, name) => set((state) => {
    if (id >= 0) return state;
    const trimmedName = name.slice(0, 100);
    const availableStamps = state.availableStamps.map((stamp) =>
      stamp.id === id ? { ...stamp, name: trimmedName } : stamp,
    );
    const selectedStamp = state.selectedStamp?.id === id
      ? { ...state.selectedStamp, name: trimmedName }
      : state.selectedStamp;
    persistCapturedStamps(availableStamps);
    return { availableStamps, selectedStamp };
  }),
  deleteCapturedStamp: (id) => set((state) => {
    if (id >= 0) return state;
    const availableStamps = state.availableStamps.filter((stamp) => stamp.id !== id);
    persistCapturedStamps(availableStamps);
    return {
      availableStamps,
      selectedStamp: state.selectedStamp?.id === id ? null : state.selectedStamp,
    };
  }),

  isDragging: false,
  setIsDragging: (dragging) => set({ isDragging: dragging }),
  dragBatch: [],
  addToDragBatch: (edits) => {
    set((state) => ({ dragBatch: [...state.dragBatch, ...edits] }));
  },
  clearDragBatch: () => set({ dragBatch: [] }),
}));

export default useTileEditorStore;
