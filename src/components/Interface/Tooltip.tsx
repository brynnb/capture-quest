import React from "react";
import styled from "styled-components";

interface TooltipProps {
  text: string;
  isVisible: boolean;
}

const TooltipContainer = styled.div<{ $isVisible: boolean }>`
  position: absolute;
  background-color: rgba(26, 27, 65, 0.95);
  color: #fff;
  padding: 10px 16px;
  border: 2px solid #4a4ba6;
  border-radius: 12px;
  font-family: 'Outfit', sans-serif;
  font-weight: 600;
  font-size: 14px;
  white-space: pre-wrap;
  max-width: 250px;
  pointer-events: none;
  opacity: ${({ $isVisible }) => ($isVisible ? 1 : 0)};
  transform: translateX(-50%) translateY(${({ $isVisible }) => ($isVisible ? "-10px" : "0px")});
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  z-index: 10000;
  top: -60px;
  left: 50%;
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(8px);

  &::after {
    content: '';
    position: absolute;
    bottom: -8px;
    left: 50%;
    transform: translateX(-50%);
    border-left: 8px solid transparent;
    border-right: 8px solid transparent;
    border-top: 8px solid #4a4ba6;
  }
`;

const Tooltip: React.FC<TooltipProps> = ({ text, isVisible }) => {
  return <TooltipContainer $isVisible={isVisible}>{text}</TooltipContainer>;
};

export default Tooltip;
