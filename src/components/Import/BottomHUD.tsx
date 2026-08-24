import { useEffect, useRef, useState } from "react";
import styled from "styled-components";
import {
  FiBookOpen,
  FiChevronLeft,
  FiChevronRight,
  FiCrosshair,
  FiGrid,
  FiHelpCircle,
  FiHome,
  FiLogOut,
  FiMap,
  FiMenu,
  FiMessageSquare,
  FiSettings,
  FiTool,
  FiUser,
  FiUsers,
  FiVolume2,
  FiVolumeX,
} from "react-icons/fi";
import Chatbox from "./Chatbox";
import MobileControls from "./MobileControls";
import PokemonDialogueBox from "@/components/PokemonDialogueBox";
import useGameStatusStore from "@stores/GameStatusStore";
import useGameScreenStore from "@stores/GameScreenStore";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";
import { WorldSocket, OpCodes } from "@/net";
import AudioManager from "@/services/audio/AudioManager";
import usePokeBattleStore from "@stores/PokeBattleStore";
import usePokemonDialogueStore from "@stores/PokemonDialogueStore";
import { cancelActiveCutscene } from "@/phaser-game/services/CutsceneService";
import { IS_LOCAL_DEV } from "@/config";
import useWorldDebugStore from "@/stores/WorldDebugStore";
import useDebugSceneStore from "@/stores/DebugSceneStore";

const HudLayer = styled.div`
  position: absolute;
  inset: 0;
  z-index: 1000;
  pointer-events: none;
`;

const Rail = styled.aside<{ $collapsed: boolean }>`
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: ${(props) => (props.$collapsed ? "64px" : "272px")};
  display: flex;
  flex-direction: column;
  color: #25265f;
  background:
    linear-gradient(180deg, rgba(226, 227, 255, 0.96), rgba(192, 193, 255, 0.86)),
    url("/assets/pokewallpaper2.png");
  background-size: auto, 260px;
  border-right: 4px solid #4a4ba6;
  box-shadow: 12px 0 36px rgba(20, 21, 67, 0.24);
  backdrop-filter: blur(14px);
  box-sizing: border-box;
  pointer-events: auto;
  overflow: hidden;
  transition: width 180ms ease;

  @media (max-width: 850px), (pointer: coarse) {
    top: calc(10px + env(safe-area-inset-top, 0px));
    bottom: ${(props) =>
      props.$collapsed
        ? "auto"
        : "calc(10px + env(safe-area-inset-bottom, 0px))"};
    left: calc(10px + env(safe-area-inset-left, 0px));
    z-index: 40;
    width: ${(props) => (props.$collapsed ? "44px" : "min(292px, calc(100vw - 20px))")};
    height: ${(props) => (props.$collapsed ? "44px" : "auto")};
    max-height: ${(props) => (props.$collapsed ? "44px" : "none")};
    border: 2px solid #4a4ba6;
    border-radius: 14px;
    box-shadow: 0 18px 48px rgba(20, 21, 67, 0.34);
    transition: width 160ms ease, height 160ms ease;
  }
`;

const RailToggle = styled.button`
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 3;
  display: grid;
  width: 40px;
  height: 40px;
  padding: 0;
  place-items: center;
  color: #2e2f66;
  background: rgba(255, 255, 255, 0.7);
  border: 2px solid #4a4ba6;
  border-radius: 10px;
  box-shadow: 0 3px 0 #4a4ba6;

  svg {
    width: 20px;
    height: 20px;
  }

  &:hover {
    background: #fff;
  }

  &:active {
    transform: translateY(2px);
    box-shadow: 0 1px 0 #4a4ba6;
  }

  @media (max-width: 850px), (pointer: coarse) {
    top: 0;
    right: 0;
    width: 40px;
    height: 40px;
    border: 0;
    box-shadow: none;
  }
`;

