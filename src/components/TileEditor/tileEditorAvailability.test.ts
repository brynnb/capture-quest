import { describe, expect, it } from "vitest";

import { canPresentTileManager } from "./tileEditorAvailability";

describe("canPresentTileManager", () => {
  it("never presents the editor in production, including to GM characters", () => {
    expect(canPresentTileManager(false, 0)).toBe(false);
    expect(canPresentTileManager(false, 1)).toBe(false);
  });

  it("presents the editor only to privileged local characters", () => {
    expect(canPresentTileManager(true, 0)).toBe(false);
    expect(canPresentTileManager(true, 1)).toBe(true);
  });
});
