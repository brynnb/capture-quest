import React, { useState, useMemo, useRef, useCallback, useEffect } from "react";
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
  width: 300px;
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
  padding: 32px;
  gap: 20px;
  overflow-y: auto;
`;

const NoSelectionText = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.25);
  font-family: "Outfit", sans-serif;
  font-size: 16px;
  font-weight: 600;
  text-align: center;
`;

const TileInfo = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
  text-align: center;
`;

const AnimPreview = styled.div`
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  justify-content: center;
`;

const FrameBox = styled.div<{ $isActive?: boolean }>`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
`;

const FrameImage = styled.div<{ $isActive?: boolean }>`
  width: 64px;
  height: 64px;
  border: 3px solid ${({ $isActive }) => ($isActive ? "#a7edfe" : "rgba(255, 255, 255, 0.15)")};
  border-radius: 8px;
  background: #000;
  overflow: hidden;
  box-shadow: ${({ $isActive }) => ($isActive ? "0 0 8px rgba(167, 237, 254, 0.3)" : "none")};

  img {
    width: 100%;
    height: 100%;
    image-rendering: pixelated;
    display: block;
  }
`;

const FrameLabel = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 10px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.4);
  text-transform: uppercase;
`;

const LivePreview = styled.div`
  width: 96px;
  height: 96px;
  border: 3px solid #a7edfe;
  border-radius: 12px;
  background: #000;
  overflow: hidden;

  img {
    width: 100%;
    height: 100%;
    image-rendering: pixelated;
    display: block;
  }
`;

const LiveLabel = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 700;
  color: #a7edfe;
  text-align: center;
`;

const UploadArea = styled.label`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 200px;
  height: 80px;
  border: 3px dashed rgba(167, 237, 254, 0.3);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
  color: rgba(167, 237, 254, 0.5);
  font-family: "Outfit", sans-serif;
  font-size: 12px;
  font-weight: 600;
  gap: 4px;

  &:hover {
    border-color: #a7edfe;
    color: #a7edfe;
    background: rgba(167, 237, 254, 0.05);
  }

  input {
    display: none;
  }
`;

const SpeedControl = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  font-family: "Outfit", sans-serif;
  font-size: 13px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.6);
`;

