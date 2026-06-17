import React, { useEffect } from "react";
import styled from "styled-components";
import ToolBar from "./ToolBar";
import TilePalette from "./TilePalette";
import useTileEditorStore from "@/stores/TileEditorStore";
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

const PanelWrapper = styled.div`
  position: relative;
  width: 100%;
  height: 100%;
`;

const TileEditorPanel: React.FC = () => {
  const tilePropertiesLoaded = useTileEditorStore((s) => s.tilePropertiesLoaded);

  useEffect(() => {
    if (!tilePropertiesLoaded) {
      TileEditorNet.requestTileProperties();
    }
  }, [tilePropertiesLoaded]);

  return (
    <PanelWrapper>
      <PanelTitle>Tile Manager</PanelTitle>
      <PanelContainer>
        <ToolBar />
        <TilePalette />
      </PanelContainer>
    </PanelWrapper>
  );
};

export default TileEditorPanel;
