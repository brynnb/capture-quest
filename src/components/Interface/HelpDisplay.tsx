import React from "react";
import styled from "styled-components";
import useGameStatusStore from "@stores/GameStatusStore";
import ActionButton from "@components/Interface/ActionButton";

const CenterViewport = styled.div`
  width: 902px;
  height: 650px;
  position: absolute;
  left: 50%;
  top: 30px;
  transform: translateX(-50%);
  background: rgba(192, 193, 255, 0.57);
  backdrop-filter: blur(12px);
  border: 4px solid #4a4ba6;
  border-radius: 30px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 50px 80px 50px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.3);
  transition: all 0.3s ease;
  box-sizing: border-box;
`;

const HelpTitle = styled.h2`
  font-family: "Outfit", sans-serif;
  text-transform: uppercase;
  font-weight: 800;
  font-size: 32px;
  text-align: center;
  margin: 0 0 20px 0;
  color: #2e2f66;
`;

const HelpContent = styled.div`
  flex: 1;
  width: 100%;
  overflow-y: auto;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-size: 16px;
  line-height: 1.6;
  padding-right: 10px;

  scrollbar-width: thin;
  scrollbar-color: rgba(74, 75, 166, 0.4) transparent;

  &::-webkit-scrollbar {
    width: 8px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background-color: rgba(74, 75, 166, 0.4);
    border-radius: 4px;
  }
`;

const SectionHeading = styled.h3`
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  font-size: 20px;
  color: #2e2f66;
  margin: 20px 0 8px 0;
  text-transform: uppercase;

  &:first-child {
    margin-top: 0;
  }
`;

const Paragraph = styled.p`
  margin: 0 0 12px 0;
  font-weight: 500;
  color: #3e3f7a;
`;

const DemoBadge = styled.div`
  display: inline-block;
  background: #ffde75;
  border: 3px solid #c4a830;
  border-radius: 12px;
  padding: 6px 16px;
  font-weight: 800;
  font-size: 14px;
  text-transform: uppercase;
  color: #6b5a00;
  margin-bottom: 16px;
`;

const FeatureList = styled.ul`
  margin: 0 0 12px 0;
  padding-left: 20px;

  li {
    margin-bottom: 6px;
    font-weight: 500;
    color: #3e3f7a;
  }
`;

const HelpDisplay: React.FC = () => {
  const { toggleHelp } = useGameStatusStore();

  return (
    <CenterViewport>
      <HelpTitle>About This Project</HelpTitle>
      <DemoBadge>Demo / Showcase Build</DemoBadge>
      <HelpContent>
        <Paragraph>
          Welcome to CaptureQuest, a multiplayer online game inspired by the
          original Pok&eacute;mon GameBoy games. Explore the Kanto region,
          catch Pok&eacute;mon, battle trainers, and chat with other players
          in a shared world.
        </Paragraph>

        <Paragraph>
          This is a demo build and not all features are fully functional.
          Scripted in-game events are now very fully implemented, but they have
          not been thoroughly tested, so there will still be bugs and strange
          edges. Enjoy exploring what&rsquo;s here and have fun!
        </Paragraph>

        <SectionHeading>Controls</SectionHeading>
        <FeatureList>
          <li>Use WASD or arrow keys to move your character</li>
          <li>Click/tap on the map to move to a location</li>
          <li>Use the toolbar icons to access the Pok&eacute;dex, inventory, fishing, and more</li>
          <li>Walk into tall grass to encounter wild Pok&eacute;mon</li>
          <li>Visit Pok&eacute; Marts and Pok&eacute;mon Centers in towns</li>
          <li>Chat with other players using the chatbox</li>
        </FeatureList>

        <SectionHeading>Disclaimer</SectionHeading>
        <Paragraph>
          This is a personal portfolio/showcase project and is not affiliated
          with or endorsed by Nintendo, Game Freak, or The Pok&eacute;mon
          Company. All original Pok&eacute;mon assets and trademarks belong to
          their respective owners.
        </Paragraph>
      </HelpContent>

      <ActionButton
        text="Done"
        onClick={toggleHelp}
        customCSS={`position: absolute; bottom: 30px; right: 30px; width: 120px;`}
      />
    </CenterViewport>
  );
};

export default HelpDisplay;
