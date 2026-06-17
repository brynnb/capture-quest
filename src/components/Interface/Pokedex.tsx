import React, { useState, useEffect, useRef, useCallback } from "react";
import styled from "styled-components";
import usePokedexStore, { PokedexSpecies } from "@/stores/PokedexStore";
import useGameStatusStore from "@stores/GameStatusStore";
import { WorldSocket, OpCodes } from "@/net";
import AudioManager from "@/services/audio/AudioManager";
import { cryPathForPokemon } from "@/services/audio/pokemonMusic";

const TYPE_COLORS: Record<string, string> = {
  NORMAL: "#A8A878", FIRE: "#F08030", WATER: "#6890F0", ELECTRIC: "#F8D030",
  GRASS: "#78C850", ICE: "#98D8D8", FIGHTING: "#C03028", POISON: "#A040A0",
  GROUND: "#E0C068", FLYING: "#A890F0", PSYCHIC: "#F85888", BUG: "#A8B820",
  ROCK: "#B8A038", GHOST: "#705898", DRAGON: "#7038F8",
};

const Overlay = styled.div`
  position: fixed;
  top: 0; left: 0; width: 100%; height: 100%;
  display: flex; align-items: center; justify-content: center;
  z-index: 2000;
`;

const Container = styled.div`
  width: 780px;
  height: 560px;
  display: flex;
  font-family: 'Outfit', monospace, sans-serif;
  filter: drop-shadow(0 20px 50px rgba(0,0,0,0.5));
  border-radius: 12px;
  overflow: hidden;
  border: 4px solid #383838;
  background: #f8f0e0;
`;

const ListPanel = styled.div`
  width: 280px;
  background: #e8d8b8;
  border-right: 3px solid #383838;
  display: flex;
  flex-direction: column;
`;

const ListHeader = styled.div`
  padding: 12px 16px;
  border-bottom: 3px solid #383838;
  font-weight: 800;
  font-size: 16px;
  color: #1a1a1a;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #d8c8a8;
`;

const CountBadge = styled.span`
  font-size: 11px;
  font-weight: 600;
  color: #666;
`;

const ListScroll = styled.div`
  flex: 1;
  overflow-y: auto;
  &::-webkit-scrollbar { width: 6px; }
  &::-webkit-scrollbar-thumb { background: #b8a888; border-radius: 3px; }
`;

const ListEntry = styled.div<{ $selected: boolean; $seen: boolean }>`
  padding: 6px 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: ${p => p.$seen ? 'pointer' : 'default'};
  background: ${p => p.$selected ? '#c8b898' : 'transparent'};
  border-left: 3px solid ${p => p.$selected ? '#e53935' : 'transparent'};
  opacity: ${p => p.$seen ? 1 : 0.4};
  transition: background 0.1s;
  &:hover {
    background: ${p => p.$seen ? '#d8c8a8' : 'transparent'};
  }
`;

const EntryNumber = styled.span`
  font-size: 11px;
  font-weight: 600;
  color: #888;
  min-width: 36px;
`;

const EntryName = styled.span<{ $seen: boolean }>`
  font-size: 13px;
  font-weight: 700;
  color: #1a1a1a;
  text-transform: capitalize;
  flex: 1;
`;

const StatusIcon = styled.span<{ $caught: boolean }>`
  font-size: 12px;
  color: ${p => p.$caught ? '#e53935' : '#aaa'};
  min-width: 18px;
  text-align: center;
`;

const DetailPanel = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 20px;
  gap: 16px;
`;

const SpriteArea = styled.div`
  display: flex;
  align-items: center;
  gap: 20px;
`;

const SpriteBox = styled.div`
  width: 120px;
  height: 120px;
  background: white;
  border: 3px solid #383838;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  img {
    width: 96px;
    height: 96px;
    image-rendering: pixelated;
    object-fit: contain;
  }
`;

const InfoArea = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
`;

const PokeName = styled.div`
  font-size: 24px;
  font-weight: 800;
  color: #1a1a1a;
  text-transform: capitalize;
`;

const PokeNumber = styled.span`
  font-size: 14px;
  font-weight: 600;
  color: #888;
  margin-left: 8px;
`;

const TypeRow = styled.div`
  display: flex;
  gap: 6px;
`;

const TypeBadge = styled.span<{ $color: string }>`
  font-size: 11px;
  font-weight: 700;
  color: white;
  background: ${p => p.$color};
  padding: 2px 10px;
  border-radius: 3px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
`;

const CategoryText = styled.div`
  font-size: 12px;
  font-weight: 600;
  color: #666;
  font-style: italic;
`;

const StatsRow = styled.div`
  display: flex;
  gap: 24px;
  font-size: 13px;
  color: #555;
  font-weight: 600;
`;

const CryButton = styled.button`
  align-self: flex-start;
  background: #e8d8b8;
  border: 2px solid #383838;
  border-radius: 4px;
  color: #1a1a1a;
  cursor: pointer;
  font-family: 'Outfit', monospace, sans-serif;
  font-size: 12px;
  font-weight: 800;
  padding: 5px 12px;

  &:hover {
    background: #d8c8a8;
  }
`;

