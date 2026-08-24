import styled from "styled-components";

export const GameFrameOverlay = styled.div<{
  $tint?: string;
  $zIndex?: number;
}>`
  position: absolute;
  inset: 0;
  z-index: ${(p) => p.$zIndex ?? 9999};
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: all;
  box-sizing: border-box;
  padding: max(12px, env(safe-area-inset-top, 0px))
    max(12px, env(safe-area-inset-right, 0px))
    max(12px, env(safe-area-inset-bottom, 0px))
    max(12px, env(safe-area-inset-left, 0px));
  overflow: auto;
  overscroll-behavior: contain;
  background: ${(p) => p.$tint ?? "transparent"};

  @media (max-width: 850px), (pointer: coarse), (max-height: 600px) {
    align-items: flex-start;
  }
`;
