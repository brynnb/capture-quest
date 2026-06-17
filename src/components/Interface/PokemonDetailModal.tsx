import React, { useCallback } from "react";
import styled from "styled-components";
import type { MoveDTO, PokemonDTO } from "@/net/generated/world_api";
import AudioManager from "@/services/audio/AudioManager";
import { cryPathForPokemon } from "@/services/audio/pokemonMusic";

const TYPE_COLORS: Record<string, string> = {
  NORMAL: "#A8A878",
  FIRE: "#F08030",
  WATER: "#6890F0",
  ELECTRIC: "#F8D030",
  GRASS: "#78C850",
  ICE: "#98D8D8",
  FIGHTING: "#C03028",
  POISON: "#A040A0",
  GROUND: "#E0C068",
  FLYING: "#A890F0",
  PSYCHIC: "#F85888",
  BUG: "#A8B820",
  ROCK: "#B8A038",
  GHOST: "#705898",
  DRAGON: "#7038F8",
};

const Overlay = styled.div`
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5000;
`;

const Modal = styled.div`
  width: 420px;
  background: #f8f0e0;
  border: 4px solid #383838;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  font-family: 'Outfit', monospace, sans-serif;
  overflow: hidden;
`;

const Header = styled.div`
  background: #e8d8b8;
  border-bottom: 3px solid #383838;
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
`;

const SpriteBox = styled.div`
  width: 80px;
  height: 80px;
  background: white;
  border: 2px solid #383838;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  img {
    width: 72px;
    height: 72px;
    image-rendering: pixelated;
    object-fit: contain;
  }
`;

const HeaderInfo = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
`;

const NameRow = styled.div`
  display: flex;
  align-items: baseline;
  gap: 8px;
`;

const PokeName = styled.span`
  font-size: 22px;
  font-weight: 800;
  color: #1a1a1a;
  text-transform: capitalize;
`;

const PokeLevel = styled.span`
  font-size: 16px;
  font-weight: 600;
  color: #555;
`;

const TypeRow = styled.div`
  display: flex;
  gap: 6px;
`;

const TypeBadge = styled.span<{ $color: string }>`
  font-size: 11px;
  font-weight: 700;
  color: white;
  background: ${props => props.$color};
  padding: 2px 10px;
  border-radius: 3px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
`;

const HPRow = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
`;

const HPLabel = styled.span`
  font-size: 12px;
  font-weight: 700;
  color: #555;
`;

const HPBarOuter = styled.div`
  flex: 1;
  height: 8px;
  background: #383838;
  border-radius: 4px;
  overflow: hidden;
`;

const HPBarInner = styled.div<{ $percent: number }>`
  width: ${props => props.$percent}%;
  height: 100%;
  background: ${props =>
    props.$percent > 50 ? '#4caf50'
      : props.$percent > 20 ? '#ffa726'
        : '#e53935'};
  transition: width 0.3s ease;
`;

const HPText = styled.span`
  font-size: 12px;
  font-weight: 600;
  color: #333;
  min-width: 60px;
  text-align: right;
`;

const CryButton = styled.button`
  align-self: flex-start;
  background: white;
  border: 2px solid #383838;
  border-radius: 4px;
  color: #1a1a1a;
  cursor: pointer;
  font-family: 'Outfit', sans-serif;
  font-size: 11px;
  font-weight: 800;
  padding: 4px 10px;

  &:hover {
    background: #f0e8d8;
  }
`;

const Body = styled.div`
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
`;

const SectionTitle = styled.div`
  font-size: 13px;
  font-weight: 700;
  color: #888;
  text-transform: uppercase;
  letter-spacing: 1px;
  border-bottom: 2px solid #d8c8a8;
  padding-bottom: 4px;
  margin-bottom: 2px;
`;

const StatsGrid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px 16px;
`;

const StatRow = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const StatLabel = styled.span`
  font-size: 13px;
  font-weight: 600;
  color: #666;