const RailBody = styled.div<{ $collapsed: boolean }>`
  display: ${(props) => (props.$collapsed ? "none" : "flex")};
  min-height: 0;
  flex: 1;
  flex-direction: column;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
`;

const Brand = styled.header`
  padding: 18px 60px 15px 18px;
  border-bottom: 2px solid rgba(74, 75, 166, 0.2);
`;

const BrandTitle = styled.div`
  font-family: "Pokemon GB", "Outfit", sans-serif;
  font-size: 15px;
  font-weight: 900;
  letter-spacing: 0.04em;
  color: #2e2f66;
`;

const Location = styled.div`
  margin-top: 6px;
  overflow: hidden;
  color: #4a4ba6;
  font-family: "Outfit", sans-serif;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
`;

const TrainerSummary = styled.div`
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
  margin: 12px 12px 4px;
  padding: 11px;
  background: rgba(255, 255, 255, 0.54);
  border: 2px solid rgba(74, 75, 166, 0.34);
  border-radius: 14px;
`;

const TrainerAvatar = styled.div`
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  color: #2e2f66;
  background: #a7edfe;
  border: 2px solid #4a4ba6;
  border-radius: 50%;

  svg {
    width: 21px;
    height: 21px;
  }
`;

const TrainerName = styled.div`
  overflow: hidden;
  color: #25265f;
  font-family: "Outfit", sans-serif;
  font-size: 15px;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const TrainerMoney = styled.div`
  color: #5b5c91;
  font-family: "Outfit", sans-serif;
  font-size: 11px;
  font-weight: 700;
`;

const RailSection = styled.section`
  padding: 10px 12px 0;
`;

const SectionLabel = styled.h2`
  margin: 0 0 7px 4px;
  color: #63649a;
  font-family: "Outfit", sans-serif;
  font-size: 9px;
  font-weight: 900;
  letter-spacing: 0.13em;
  text-transform: uppercase;
`;

const ButtonGrid = styled.div<{ $columns?: number }>`
  display: grid;
  grid-template-columns: repeat(${(props) => props.$columns ?? 2}, minmax(0, 1fr));
  gap: 7px;
`;

const RailButton = styled.button<{ $active?: boolean; $danger?: boolean }>`
  display: flex;
  min-width: 0;
  min-height: 45px;
  align-items: center;
  justify-content: flex-start;
  gap: 8px;
  padding: 8px 9px;
  color: ${(props) => (props.$danger ? "#7f2633" : "#2e2f66")};
  background: ${(props) =>
    props.$active
      ? "linear-gradient(180deg, #c8f3fe, #a7edfe)"
      : props.$danger
        ? "rgba(255, 204, 217, 0.72)"
        : "rgba(255, 255, 255, 0.62)"};
  border: 2px solid ${(props) => (props.$danger ? "#a84d65" : "#4a4ba6")};
  border-radius: 10px;
  box-shadow: 0 3px 0 ${(props) => (props.$danger ? "#a84d65" : "#4a4ba6")};
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 850;
  line-height: 1.05;
  text-align: left;

  svg {
    flex: 0 0 auto;
    width: 17px;
    height: 17px;
  }

  &:hover {
    background: ${(props) => (props.$danger ? "#ffdbe5" : "#fff")};
    transform: translateY(-1px);
  }

  &:active {
    transform: translateY(2px);
    box-shadow: 0 1px 0 ${(props) => (props.$danger ? "#a84d65" : "#4a4ba6")};
  }
`;

const RailFooter = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 7px;
  padding: 12px;
  border-top: 2px solid rgba(74, 75, 166, 0.18);
`;

const DebugReadout = styled.section`
  margin: auto 12px 0;
  padding: 10px;
  color: #30316e;
  background: rgba(255, 255, 255, 0.58);
  border: 2px solid rgba(74, 75, 166, 0.42);
  border-radius: 12px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
`;

