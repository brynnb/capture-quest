import React, { useState, useMemo, useRef, useCallback } from "react";
import styled from "styled-components";
import useTileEditorStore from "@/stores/TileEditorStore";
import { getTileImageUrl } from "@/phaser-game/api/tileService";
import * as TileEditorNet from "@/components/TileEditor/TileEditorNetwork";

const Container = styled.div`
  flex: 1;
  display: flex;
  overflow: hidden;
`;

const PalettePanel = styled.div`
  width: 340px;
  display: flex;
  flex-direction: column;
  border-right: 2px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  flex-shrink: 0;
`;

const PaletteHeader = styled.div`
  padding: 16px;
  flex-shrink: 0;
`;

const SectionTitle = styled.h3`
  margin: 0 0 10px 0;
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 1px;
`;

const SearchBar = styled.input`
  width: 100%;
  padding: 8px 12px;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.08);
  border: 2px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  font-family: "Outfit", sans-serif;
  font-weight: 500;
  font-size: 13px;
  color: white;
  outline: none;

  &:focus {
    border-color: #a7edfe;
    background: rgba(255, 255, 255, 0.12);
  }

  &::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }
`;

const TileGrid = styled.div`
  flex: 1;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(48px, 1fr));
  grid-auto-rows: min-content;
  align-content: start;
  gap: 4px;
  overflow-y: auto;
  padding: 0 16px 16px 16px;

  &::-webkit-scrollbar {
    width: 6px;
  }
  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
  }
  &::-webkit-scrollbar-track {
    background: transparent;
  }
`;

const TileItem = styled.div<{ $isSelected: boolean }>`
  aspect-ratio: 1;
  border: 2px solid ${({ $isSelected }) => ($isSelected ? "#a7edfe" : "rgba(255, 255, 255, 0.1)")};
  border-radius: 6px;
  cursor: pointer;
  overflow: hidden;
  background: #000;
  transition: border-color 0.1s ease, transform 0.1s ease;
  box-shadow: ${({ $isSelected }) => ($isSelected ? "0 0 8px rgba(167, 237, 254, 0.4)" : "none")};

  &:hover {
    border-color: #a7edfe;
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

const DetailPanel = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  gap: 24px;
`;

const NoSelectionText = styled.div`
  color: rgba(255, 255, 255, 0.25);
  font-family: "Outfit", sans-serif;
  font-size: 16px;
  font-weight: 600;
  text-align: center;
`;

const PreviewRow = styled.div`
  display: flex;
  align-items: center;
  gap: 40px;
`;

const PreviewBox = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
`;

const PreviewLabel = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.5);
  text-transform: uppercase;
  letter-spacing: 1px;
`;

const PreviewImage = styled.div<{ $hasImage: boolean }>`
  width: 128px;
  height: 128px;
  border: 3px solid rgba(255, 255, 255, 0.15);
  border-radius: 12px;
  background: ${({ $hasImage }) => ($hasImage ? "#000" : "rgba(255, 255, 255, 0.05)")};
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;

  img {
    width: 100%;
    height: 100%;
    image-rendering: pixelated;
    display: block;
  }
`;

const Arrow = styled.div`
  font-size: 28px;
  color: rgba(255, 255, 255, 0.3);
`;

const TileInfo = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
  text-align: center;
`;

const UploadArea = styled.label`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 128px;
  height: 128px;
  border: 3px dashed rgba(167, 237, 254, 0.3);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
  color: rgba(167, 237, 254, 0.5);
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 600;
  gap: 6px;

  &:hover {
    border-color: #a7edfe;
    color: #a7edfe;
    background: rgba(167, 237, 254, 0.05);
  }

  input {
    display: none;
  }
`;

const UploadIcon = styled.div`
  font-size: 28px;
`;

const ActionButton = styled.button<{ $variant?: "primary" | "danger" }>`
  padding: 10px 28px;
  border: 2px solid ${({ $variant }) => ($variant === "danger" ? "#ff6b6b" : "#a7edfe")};
  border-radius: 10px;
  background: ${({ $variant }) => ($variant === "danger" ? "rgba(255, 107, 107, 0.15)" : "rgba(167, 237, 254, 0.15)")};
  color: ${({ $variant }) => ($variant === "danger" ? "#ff6b6b" : "#a7edfe")};
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    background: ${({ $variant }) => ($variant === "danger" ? "rgba(255, 107, 107, 0.25)" : "rgba(167, 237, 254, 0.25)")};
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
`;

const ButtonRow = styled.div`
  display: flex;
  gap: 12px;
`;

const StatusMessage = styled.div<{ $isError?: boolean }>`
  font-family: "Outfit", sans-serif;
  font-size: 13px;
  font-weight: 600;
  color: ${({ $isError }) => ($isError ? "#ff6b6b" : "#6bff8e")};
