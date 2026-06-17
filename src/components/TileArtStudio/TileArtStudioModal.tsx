import React, { useState } from "react";
import styled from "styled-components";
import useGameStatusStore from "@/stores/GameStatusStore";
import ReplaceTileTab from "./ReplaceTileTab";
import StampCreatorTab from "./StampCreatorTab";
import AnimatedTilesTab from "./AnimatedTilesTab";

type ArtStudioTab = "replace" | "stamps" | "animated";

const Overlay = styled.div`
  position: fixed;
  inset: 0;
  z-index: 99999;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
`;

const ModalContainer = styled.div`
  width: 95vw;
  height: 92vh;
  background: #1a1a2e;
  border: 3px solid #4a4ba6;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6);
`;

const Header = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: #4a4ba6;
  flex-shrink: 0;
`;

const Title = styled.h1`
  margin: 0;
  font-family: "Outfit", sans-serif;
  font-size: 22px;
  font-weight: 800;
  color: white;
  letter-spacing: 0.5px;
`;

const CloseButton = styled.button`
  background: rgba(255, 255, 255, 0.15);
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 8px;
  color: white;
  font-size: 18px;
  width: 36px;
  height: 36px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;

  &:hover {
    background: rgba(255, 255, 255, 0.25);
    border-color: rgba(255, 255, 255, 0.5);
  }
`;

const TabBar = styled.div`
  display: flex;
  gap: 0;
  background: #2a2a4a;
  flex-shrink: 0;
`;

const Tab = styled.button<{ $isActive: boolean }>`
  flex: 1;
  padding: 12px 24px;
  border: none;
  border-bottom: 3px solid ${({ $isActive }) => ($isActive ? "#a7edfe" : "transparent")};
  background: ${({ $isActive }) => ($isActive ? "rgba(167, 237, 254, 0.1)" : "transparent")};
  color: ${({ $isActive }) => ($isActive ? "#a7edfe" : "rgba(255, 255, 255, 0.5)")};
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s ease;
  letter-spacing: 0.3px;

  &:hover {
    background: rgba(167, 237, 254, 0.05);
    color: ${({ $isActive }) => ($isActive ? "#a7edfe" : "rgba(255, 255, 255, 0.7)")};
  }
`;

const TabContent = styled.div`
  flex: 1;
  overflow: hidden;
  display: flex;
`;

const tabs: { id: ArtStudioTab; label: string }[] = [
  { id: "replace", label: "Replace Tile" },
  { id: "stamps", label: "Stamp Creator" },
  { id: "animated", label: "Animated Tiles" },
];

const TileArtStudioModal: React.FC = () => {
  const isOpen = useGameStatusStore((s) => s.isArtStudioOpen);
  const toggleArtStudio = useGameStatusStore((s) => s.toggleArtStudio);
  const [activeTab, setActiveTab] = useState<ArtStudioTab>("replace");

  if (!isOpen) return null;

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      toggleArtStudio();
    }
  };

  return (
    <Overlay onClick={handleOverlayClick}>
      <ModalContainer onClick={(e) => e.stopPropagation()}>
        <Header>
          <Title>Tile Art Studio</Title>
          <CloseButton onClick={toggleArtStudio} title="Close">
            ✕
          </CloseButton>
        </Header>

        <TabBar>
          {tabs.map((tab) => (
            <Tab
              key={tab.id}
              $isActive={activeTab === tab.id}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.label}
            </Tab>
          ))}
        </TabBar>

        <TabContent>
          {activeTab === "replace" && <ReplaceTileTab />}
          {activeTab === "stamps" && <StampCreatorTab />}
          {activeTab === "animated" && <AnimatedTilesTab />}
        </TabContent>
      </ModalContainer>
    </Overlay>
  );
};

export default TileArtStudioModal;
