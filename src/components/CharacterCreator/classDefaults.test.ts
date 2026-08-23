import { describe, expect, test } from "vitest";
import { findDefaultCharacterClass } from "./classDefaults";

describe("character creator class defaults", () => {
  test("selects Bug Catcher independent of response ordering and casing", () => {
    const classes = [
      { id: 2, name: "Hiker" },
      { id: 1, name: "  BUG CATCHER  " },
      { id: 3, name: "Fisher" },
    ];

    expect(findDefaultCharacterClass(classes)?.id).toBe(1);
  });

  test("does not silently select an unrelated class", () => {
    expect(findDefaultCharacterClass([{ id: 2, name: "Hiker" }])).toBeUndefined();
  });
});
