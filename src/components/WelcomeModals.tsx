import { useState } from "react";
import styled from "styled-components";
import useGameStatusStore from "@stores/GameStatusStore";
import AudioManager from "@/services/audio/AudioManager";

const ModalOverlay = styled.div<{ $visible: boolean; $zIndex: number }>`
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  rgba(3, 78, 255, 0.16);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: ${(props) => props.$zIndex};
  transition: opacity 0.4s ease;
  opacity: ${(props) => (props.$visible ? 1 : 0)};
  pointer-events: ${(props) => (props.$visible ? "auto" : "none")};
  overflow-y: auto;
  overflow-x: hidden;
  padding: 40px 0;
  box-sizing: border-box;
`;

const ModalContent = styled.div<{ $maxWidth?: string }>`
  background: rgba(167, 237, 254, 0.95);
  backdrop-filter: blur(10px);
  width: 95%;
  max-width: ${(props) => props.$maxWidth || "700px"};
  max-height: 90vh;
  border-radius: 30px;
  border: 4px solid #2c5e6d;
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.4);
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  animation: modalSlide 0.5s cubic-bezier(0.16, 1, 0.3, 1);

  @keyframes modalSlide {
    from {
      transform: translateY(20px);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }

  @media (max-width: 850px) {
    max-height: none;
    width: 100%;
    max-width: 100%;
    border-radius: 0;
    margin-bottom: 0;
    border: none;
  }

  @media (max-height: 999px) and (min-width: 851px) {
    max-width: 900px;
  }
`;

const ModalMain = styled.div`
  display: flex;
  flex-direction: column;

  @media (max-height: 999px) and (min-width: 851px) {
    flex-direction: row;
  }
`;

const ImageContainer = styled.div`
  width: 100%;
  background-image: url("/assets/animebg.jpg");
  background-size: cover;
  background-position: center;
  display: flex;
  align-items: center;
  justify-content: center;
  border-top-left-radius: 20px;
  border-top-right-radius: 20px;
  overflow: hidden;
  padding: 40px;
  box-sizing: border-box;

  @media (max-width: 850px) {
    height: 200px;
  }

  @media (max-height: 999px) and (min-width: 851px) {
    width: 40%;
    min-width: 40%;
    border-top-right-radius: 0;
    border-bottom-left-radius: 20px;
    padding: 20px;
  }
`;

const ModalImage = styled.img`
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
`;

const ModalBody = styled.div<{ $center?: boolean }>`
  width: 100%;
  padding: 40px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  text-align: center;
  box-sizing: border-box;

  @media (max-width: 850px) {
    padding: 30px;
  }

  @media (max-height: 999px) and (min-width: 851px) {
    padding: 20px 30px;
    overflow-y: auto;
  }

  h2 {
    margin-top: 0;
    font-family: "Outfit", sans-serif;
    font-weight: 800;
    font-size: 2rem;
    background: linear-gradient(90deg, #4a4ba6, #c0c1ff);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  p {
    line-height: 1.6;
    font-family: "Outfit", sans-serif;
    font-size: 1.1rem;
    color: #4a4ba6;
    margin-bottom: 20px;
  }

  a {
    color: #4a4ba6;
    font-weight: 700;
    text-decoration: underline;
    transition: color 0.2s;
    &:hover {
      color: #c0c1ff;
    }
  }
`;

const ModalFooter = styled.div`
  padding: 20px 30px;
  background: rgba(0, 0, 0, 0.05);
  border-top: 1px solid rgba(0, 0, 0, 0.05);
  font-family: "Outfit", sans-serif;
  font-size: 0.75rem;
  color: #2c5e6d;
  line-height: 1.4;
  box-sizing: border-box;
`;

const CloseButton = styled.button`
  margin-top: 10px;
  padding: 16px 24px;
  background-color: #a7edfe;
  border: 3px solid #2c5e6d;
  border-radius: 16px;
  color: #2c5e6d;
  font-family: "Outfit", "Inter", system-ui, sans-serif;
  font-weight: 800;
  font-size: 1.1rem;
  cursor: pointer;
  width: 100%;
  transition: all 0.1s ease-in-out;
  box-sizing: border-box;
  box-shadow: 0 4px 0 #2c5e6d;

  &:hover {
    background-color: #c7f2fe;
    transform: translateY(-2px);
    box-shadow: 0 6px 0 #2c5e6d;
  }

  &:active {
    transform: translateY(2px);
    box-shadow: 0 2px 0 #3c5aa6;
  }
`;

const SoundToggleWrapper = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 20px;
  margin-bottom: 5px;
  padding: 10px;
  background: rgba(0, 0, 0, 0.03);
  border-radius: 12px;
  border: 1px solid rgba(0, 0, 0, 0.05);