const DebugReadoutTitle = styled.div`
  margin-bottom: 6px;
  color: #4a4ba6;
  font-family: "Outfit", sans-serif;
  font-size: 9px;
  font-weight: 900;
  letter-spacing: 0.12em;
  text-transform: uppercase;
`;

const DebugReadoutText = styled.div`
  overflow-wrap: anywhere;
  white-space: pre-line;
  font-family: "Pokemon GB", "Outfit", monospace;
  font-size: 8px;
  font-weight: 700;
  line-height: 1.45;
`;

const ChatDock = styled.div<{ $collapsed: boolean; $mobileOpen: boolean }>`
  position: absolute;
  bottom: 16px;
  left: ${(props) => (props.$collapsed ? "80px" : "288px")};
  z-index: 20;
  width: min(520px, calc(100vw - ${(props) => (props.$collapsed ? "176px" : "384px")}));
  height: 214px;
  pointer-events: auto;
  transition: left 180ms ease;

  @media (max-width: 850px), (pointer: coarse) {
    top: calc(64px + env(safe-area-inset-top, 0px));
    right: calc(10px + env(safe-area-inset-right, 0px));
    bottom: calc(172px + env(safe-area-inset-bottom, 0px));
    left: calc(10px + env(safe-area-inset-left, 0px));
    z-index: 35;
    display: ${(props) => (props.$mobileOpen ? "block" : "none")};
    width: auto;
    height: auto;
    min-height: 220px;
  }

  @media (max-height: 600px) and (max-width: 850px),
    (max-height: 600px) and (pointer: coarse) {
    bottom: calc(10px + env(safe-area-inset-bottom, 0px));
    min-height: 0;
  }
`;

const DialogueAnchor = styled.div<{ $collapsed: boolean }>`
  position: absolute;
  right: auto;
  bottom: 246px;
  left: 50%;
  z-index: 22;
  width: min(600px, calc(100vw - 32px));
  height: 0;
  transform: translateX(-50%);
  pointer-events: none;

  @media (max-width: 850px), (pointer: coarse) {
    bottom: calc(168px + env(safe-area-inset-bottom, 0px));
    width: calc(
      100vw - 20px - env(safe-area-inset-left, 0px) -
        env(safe-area-inset-right, 0px)
    );
  }
`;

const MobileChatToggle = styled.button<{ $open: boolean }>`
  display: none;

  @media (max-width: 850px), (pointer: coarse) {
    position: absolute;
    top: calc(10px + env(safe-area-inset-top, 0px));
    right: calc(10px + env(safe-area-inset-right, 0px));
    z-index: 42;
    display: grid;
    width: 44px;
    height: 44px;
    padding: 0;
    place-items: center;
    color: #2e2f66;
    background: ${(props) => (props.$open ? "#a7edfe" : "rgba(226, 227, 255, 0.88)")};
    border: 2px solid #4a4ba6;
    border-radius: 12px;
    box-shadow: 0 4px 0 #4a4ba6;
    pointer-events: auto;

    svg {
      width: 20px;
      height: 20px;
    }
  }
`;

function useCompactViewport(): boolean {
  const [compact, setCompact] = useState(false);

  useEffect(() => {
    const query = window.matchMedia("(max-width: 850px), (pointer: coarse)");
    const update = () => setCompact(query.matches);
    update();
    query.addEventListener?.("change", update);
    return () => query.removeEventListener?.("change", update);
  }, []);

  return compact;
}

