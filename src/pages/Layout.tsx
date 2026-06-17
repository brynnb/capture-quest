import styled, { keyframes, css } from "styled-components";
import useGameScreenStore from "@stores/GameScreenStore";
import ScreenRouter from "@/components/ScreenRouter";
import { useEffect, useState, useCallback } from "react";
import useGameStatusStore from "@stores/GameStatusStore";
import LoadingScreen from "@/components/LoadingScreen";
import AudioService from "@/services/audio/AudioService";
import PhaserEngine from "@/phaser-game/PhaserEngine";
import { WelcomeModals } from "@/components/WelcomeModals";
import PokeBattleDisplay from "@/components/PokeBattleDisplay";
import { IS_LOCAL_DEV } from "@/config";

// Design dimensions (frame wrapper + margins)
const DESIGN_WIDTH = 1470; // 1440 + 30 (frame borders)
const DESIGN_HEIGHT = 1350; // 1086 + frame borders + margins for signature/disclaimer
const MIN_SCALE = 0.4; // Don't scale below 40%
const MAX_SCALE = 1; // Don't scale above 100%

// Responsive scaling wrapper
const ScalerContainer = styled.div`
  width: 100%;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior-y: none;
`;

interface ScaledContentProps {
  $scale: number;
}

const ScaledContent = styled.div<ScaledContentProps>`
  transform: scale(${(props) => props.$scale});
  transform-origin: top center;
  width: ${DESIGN_WIDTH}px;
  height: ${DESIGN_HEIGHT}px;
  /* Pull up following elements (or the bottom of the page) based on how much we shrunk */
  margin-bottom: ${(props) => DESIGN_HEIGHT * (props.$scale - 1)}px;
`;

const FRAME_BORDER = 21; // Match the pokemon frame border width
const POKEMON_FRAME_PATH = "/assets/pokemon_frame/frame-hd.png";

interface MainContainerProps {
  $backgroundType: "login" | "characterCreation" | "none";
}

// Outer wrapper that includes both the frame and the content
const FrameWrapper = styled.div`
  position: relative;
  width: ${1440 + FRAME_BORDER * 2}px;
  height: ${1080 + FRAME_BORDER * 2}px;
  margin: 50px auto 10px auto;

  /* Pokemon Frame Style */
  border-style: solid;
  border-width: ${FRAME_BORDER}px;
  border-image: url("${POKEMON_FRAME_PATH}") 42 round;
  image-rendering: pixelated;
  background: var(--pkmn-neutral);
  background-clip: padding-box; /* Prevents background from bleeding under the border */
  box-sizing: border-box;
`;

const Disclaimer = styled.p`
  color: #908e8eff;
  font-family: "Press Start 2P", sans-serif;
  font-size: 10px;
  margin: 0;
  max-width: 700px;
  opacity: 0.9;
  line-height: 1.6;
  text-align: right;
`;

const Signature = styled.div`
  color: #ffccd9;
  font-family: "Pokemon GB", sans-serif;
  font-size: 16px;
  font-weight: 700;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.5);
  letter-spacing: 0.5px;
  text-align: left;

  a {
    color: #a7edfe;
    text-decoration: none;
    transition: all 0.2s ease;
    &:hover {
      color: #ffccd9;
      text-shadow: 0 0 8px rgba(255, 204, 217, 0.4);
    }
  }
`;

const FooterRow = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  width: ${1440}px;
  margin: 0px auto 50px auto;
  gap: 40px;
`;

const scrollDiagonal = keyframes`
  from {
    background-position: 0 0;
  }
  to {
    background-position: -400px 400px;
  }
`;

// The actual game content area - sits under transparent portions of frame
const MainContainer = styled.div<MainContainerProps>`
  position: absolute;
  top: 0;
  left: 0;
  width: 1440px;
  height: 1080px;
  display: grid;
  gap: 0px 0px;
  grid-auto-flow: row;
  justify-items: center;
  align-items: center;
  overflow: hidden;
  z-index: 5;
  ${(props) =>
    props.$backgroundType === "login" &&
    `
    background: transparent;
  `}
  ${(props) =>
    props.$backgroundType === "characterCreation" &&
    css`
      background:
        linear-gradient(rgba(192, 193, 255, 0.4), rgba(192, 193, 255, 0.4)),
        url("/assets/pokewallpaper2.png");
      background-repeat: repeat;
      background-size: 400px;
      backdrop-filter: blur(10px);
      animation: ${scrollDiagonal} 40s linear infinite;
    `}
