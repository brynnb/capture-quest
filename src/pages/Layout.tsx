import styled, { keyframes, css } from "styled-components";
import useGameScreenStore from "@stores/GameScreenStore";
import ScreenRouter from "@/components/ScreenRouter";
import { useEffect } from "react";
import useGameStatusStore from "@stores/GameStatusStore";
import LoadingScreen from "@/components/LoadingScreen";
import AudioService from "@/services/audio/AudioService";
import PhaserEngine from "@/phaser-game/PhaserEngine";
import { WelcomeModals } from "@/components/WelcomeModals";
import PokeBattleDisplay from "@/components/PokeBattleDisplay";
import { IS_LOCAL_DEV } from "@/config";

const AppViewport = styled.div`
  width: 100%;
  height: var(--cq-viewport-height, 100dvh);
  padding: max(8px, env(safe-area-inset-top, 0px))
    max(8px, env(safe-area-inset-right, 0px))
    max(8px, env(safe-area-inset-bottom, 0px))
    max(8px, env(safe-area-inset-left, 0px));
  box-sizing: border-box;
  overflow: hidden;

  @media (max-width: 850px), (pointer: coarse) {
    padding: 0;
  }
`;

const FRAME_BORDER = 10;
const POKEMON_FRAME_PATH = "/assets/pokemon_frame/frame-hd.png";

interface MainContainerProps {
  $backgroundType: "login" | "characterCreation" | "none";
}

// Outer wrapper that includes both the frame and the content
const FrameWrapper = styled.div`
  position: relative;
  width: 100%;
  height: 100%;

  border-style: solid;
  border-width: ${FRAME_BORDER}px;
  border-image: url("${POKEMON_FRAME_PATH}") 42 round;
  image-rendering: pixelated;
  background: var(--pkmn-neutral);
  background-clip: padding-box; /* Prevents background from bleeding under the border */
  box-sizing: border-box;

  @media (max-width: 850px), (pointer: coarse) {
    border-width: 0;
  }
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

  @media (max-width: 850px), (max-height: 700px), (pointer: coarse) {
    max-width: 100%;
    font-size: 7px;
    line-height: 1.35;
    text-align: center;
  }
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

  @media (max-width: 850px), (max-height: 700px), (pointer: coarse) {
    font-size: 11px;
    line-height: 1.35;
    text-align: center;
  }
`;

const FooterRow = styled.div<{ $hidden: boolean }>`
  position: absolute;
  right: 24px;
  bottom: 18px;
  left: 24px;
  z-index: 40;
  display: ${(props) => (props.$hidden ? "none" : "flex")};
  justify-content: space-between;
  align-items: flex-end;
  gap: 40px;
  pointer-events: none;

  a {
    pointer-events: auto;
  }

  @media (max-width: 850px), (max-height: 700px), (pointer: coarse) {
    right: max(10px, env(safe-area-inset-right, 0px));
    bottom: max(8px, env(safe-area-inset-bottom, 0px));
    left: max(10px, env(safe-area-inset-left, 0px));
    display: ${(props) => (props.$hidden ? "none" : "flex")};
    flex-direction: column;
    align-items: center;
    gap: 5px;
    padding: 7px 10px;
    border: 1px solid rgba(255, 255, 255, 0.28);
    border-radius: 12px;
    background: rgba(26, 28, 70, 0.72);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
    backdrop-filter: blur(8px);
  }
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
  inset: 0;
  width: 100%;
  height: 100%;
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

const useDynamicViewportHeight = () => {
  useEffect(() => {
    const updateHeight = () => {
      const height = window.visualViewport?.height ?? window.innerHeight;
      document.documentElement.style.setProperty(
        "--cq-viewport-height",
        `${Math.round(height)}px`,
      );
    };
    updateHeight();
    window.addEventListener("resize", updateHeight);
    window.visualViewport?.addEventListener("resize", updateHeight);
    return () => {
      window.removeEventListener("resize", updateHeight);
      window.visualViewport?.removeEventListener("resize", updateHeight);
    };
  }, []);
};

const LayoutContent = () => {
  const { currentScreen } = useGameScreenStore();
  const { mainRef } = useLayout();
  useDynamicViewportHeight();

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
      <AppViewport>
        <FrameWrapper>
          <MainContainer
            ref={mainRef}
            id="main"
            onContextMenu={handleContextMenu}
            $backgroundType={backgroundType}
          >
            <PhaserEngine isActive={showPhaserGame} />
            <ScreenRouter />
            <PokeBattleDisplay />
          </MainContainer>
          <GlobalLoadingOverlay />
          <FooterRow $hidden={showPhaserGame || isCharacterCreation}>
            <Signature>
              Made with ❤️ by <a href="https://www.brynnbateman.com/" target="_blank" rel="noopener noreferrer">Brynn</a> — Hire me!<br /><a href="https://discord.gg/vH4GPChWKY" target="_blank" rel="noopener noreferrer">Join the Discord!</a>
            </Signature>
            <Disclaimer>
              CaptureQuest is a parody fan project and is not affiliated with,
              endorsed by, or connected to Nintendo, Game Freak, or The Pokémon
              Company. Pokémon is a registered trademark of Nintendo.
            </Disclaimer>
          </FooterRow>
        </FrameWrapper>
      </AppViewport>
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
