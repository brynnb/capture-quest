import React, { useState } from "react";
import styled from "styled-components";
import Tooltip from "./Tooltip";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

const TooltipWrapper = styled.div`
  position: relative;
  display: inline-block;
`;

const StyledActionButton = styled.button.attrs({ className: "action-button" }) <{
  $isPressed: boolean;
  $marginBottom: string;
  $customCSS?: string;
}>`
  width: 100%;
  height: 40px;
  box-sizing: border-box;
  background-color: ${({ $isPressed }) => ($isPressed ? "#a7edfe" : "#c0c1ff")};
  border: 3px solid #4a4ba6;
  border-radius: 12px;
  cursor: pointer;
  outline: none;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  font-size: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  text-transform: none;
  margin-bottom: ${({ $marginBottom }) => $marginBottom};
  user-select: none;
  box-shadow: 0 4px 0 #4a4ba6;
  transition: all 0.1s ease-in-out;

  &:focus {
    outline: none;
  }

  &:hover:not(:disabled) {
    background-color: #d1d2ff;
    transform: translateY(-2px);
    box-shadow: 0 6px 0 #4a4ba6;
  }

  &:active:not(:disabled) {
    transform: translateY(2px);
    box-shadow: 0 2px 0 #4a4ba6;
  }

  ${({ $customCSS }) => $customCSS}
`;

interface ActionButtonProps {
  text: string;
  onClick?: () => void;
  onMouseDown?: () => void;
  onMouseUp?: () => void;
  onMouseLeave?: () => void;
  marginBottom?: string;
  customCSS?: string;
  isToggleable?: boolean;
  isPressed?: boolean;
  tooltip?: string;
  showTooltip?: boolean;
  clickSound?: string | null;
}

const ActionButton: React.FC<ActionButtonProps> = ({
  text,
  onClick,
  onMouseDown,
  onMouseUp,
  onMouseLeave,
  marginBottom = "7px",
  customCSS,
  isToggleable = false,
  isPressed: isPressedProp,
  tooltip,
  showTooltip: showTooltipProp = false,
  clickSound = sfxPathForConstant("SFX_PRESS_AB"),
}) => {
  const [isInternalPressed, setIsInternalPressed] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const isPressed =
    isPressedProp !== undefined ? isPressedProp : isInternalPressed;

  const showTooltip = tooltip ? (isHovered || showTooltipProp) : false;

  const handleClick = () => {
    if (clickSound) {
      AudioManager.playSFX(clickSound);
    }
    if (isToggleable && isPressedProp === undefined) {
      setIsInternalPressed((prev) => !prev);
    }
    if (onClick && typeof onClick === "function") {
      onClick();
    }
  };

  const handleMouseEvents = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (!isToggleable && isPressedProp === undefined) {
      if (event.type === "mousedown") {
        setIsInternalPressed(true);
      } else if (event.type === "mouseup" || event.type === "mouseleave") {
        setIsInternalPressed(false);
      }
    }

    // Call custom handlers if provided
    if (event.type === "mousedown" && onMouseDown) {
      onMouseDown();
    } else if (event.type === "mouseup" && onMouseUp) {
      onMouseUp();
    } else if (event.type === "mouseleave" && onMouseLeave) {
      onMouseLeave();
    }

    // Handle hover for tooltip
    if (event.type === "mouseenter") {
      setIsHovered(true);
    } else if (event.type === "mouseleave") {
      setIsHovered(false);
    }
  };

  const button = (
    <StyledActionButton
      $isPressed={isPressed}
      $marginBottom={marginBottom}
      $customCSS={customCSS}
      onMouseDown={handleMouseEvents}
      onMouseUp={handleMouseEvents}
      onMouseEnter={handleMouseEvents}
      onMouseLeave={handleMouseEvents}
      onClick={handleClick}
    >
      {text}
    </StyledActionButton>
  );

  if (tooltip) {
    return (
      <TooltipWrapper>
        {button}
        <Tooltip text={tooltip} isVisible={showTooltip} />
      </TooltipWrapper>
    );
  }

  return button;
};

export default ActionButton;
