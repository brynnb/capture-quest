import React from "react";
import styled from "styled-components";
import SystemOptions from "./SystemOptions";
import Chatbox from "./Chatbox";
import PokemonDialogueBox from "@/components/PokemonDialogueBox";
import ActionButton from "@components/Interface/ActionButton";
import useGameStatusStore from "@stores/GameStatusStore";
import useGameScreenStore from "@stores/GameScreenStore";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";
import { WorldSocket, OpCodes } from "@/net";
import AudioManager from "@/services/audio/AudioManager";
import usePokeBattleStore from "@stores/PokeBattleStore";
import { cancelActiveCutscene } from "@/phaser-game/services/CutsceneService";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

const HUDContainer = styled.div`
  position: absolute;
  bottom: 30px;
  left: 40px;
  right: 40px;
  height: 300px; /* Fixed height for all bottom elements */
  display: flex;
  gap: 15px;
  align-items: flex-end;
  z-index: 1000;
`;

const SidebarSection = styled.div<{ $isHidden?: boolean }>`
  width: 242px;
  height: 100%;
  background: rgba(192, 193, 255, 0.57);
  backdrop-filter: blur(12px);
  border: 4px solid #4a4ba6;
  border-radius: 24px;
  padding: 20px 10px;
  box-sizing: border-box;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.2);
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
  justify-content: flex-start;
  opacity: ${({ $isHidden }) => ($isHidden ? 0 : 1)};
  pointer-events: ${({ $isHidden }) => ($isHidden ? "none" : "auto")};
  transition: opacity 0.2s ease;
`;

const ChatSection = styled.div`
  flex: 1;
  height: 100%;
  min-width: 0;
  position: relative;
`;

const BottomHUD: React.FC = () => {
  const {
    isPokedexOpen,
    togglePokedex,
    isGroupOpen,
    toggleGroup,
    setCurrentMap,
    isInventoryOpen,
    toggleInventory,
    isWarpMode,
    toggleWarpMode,
    isCameraFollowEnabled,
    toggleCameraFollow,
  } = useGameStatusStore();

  const { setScreen } = useGameScreenStore();
  const { setPendingSelectName } = useCharacterSelectStore();
  const { characterProfile } = usePlayerCharacterStore();

  const handleQuit = async () => {
    if (characterProfile?.name) {
      setPendingSelectName(characterProfile.name);
    }
    cancelActiveCutscene("character quit");
    await WorldSocket.sendJsonMessage(OpCodes.CharacterQuitRequest, {});
    AudioManager.stopMusic(true);
    AudioManager.stopAllAmbients();
    await setCurrentMap(null as any);
    usePokeBattleStore.getState().closeBattle();
    setScreen("characterSelect");
  };

  const handleWarpHome = async () => {
    await WorldSocket.sendJsonMessage(OpCodes.WarpHomeRequest, {});
  };

  return (
    <HUDContainer id="bottom-hud">
      <SidebarSection className="left-sidebar-bottom-section">
        <SystemOptions showInventory={false} />
        <ActionButton
          text="Warp Home"
          onClick={handleWarpHome}
          marginBottom="4px"
        />
        <ActionButton
          text="Quit"
          onClick={handleQuit}
          marginBottom="0"
        />
      </SidebarSection>

      <ChatSection>
        <PokemonDialogueBox />
        <Chatbox />
      </ChatSection>

      <SidebarSection
        className="right-sidebar-bottom-section"
        $isHidden={isInventoryOpen}
      >
        <ActionButton
          text="Party"
          onClick={toggleGroup}
          isPressed={isGroupOpen}
          isToggleable={true}
          marginBottom="4px"
        />
        <ActionButton
          text="Pokédex"
          onClick={togglePokedex}
          isPressed={isPokedexOpen}
          isToggleable={true}
          marginBottom="4px"
        />
        <ActionButton
          text="Instant Warp"
          onClick={toggleWarpMode}
          isPressed={isWarpMode}
          isToggleable={true}
          marginBottom="4px"
        />
        <ActionButton
          text="View Map"
          onClick={toggleCameraFollow}
          isPressed={!isCameraFollowEnabled}
          isToggleable={true}
          marginBottom="4px"
        />
        <ActionButton
          text="Inventory"
          onClick={toggleInventory}
          isPressed={isInventoryOpen}
          isToggleable={true}
          marginBottom="0"
          clickSound={
            isInventoryOpen
              ? sfxPathForConstant("SFX_TURN_OFF_PC")
              : sfxPathForConstant("SFX_TURN_ON_PC")
          }
        />
      </SidebarSection>
    </HUDContainer>
  );
};

export default BottomHUD;
