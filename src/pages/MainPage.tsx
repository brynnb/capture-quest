import { useEffect, useState } from "react";
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
import {
  COMPACT_TOUCH_LAYOUT_QUERY,
  CONFIRM_INSTANT_WARP_EVENT,
  type ConfirmInstantWarpEventDetail,
  isCompactTouchLayout,
} from "@/phaser-game/instantWarp";

const WarpModeOverlay = styled.div<{ $compact: boolean }>`
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
  animation: ${(props) =>
    props.$compact ? "none" : "warpPulse 2s ease-in-out infinite"};

  @keyframes warpPulse {
    0%, 100% { opacity: 0.9; }
    50% { opacity: 1; }
  }

  @media (max-width: 850px), (pointer: coarse) {
    top: auto;
    right: calc(12px + env(safe-area-inset-right, 0px));
    bottom: calc(14px + env(safe-area-inset-bottom, 0px));
    left: calc(12px + env(safe-area-inset-left, 0px));
    transform: none;
    display: grid;
    width: auto;
    max-width: 440px;
    margin: 0 auto;
    box-sizing: border-box;
    gap: 10px;
    padding: 14px;
    border-radius: 14px;
    font-size: 15px;
    line-height: 1.3;
    pointer-events: auto;
  }
`;

const WarpInstruction = styled.div`
  text-align: center;
`;

const WarpSelection = styled.div`
  color: #ffe2e7;
  font-family: "Pokemon GB", "Outfit", sans-serif;
  font-size: 13px;
  text-align: center;
`;

const WarpActions = styled.div`
  display: flex;
  gap: 10px;
  justify-content: center;
`;

const WarpActionButton = styled.button<{ $confirm?: boolean }>`
  min-width: 0;
  min-height: 46px;
  flex: 1 1 0;
  padding: 9px 12px;
  color: ${(props) => (props.$confirm ? "#173f2b" : "#522b42")};
  background: ${(props) =>
    props.$confirm
      ? "linear-gradient(180deg, #baf7d0, #7fe3a5)"
      : "linear-gradient(180deg, #ffe1e8, #ffc4d2)"};
  border: 2px solid ${(props) => (props.$confirm ? "#347b51" : "#9b4b67")};
  border-radius: 10px;
  box-shadow: 0 3px 0 ${(props) => (props.$confirm ? "#285e3e" : "#71364d")};
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 900;
  touch-action: manipulation;
  -webkit-tap-highlight-color: transparent;

  &:disabled {
    cursor: not-allowed;
    filter: grayscale(0.55);
    opacity: 0.58;
  }

  &:not(:disabled):active {
    transform: translateY(2px);
    box-shadow: 0 1px 0 ${(props) => (props.$confirm ? "#285e3e" : "#71364d")};
  }
`;

const GameScreenRoot = styled.div`
  display: contents;
`;

function useCompactTouchLayout(): boolean {
  const [compact, setCompact] = useState(isCompactTouchLayout);

  useEffect(() => {
    const query = window.matchMedia(COMPACT_TOUCH_LAYOUT_QUERY);
    const update = () => setCompact(query.matches);
    update();
    query.addEventListener?.("change", update);
    return () => query.removeEventListener?.("change", update);
  }, []);

  return compact;
}

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
    isMapLoading,
    pendingInstantWarpTarget,
    setWarpMode,
  } = useGameStatusStore();
  const compactTouchLayout = useCompactTouchLayout();

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

  useEffect(() => {
    if (!compactTouchLayout || !isWarpMode) return;

    const cancelWithEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setWarpMode(false);
    };
    window.addEventListener("keydown", cancelWithEscape);
    return () => window.removeEventListener("keydown", cancelWithEscape);
  }, [compactTouchLayout, isWarpMode, setWarpMode]);

  const confirmInstantWarp = () => {
    if (!pendingInstantWarpTarget || isMapLoading) return;
    window.dispatchEvent(
      new CustomEvent<ConfirmInstantWarpEventDetail>(
        CONFIRM_INSTANT_WARP_EVENT,
        { detail: { target: pendingInstantWarpTarget } },
      ),
    );
  };

  return (
    <GameScreenRoot data-testid="game-screen">
      <ZoneBackground />

      {isWarpMode && (
        <WarpModeOverlay
          $compact={compactTouchLayout}
          role={compactTouchLayout ? "dialog" : "status"}
          aria-label="Instant warp"
          aria-live="polite"
        >
          {compactTouchLayout ? (
            <>
              <WarpInstruction>
                {pendingInstantWarpTarget
                  ? "Check the marked tile, then confirm your warp."
                  : "Tap a tile to select your warp destination."}
              </WarpInstruction>
              {pendingInstantWarpTarget && (
                <WarpSelection data-testid="instant-warp-selection">
                  Selected tile: {pendingInstantWarpTarget.x},{" "}
                  {pendingInstantWarpTarget.y}
                </WarpSelection>
              )}
              <WarpActions>
                <WarpActionButton
                  type="button"
                  onClick={() => setWarpMode(false)}
                >
                  Cancel
                </WarpActionButton>
                <WarpActionButton
                  type="button"
                  $confirm
                  disabled={!pendingInstantWarpTarget || isMapLoading}
                  onClick={confirmInstantWarp}
                >
                  Confirm Warp
                </WarpActionButton>
              </WarpActions>
            </>
          ) : (
            "Click any tile to warp there"
          )}
        </WarpModeOverlay>
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
    </GameScreenRoot>
  );
};

export default MainPage;
