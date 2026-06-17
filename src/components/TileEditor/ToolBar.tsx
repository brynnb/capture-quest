import React from "react";
import styled from "styled-components";
import useTileEditorStore, { TileEditorTool } from "@/stores/TileEditorStore";

const ToolBarContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 60px;
  flex-shrink: 0;
  align-items: center;
  padding-top: 4px;
`;

const ToolButton = styled.button<{ $isActive: boolean }>`
  width: 48px;
  height: 48px;
  border: 3px solid ${({ $isActive }) => ($isActive ? "#6a6bf6" : "#4a4ba6")};
  border-radius: 12px;
  background: ${({ $isActive }) => ($isActive ? "#a7edfe" : "rgba(255, 255, 255, 0.7)")};
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  transition: all 0.15s ease;
  box-shadow: ${({ $isActive }) => ($isActive ? "0 0 8px rgba(106, 107, 246, 0.5)" : "0 2px 0 #4a4ba6")};

  &:hover {
    background: #d1d2ff;
    transform: translateY(-1px);
  }

  &:active {
    transform: translateY(1px);
    box-shadow: 0 1px 0 #4a4ba6;
  }
`;

const Divider = styled.div`
  width: 36px;
  height: 2px;
  background: #4a4ba6;
  opacity: 0.3;
  border-radius: 1px;
`;

const BrushSizeControl = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
`;

const BrushSizeLabel = styled.div`
  font-family: "Outfit", sans-serif;
  font-weight: 700;
  font-size: 11px;
  color: #4a4ba6;
  text-align: center;
`;

const BrushSizeButtons = styled.div`
  display: flex;
  gap: 2px;
`;

const SmallButton = styled.button`
  width: 22px;
  height: 22px;
  border: 2px solid #4a4ba6;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.7);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: bold;
  color: #4a4ba6;
  padding: 0;
  line-height: 1;

  &:hover {
    background: #d1d2ff;
  }

  &:active {
    transform: translateY(1px);
  }
`;

const UndoButton = styled(ToolButton)`
  margin-top: auto;
`;

interface ToolDef {
  id: TileEditorTool;
  label: string;
  icon: string;
}

const tools: ToolDef[] = [
  { id: "single", label: "Single Tile", icon: "✏️" },
  { id: "brush", label: "Brush", icon: "🖌️" },
  { id: "eraser", label: "Eraser", icon: "🧹" },
  { id: "fill", label: "Fill", icon: "🪣" },
  { id: "stamp", label: "Stamp", icon: "🏗️" },
];

const ToolBar: React.FC = () => {
  const {
    selectedTool,
    setSelectedTool,
    brushSize,
    increaseBrushSize,
    decreaseBrushSize,
    undoStack,
    popUndo,
  } = useTileEditorStore();

  const handleUndo = () => {
    const entry = popUndo();
    if (!entry) return;
    // Dispatch undo event — TileViewer will handle sending to server and local update
    window.dispatchEvent(
      new CustomEvent("tileEditorUndo", { detail: entry }),
    );
  };

  return (
    <ToolBarContainer>
      {tools.map((tool) => (
        <ToolButton
          key={tool.id}
          $isActive={selectedTool === tool.id}
          onClick={() => setSelectedTool(tool.id)}
          title={tool.label}
        >
          {tool.icon}
        </ToolButton>
      ))}

      {(selectedTool === "brush" || selectedTool === "eraser") && (
        <>
          <Divider />
          <BrushSizeControl>
            <BrushSizeLabel>{brushSize}x{brushSize}</BrushSizeLabel>
            <BrushSizeButtons>
              <SmallButton onClick={decreaseBrushSize} title="Decrease brush size">-</SmallButton>
              <SmallButton onClick={increaseBrushSize} title="Increase brush size">+</SmallButton>
            </BrushSizeButtons>
          </BrushSizeControl>
        </>
      )}

      <Divider />
      <UndoButton
        $isActive={false}
        onClick={handleUndo}
        title={`Undo (${undoStack.length} remaining)`}
        disabled={undoStack.length === 0}
        style={{ opacity: undoStack.length === 0 ? 0.4 : 1 }}
      >
        ↩️
      </UndoButton>
    </ToolBarContainer>
  );
};

export default ToolBar;
