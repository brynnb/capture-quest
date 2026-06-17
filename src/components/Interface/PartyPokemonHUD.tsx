import React, {
  useState,
  useRef,
  useCallback,
  useEffect,
  useMemo,
} from "react";
import { createPortal } from "react-dom";
import styled from "styled-components";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import useStaticDataStore from "@/stores/StaticDataStore";
import type { MoveDTO, PokemonDTO } from "@/net/generated/world_api";
import type { HomeTownData } from "@/services/characterService";
import PokemonDetailModal from "./PokemonDetailModal";
import { WorldSocket, OpCodes } from "@/net";

const PartyContainer = styled.div`
  position: absolute;
  bottom: 345px;
  right: 40px;
  width: 242px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  z-index: 1000;
`;

const PokemonEntry = styled.div<{ $isPlaceholder?: boolean }>`
  width: 100%;
  height: 64px;
  background: ${p => p.$isPlaceholder ? 'rgba(192, 193, 255, 0.25)' : 'rgba(192, 193, 255, 0.57)'};
  backdrop-filter: blur(12px);
  border: 3px ${p => p.$isPlaceholder ? 'dashed #4a4ba680' : 'solid #4a4ba6'};
  border-radius: 16px;
  display: flex;
  padding: 6px 10px;
  box-sizing: border-box;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
  transition: transform 0.15s ease, opacity 0.15s ease, background 0.15s ease, border-color 0.15s ease;
  position: relative;
  overflow: hidden;
  cursor: pointer;
  opacity: ${p => p.$isPlaceholder ? 0.4 : 1};

  &:hover {
    background: ${p => p.$isPlaceholder ? undefined : 'rgba(192, 193, 255, 0.7)'};
    transform: ${p => p.$isPlaceholder ? 'none' : 'translateX(-4px)'};
  }
`;

const FloatingClone = styled.div<{ $uiScale: number }>`
  position: fixed;
  width: 242px;
  height: 64px;
  background: rgba(192, 193, 255, 0.85);
  backdrop-filter: blur(12px);
  border: 3px solid #2a2ba6;
  border-radius: 16px;
  display: flex;
  padding: 6px 10px;
  box-sizing: border-box;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25);
  z-index: 9999;
  pointer-events: none;
  overflow: hidden;
  transform: scale(${p => p.$uiScale * 1.03});
  transform-origin: top left;
`;

const EmptySlot = styled.div`
  width: 100%;
  height: 64px;
  background: rgba(192, 193, 255, 0.2);
  backdrop-filter: blur(12px);
  border: 3px dashed #4a4ba680;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
`;

const GrabHandle = styled.div`
  width: 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  cursor: grab;
  opacity: 0;
  transition: opacity 0.15s ease;
  flex-shrink: 0;
  margin-left: auto;
  padding: 0 2px;

  &:active {
    cursor: grabbing;
  }
`;

const GrabDot = styled.div`
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #4a4ba6;
`;

const DraggableEntry = styled.div`
  position: relative;

  &:hover ${GrabHandle} {
    opacity: 1;
  }
`;

const EmptyParty = styled.div`
  width: 100%;
  height: 64px;
  background: rgba(192, 193, 255, 0.35);
  backdrop-filter: blur(12px);
  border: 3px dashed #4a4ba6;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  font-family: 'Outfit', sans-serif;
  font-weight: 600;
  font-size: 14px;
  color: #4a4ba6;
  opacity: 0.7;
`;

const ImageWrapper = styled.div`
  width: 52px;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    image-rendering: pixelated;
  }
`;

const InfoColumn = styled.div`
  flex: 1;
  margin-left: 10px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0px;
  position: relative;
`;

const PokemonName = styled.div`
  font-family: 'Outfit', sans-serif;
  font-weight: 800;
  font-size: 15px;
  color: #1a1b41;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.1;
  text-transform: capitalize;
`;

const PokemonLevel = styled.div`
  font-family: 'Outfit', sans-serif;
  font-weight: 600;
  font-size: 13px;
  color: #4a4ba6;
  margin-top: 1px;
`;

