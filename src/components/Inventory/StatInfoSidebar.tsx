import React, { useEffect } from "react";
import styled from "styled-components";
import { OpCodes, WorldSocket } from "@/net";
import {
  BadgeAtlasIcon,
  BADGE_FLAGS,
  BADGE_NAMES,
} from "@/components/Interface/BadgeAtlasIcon";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePokedexStore from "@/stores/PokedexStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";

const StatInfoBarWrapper = styled.div.attrs({
  className: "stat-info-bar-wrapper",
})`
  position: absolute;
  left: 40px;
  top: 30px;
  width: 242px;
  background: rgba(248, 240, 224, 0.92);
  backdrop-filter: blur(12px);
  border: 4px solid #383838;
  border-radius: 8px;
  padding: 12px;
  box-sizing: border-box;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.22);
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 10px;
  font-family: "Pokemon GB", "Press Start 2P", "Outfit", monospace;
`;

const TrainerHeader = styled.div`
  display: grid;
  grid-template-columns: 58px 1fr;
  gap: 10px;
  align-items: center;
  min-width: 0;
`;

const TrainerPortrait = styled.img`
  width: 58px;
  height: 58px;
  object-fit: contain;
  image-rendering: pixelated;
  background: #ffffff;
  border: 3px solid #383838;
  border-radius: 6px;
  padding: 4px;
  box-sizing: border-box;
`;

const TrainerText = styled.div`
  min-width: 0;
`;

const Name = styled.div`
  color: #1a1a1a;
  font-size: 14px;
  font-weight: 900;
  line-height: 1.2;
  overflow-wrap: anywhere;
`;

const InfoPanel = styled.div`
  background: #ffffff;
  border: 3px solid #383838;
  border-radius: 6px;
  padding: 9px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
`;

const SectionTitle = styled.div`
  color: #e53935;
  font-size: 9px;
  font-weight: 900;
  line-height: 1;
  text-transform: uppercase;
`;

const StatGrid = styled.div`
  display: flex;
  flex-direction: column;
  gap: 7px;
`;

const StatRow = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
`;

const StatLabel = styled.span`
  color: #555555;
  font-size: 8px;
  font-weight: 800;
  line-height: 1.2;
  text-transform: uppercase;
`;

const StatValue = styled.span`
  color: #1a1a1a;
  font-size: 10px;
  font-weight: 900;
  line-height: 1.2;
  text-align: right;
  white-space: nowrap;
`;

const BadgeGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 4px;
  align-items: center;
`;

const BadgeCell = styled.div`
  min-width: 0;
  display: flex;
  justify-content: center;
`;

const LoadingText = styled.div`
  color: #666666;
  font-size: 9px;
  line-height: 1.4;
  text-align: center;
`;

function getTrainerImage(gender?: number): string {
  switch (gender) {
    case 1:
      return "/assets/trainerf.png";
    case 2:
      return "/assets/trainernb.png";
    case 0:
    default:
      return "/assets/trainerm.png";
  }
}

function formatMoney(amount: number): string {
  return `\u00a5${amount.toLocaleString()}`;
}

function formatPlayTime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes.toString().padStart(2, "0")}m`;
}

const StatInfoBar: React.FC = () => {
  const characterProfile = usePlayerCharacterStore((state) => state.characterProfile);
  const trainerCard = usePokedexStore((state) => state.trainerCard);
  const money = useCQInventoryStore((state) => state.money);
  const inventoryItems = useCQInventoryStore((state) => state.items);
  const party = usePokemonPartyStore((state) => state.party);

  useEffect(() => {
    WorldSocket.sendJsonMessage(OpCodes.TrainerCardRequest, {});
  }, []);

  if (!characterProfile?.name) {
    return (
      <StatInfoBarWrapper>
        <LoadingText>Loading trainer...</LoadingText>
      </StatInfoBarWrapper>
    );
  }

  const earnedBadges = new Set(trainerCard?.badges || []);
  const badgeCount = trainerCard?.badgeCount ?? earnedBadges.size;
  const itemCount = inventoryItems.reduce(
    (sum, item) => sum + Math.max(0, item.instance.quantity || 0),
    0,
  );
  const pokedexSeen = trainerCard?.pokedexSeen ?? 0;
  const pokedexCaught = trainerCard?.pokedexCaught ?? 0;
  const displayMoney = trainerCard?.money ?? money;

  return (
    <StatInfoBarWrapper>
      <TrainerHeader>
        <TrainerPortrait
          src={getTrainerImage(characterProfile.gender)}
          alt=""
        />
        <TrainerText>
          <Name>{trainerCard?.name || characterProfile.name}</Name>
        </TrainerText>
      </TrainerHeader>

      <InfoPanel>
        <SectionTitle>Trainer</SectionTitle>
        <StatGrid>
          <StatRow>
            <StatLabel>Money</StatLabel>
            <StatValue>{formatMoney(displayMoney)}</StatValue>
          </StatRow>
          <StatRow>
            <StatLabel>Badges</StatLabel>
            <StatValue>{badgeCount} / 8</StatValue>
          </StatRow>
          <StatRow>
            <StatLabel>Play Time</StatLabel>
            <StatValue>{formatPlayTime(trainerCard?.timePlayed ?? characterProfile.timePlayed ?? 0)}</StatValue>
          </StatRow>
        </StatGrid>
      </InfoPanel>

      <InfoPanel>
        <SectionTitle>Badges</SectionTitle>
        <BadgeGrid>
          {BADGE_FLAGS.map((flag, index) => (
            <BadgeCell key={flag}>
              <BadgeAtlasIcon
                index={index}
                earned={earnedBadges.has(flag)}
                size={19}
                title={`${BADGE_NAMES[index]} Badge`}
              />
            </BadgeCell>
          ))}
        </BadgeGrid>
      </InfoPanel>

      <InfoPanel>
        <SectionTitle>Pokedex</SectionTitle>
        <StatGrid>
          <StatRow>
            <StatLabel>Seen</StatLabel>
            <StatValue>{pokedexSeen} / 151</StatValue>
          </StatRow>
          <StatRow>
            <StatLabel>Caught</StatLabel>
            <StatValue>{pokedexCaught} / 151</StatValue>
          </StatRow>
        </StatGrid>
      </InfoPanel>

      <InfoPanel>
        <SectionTitle>Pokemon</SectionTitle>
        <StatGrid>
          <StatRow>
            <StatLabel>Party</StatLabel>
            <StatValue>{party.length} / 6</StatValue>
          </StatRow>
          <StatRow>
            <StatLabel>Inventory Items</StatLabel>
            <StatValue>{itemCount}</StatValue>
          </StatRow>
        </StatGrid>
      </InfoPanel>
    </StatInfoBarWrapper>
  );
};

export default StatInfoBar;
