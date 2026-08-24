import React, { useState, useMemo, useRef, useCallback, useEffect } from "react";
import styled from "styled-components";
import useTileEditorStore, { StampDefinition } from "@/stores/TileEditorStore";
import { getTileImageUrl } from "@/phaser-game/api/tileService";

const PaletteContainer = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
`;

const SearchBar = styled.input`
  width: 100%;
  padding: 6px 10px;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.7);
  border: 2px solid #4a4ba6;
  border-radius: 10px;
  font-family: "Outfit", sans-serif;
  font-weight: 500;
  font-size: 13px;
  color: #281e16;
  outline: none;
  flex-shrink: 0;
  margin-bottom: 8px;

  &:focus {
    border-color: #6a6bf6;
    background: rgba(255, 255, 255, 0.9);
  }

  &::placeholder {
    color: #999;
    font-weight: 400;
  }
`;

const TileGrid = styled.div`
  flex: 1;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(40px, 1fr));
  grid-auto-rows: min-content;
  align-content: start;
  gap: 3px;
  overflow-y: auto;
  padding: 2px;

  &::-webkit-scrollbar {
    width: 6px;
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: #4a4ba6;
    border-radius: 3px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.05);
    border-radius: 3px;
  }
`;

const TileItem = styled.div<{ $isSelected: boolean }>`
  width: 100%;
  aspect-ratio: 1;
  border: 2px solid ${({ $isSelected }) => ($isSelected ? "#6a6bf6" : "rgba(74, 75, 166, 0.3)")};
  border-radius: 6px;
  cursor: pointer;
  overflow: hidden;
  background: #000;
  position: relative;
  transition: border-color 0.1s ease, transform 0.1s ease;
  box-shadow: ${({ $isSelected }) => ($isSelected ? "0 0 6px rgba(106, 107, 246, 0.6)" : "none")};

  &:hover {
    border-color: #6a6bf6;
    transform: scale(1.08);
    z-index: 1;
  }

  img {
    width: 100%;
    height: 100%;
    image-rendering: pixelated;
    display: block;
  }
`;

const TileTooltip = styled.div`
  position: absolute;
  bottom: 100%;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0, 0, 0, 0.85);
  color: white;
  padding: 3px 8px;
  border-radius: 6px;
  font-family: "Outfit", sans-serif;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  pointer-events: none;
  z-index: 10;
  margin-bottom: 4px;
`;

const SelectedInfo = styled.div`
  flex-shrink: 0;
  padding: 6px 8px;
  background: rgba(255, 255, 255, 0.5);
  border-radius: 8px;
  margin-top: 6px;
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 600;
  color: #4a4ba6;
  display: flex;
  align-items: center;
  gap: 8px;
`;

const SelectedPreview = styled.img`
  width: 32px;
  height: 32px;
  image-rendering: pixelated;
  border: 2px solid #4a4ba6;
  border-radius: 4px;
  background: #000;
`;

const StampCard = styled.div<{ $isSelected: boolean }>`
  display: flex;
  gap: 8px;
  padding: 6px;
  border: 2px solid ${({ $isSelected }) => ($isSelected ? "#6a6bf6" : "rgba(74, 75, 166, 0.3)")};
  border-radius: 8px;
  cursor: pointer;
  background: ${({ $isSelected }) => ($isSelected ? "rgba(106, 107, 246, 0.1)" : "rgba(255, 255, 255, 0.3)")};
  transition: all 0.1s ease;

  &:hover {
    border-color: #6a6bf6;
    background: rgba(106, 107, 246, 0.05);
  }
`;

const StampPreviewGrid = styled.div`
  display: grid;
  gap: 0;
  flex-shrink: 0;
  border: 1px solid rgba(74, 75, 166, 0.3);
  border-radius: 3px;
  overflow: hidden;
  background: #000;

  img {
    width: 16px;
    height: 16px;
    image-rendering: pixelated;
    display: block;
  }
`;

const StampEmptyCell = styled.div`
  background:
    linear-gradient(45deg, rgba(255, 255, 255, 0.15) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(255, 255, 255, 0.15) 25%, transparent 25%);
  background-size: 4px 4px;
`;

const StampCardInfo = styled.div`
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
`;

const StampCardName = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 700;
  color: #4a4ba6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const StampNameInput = styled.input`
  width: 100%;
  min-width: 0;
  padding: 3px 5px;
  border: 1px solid rgba(74, 75, 166, 0.45);
  border-radius: 5px;
  color: #4a4ba6;
  background: rgba(255, 255, 255, 0.7);
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 700;
  box-sizing: border-box;