`;

const GlobalLoadingOverlay = () => {
  const isMapLoading = useGameStatusStore((state) => state.isMapLoading);
  if (!isMapLoading) return null;
  return <LoadingScreen isIndeterminate isGlobal />;
};

import { LayoutProvider, useLayout } from "@/contexts/LayoutContext";

// Custom hook for responsive scaling
const useResponsiveScale = () => {
  const [scale, setScale] = useState(1);
  const setUIScale = useGameStatusStore((s) => s.setUIScale);

  const calculateScale = useCallback(() => {
    const viewportWidth = window.visualViewport ? window.visualViewport.width : window.innerWidth;
    const viewportHeight = window.visualViewport ? window.visualViewport.height : window.innerHeight;

    // Calculate scale based on both dimensions, take the smaller to fit
    const scaleX = viewportWidth / DESIGN_WIDTH;
    const scaleY = viewportHeight / DESIGN_HEIGHT;
    const newScale = Math.min(scaleX, scaleY);

    // Clamp between min and max
    const clampedScale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, newScale));

    // Only update state if change is significant (> 1%) to avoid toolbar-toggle loops
    const currentScale = useGameStatusStore.getState().uiScale;
    if (Math.abs(clampedScale - currentScale) > 0.01) {
      setScale(clampedScale);
      setUIScale(clampedScale);
    }
  }, [setUIScale]);

  useEffect(() => {
    // Calculate on mount
    calculateScale();

    // Recalculate on resize
    window.addEventListener("resize", calculateScale);
    return () => window.removeEventListener("resize", calculateScale);
  }, [calculateScale]);

  return scale;
};

const LayoutContent = () => {
  const { currentScreen } = useGameScreenStore();
  const { mainRef } = useLayout();
  const scale = useResponsiveScale();

  // Phaser game should only be active on the game screen
  const showPhaserGame = currentScreen === "game";

  const isLogin =
    currentScreen === "title" ||
    currentScreen === "login" ||
    currentScreen === "register";
  const isCharacterCreation =
    currentScreen === "characterCreate" || currentScreen === "characterSelect";

  const backgroundType = isLogin
    ? "login"
    : isCharacterCreation
      ? "characterCreation"
      : "none";

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
  };

  return (
    <>
      {!IS_LOCAL_DEV && <WelcomeModals />}
      <AudioService />
      <ScalerContainer>
        <ScaledContent $scale={scale}>
          <FrameWrapper>
            {/* Main game content */}
            <MainContainer
              ref={mainRef}
              id="main"
              onContextMenu={handleContextMenu}
              $backgroundType={backgroundType}
            >
              {/* Phaser 2D game engine - only active on game screen */}
              <PhaserEngine isActive={showPhaserGame} />
              <ScreenRouter />
              <PokeBattleDisplay />
            </MainContainer>
            <GlobalLoadingOverlay />

          </FrameWrapper>
          <FooterRow>
            <Signature>
              Made with ❤️ by <a href="https://www.brynnbateman.com/" target="_blank" rel="noopener noreferrer">Brynn</a> — Hire me!<br /><a href="https://discord.gg/vH4GPChWKY" target="_blank" rel="noopener noreferrer">Join the Discord!</a>
            </Signature>
            <Disclaimer>
              CaptureQuest is a parody fan project and is not affiliated with,
              endorsed by, or connected to Nintendo, Game Freak, or The Pokémon
              Company. Pokémon is a registered trademark of Nintendo.
            </Disclaimer>
          </FooterRow>
        </ScaledContent>
      </ScalerContainer>
    </>
  );
};

const Layout = () => {
  return (
    <LayoutProvider>
      <LayoutContent />
    </LayoutProvider>
  );
};

export default Layout;
