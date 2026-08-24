import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import useTileEditorStore from "./TileEditorStore";

const STORAGE_KEY = "capturequest.tile-editor-stamps.v1";

describe("TileEditorStore captured stamps", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useTileEditorStore.setState({
      selectedTool: "single",
      selectedStamp: null,
      availableStamps: [],
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("saves captured tile groups locally and selects them for placement", () => {
    vi.spyOn(Date, "now").mockReturnValue(12345);

    const stamp = useTileEditorStore.getState().addCapturedStamp(
      [[10, 11], [12, 0]],
      2,
      2,
    );

    expect(stamp.id).toBe(-12345);
    expect(useTileEditorStore.getState()).toMatchObject({
      selectedTool: "stamp",
      selectedStamp: stamp,
      availableStamps: [stamp],
    });
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([stamp]);
  });

  it("merges shared server stamps without discarding captured stamps", () => {
    const captured = useTileEditorStore.getState().addCapturedStamp([[10]], 1, 1);
    const shared = {
      id: 8,
      name: "Poké Center",
      widthTiles: 1,
      heightTiles: 1,
      tileImageIds: [[20]],
    };

    useTileEditorStore.getState().setAvailableStamps([shared]);

    expect(useTileEditorStore.getState().availableStamps).toEqual([captured, shared]);
  });

  it("deletes only locally captured stamps", () => {
    const captured = useTileEditorStore.getState().addCapturedStamp([[10]], 1, 1);
    useTileEditorStore.getState().deleteCapturedStamp(captured.id);

    expect(useTileEditorStore.getState().availableStamps).toEqual([]);
    expect(useTileEditorStore.getState().selectedStamp).toBeNull();
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([]);
  });

  it("renames a captured stamp in both active state and storage", () => {
    const captured = useTileEditorStore.getState().addCapturedStamp([[10]], 1, 1);
    useTileEditorStore.getState().renameCapturedStamp(captured.id, "Pallet House");

    expect(useTileEditorStore.getState().selectedStamp?.name).toBe("Pallet House");
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]")[0].name)
      .toBe("Pallet House");
  });
});
