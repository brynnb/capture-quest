export const CONFIRM_INSTANT_WARP_EVENT = "capturequest:confirm-instant-warp";

export const COMPACT_TOUCH_LAYOUT_QUERY =
  "(max-width: 850px), (pointer: coarse)";

export interface InstantWarpTarget {
  mapId: number;
  x: number;
  y: number;
}

export interface ConfirmInstantWarpEventDetail {
  target: InstantWarpTarget;
}

export function isCompactTouchLayout(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia(COMPACT_TOUCH_LAYOUT_QUERY).matches
  );
}
