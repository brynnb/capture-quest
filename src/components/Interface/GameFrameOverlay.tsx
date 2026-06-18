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
  background: ${(p) => p.$tint ?? "transparent"};
`;