const HPBarContainer = styled.div`
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 5px;
  background: rgba(0, 0, 0, 0.3);
  overflow: hidden;
`;

const HPBarFill = styled.div<{ $percent: number }>`
  width: ${props => props.$percent}%;
  height: 100%;
  background: ${props =>
    props.$percent > 50
      ? 'linear-gradient(90deg, #6ddb6d, #4caf50)'
      : props.$percent > 20
        ? 'linear-gradient(90deg, #ffde75, #ffa726)'
        : 'linear-gradient(90deg, #ff6b6b, #e53935)'};
  transition: width 0.3s ease;
`;

const StatusBadge = styled.span<{ $status: string }>`
  font-family: 'Outfit', sans-serif;
  font-weight: 700;
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
  margin-left: 6px;
  color: white;
  background: ${props => {
    switch (props.$status) {
      case 'BRN': return '#e57373';
      case 'FRZ': return '#4fc3f7';
      case 'PAR': return '#fff176';
      case 'PSN': return '#ba68c8';
      case 'TOX': return '#9c27b0';
      case 'SLP': return '#90a4ae';
      default: return 'transparent';
    }
  }};
  ${props => props.$status === 'PAR' && 'color: #333;'}
`;

const FlyOverlay = styled.div`
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5600;
  background: rgba(10, 12, 28, 0.22);
`;

const FlyModal = styled.div`
  width: min(420px, calc(100vw - 32px));
  max-height: min(620px, calc(100vh - 32px));
  background: #f8f0e0;
  border: 4px solid #383838;
  border-radius: 8px;
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.35);
  font-family: 'Outfit', sans-serif;
  overflow: hidden;
`;

const FlyHeader = styled.div`
  background: #e8d8b8;
  border-bottom: 3px solid #383838;
  padding: 12px 16px;
`;

const FlyTitle = styled.div`
  color: #1a1a1a;
  font-size: 18px;
  font-weight: 800;
`;

const FlySubtitle = styled.div`
  color: #555;
  font-size: 12px;
  font-weight: 700;
  margin-top: 3px;
`;

const FlyList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 440px;
  overflow-y: auto;
  padding: 12px;
`;

const FlyDestinationButton = styled.button`
  width: 100%;
  min-height: 44px;
  border: 2px solid #383838;
  border-radius: 6px;
  background: white;
  color: #1a1a1a;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  padding: 7px 10px;
  font-family: 'Outfit', sans-serif;
  text-align: left;

  &:hover {
    background: #f0fff0;
  }
`;

const FlyDestinationName = styled.span`
  font-size: 14px;
  font-weight: 800;
`;

const FlyDestinationMeta = styled.span`
  color: #666;
  font-size: 11px;
  font-weight: 700;
`;

const FlyEmptyState = styled.div`
  color: #555;
  font-size: 13px;
  font-weight: 700;
  padding: 20px 12px;
  text-align: center;
`;

const FlyCancelButton = styled.button`
  width: 100%;
  padding: 10px;
  background: #e8d8b8;
  border: none;
  border-top: 3px solid #383838;
  color: #383838;
  cursor: pointer;
  font-family: 'Outfit', sans-serif;
  font-size: 14px;
  font-weight: 800;

  &:hover {
    background: #d8c8a8;
  }
