import React from "react";
import styled from "styled-components";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";
import useGameStatusStore from "@stores/GameStatusStore";

interface SelectionButtonProps {
  $isSelected: boolean;
  $isDisabled?: boolean;
  $width?: string;
  $height?: string;
  onClick: () => void;
  children: React.ReactNode;
  disabled?: boolean;
}

const StyledButton = styled.button.attrs({ className: "selection-button" }) <{
  $isSelected: boolean;
  $isDisabled?: boolean;
  $width?: string;
  $height?: string;
}>`
  width: ${({ $width }) => $width ?? "auto"};
  min-width: ${({ $width }) => $width ?? "230px"};
  padding: ${({ $width }) => ($width ? "0 10px" : "0 40px")};
  height: ${({ $height }) => $height ?? "70px"};
  background-color: ${({ $isSelected }) => ($isSelected ? "#a7edfe" : "#c0c1ff")};
  border: 4px solid #4a4ba6;
  border-radius: 20px;
  cursor: ${({ $isDisabled }) => ($isDisabled ? "not-allowed" : "pointer")};
  outline: none;
  color: #2e2f66;
  font-family: "Outfit", "Inter", system-ui, sans-serif;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
  text-transform: none;
  margin-bottom: 3px;
  opacity: ${({ $isDisabled }) => ($isDisabled ? 0.6 : 1)};
  white-space: nowrap;
  font-size: clamp(12px, 24px, 28px);
  text-overflow: ellipsis;
  overflow: hidden;
  box-shadow: 0 4px 0 #4a4ba6;
  transition: all 0.1s ease-in-out;

  &:focus {
    outline: none;
  }

  &:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 6px 0 #4a4ba6;
    background-color: #d1d2ff;
  }

  &:active:not(:disabled) {
    transform: translateY(2px);
    box-shadow: 0 2px 0 #4a4ba6;
  }
`;

const SelectionButton = ({
  $isSelected,
  $isDisabled,
  $width,
  $height,
  onClick,
  children,
  disabled,
}: SelectionButtonProps) => {
  const handleClick = () => {
    if (!AudioManager.isInitialized()) {
      const { sfxVolume, ambientVolume, musicVolume, isMuted } = useGameStatusStore.getState();
      AudioManager.initialize({
        sfx: sfxVolume,
        ambient: ambientVolume,
        music: musicVolume,
        muted: isMuted
      });
    }
    const sfx = sfxPathForConstant("SFX_PRESS_AB");
    if (sfx) AudioManager.playSFX(sfx);
    onClick();
  };

  return (
    <StyledButton
      $isSelected={$isSelected}
      $isDisabled={$isDisabled}
      $width={$width}
      $height={$height}
      onClick={handleClick}
      disabled={disabled}
    >
      {children}
    </StyledButton>
  );
};

export default SelectionButton;
