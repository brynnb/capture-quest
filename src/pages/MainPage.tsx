import { useEffect } from "react";
import styled from "styled-components";
import useGameScreenStore from "@stores/GameScreenStore";
import useGameStatusStore from "@stores/GameStatusStore";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";
import ZoneBackground from "@components/Import/ZoneBackground";
import InventoryLayout from "@components/Inventory/InventoryLayout";
import Pokedex from "@/components/Interface/Pokedex";
import TrainerCard from "@/components/Interface/TrainerCard";
import OptionsDisplay from "@/components/Interface/OptionsDisplay";
import HelpDisplay from "@/components/Interface/HelpDisplay";

import BottomHUD from "@components/Import/BottomHUD";
import PartyPokemonHUD from "@/components/Interface/PartyPokemonHUD";
import PokeMartShop from "@/components/Interface/PokeMartShop";
import PokemonPC from "@/components/Interface/PokemonPC";
import SlotMachine from "@/components/Interface/SlotMachine";

const WarpModeOverlay = styled.div`
  position: absolute;
  top: 40px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(74, 75, 166, 0.85);
  backdrop-filter: blur(8px);
  color: #fff;
  font-family: "Outfit", sans-serif;
  font-weight: 700;
  font-size: 20px;
  padding: 12px 28px;
  border-radius: 16px;
  border: 3px solid #6b6cdb;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
  z-index: 2000;
  pointer-events: none;
  animation: warpPulse 2s ease-in-out infinite;

  @keyframes warpPulse {
    0%, 100% { opacity: 0.9; }
    50% { opacity: 1; }
  }
`;

const MainPage: React.FC = () => {
  const { setScreen } = useGameScreenStore();
  const { characterProfile, characterProfileLoaded } = usePlayerCharacterStore(
    (state) => ({
      characterProfile: state.characterProfile,
      characterProfileLoaded: state.characterProfileLoaded,
    }),
  );
  const {
    initializeMaps,
    isInventoryOpen,
    isPokedexOpen,
    isTrainerCardOpen,
    isOptionsOpen,
    isHelpOpen,
    isGroupOpen,
    isWarpMode,
  } = useGameStatusStore();

  useEffect(() => {
    if (
      characterProfileLoaded &&
      !characterProfile?.id &&
      !characterProfile?.name
    ) {
      setScreen("characterCreate");
      return;
    }

    if (characterProfileLoaded) {
      initializeMaps();
    }
  }, [characterProfile, characterProfileLoaded, setScreen, initializeMaps]);

  return (
    <>
      <ZoneBackground />

      {isWarpMode && (
        <WarpModeOverlay>Click any tile to warp there</WarpModeOverlay>
      )}

      <BottomHUD />

      {isInventoryOpen && <InventoryLayout />}

      {isGroupOpen &&
        !isInventoryOpen &&
        !isOptionsOpen && <PartyPokemonHUD />}
      {isPokedexOpen && <Pokedex />}
      {isTrainerCardOpen && <TrainerCard />}
      {isOptionsOpen && <OptionsDisplay />}
      {isHelpOpen && <HelpDisplay />}
      <PokeMartShop />
      <PokemonPC />
      <SlotMachine />
    </>
  );
};

export default MainPage;