const BottomHUD = () => {
  const {
    currentMap,
    getMapNameById,
    isPokedexOpen,
    togglePokedex,
    isTrainerCardOpen,
    toggleTrainerCard,
    isGroupOpen,
    toggleGroup,
    setCurrentMap,
    isInventoryOpen,
    toggleInventory,
    isOptionsOpen,
    toggleOptions,
    isHelpOpen,
    toggleHelp,
    isMuted,
    toggleMute,
    isWarpMode,
    toggleWarpMode,
    isCameraFollowEnabled,
    toggleCameraFollow,
    isHudSidebarCollapsed,
    setHudSidebarCollapsed,
    toggleHudSidebar,
    isMobileChatOpen,
    setMobileChatOpen,
    toggleMobileChat,
    isTileManagerOpen,
    toggleTileManager,
  } = useGameStatusStore();
  const compact = useCompactViewport();
  const dialogueOpen = usePokemonDialogueStore((state) => state.isOpen);
  const previousCompact = useRef<boolean | null>(null);
  const { setScreen } = useGameScreenStore();
  const { setPendingSelectName } = useCharacterSelectStore();
  const { characterProfile } = usePlayerCharacterStore();
  const { tileInfo, viewMode } = useWorldDebugStore();
  const { isOpen: isDebugSceneOpen, toggleOpen: toggleDebugScene } = useDebugSceneStore();

  useEffect(() => {
    const enteringCompact = compact && previousCompact.current !== true;
    previousCompact.current = compact;
    if (!enteringCompact) return;
    setHudSidebarCollapsed(true);
    setMobileChatOpen(false);
    if (isGroupOpen) toggleGroup();
  }, [
    compact,
    isGroupOpen,
    setHudSidebarCollapsed,
    setMobileChatOpen,
    toggleGroup,
  ]);

  useEffect(() => {
    if (!compact || !dialogueOpen) return;
    setMobileChatOpen(false);
    setHudSidebarCollapsed(true);
  }, [
    compact,
    dialogueOpen,
    setHudSidebarCollapsed,
    setMobileChatOpen,
  ]);

  useEffect(() => {
    if (!compact) return;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (isMobileChatOpen) setMobileChatOpen(false);
      if (!isHudSidebarCollapsed) setHudSidebarCollapsed(true);
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [
    compact,
    isHudSidebarCollapsed,
    isMobileChatOpen,
    setHudSidebarCollapsed,
    setMobileChatOpen,
  ]);

  const runRailAction = (action: () => void) => {
    action();
    if (compact) setHudSidebarCollapsed(true);
  };

  const handleQuit = async () => {
    if (characterProfile?.name) {
      setPendingSelectName(characterProfile.name);
    }
    cancelActiveCutscene("character quit");
    await WorldSocket.sendJsonMessage(OpCodes.CharacterQuitRequest, {});
    AudioManager.stopMusic(true);
    AudioManager.stopAllAmbients();
    await setCurrentMap(null);
    usePokeBattleStore.getState().closeBattle();
    setScreen("characterSelect");
  };

  const handleWarpHome = async () => {
    await WorldSocket.sendJsonMessage(OpCodes.WarpHomeRequest, {});
  };

  const locationName = currentMap == null
    ? "Exploring Kanto"
    : getMapNameById(currentMap) ?? `Map ${currentMap}`;

  return (
    <HudLayer id="bottom-hud">
      <Rail $collapsed={isHudSidebarCollapsed} aria-label="Game menu">
        <RailToggle
          type="button"
          onClick={toggleHudSidebar}
          aria-label={isHudSidebarCollapsed ? "Open game menu" : "Collapse game menu"}
          aria-expanded={!isHudSidebarCollapsed}
        >
          {isHudSidebarCollapsed
            ? compact ? <FiMenu /> : <FiChevronRight />
            : <FiChevronLeft />}
        </RailToggle>
        <RailBody $collapsed={isHudSidebarCollapsed}>
          <Brand>
            <BrandTitle>CAPTUREQUEST</BrandTitle>
            <Location>{locationName}</Location>
          </Brand>
          <TrainerSummary>
            <TrainerAvatar><FiUser /></TrainerAvatar>
            <div>
              <TrainerName>{characterProfile?.name ?? "Trainer"}</TrainerName>
              <TrainerMoney>
                ¥{(characterProfile?.pokedollars ?? 0).toLocaleString()}
              </TrainerMoney>
            </div>
          </TrainerSummary>

          <RailSection>
            <SectionLabel>Trainer</SectionLabel>
            <ButtonGrid>
              <RailButton $active={isGroupOpen} onClick={() => runRailAction(toggleGroup)}>
                <FiUsers /> Party
              </RailButton>
              <RailButton $active={isInventoryOpen} onClick={() => runRailAction(toggleInventory)}>
                <FiGrid /> Bag
              </RailButton>
              <RailButton $active={isPokedexOpen} onClick={() => runRailAction(togglePokedex)}>
                <FiBookOpen /> Pokédex
              </RailButton>
              <RailButton $active={isTrainerCardOpen} onClick={() => runRailAction(toggleTrainerCard)}>
                <FiUser /> Trainer
              </RailButton>
            </ButtonGrid>
          </RailSection>

          <RailSection>
            <SectionLabel>World</SectionLabel>
            <ButtonGrid>
              <RailButton $active={!isCameraFollowEnabled} onClick={() => runRailAction(toggleCameraFollow)}>
                <FiMap /> View Map
              </RailButton>
              <RailButton $active={isWarpMode} onClick={() => runRailAction(toggleWarpMode)}>
                <FiCrosshair /> Instant Warp
              </RailButton>
              {(characterProfile?.gm ?? 0) > 0 && (
                <RailButton
                  type="button"
                  $active={isTileManagerOpen}
                  onClick={() => runRailAction(toggleTileManager)}
                  data-testid="tile-manager-toggle"
                >
                  <FiTool /> Tile Manager
                </RailButton>
              )}
              {IS_LOCAL_DEV && (
                <RailButton
                  type="button"
                  $active={isDebugSceneOpen}
                  onClick={() => runRailAction(toggleDebugScene)}
                >
                  <FiMap /> Map / Scenarios
                </RailButton>
              )}
            </ButtonGrid>
          </RailSection>

          <RailSection>
            <SectionLabel>System</SectionLabel>
            <ButtonGrid>
              <RailButton $active={isOptionsOpen} onClick={() => runRailAction(toggleOptions)}>
                <FiSettings /> Options
              </RailButton>
              <RailButton $active={isHelpOpen} onClick={() => runRailAction(toggleHelp)}>
                <FiHelpCircle /> Help
              </RailButton>
              <RailButton $active={isMuted} onClick={() => runRailAction(toggleMute)}>
                {isMuted ? <FiVolumeX /> : <FiVolume2 />} Mute
              </RailButton>
            </ButtonGrid>
          </RailSection>

          {IS_LOCAL_DEV && (
            <DebugReadout data-testid="world-debug-readout">
              <DebugReadoutTitle>World Inspector</DebugReadoutTitle>
              <DebugReadoutText>{`${tileInfo}\n\n${viewMode}`}</DebugReadoutText>
            </DebugReadout>
          )}

          <RailFooter>
            <RailButton onClick={() => void handleWarpHome()}>
              <FiHome /> Warp Home
            </RailButton>
            <RailButton $danger onClick={() => void handleQuit()}>
              <FiLogOut /> Quit
            </RailButton>
          </RailFooter>
        </RailBody>
      </Rail>

      {!dialogueOpen && (
        <MobileChatToggle
          type="button"
          $open={isMobileChatOpen}
          onClick={toggleMobileChat}
          aria-label={isMobileChatOpen ? "Close trainer chat" : "Open trainer chat"}
          aria-expanded={isMobileChatOpen}
        >
          <FiMessageSquare />
        </MobileChatToggle>
      )}

      <ChatDock
        $collapsed={isHudSidebarCollapsed}
        $mobileOpen={isMobileChatOpen}
      >
        <Chatbox />
      </ChatDock>
      <DialogueAnchor $collapsed={isHudSidebarCollapsed}>
        <PokemonDialogueBox />
      </DialogueAnchor>
      <MobileControls />
    </HudLayer>
  );
};

export default BottomHUD;
