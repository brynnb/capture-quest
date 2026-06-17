import React, { useEffect } from "react";
import styled from "styled-components";
import { canPokemonLearnTMHM } from "@/constants/tmhmMoveData";
import type { PhaserMoveFull, PokemonDTO } from "@/net/generated/world_api";
import { ITEM_TYPE_HM, type CQInventoryItem } from "@/stores/CQInventoryStore";

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
  inset: 0;
  z-index: 6000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.25);
`;

const Modal = styled.div`
  width: 620px;
  max-height: 86vh;
  background: #f8f0e0;
  border: 4px solid #383838;
  border-radius: 8px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.45);
  font-family: "Outfit", sans-serif;
  overflow: hidden;
  display: flex;
  flex-direction: column;
`;

const Header = styled.div`
  background: #e8d8b8;
  border-bottom: 3px solid #383838;
  padding: 14px 18px;
`;

const Title = styled.div`
  color: #1a1a1a;
  font-size: 20px;
  font-weight: 800;
`;

const Subtitle = styled.div`
  color: #555;
  font-size: 13px;
  font-weight: 700;
  margin-top: 3px;
`;

const Body = styled.div`
  padding: 14px 18px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
`;

const SectionTitle = styled.div`
  font-size: 12px;
  font-weight: 800;
  color: #777;
  text-transform: uppercase;
  letter-spacing: 1px;
  border-bottom: 2px solid #d8c8a8;
  padding-bottom: 4px;
`;

const MovePanel = styled.div`
  background: white;
  border: 2px solid #d8c8a8;
  border-radius: 6px;
  padding: 12px;
`;

const MoveNameRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
`;

const MoveName = styled.div`
  color: #1a1a1a;
  font-size: 18px;
  font-weight: 800;
  text-transform: capitalize;
`;

const TypeBadge = styled.span<{ $color: string }>`
  flex-shrink: 0;
  background: ${(props) => props.$color};
  color: white;
  border-radius: 3px;
  padding: 3px 10px;
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.5px;
`;

const MoveStats = styled.div`
  display: flex;
  gap: 16px;
  margin-top: 10px;
  color: #333;
  font-size: 12px;
  font-weight: 700;
`;

const MoveEffect = styled.div`
  color: #555;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.35;
  margin-top: 10px;
`;

const PartyList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
`;

const PokemonButton = styled.button<{ $disabled?: boolean }>`
  width: 100%;
  background: ${(props) => (props.$disabled ? "#e3e3e3" : "white")};
  border: 2px solid ${(props) => (props.$disabled ? "#999" : "#4a4ba6")};
  border-radius: 6px;
  padding: 8px 10px;
  display: grid;
  grid-template-columns: 52px 1fr auto;
  gap: 10px;
  align-items: center;
  cursor: ${(props) => (props.$disabled ? "not-allowed" : "pointer")};
  opacity: ${(props) => (props.$disabled ? 0.55 : 1)};
  text-align: left;
  transition: transform 0.1s ease, box-shadow 0.1s ease, background 0.1s ease;

  &:hover {
    background: ${(props) => (props.$disabled ? "#e3e3e3" : "#f0fff0")};
    transform: ${(props) => (props.$disabled ? "none" : "translateY(-1px)")};
    box-shadow: ${(props) =>
      props.$disabled ? "none" : "0 4px 8px rgba(0, 0, 0, 0.14)"};
  }
`;

const Sprite = styled.img`
  width: 48px;
  height: 48px;
  image-rendering: pixelated;
  object-fit: contain;
`;

const PokemonInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
`;

const PokemonName = styled.div`
  color: #1a1a1a;
  font-size: 15px;
  font-weight: 800;
  text-transform: capitalize;
`;

const MoveCount = styled.div`
  color: #4a4ba6;
  font-size: 12px;
  font-weight: 800;
`;

const MoveChips = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
`;

const MoveChip = styled.span`
  background: #eee4cc;
  border: 1px solid #d8c8a8;
  color: #333;
  border-radius: 3px;
  padding: 2px 5px;
  font-size: 10px;
  font-weight: 700;
  text-transform: capitalize;
`;

const EmptyMoves = styled.span`
  color: #777;
  font-size: 11px;
  font-weight: 700;
`;

const UnavailableReason = styled.div`
  color: #666;
  font-size: 10px;
  font-weight: 800;
`;

const Footer = styled.div`
  border-top: 3px solid #383838;
  background: #e8d8b8;