`;

const ReplaceTileTab: React.FC = () => {
  const { tileProperties, tilePropertiesLoaded } = useTileEditorStore();
  const [selectedTileId, setSelectedTileId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [newImageFile, setNewImageFile] = useState<File | null>(null);
  const [newImagePreview, setNewImagePreview] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; isError: boolean } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Load tile properties if not loaded
  React.useEffect(() => {
    if (!tilePropertiesLoaded) {
      TileEditorNet.requestTileProperties();
    }
  }, [tilePropertiesLoaded]);

  const allTiles = useMemo(() => {
    if (tilePropertiesLoaded && tileProperties.length > 0) {
      return tileProperties;
    }
    return Array.from({ length: 826 }, (_, i) => ({
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

  const selectedProp = allTiles.find((t) => t.tileImageId === selectedTileId);

  const handleFileChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Validate it's a PNG
    if (!file.type.startsWith("image/")) {
      setStatusMsg({ text: "Please select an image file", isError: true });
      return;
    }

    // Validate dimensions
    const img = new Image();
    img.onload = () => {
      if (img.width !== 16 || img.height !== 16) {
        setStatusMsg({ text: `Image must be 16×16 pixels (got ${img.width}×${img.height})`, isError: true });
        setNewImageFile(null);
        setNewImagePreview(null);
        URL.revokeObjectURL(img.src);
        return;
      }
      setNewImageFile(file);
      setNewImagePreview(img.src);
      setStatusMsg(null);
    };
    img.onerror = () => {
      setStatusMsg({ text: "Failed to load image", isError: true });
    };
    img.src = URL.createObjectURL(file);
  }, []);

  const handleApply = useCallback(async () => {
    if (!selectedTileId || !newImageFile) return;
    setUploading(true);
    setStatusMsg(null);

    try {
      const formData = new FormData();
      formData.append("tileImageId", String(selectedTileId));
      formData.append("image", newImageFile);

      const resp = await fetch("/api/tiles/replace", {
        method: "POST",
        body: formData,
      });

      const data = await resp.json();
      if (!data.success) {
        setStatusMsg({ text: data.error || "Upload failed", isError: true });
        return;
      }

      setStatusMsg({ text: "Tile replaced successfully!", isError: false });
      setNewImageFile(null);
      setNewImagePreview(null);
      if (fileInputRef.current) fileInputRef.current.value = "";

      // Dispatch event to bust Phaser texture cache for this tile
      window.dispatchEvent(
        new CustomEvent("tileImageReplaced", { detail: { tileImageId: selectedTileId } }),
      );
    } catch {
      setStatusMsg({ text: "Network error — is the server running?", isError: true });
    } finally {
      setUploading(false);
    }
  }, [selectedTileId, newImageFile]);

  const handleClear = useCallback(() => {
    setNewImageFile(null);
    setNewImagePreview(null);
    setStatusMsg(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }, []);

  const handleSearchKeyDown = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  return (
    <Container>
      <PalettePanel>
        <PaletteHeader>
          <SectionTitle>Select Tile to Replace</SectionTitle>
          <SearchBar
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={handleSearchKeyDown}
            placeholder="Search tiles by name or ID..."
            autoComplete="off"
          />
        </PaletteHeader>
        <TileGrid>
          {filteredTiles.map((tile) => (
            <TileItem
              key={tile.tileImageId}
              $isSelected={selectedTileId === tile.tileImageId}
              onClick={() => setSelectedTileId(tile.tileImageId)}
              title={`${tile.name} (#${tile.tileImageId})`}
            >
              <img
                src={getTileImageUrl(tile.tileImageId)}
                alt={tile.name}
                loading="lazy"
              />
            </TileItem>
          ))}
        </TileGrid>
      </PalettePanel>

      <DetailPanel>
        {!selectedTileId ? (
          <NoSelectionText>
            Select a tile from the palette to replace its image
          </NoSelectionText>
        ) : (
          <>
            <TileInfo>
              {selectedProp?.name || `Tile ${selectedTileId}`} (#{selectedTileId})
              {selectedProp && selectedProp.collisionType !== 0 ? " — Blocked" : " — Walkable"}
            </TileInfo>

            <PreviewRow>
              <PreviewBox>
                <PreviewLabel>Current</PreviewLabel>
                <PreviewImage $hasImage>
                  <img
                    src={getTileImageUrl(selectedTileId)}
                    alt="Current tile"
                  />
                </PreviewImage>
              </PreviewBox>

              <Arrow>→</Arrow>

              <PreviewBox>
                <PreviewLabel>New</PreviewLabel>
                {newImagePreview ? (
                  <PreviewImage $hasImage>
                    <img src={newImagePreview} alt="New tile" />
                  </PreviewImage>
                ) : (
                  <UploadArea>
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="image/png,image/gif,image/jpeg,image/webp"
                      onChange={handleFileChange}
                    />
                    <UploadIcon>+</UploadIcon>
                    Upload 16×16
                  </UploadArea>
                )}
              </PreviewBox>
            </PreviewRow>

            <ButtonRow>
              {newImagePreview && (
                <>
                  <ActionButton
                    onClick={handleApply}
                    disabled={uploading}
                  >
                    {uploading ? "Uploading..." : "Apply Replacement"}
                  </ActionButton>
                  <ActionButton $variant="danger" onClick={handleClear} disabled={uploading}>
                    Clear
                  </ActionButton>
                </>
              )}
            </ButtonRow>

            {statusMsg && (
              <StatusMessage $isError={statusMsg.isError}>
                {statusMsg.text}
              </StatusMessage>
            )}
          </>
        )}
      </DetailPanel>
    </Container>
  );
};

export default ReplaceTileTab;