`;

const StatValue = styled.span`
  font-size: 14px;
  font-weight: 800;
  color: #1a1a1a;
`;

const MovesGrid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
`;

const MoveCard = styled.div<{ $typeColor: string; $clickable?: boolean }>`
  background: white;
  border: 2px solid ${props => props.$typeColor};
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  cursor: ${props => props.$clickable ? 'pointer' : 'default'};
  transition: transform 0.1s ease, box-shadow 0.1s ease;

  ${props => props.$clickable && `
    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
      background: #f0fff0;
    }
  `}
`;

const MoveName = styled.div`
  font-size: 13px;
  font-weight: 700;
  color: #1a1a1a;
  text-transform: capitalize;
`;

const MoveInfo = styled.div`
  font-size: 10px;
  font-weight: 500;
  color: #888;
  display: flex;
  justify-content: space-between;
`;

const EmptyMove = styled.div`
  background: #eee;
  border: 2px dashed #ccc;
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 2px;
  font-size: 12px;
  color: #aaa;
  min-height: 42px;
  box-sizing: border-box;
`;

const StatusRow = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
`;

const StatusLabel = styled.span`
  font-size: 13px;
  font-weight: 600;
  color: #666;
`;

const StatusBadgeLarge = styled.span<{ $status: string }>`
  font-size: 12px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 4px;
  color: white;
  background: ${props => {
    switch (props.$status) {
      case 'BRN': return '#e57373';
      case 'FRZ': return '#4fc3f7';
      case 'PAR': return '#fff176';
      case 'PSN': return '#ba68c8';
      case 'TOX': return '#9c27b0';
      case 'SLP': return '#90a4ae';
      default: return '#4caf50';
    }
  }};
  ${props => props.$status === 'PAR' && 'color: #333;'}
`;

const CloseButton = styled.button`
  width: 100%;
  padding: 10px;
  background: #e8d8b8;
  border: none;
  border-top: 3px solid #383838;
  font-family: 'Outfit', sans-serif;
  font-size: 14px;
  font-weight: 700;
  color: #383838;
  cursor: pointer;
  transition: background 0.2s;

  &:hover {
    background: #d8c8a8;
  }
`;

const ItemBanner = styled.div`
  background: #d4edda;
  border-bottom: 2px solid #28a745;
  padding: 8px 16px;
  font-family: 'Outfit', sans-serif;
  font-size: 13px;
  font-weight: 700;
  color: #155724;
  text-align: center;