const DescriptionBox = styled.div`
  background: white;
  border: 2px solid #383838;
  border-radius: 6px;
  padding: 14px;
  font-size: 13px;
  line-height: 1.6;
  color: #333;
  flex: 1;
  overflow-y: auto;
  &::-webkit-scrollbar { width: 4px; }
  &::-webkit-scrollbar-thumb { background: #ccc; border-radius: 2px; }
`;

const EmptyDetail = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #aaa;
  font-size: 14px;
  font-weight: 600;
`;

const Pokedex: React.FC = () => {
  const { species, isLoaded, statusMap } = usePokedexStore();
  const { togglePokedex } = useGameStatusStore();
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isLoaded) {
      WorldSocket.sendJsonMessage(OpCodes.PokedexListRequest, {});
    } else {
      WorldSocket.sendJsonMessage(OpCodes.PokedexStatusRequest, {});
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleClose = useCallback(() => {
    togglePokedex();
  }, [togglePokedex]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [handleClose]);

  const allEntries: { id: number; species?: PokedexSpecies; seen: boolean; caught: boolean }[] = [];
  const speciesMap = new Map(species.map(s => [s.id, s]));
  for (let i = 1; i <= 151; i++) {
    const s = speciesMap.get(i);
    const status = statusMap.get(i);
    allEntries.push({
      id: i,
      species: s,
      seen: status?.seen ?? false,
      caught: status?.caught ?? false,
    });
  }

  const selected = selectedId != null ? speciesMap.get(selectedId) : null;
  const selectedStatus = selectedId != null ? statusMap.get(selectedId) : null;

  const seenCount = allEntries.filter(e => e.seen).length;
  const caughtCount = allEntries.filter(e => e.caught).length;

  const getTypeColor = (type: string) =>
    TYPE_COLORS[type.toUpperCase()] || "#A8A878";

  const formatWeight = (w: number) => {
    const lbs = w / 10;
    return `${lbs.toFixed(1)} lbs`;
  };

  const playCry = useCallback((entry: PokedexSpecies) => {
    const path = cryPathForPokemon(entry.name, entry.crySfx);
    if (path) {
      void AudioManager.playSFX(path, 0.8);
    }
  }, []);

  return (
    <Overlay onClick={handleClose}>
      <Container onClick={e => e.stopPropagation()}>
        <ListPanel>
          <ListHeader>
            POK\u00c9DEX
            <CountBadge>Seen {seenCount} / Caught {caughtCount}</CountBadge>
          </ListHeader>
          <ListScroll ref={listRef}>
            {allEntries.map(entry => (
              <ListEntry
                key={entry.id}
                $selected={entry.id === selectedId}
                $seen={entry.seen}
                onClick={() => {
                  if (entry.seen) setSelectedId(entry.id);
                }}
              >
                <StatusIcon $caught={entry.caught}>
                  {entry.caught ? "\u25cf" : entry.seen ? "\u25cb" : "\u2014"}
                </StatusIcon>
                <EntryNumber>#{entry.id.toString().padStart(3, "0")}</EntryNumber>
                <EntryName $seen={entry.seen}>
                  {entry.seen && entry.species ? entry.species.name.toLowerCase() : "???"}
                </EntryName>
              </ListEntry>
            ))}
          </ListScroll>
        </ListPanel>

        <DetailPanel>
          {selected && selectedStatus?.seen ? (
            <>
              <SpriteArea>
                <SpriteBox>
                  <img
                    src={`https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/${selected.id}.png`}
                    alt={selected.name}
                  />
                </SpriteBox>
                <InfoArea>
                  <PokeName>
                    {selected.name.toLowerCase()}
                    <PokeNumber>#{selected.id.toString().padStart(3, "0")}</PokeNumber>
                  </PokeName>
                  <TypeRow>
                    <TypeBadge $color={getTypeColor(selected.type1)}>
                      {selected.type1}
                    </TypeBadge>
                    {selected.type2 && selected.type2 !== selected.type1 && (
                      <TypeBadge $color={getTypeColor(selected.type2)}>
                        {selected.type2}
                      </TypeBadge>
                    )}
                  </TypeRow>
                  {selected.pokedexType && (
                    <CategoryText>{selected.pokedexType} Pok\u00e9mon</CategoryText>
                  )}
                  <StatsRow>
                    {selected.height && <span>HT: {selected.height}</span>}
                    {selected.weight != null && <span>WT: {formatWeight(selected.weight)}</span>}
                  </StatsRow>
                  <StatsRow>
                    <span>{selectedStatus.caught ? "\u25cf Caught" : "\u25cb Seen only"}</span>
                  </StatsRow>
                  {selected.crySfx && (
                    <CryButton onClick={() => playCry(selected)}>Cry</CryButton>
                  )}
                </InfoArea>
              </SpriteArea>
              <DescriptionBox>
                {selected.pokedexText || "No data available."}
              </DescriptionBox>
            </>
          ) : (
            <EmptyDetail>
              {selectedId ? "No data available." : "Select a Pok\u00e9mon to view details."}
            </EmptyDetail>
          )}
        </DetailPanel>
      </Container>
    </Overlay>
  );
};

export default Pokedex;
