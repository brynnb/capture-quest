import React, { useEffect } from "react";
import styled from "styled-components";
import ToolBar from "./ToolBar";
import TilePalette from "./TilePalette";
import useTileEditorStore from "@/stores/TileEditorStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import * as TileEditorNet from "./TileEditorNetwork";

const PanelContainer = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  gap: 12px;
  background: rgba(192, 193, 255, 0.57);
  backdrop-filter: blur(12px);
  border: 4px solid #4a4ba6;
  border-radius: 24px;
  padding: 12px;
  box-sizing: border-box;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.2);
`;

const PanelTitle = styled.div`
  position: absolute;
  top: -14px;
  left: 20px;
  background: #4a4ba6;
  color: white;
  padding: 2px 12px;
  border-radius: 8px;
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  font-size: 13px;
  letter-spacing: 0.5px;
`;

const CloseButton = styled.button`
  position: absolute;
  z-index: 2;
  top: 8px;
  right: 10px;
  min-width: 44px;
  min-height: 40px;
  padding: 6px 12px;
  color: #fff;
  background: #4a4ba6;
  border: 2px solid #303176;
  border-radius: 10px;
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    background: #5d5fbd;
  }
`;

const PanelWrapper = styled.div`
  position: relative;
  width: 100%;
  height: 100%;
`;

const TileEditorPanel: React.FC = () => {
  const tilePropertiesLoaded = useTileEditorStore((s) => s.tilePropertiesLoaded);
  const canUseAdminTileTools = usePlayerCharacterStore((s) => (s.characterProfile?.gm ?? 0) > 0);
  const toggleTileManager = useGameStatusStore((s) => s.toggleTileManager);

  useEffect(() => {
    if (canUseAdminTileTools && !tilePropertiesLoaded) {
      TileEditorNet.requestTileProperties();
    }
  }, [canUseAdminTileTools, tilePropertiesLoaded]);

  if (!canUseAdminTileTools) return null;

  return (
    <PanelWrapper>
      <PanelTitle>Tile Manager</PanelTitle>
      <CloseButton
        type="button"
        onClick={toggleTileManager}
        aria-label="Close Tile Manager"
        data-testid="tile-manager-close"
      >
        Close
      </CloseButton>
      <PanelContainer>
        <ToolBar />
        <TilePalette />
      </PanelContainer>
    </PanelWrapper>
  );
};

export default TileEditorPanel;