`;

const StampCardMeta = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 10px;
  font-weight: 500;
  color: #999;
`;

const StampList = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow-y: auto;
  padding: 2px;

  &::-webkit-scrollbar {
    width: 6px;
    background: transparent;
  }
  &::-webkit-scrollbar-thumb {
    background: #4a4ba6;
    border-radius: 3px;
  }
`;

const StampActions = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 2px 2px 8px;
`;

const CaptureStampButton = styled.button`
  min-height: 38px;
  padding: 6px 10px;
  border: 2px solid #4a4ba6;
  border-radius: 9px;
  color: #fff;
  background: #5d5fbd;
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    background: #6a6bf6;
  }
`;

const StampHint = styled.div`
  flex: 1;
  color: #4a4ba6;
  font-family: "Outfit", sans-serif;
  font-size: 11px;
  font-weight: 700;
`;

const DeleteStampButton = styled.button`
  align-self: center;
  min-width: 30px;
  min-height: 30px;
  border: 0;
  border-radius: 7px;
  color: #8e2746;
  background: rgba(255, 255, 255, 0.65);
  font-weight: 900;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    color: #fff;
    background: #d94f70;
  }
`;

const EmptyStamps = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 600;
  text-align: center;
  padding: 20px;
`;

const TOTAL_TILE_IMAGES = 826;

const TilePalette: React.FC = () => {
  const {
    selectedTool,
    selectedTileImageId,
    setSelectedTileImageId,
    tileProperties,
    tilePropertiesLoaded,
    selectedStamp,
    setSelectedStamp,
    availableStamps,
    setAvailableStamps,
    deleteCapturedStamp,
    renameCapturedStamp,
    setSelectedTool,
  } = useTileEditorStore();

  const [searchQuery, setSearchQuery] = useState("");
  const [hoveredTile, setHoveredTile] = useState<number | null>(null);
  const [paletteTab, setPaletteTab] = useState<"tiles" | "stamps">("tiles");
  const [serverStampsLoaded, setServerStampsLoaded] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const selectedTileRef = useRef<HTMLDivElement>(null);

  // Auto-switch tabs based on selected tool
  useEffect(() => {
    if (selectedTool === "stamp") {
      setPaletteTab("stamps");
    } else {
      setPaletteTab("tiles");
    }
  }, [selectedTool]);

  // Load server-backed stamps once; setAvailableStamps preserves locally
  // captured stamps while merging these shared definitions.
  useEffect(() => {
    if (paletteTab === "stamps" && !serverStampsLoaded) {
      setServerStampsLoaded(true);
      fetch("/api/tiles/stamps")
        .then((r) => r.json())
        .then((data) => {
          if (data.success && Array.isArray(data.stamps)) {
            setAvailableStamps(
              data.stamps.map((s: { id: number; name: string; widthTiles: number; heightTiles: number; tileImageIds: number[][] }) => ({
                id: s.id,
                name: s.name,
                widthTiles: s.widthTiles,
                heightTiles: s.heightTiles,
                tileImageIds: s.tileImageIds,
              })),
            );
          }
        })
        .catch(() => {});
    }
  }, [paletteTab, serverStampsLoaded, setAvailableStamps]);

  useEffect(() => {
    if (paletteTab !== "tiles" || !selectedTileImageId) return;
    selectedTileRef.current?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [paletteTab, selectedTileImageId]);

  // Build the list of all tile image IDs (1-826)
  // If tile properties are loaded, use them; otherwise generate a basic list
  const allTiles = useMemo(() => {
    if (tilePropertiesLoaded && tileProperties.length > 0) {
      return tileProperties;
    }
    // Fallback: generate basic entries for all 826 tiles
    return Array.from({ length: TOTAL_TILE_IMAGES }, (_, i) => ({
      tileImageId: i + 1,
      name: `Tile ${i + 1}`,
      collisionType: 0,
      isUserEditable: 1,
    }));
  }, [tileProperties, tilePropertiesLoaded]);

  const filteredTiles = useMemo(() => {
    if (!searchQuery.trim()) return allTiles;
    const q = searchQuery.toLowerCase();
    return allTiles.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        String(t.tileImageId).includes(q),
    );
  }, [allTiles, searchQuery]);

  const handleSearchKeyDown = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  const selectedProp = allTiles.find((t) => t.tileImageId === selectedTileImageId);

  const handleStampSelect = useCallback((stamp: StampDefinition) => {
    setSelectedStamp(stamp);
  }, [setSelectedStamp]);

  const handleCaptureNewStamp = useCallback(() => {
    setSelectedStamp(null);
    setSelectedTool("stamp");
  }, [setSelectedStamp, setSelectedTool]);

  return (
    <PaletteContainer>
      {paletteTab === "tiles" ? (
        <>
          <SearchBar
            ref={searchRef}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={handleSearchKeyDown}
            placeholder="Search tiles..."
            autoComplete="off"
          />

          <TileGrid>
            {filteredTiles.map((tile) => (
              <TileItem
                key={tile.tileImageId}
                ref={selectedTileImageId === tile.tileImageId ? selectedTileRef : undefined}
                $isSelected={selectedTileImageId === tile.tileImageId}
                onClick={() => setSelectedTileImageId(tile.tileImageId)}
                onMouseEnter={() => setHoveredTile(tile.tileImageId)}
                onMouseLeave={() => setHoveredTile(null)}
              >
                <img
                  src={getTileImageUrl(tile.tileImageId)}
                  alt={tile.name}
                  loading="lazy"
                />
                {hoveredTile === tile.tileImageId && (
                  <TileTooltip>
                    {tile.name} (#{tile.tileImageId})
                    {tile.collisionType !== 0 ? " [blocked]" : ""}
                  </TileTooltip>
                )}
              </TileItem>
            ))}
          </TileGrid>

          {selectedTileImageId && selectedProp && (
            <SelectedInfo>
              <SelectedPreview
                src={getTileImageUrl(selectedTileImageId)}
                alt={selectedProp.name}
              />
              <span>
                {selectedProp.name} (#{selectedTileImageId})
                {selectedProp.collisionType !== 0 ? " — Blocked" : " — Walkable"}
              </span>
            </SelectedInfo>
          )}
        </>
      ) : (
        <StampList>
          <StampActions>
            <CaptureStampButton type="button" onClick={handleCaptureNewStamp}>
              Capture New Stamp
            </CaptureStampButton>
            <StampHint>
              {selectedStamp
                ? `Selected: ${selectedStamp.name}. Click the map to place it.`
                : "Drag over the map to capture a reusable group of tiles."}
            </StampHint>
          </StampActions>
          {availableStamps.length === 0 ? (
            <EmptyStamps>
              Drag over a building or other map area to create your first stamp.
            </EmptyStamps>
          ) : (
            availableStamps.map((stamp) => {
              const cellSize = Math.max(
                4,
                Math.min(16, 160 / stamp.widthTiles, 112 / stamp.heightTiles),
              );
              return (
                <StampCard
                  key={stamp.id}
                  $isSelected={selectedStamp?.id === stamp.id}
                  onClick={() => handleStampSelect(stamp)}
                >
                  <StampPreviewGrid
                    style={{ gridTemplateColumns: `repeat(${stamp.widthTiles}, ${cellSize}px)` }}
                  >
                    {stamp.tileImageIds.flat().map((tid, i) => (
                      tid > 0 ? (
                        <img
                          key={i}
                          src={getTileImageUrl(tid)}
                          alt=""
                          style={{ width: cellSize, height: cellSize }}
                        />
                      ) : (
                        <StampEmptyCell
                          key={i}
                          style={{ width: cellSize, height: cellSize }}
                        />
                      )
                    ))}
                  </StampPreviewGrid>
                  <StampCardInfo>
                    {stamp.id < 0 ? (
                      <StampNameInput
                        value={stamp.name}
                        aria-label={`Name ${stamp.name}`}
                        onClick={(event) => event.stopPropagation()}
                        onKeyDown={handleSearchKeyDown}
                        onChange={(event) => renameCapturedStamp(stamp.id, event.target.value)}
                      />
                    ) : (
                      <StampCardName>{stamp.name}</StampCardName>
                    )}
                    <StampCardMeta>{stamp.widthTiles}×{stamp.heightTiles} tiles</StampCardMeta>
                  </StampCardInfo>
                  {stamp.id < 0 && (
                    <DeleteStampButton
                      type="button"
                      title={`Delete ${stamp.name}`}
                      aria-label={`Delete ${stamp.name}`}
                      onClick={(event) => {
                        event.stopPropagation();
                        deleteCapturedStamp(stamp.id);
                      }}
                    >
                      ×
                    </DeleteStampButton>
                  )}
                </StampCard>
              );
            })
          )}
        </StampList>
      )}
    </PaletteContainer>
  );
};

export default TilePalette;