const SpeedInput = styled.input`
  width: 80px;
  padding: 6px 10px;
  background: rgba(255, 255, 255, 0.08);
  border: 2px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  font-family: "Outfit", sans-serif;
  font-size: 13px;
  color: white;
  outline: none;
  text-align: center;

  &:focus {
    border-color: #a7edfe;
  }
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

const AnimatedTilesTab: React.FC = () => {
  const { tileProperties, tilePropertiesLoaded } = useTileEditorStore();
  const [selectedTileId, setSelectedTileId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [frameFiles, setFrameFiles] = useState<File[]>([]);
  const [framePreviews, setFramePreviews] = useState<string[]>([]);
  const [frameDuration, setFrameDuration] = useState(500);
  const [uploading, setUploading] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; isError: boolean } | null>(null);
  const [liveFrameIndex, setLiveFrameIndex] = useState(0);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
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

  // Live animation preview
  const allFrameUrls = useMemo(() => {
    if (!selectedTileId) return [];
    const base = [getTileImageUrl(selectedTileId)];
    return [...base, ...framePreviews];
  }, [selectedTileId, framePreviews]);

  useEffect(() => {
    if (allFrameUrls.length <= 1) {
      setLiveFrameIndex(0);
      return;
    }
    const interval = setInterval(() => {
      setLiveFrameIndex((prev) => (prev + 1) % allFrameUrls.length);
    }, frameDuration);
    return () => clearInterval(interval);
  }, [allFrameUrls.length, frameDuration]);

  const handleFrameFiles = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length === 0) return;

    const validFiles: File[] = [];
    const previews: string[] = [];
    let processed = 0;

    files.forEach((file) => {
      const img = new Image();
      img.onload = () => {
        if (img.width === 16 && img.height === 16) {
          validFiles.push(file);
          previews.push(img.src);
        }
        processed++;
        if (processed === files.length) {
          setFrameFiles((prev) => [...prev, ...validFiles]);
          setFramePreviews((prev) => [...prev, ...previews]);
          if (validFiles.length < files.length) {
            setStatusMsg({
              text: `${files.length - validFiles.length} file(s) skipped (not 16×16)`,
              isError: true,
            });
          } else {
            setStatusMsg(null);
          }
        }
      };
      img.onerror = () => {
        processed++;
        if (processed === files.length) {
          setFrameFiles((prev) => [...prev, ...validFiles]);
          setFramePreviews((prev) => [...prev, ...previews]);
        }
      };
      img.src = URL.createObjectURL(file);
    });
  }, []);

  const handleSave = useCallback(async () => {
    if (!selectedTileId || frameFiles.length === 0) return;
    setUploading(true);
    setStatusMsg(null);

    try {
      const formData = new FormData();
      formData.append("tileImageId", String(selectedTileId));
      formData.append("frameDurationMs", String(frameDuration));
      frameFiles.forEach((f) => formData.append("frames", f));

      const resp = await fetch("/api/tiles/animation", {
        method: "POST",
        body: formData,
      });

      const data = await resp.json();
      if (!data.success) {
        setStatusMsg({ text: data.error || "Upload failed", isError: true });
        return;
      }

      setStatusMsg({ text: `Saved ${data.frames} animation frame(s)!`, isError: false });
      setFrameFiles([]);
      setFramePreviews([]);
      if (fileInputRef.current) fileInputRef.current.value = "";
    } catch {
      setStatusMsg({ text: "Network error — is the server running?", isError: true });
    } finally {
      setUploading(false);
    }
  }, [selectedTileId, frameFiles, frameDuration]);

  const handleClearFrames = useCallback(() => {
    framePreviews.forEach((url) => URL.revokeObjectURL(url));
    setFrameFiles([]);
    setFramePreviews([]);
    setStatusMsg(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }, [framePreviews]);

  const handleSearchKeyDown = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  const handleSpeedKeyDown = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  return (
    <Container>
      <PalettePanel>
        <PaletteHeader>
          <SectionTitle>Select Base Tile</SectionTitle>
          <SearchBar
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={handleSearchKeyDown}
            placeholder="Search tiles..."
            autoComplete="off"
          />
        </PaletteHeader>
        <TileGrid>
          {filteredTiles.map((tile) => (
            <TileItem
              key={tile.tileImageId}
              $isSelected={selectedTileId === tile.tileImageId}
              onClick={() => {
                setSelectedTileId(tile.tileImageId);
                handleClearFrames();
              }}
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
            Select a base tile, then add animation frames
          </NoSelectionText>
        ) : (
          <>
            <TileInfo>
              {selectedProp?.name || `Tile ${selectedTileId}`} (#{selectedTileId})
            </TileInfo>

            <AnimPreview>
              <FrameBox>
                <FrameImage $isActive={liveFrameIndex === 0}>
                  <img src={getTileImageUrl(selectedTileId)} alt="Base" />
                </FrameImage>
                <FrameLabel>Base</FrameLabel>
              </FrameBox>

              {framePreviews.map((url, i) => (
                <FrameBox key={i}>
                  <FrameImage $isActive={liveFrameIndex === i + 1}>
                    <img src={url} alt={`Frame ${i + 1}`} />
                  </FrameImage>
                  <FrameLabel>Frame {i + 1}</FrameLabel>
                </FrameBox>
              ))}
            </AnimPreview>

            {allFrameUrls.length > 1 && (
              <>
                <LiveLabel>Live Preview</LiveLabel>
                <LivePreview>
                  <img src={allFrameUrls[liveFrameIndex]} alt="Animation preview" />
                </LivePreview>
              </>
            )}

            <UploadArea>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/png,image/gif,image/jpeg,image/webp"
                multiple
                onChange={handleFrameFiles}
              />
              + Add Animation Frames (16×16)
            </UploadArea>

            <SpeedControl>
              Speed:
              <SpeedInput
                type="number"
                value={frameDuration}
                onChange={(e) => setFrameDuration(Math.max(50, parseInt(e.target.value) || 500))}
                onKeyDown={handleSpeedKeyDown}
                min={50}
                step={50}
              />
              ms per frame
            </SpeedControl>

            {frameFiles.length > 0 && (
              <ButtonRow>
                <ActionButton onClick={handleSave} disabled={uploading}>
                  {uploading ? "Saving..." : `Save ${frameFiles.length} Frame(s)`}
                </ActionButton>
                <ActionButton $variant="danger" onClick={handleClearFrames} disabled={uploading}>
                  Clear Frames
                </ActionButton>
              </ButtonRow>
            )}

            {statusMsg && (
              <StatusMessage $isError={statusMsg.isError}>{statusMsg.text}</StatusMessage>
            )}
          </>
        )}
      </DetailPanel>
    </Container>
  );
};

export default AnimatedTilesTab;