`;

const SoundLabel = styled.span`
  font-family: "Outfit", sans-serif;
  font-size: 0.9rem;
  font-weight: 600;
  color: #2c5e6d;
  display: flex;
  align-items: center;
  gap: 8px;
`;

const ToggleSwitch = styled.div<{ $active: boolean }>`
  position: relative;
  width: 44px;
  height: 24px;
  background: ${(props) => (props.$active ? "#a7edfe" : "rgba(0, 0, 0, 0.1)")};
  border-radius: 12px;
  border: 1px solid #4a4ba6;
  cursor: pointer;
  transition: all 0.3s ease;

  &::after {
    content: "";
    position: absolute;
    top: 3px;
    left: ${(props) => (props.$active ? "23px" : "3px")};
    width: 18px;
    height: 18px;
    background: white;
    border-radius: 50%;
    transition: left 0.3s cubic-bezier(0.18, 0.89, 0.35, 1.15);
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
  }
`;

export const WelcomeModals = () => {
  const shouldShowDesktopWarning = window.innerWidth < 768;
  const [welcomeVisible, setWelcomeVisible] = useState(
    !shouldShowDesktopWarning,
  );
  const [warningVisible, setWarningVisible] = useState(
    shouldShowDesktopWarning,
  );
  const { isMuted, toggleMute } = useGameStatusStore();

  const initializeAudio = async () => {
    if (!AudioManager.isInitialized()) {
      const { sfxVolume, ambientVolume, musicVolume, isMuted } =
        useGameStatusStore.getState();
      await AudioManager.initialize({
        sfx: sfxVolume,
        ambient: ambientVolume,
        music: musicVolume,
        muted: isMuted,
      });
    }
  };

  const handleCloseWelcome = async () => {
    await initializeAudio();
    AudioManager.playMusic("/sound/title.mp3", 0, true);
    setWelcomeVisible(false);
  };

  const handleCloseWarning = () => {
    setWarningVisible(false);
    setWelcomeVisible(true);
  };

  const handleToggleSound = () => {
    initializeAudio();
    toggleMute();
  };

  if (!welcomeVisible && !warningVisible) return null;

  return (
    <>
      <ModalOverlay $visible={welcomeVisible} $zIndex={1000}>
        <ModalContent>
          <ModalMain>
            <ImageContainer>
              <ModalImage
                src="/assets/capturequestlogo.png"
                alt="CaptureQuest Logo"
              />
            </ImageContainer>
            <ModalBody>
              <h2 style={{ marginBottom: "0px" }}>Gotta Capture 'Em All!</h2>
              <p style={{ textAlign: "left" }}>
                CaptureQuest is a fan-made MMO based on the original
                Pokémon GameBoy games from 1996. It's been very well
                implemented true to the original, but not rigorously tested, so you
                should still expect bugs and sharp edges while exploring.
                Assets, logic, and many other parts have been directly
                extracted from the original GameBoy cartridges using my tooling{" "}
                <a
                  href="https://github.com/brynnb/pokemon-gameboy-extractor-tool"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  here
                </a>
                .
              </p>
              <p style={{ fontWeight: 500, marginTop: "10px" }}>
                Made with ❤️ by{" "}
                <a href="https://www.brynnbateman.com" target="_blank">
                  Brynn
                </a>{" "}
                - Hire me!
              </p>

              <SoundToggleWrapper>
                <SoundLabel>
                  {isMuted ? "🔈 Sound Muted" : "🔊 Sound Enabled"}
                </SoundLabel>
                <ToggleSwitch $active={!isMuted} onClick={handleToggleSound} />
              </SoundToggleWrapper>

              <CloseButton onClick={handleCloseWelcome}>
                Begin Your Journey
              </CloseButton>
            </ModalBody>
          </ModalMain>
          <ModalFooter>
            CaptureQuest is a parody non-commercial project and is not
            affiliated with, endorsed by, or connected to Nintendo, Game Freak,
            or The Pokémon Company. Pokémon is a registered trademark of
            Nintendo.
          </ModalFooter>
        </ModalContent>
      </ModalOverlay>

      <ModalOverlay $visible={warningVisible} $zIndex={1001}>
        <ModalContent $maxWidth="500px">
          <ModalBody $center>
            <h2 style={{ marginBottom: "20px" }}>🖥️ Desktop Recommended</h2>
            <p>
              CaptureQuest uses a fixed-size classic UI and is designed for
              desktop browsers and larger screen resolutions. You may experience
              limited functionality or display issues on mobile devices and
              smaller screens.
            </p>
            <CloseButton onClick={handleCloseWarning}>
              Continue Anyway
            </CloseButton>
          </ModalBody>
        </ModalContent>
      </ModalOverlay>
    </>
  );
};