`;

const CloseButton = styled.button`
  width: 100%;
  padding: 10px;
  border: 0;
  background: transparent;
  color: #383838;
  cursor: pointer;
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 800;

  &:hover {
    background: #d8c8a8;
  }
`;

interface TMHMTeachModalProps {
  inventoryItem: CQInventoryItem;
  move: PhaserMoveFull;
  party: PokemonDTO[];
  onChoosePokemon: (partySlot: number) => void;
  onClose: () => void;
}

const TMHMTeachModal: React.FC<TMHMTeachModalProps> = ({
  inventoryItem,
  move,
  party,
  onChoosePokemon,
  onClose,
}) => {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const moveName = move.name;
  const moveType = move.type || "NORMAL";
  const typeColor = TYPE_COLORS[moveType.toUpperCase()] || TYPE_COLORS.NORMAL;
  const moveCountLabel = inventoryItem.item.itemType === ITEM_TYPE_HM ? "HM" : "TM";
  const partyOptions = party
    .map((pokemon, index) => {
      const knownMoves = pokemon.moves || [];
      const compatible = canPokemonLearnTMHM(pokemon.id, move.id);
      const alreadyKnows = knownMoves.some((knownMove) => knownMove.id === move.id);
      const selectable = compatible && !alreadyKnows;
      const unavailableReason = alreadyKnows
        ? "Already knows it"
        : compatible
          ? undefined
          : "Can't learn it";

      return {
        alreadyKnows,
        compatible,
        index,
        knownMoves,
        pokemon,
        selectable,
        unavailableReason,
      };
    })
    .sort((a, b) => {
      if (a.selectable !== b.selectable) return a.selectable ? -1 : 1;
      return a.index - b.index;
    });

  return (
    <Overlay onClick={onClose}>
      <Modal onClick={(e) => e.stopPropagation()}>
        <Header>
          <Title>{inventoryItem.item.name}</Title>
          <Subtitle>{moveCountLabel} move tutor</Subtitle>
        </Header>

        <Body>
          <section>
            <SectionTitle>Move</SectionTitle>
            <MovePanel>
              <MoveNameRow>
                <MoveName>{moveName.toLowerCase()}</MoveName>
                <TypeBadge $color={typeColor}>{moveType}</TypeBadge>
              </MoveNameRow>
              <MoveStats>
                <span>Power {move.power && move.power > 0 ? move.power : "-"}</span>
                <span>Accuracy {move.accuracy && move.accuracy > 0 ? `${move.accuracy}%` : "-"}</span>
                <span>PP {move.pp ?? "-"}</span>
              </MoveStats>
              {move.effect && <MoveEffect>{move.effect}</MoveEffect>}
            </MovePanel>
          </section>

          <section>
            <SectionTitle>Choose Pokémon</SectionTitle>
            <PartyList>
              {partyOptions.map((option) => {
                const {
                  index,
                  knownMoves,
                  pokemon,
                  selectable,
                  unavailableReason,
                } = option;
                return (
                  <PokemonButton
                    key={`${pokemon.id}-${index}`}
                    $disabled={!selectable}
                    disabled={!selectable}
                    onClick={() => onChoosePokemon(index)}
                  >
                    <Sprite
                      src={`https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/${pokemon.id}.png`}
                      alt={pokemon.name}
                    />
                    <PokemonInfo>
                      <PokemonName>
                        {pokemon.name.toLowerCase()} Lv. {pokemon.level}
                      </PokemonName>
                      <MoveChips>
                        {knownMoves.length > 0 ? (
                          knownMoves.map((knownMove) => (
                            <MoveChip key={`${pokemon.id}-${index}-${knownMove.id}`}>
                              {knownMove.name.toLowerCase()}
                            </MoveChip>
                          ))
                        ) : (
                          <EmptyMoves>No moves</EmptyMoves>
                        )}
                        {unavailableReason && (
                          <UnavailableReason>{unavailableReason}</UnavailableReason>
                        )}
                      </MoveChips>
                    </PokemonInfo>
                    <MoveCount>{knownMoves.length}/4 Moves</MoveCount>
                  </PokemonButton>
                );
              })}
            </PartyList>
          </section>
        </Body>

        <Footer>
          <CloseButton onClick={onClose}>Close</CloseButton>
        </Footer>
      </Modal>
    </Overlay>
  );
};

export default TMHMTeachModal;
