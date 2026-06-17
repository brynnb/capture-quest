import React, { useEffect, useCallback } from "react";
import styled from "styled-components";
import usePokedexStore from "@/stores/PokedexStore";
import useGameStatusStore from "@stores/GameStatusStore";
import { WorldSocket, OpCodes } from "@/net";
import { BadgeAtlasIcon, BADGE_FLAGS, BADGE_NAMES } from "./BadgeAtlasIcon";

const Overlay = styled.div`
  position: fixed;
  top: 0; left: 0; width: 100%; height: 100%;
  display: flex; align-items: center; justify-content: center;
  z-index: 2000;
`;

const Card = styled.div`
  width: 420px;
  background: #f8f0e0;
  border: 4px solid #383838;
  border-radius: 12px;
  overflow: hidden;
  font-family: 'Outfit', monospace, sans-serif;
  filter: drop-shadow(0 16px 40px rgba(0,0,0,0.4));
`;

const CardHeader = styled.div`
  background: #e53935;
  color: white;
  padding: 14px 20px;
  font-size: 18px;
  font-weight: 800;
  letter-spacing: 1px;
  border-bottom: 3px solid #383838;
`;

const CardBody = styled.div`
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
`;

const InfoRow = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const Label = styled.span`
  font-size: 13px;
  font-weight: 600;
  color: #888;
`;

const Value = styled.span`
  font-size: 15px;
  font-weight: 700;
  color: #1a1a1a;
`;

const Divider = styled.div`
  height: 2px;
  background: #d8c8a8;
`;

const BadgeSection = styled.div`
  display: flex;
  flex-direction: column;
  gap: 10px;
`;

const BadgeSectionTitle = styled.div`
  font-size: 14px;
  font-weight: 700;
  color: #1a1a1a;
`;

const BadgeGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
`;

const BadgeSlot = styled.div<{ $earned: boolean }>`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px 4px;
  background: ${p => p.$earned ? 'white' : '#e8e0d0'};
  border: 2px solid ${p => p.$earned ? '#383838' : '#ccc'};
  border-radius: 8px;
`;

const BadgeArtFrame = styled.div`
  width: 38px;
  height: 41px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const BadgeName = styled.span<{ $earned: boolean }>`
  font-size: 9px;
  font-weight: 700;
  color: ${p => p.$earned ? '#333' : '#aaa'};
  text-align: center;
`;

const PokedexRow = styled.div`
  display: flex;
  gap: 24px;
`;

const PokedexStat = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const PokedexLabel = styled.span`
  font-size: 11px;
  font-weight: 600;
  color: #888;
`;

const PokedexValue = styled.span`
  font-size: 18px;
  font-weight: 800;
  color: #1a1a1a;
`;

function formatPlayTime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes.toString().padStart(2, "0")}m`;
}

function formatMoney(amount: number): string {
  return `\u00a5${amount.toLocaleString()}`;
}

const TrainerCard: React.FC = () => {
  const { trainerCard } = usePokedexStore();
  const { toggleTrainerCard } = useGameStatusStore();

  useEffect(() => {
    WorldSocket.sendJsonMessage(OpCodes.TrainerCardRequest, {});
  }, []);

  const handleClose = useCallback(() => {
    toggleTrainerCard();
  }, [toggleTrainerCard]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [handleClose]);

  if (!trainerCard) {
    return (
      <Overlay onClick={handleClose}>
        <Card onClick={e => e.stopPropagation()}>
          <CardHeader>TRAINER CARD</CardHeader>
          <CardBody>
            <Value style={{ textAlign: "center", color: "#888" }}>Loading...</Value>
          </CardBody>
        </Card>
      </Overlay>
    );
  }

  const earnedBadges = new Set(trainerCard.badges || []);

  return (
    <Overlay onClick={handleClose}>
      <Card onClick={e => e.stopPropagation()}>
        <CardHeader>TRAINER CARD</CardHeader>
        <CardBody>
          <InfoRow>
            <Label>Name</Label>
            <Value>{trainerCard.name}</Value>
          </InfoRow>

          <InfoRow>
            <Label>Money</Label>
            <Value>{formatMoney(trainerCard.money)}</Value>
          </InfoRow>

          <InfoRow>
            <Label>Play Time</Label>
            <Value>{formatPlayTime(trainerCard.timePlayed)}</Value>
          </InfoRow>

          <Divider />

          <PokedexRow>
            <PokedexStat>
              <PokedexLabel>POK\u00c9DEX Seen</PokedexLabel>
              <PokedexValue>{trainerCard.pokedexSeen}</PokedexValue>
            </PokedexStat>
            <PokedexStat>
              <PokedexLabel>POK\u00c9DEX Caught</PokedexLabel>
              <PokedexValue>{trainerCard.pokedexCaught}</PokedexValue>
            </PokedexStat>
          </PokedexRow>

          <Divider />

          <BadgeSection>
            <BadgeSectionTitle>
              Badges ({trainerCard.badgeCount} / 8)
            </BadgeSectionTitle>
            <BadgeGrid>
              {BADGE_NAMES.map((name, i) => {
                const earned = earnedBadges.has(BADGE_FLAGS[i]);
                return (
                  <BadgeSlot key={name} $earned={earned}>
                    <BadgeArtFrame>
                      <BadgeAtlasIcon
                        index={i}
                        earned={earned}
                        size={34}
                        title={`${name} Badge`}
                      />
                    </BadgeArtFrame>
                    <BadgeName $earned={earned}>{name}</BadgeName>
                  </BadgeSlot>
                );
              })}
            </BadgeGrid>
          </BadgeSection>
        </CardBody>
      </Card>
    </Overlay>
  );
};

export default TrainerCard;
