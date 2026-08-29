import { describe, expect, it } from "vitest";
import {
  MOBILE_JOYSTICK_DEAD_ZONE,
  cardinalDirectionForJoystick,
} from "./mobileControls";

describe("cardinalDirectionForJoystick", () => {
  it("ignores rest jitter inside the dead zone", () => {
    expect(
      cardinalDirectionForJoystick({
        x: MOBILE_JOYSTICK_DEAD_ZONE - 0.01,
        y: 0.04,
      }),
    ).toBeNull();
  });

  it.each([
    [{ x: 0.8, y: 0.2 }, "RIGHT"],
    [{ x: -0.8, y: 0.2 }, "LEFT"],
    [{ x: 0.2, y: 0.8 }, "UP"],
    [{ x: 0.2, y: -0.8 }, "DOWN"],
  ] as const)("maps %o to %s", (vector, expected) => {
    expect(cardinalDirectionForJoystick(vector)).toBe(expected);
  });

  it("uses only the dominant axis for diagonal input", () => {
    expect(cardinalDirectionForJoystick({ x: 0.9, y: 0.7 })).toBe("RIGHT");
    expect(cardinalDirectionForJoystick({ x: -0.6, y: -0.9 })).toBe("DOWN");
  });

  it("treats malformed vector values as zero", () => {
    expect(cardinalDirectionForJoystick({ x: Number.NaN, y: 0.9 })).toBe("UP");
    expect(cardinalDirectionForJoystick(undefined)).toBeNull();
  });
});
