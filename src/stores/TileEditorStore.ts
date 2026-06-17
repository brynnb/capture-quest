import { create } from "zustand";
import type { TileProperty } from "@/net/generated/world_api";

export type TileEditorTool = "single" | "brush" | "eraser" | "fill" | "stamp";

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

  // Drag state for continuous painting
  isDragging: boolean;
  setIsDragging: (dragging: boolean) => void;
  dragBatch: UndoTileEntry[];
  addToDragBatch: (edits: UndoTileEntry[]) => void;
  clearDragBatch: () => void;
}

const MAX_UNDO_ENTRIES = 10;
const MAX_BRUSH_SIZE = 5;

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
  availableStamps: [],
  setAvailableStamps: (stamps) => set({ availableStamps: stamps }),

  isDragging: false,
  setIsDragging: (dragging) => set({ isDragging: dragging }),
  dragBatch: [],
  addToDragBatch: (edits) => {
    set((state) => ({ dragBatch: [...state.dragBatch, ...edits] }));
  },
  clearDragBatch: () => set({ dragBatch: [] }),
}));

export default useTileEditorStore;