`;

const MAX_PARTY_SIZE = 6;
const PARTY_FIELD_MOVES = new Set(["STRENGTH", "FLASH", "DIG", "TELEPORT", "FLY"]);

function normalizeMoveName(name: string): string {
  return name.trim().toUpperCase().replace(/[^A-Z0-9]/g, "");
}

function isPartyFieldMove(move: MoveDTO): boolean {
  return PARTY_FIELD_MOVES.has(normalizeMoveName(move.name));
}

export interface PendingItemUse {
  instanceId: number;
  partySlot: number;
  itemName: string;
}

export interface CursorItemUse {
  instanceId: number;
  itemName: string;
}

interface PartyPokemonHUDProps {
  containerStyle?: React.CSSProperties;
  cursorItemUse?: CursorItemUse | null;
  onCursorItemUse?: (partySlot: number) => void;
  pendingItemUse?: PendingItemUse | null;
  pendingBannerText?: string;
  onMoveSlotSelected?: (moveSlot: number) => void;
  onCancelPendingItem?: () => void;
}

const PartyPokemonHUD: React.FC<PartyPokemonHUDProps> = ({
  containerStyle,
  cursorItemUse,
  onCursorItemUse,
  pendingItemUse,
  pendingBannerText,
  onMoveSlotSelected,
  onCancelPendingItem,
}) => {
  const { party } = usePokemonPartyStore();
  const uiScale = useGameStatusStore((s) => s.uiScale);
  const homeTowns = useStaticDataStore((s) => s.homeTowns);
  const isLoadingCharCreate = useStaticDataStore((s) => s.isLoadingCharCreate);
  const loadCharCreateData = useStaticDataStore((s) => s.loadCharCreateData);
  const [selectedPokemon, setSelectedPokemon] = useState<PokemonDTO | null>(null);
  const [pendingFlyMoveName, setPendingFlyMoveName] = useState<string | null>(null);
  const [, forceRender] = useState(0);

  // All drag state lives in refs to avoid setState-during-render issues.
  // We call forceRender() explicitly when we need to re-paint.
  const dragState = useRef<{
    active: boolean;
    origIdx: number;
    order: number[];
    mouseX: number;
    mouseY: number;
    offsetX: number;
    offsetY: number;
  }>({ active: false, origIdx: -1, order: [], mouseX: 0, mouseY: 0, offsetX: 0, offsetY: 0 });

  const slotRefs = useRef<(HTMLDivElement | null)[]>([]);
  const suppressClick = useRef(false);

  const ds = dragState.current;
  const isDragging = ds.active;
  const displayOrder = isDragging ? ds.order : party.map((_, i) => i);
  const draggedOrigIdx = isDragging ? ds.origIdx : -1;

  const flyDestinations = useMemo(
    () =>
      [...homeTowns]
        .filter(
          (town) =>
            Number.isFinite(town.mapId) &&
            Number.isFinite(town.spawnX) &&
            Number.isFinite(town.spawnY),
        )
        .sort((a, b) => a.sortOrder - b.sortOrder || a.name.localeCompare(b.name)),
    [homeTowns],
  );

  useEffect(() => {
    if (!isDragging) return;

    const onMouseMove = (e: MouseEvent) => {
      ds.mouseX = e.clientX;
      ds.mouseY = e.clientY;

      // Find which slot the cursor is over
      let changed = false;
      for (let i = 0; i < slotRefs.current.length; i++) {
        const el = slotRefs.current[i];
        if (!el) continue;
        const rect = el.getBoundingClientRect();
        if (e.clientY >= rect.top && e.clientY <= rect.bottom) {
          const currentPos = ds.order.indexOf(ds.origIdx);
          if (currentPos !== i) {
            const next = [...ds.order];
            next.splice(currentPos, 1);
            next.splice(i, 0, ds.origIdx);
            ds.order = next;
            changed = true;
          }
          break;
        }
      }

      forceRender(c => c + 1);
      if (changed) {
        // Extra render to let refs settle after reorder
        requestAnimationFrame(() => forceRender(c => c + 1));
      }
    };

    const onMouseUp = () => {
      const finalOrder = ds.order;
      const identity = party.map((_, i) => i);
      const changed = finalOrder.some((v, i) => v !== identity[i]);

      ds.active = false;
      ds.origIdx = -1;
      ds.order = [];

      if (changed) {
        const newParty = finalOrder.map(i => party[i]);
        usePokemonPartyStore.getState().setParty(newParty);
        WorldSocket.sendJsonMessage(OpCodes.PokemonPartyReorderRequest, {
          order: finalOrder,
        });
      }

      suppressClick.current = true;
      setTimeout(() => { suppressClick.current = false; }, 50);
      forceRender(c => c + 1);
    };

    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, [isDragging, party, ds]);

  useEffect(() => {
    if (!pendingFlyMoveName || homeTowns.length > 0 || isLoadingCharCreate) return;
    loadCharCreateData();
  }, [homeTowns.length, isLoadingCharCreate, loadCharCreateData, pendingFlyMoveName]);

  const handleGrabMouseDown = useCallback((e: React.MouseEvent, origIdx: number, displayIdx: number) => {
    e.preventDefault();
    e.stopPropagation();
    const entryEl = slotRefs.current[displayIdx];
    const ds = dragState.current;
    ds.active = true;
    ds.origIdx = origIdx;
    ds.order = party.map((_, i) => i);
    ds.mouseX = e.clientX;
    ds.mouseY = e.clientY;
    ds.offsetX = entryEl ? e.clientX - entryEl.getBoundingClientRect().left : 0;
    ds.offsetY = entryEl ? e.clientY - entryEl.getBoundingClientRect().top : 0;
    forceRender(c => c + 1);
  }, [party]);

  const handleEntryClick = useCallback((pokemon: PokemonDTO, partySlot: number) => {
    if (suppressClick.current) return;
    if (cursorItemUse && onCursorItemUse) {
      onCursorItemUse(partySlot);
      return;
    }
    setSelectedPokemon(pokemon);
  }, [cursorItemUse, onCursorItemUse]);

  const handleFieldMoveClick = useCallback((move: MoveDTO) => {
    if (normalizeMoveName(move.name) === "FLY") {
      setSelectedPokemon(null);
      setPendingFlyMoveName(move.name);
      return;
    }

    WorldSocket.sendJsonMessage(OpCodes.FieldMoveUseRequest, {
      moveName: move.name,
    });
    setSelectedPokemon(null);
  }, []);

  const handleFlyDestinationClick = useCallback((destination: HomeTownData) => {
    if (!pendingFlyMoveName) return;
    WorldSocket.sendJsonMessage(OpCodes.FieldMoveUseRequest, {
      moveName: pendingFlyMoveName,
      mapId: destination.mapId,
      targetX: destination.spawnX,
      targetY: destination.spawnY,
    });
    setPendingFlyMoveName(null);
  }, [pendingFlyMoveName]);

  if (party.length === 0) {
    return (
      <PartyContainer id="party-pokemon-hud" style={containerStyle}>
        <EmptyParty>No Pokemon in Party</EmptyParty>
      </PartyContainer>
    );
  }

  const emptySlotCount = isDragging ? MAX_PARTY_SIZE - party.length : 0;
  const draggedPokemon = draggedOrigIdx >= 0 ? party[draggedOrigIdx] : null;

  return (
    <>
      <PartyContainer id="party-pokemon-hud" style={containerStyle}>
        {displayOrder.map((origIdx, displayIdx) => {
          const pokemon = party[origIdx];
          const hpPercent = pokemon.maxHp > 0
            ? Math.round((pokemon.curHp / pokemon.maxHp) * 100)
            : 0;
          const isBeingDragged = origIdx === draggedOrigIdx;
          return (
            <DraggableEntry
              key={`poke-${origIdx}`}
              ref={el => { slotRefs.current[displayIdx] = el; }}
            >
              <PokemonEntry
                data-cq-party-entry="true"
                $isPlaceholder={isBeingDragged}
                onClick={() => handleEntryClick(pokemon, origIdx)}
              >
                {!isBeingDragged && (
                  <>
                    <ImageWrapper>
                      <img
                        src={`https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/${pokemon.id}.png`}
                        alt={pokemon.name}
                      />
                    </ImageWrapper>
                    <InfoColumn>
                      <PokemonName>
                        {pokemon.name.toLowerCase()}
                        {pokemon.status && pokemon.status !== "" && (
                          <StatusBadge $status={pokemon.status}>{pokemon.status}</StatusBadge>
                        )}
                      </PokemonName>
                      <PokemonLevel>
                        Lv. {pokemon.level}
                        <span style={{ marginLeft: 8, fontSize: 11, color: '#666' }}>
                          {pokemon.curHp}/{pokemon.maxHp}
                        </span>
                      </PokemonLevel>
                    </InfoColumn>
                    <GrabHandle onMouseDown={(e) => handleGrabMouseDown(e, origIdx, displayIdx)}>
                      <GrabDot /><GrabDot /><GrabDot />
                      <GrabDot /><GrabDot /><GrabDot />
                    </GrabHandle>
                  </>
                )}
                <HPBarContainer>
                  <HPBarFill $percent={isBeingDragged ? 0 : hpPercent} />
                </HPBarContainer>
              </PokemonEntry>
            </DraggableEntry>
          );
        })}
        {Array.from({ length: emptySlotCount }).map((_, i) => (
          <EmptySlot key={`empty-${i}`} />
        ))}
      </PartyContainer>

      {/* Floating clone that follows cursor */}
      {isDragging && draggedPokemon && typeof document !== "undefined" &&
        createPortal(
          <FloatingClone
            $uiScale={uiScale}
            style={{
              left: ds.mouseX - ds.offsetX,
              top: ds.mouseY - ds.offsetY,
            }}
          >
            <ImageWrapper>
              <img
                src={`https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/${draggedPokemon.id}.png`}
                alt={draggedPokemon.name}
              />
            </ImageWrapper>
            <InfoColumn>
              <PokemonName>
                {draggedPokemon.name.toLowerCase()}
              </PokemonName>
              <PokemonLevel>
                Lv. {draggedPokemon.level}
                <span style={{ marginLeft: 8, fontSize: 11, color: '#666' }}>
                  {draggedPokemon.curHp}/{draggedPokemon.maxHp}
                </span>
              </PokemonLevel>
            </InfoColumn>
            <HPBarContainer>
              <HPBarFill $percent={draggedPokemon.maxHp > 0 ? Math.round((draggedPokemon.curHp / draggedPokemon.maxHp) * 100) : 0} />
            </HPBarContainer>
          </FloatingClone>,
          document.body,
        )}

      {(selectedPokemon || pendingItemUse) && (() => {
        const modalPokemon = pendingItemUse
          ? party[pendingItemUse.partySlot]
          : selectedPokemon;
        if (!modalPokemon) return null;
        return (
          <PokemonDetailModal
            pokemon={modalPokemon}
            onClose={() => {
              setSelectedPokemon(null);
              if (pendingItemUse && onCancelPendingItem) onCancelPendingItem();
            }}
            onMoveClick={pendingItemUse && onMoveSlotSelected ? onMoveSlotSelected : undefined}
            onFieldMoveClick={!pendingItemUse ? handleFieldMoveClick : undefined}
            isFieldMove={isPartyFieldMove}
            itemBannerText={pendingItemUse ? (pendingBannerText || `Use ${pendingItemUse.itemName} \u2014 select a move`) : undefined}
          />
        );
      })()}

      {pendingFlyMoveName && typeof document !== "undefined" &&
        createPortal(
          <FlyOverlay onClick={() => setPendingFlyMoveName(null)}>
            <FlyModal onClick={(e) => e.stopPropagation()}>
              <FlyHeader>
                <FlyTitle>Fly</FlyTitle>
                <FlySubtitle>Choose a Kanto destination.</FlySubtitle>
              </FlyHeader>
              {flyDestinations.length > 0 ? (
                <FlyList>
                  {flyDestinations.map((destination) => (
                    <FlyDestinationButton
                      key={`${destination.mapId}:${destination.spawnX}:${destination.spawnY}`}
                      onClick={() => handleFlyDestinationClick(destination)}
                    >
                      <FlyDestinationName>{destination.name}</FlyDestinationName>
                      {destination.description && (
                        <FlyDestinationMeta>{destination.description}</FlyDestinationMeta>
                      )}
                    </FlyDestinationButton>
                  ))}
                </FlyList>
              ) : (
                <FlyEmptyState>
                  {isLoadingCharCreate ? "Loading destinations..." : "No Fly destinations loaded."}
                </FlyEmptyState>
              )}
              <FlyCancelButton onClick={() => setPendingFlyMoveName(null)}>
                Cancel
              </FlyCancelButton>
            </FlyModal>
          </FlyOverlay>,
          document.body,
        )}
    </>
  );
};

export default PartyPokemonHUD;