`;

interface Props {
  pokemon: PokemonDTO;
  onClose: () => void;
  onMoveClick?: (moveSlot: number) => void;
  onFieldMoveClick?: (move: MoveDTO) => void;
  isFieldMove?: (move: MoveDTO) => boolean;
  itemBannerText?: string;
}

const PokemonDetailModal: React.FC<Props> = ({
  pokemon,
  onClose,
  onMoveClick,
  onFieldMoveClick,
  isFieldMove,
  itemBannerText,
}) => {
  const hpPercent = pokemon.maxHp > 0
    ? Math.round((pokemon.curHp / pokemon.maxHp) * 100)
    : 0;

  const getTypeColor = (type: string) =>
    TYPE_COLORS[type.toUpperCase()] || "#A8A878";

  const statusText = pokemon.status && pokemon.status !== ""
    ? pokemon.status
    : "OK";

  const playCry = useCallback(() => {
    const path = cryPathForPokemon(pokemon.name, pokemon.crySfx);
    if (path) {
      void AudioManager.playSFX(path, 0.8);
    }
  }, [pokemon.crySfx, pokemon.name]);

  // Fill moves array to always show 4 slots
  const moves = [...(pokemon.moves || [])];
  while (moves.length < 4) {
    moves.push(null as never);
  }

  return (
    <Overlay onClick={onClose}>
      <Modal onClick={(e) => e.stopPropagation()}>
        {itemBannerText && <ItemBanner>{itemBannerText}</ItemBanner>}
        <Header>
          <SpriteBox>
            <img
              src={`https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/${pokemon.id}.png`}
              alt={pokemon.name}
            />
          </SpriteBox>
          <HeaderInfo>
            <NameRow>
              <PokeName>{pokemon.name.toLowerCase()}</PokeName>
              <PokeLevel>Lv. {pokemon.level}</PokeLevel>
            </NameRow>
            <TypeRow>
              <TypeBadge $color={getTypeColor(pokemon.type1)}>
                {pokemon.type1}
              </TypeBadge>
              {pokemon.type2 && pokemon.type2 !== pokemon.type1 && (
                <TypeBadge $color={getTypeColor(pokemon.type2)}>
                  {pokemon.type2}
                </TypeBadge>
              )}
            </TypeRow>
            <HPRow>
              <HPLabel>HP</HPLabel>
              <HPBarOuter>
                <HPBarInner $percent={hpPercent} />
              </HPBarOuter>
              <HPText>{pokemon.curHp} / {pokemon.maxHp}</HPText>
            </HPRow>
            {pokemon.crySfx && <CryButton onClick={playCry}>Cry</CryButton>}
          </HeaderInfo>
        </Header>

        <Body>
          <div>
            <SectionTitle>Stats</SectionTitle>
            <StatsGrid>
              <StatRow>
                <StatLabel>Attack</StatLabel>
                <StatValue>{pokemon.attack}</StatValue>
              </StatRow>
              <StatRow>
                <StatLabel>Defense</StatLabel>
                <StatValue>{pokemon.defense}</StatValue>
              </StatRow>
              <StatRow>
                <StatLabel>Speed</StatLabel>
                <StatValue>{pokemon.speed}</StatValue>
              </StatRow>
              <StatRow>
                <StatLabel>Special</StatLabel>
                <StatValue>{pokemon.special}</StatValue>
              </StatRow>
            </StatsGrid>
          </div>

          <div>
            <SectionTitle>Moves</SectionTitle>
            <MovesGrid>
              {moves.map((move, i) =>
                move ? (
                  (() => {
                    const canUseMove = !!onMoveClick || (!!onFieldMoveClick && !!isFieldMove?.(move));
                    return (
                  <MoveCard
                    key={i}
                    $typeColor={getTypeColor(move.type)}
                    $clickable={canUseMove}
                    onClick={
                      onMoveClick
                        ? () => onMoveClick(i)
                        : canUseMove
                          ? () => onFieldMoveClick?.(move)
                          : undefined
                    }
                  >
                    <MoveName>{move.name.toLowerCase()}</MoveName>
                    <MoveInfo>
                      <span>{move.type}</span>
                      <span>PP {move.pp}/{move.maxPp}</span>
                      {move.power > 0 && <span>Pwr {move.power}</span>}
                      {!onMoveClick && canUseMove && <span>Field</span>}
                    </MoveInfo>
                  </MoveCard>
                    );
                  })()
                ) : (
                  <EmptyMove key={i}>---</EmptyMove>
                )
              )}
            </MovesGrid>
          </div>

          <div>
            <SectionTitle>Experience</SectionTitle>
            <StatsGrid>
              <StatRow>
                <StatLabel>EXP</StatLabel>
                <StatValue>{pokemon.exp.toLocaleString()}</StatValue>
              </StatRow>
              <StatRow>
                <StatLabel>To Next Lv.</StatLabel>
                <StatValue>
                  {pokemon.level >= 100 ? "MAX" : (pokemon.expToNextLevel ?? 0).toLocaleString()}
                </StatValue>
              </StatRow>
            </StatsGrid>
          </div>

          <StatusRow>
            <StatusLabel>Status:</StatusLabel>
            <StatusBadgeLarge $status={statusText}>
              {statusText === "OK" ? "Healthy" : statusText}
            </StatusBadgeLarge>
          </StatusRow>
        </Body>

        <CloseButton onClick={onClose}>Close</CloseButton>
      </Modal>
    </Overlay>
  );
};

export default PokemonDetailModal;
