export type MobileMovementDirection = "UP" | "DOWN" | "LEFT" | "RIGHT";

export interface MobileJoystickVector {
  x: number;
  y: number;
}

export const MOBILE_JOYSTICK_DEAD_ZONE = 0.28;

/**
 * CaptureQuest movement is tile/cardinal based. Quantize the analog nipple
 * vector by its dominant axis so diagonal thumb positions cannot bypass the
 * collision and movement rules that keyboard input uses.
 */
export function cardinalDirectionForJoystick(
  vector: MobileJoystickVector | null | undefined,
  deadZone = MOBILE_JOYSTICK_DEAD_ZONE,
): MobileMovementDirection | null {
  if (!vector) return null;

  const horizontal = Number.isFinite(vector.x) ? vector.x : 0;
  const vertical = Number.isFinite(vector.y) ? vector.y : 0;
  const absHorizontal = Math.abs(horizontal);
  const absVertical = Math.abs(vertical);

  if (Math.max(absHorizontal, absVertical) < deadZone) return null;
  if (absHorizontal > absVertical) return horizontal > 0 ? "RIGHT" : "LEFT";
  return vertical > 0 ? "UP" : "DOWN";
}

export const MOBILE_MOVE_EVENT = "capturequest:mobile-move";
export const MOBILE_INTERACT_EVENT = "capturequest:mobile-interact";
