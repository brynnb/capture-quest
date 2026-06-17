import React, { useState, useRef, useCallback, useEffect } from "react";
import styled from "styled-components";
import { getTileImageUrl } from "@/phaser-game/api/tileService";

interface StampData {
  id: number;
  name: string;
  widthTiles: number;
  heightTiles: number;
  createdAt: string;
  tileImageIds: number[][];
}

const Container = styled.div`
  flex: 1;
  display: flex;
  overflow: hidden;
`;

const CreatePanel = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px;
  gap: 20px;
  border-right: 2px solid rgba(255, 255, 255, 0.1);
`;

const StampListPanel = styled.div`
  width: 380px;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.03);
  flex-shrink: 0;
  overflow: hidden;
`;

const PanelHeader = styled.div`
  padding: 16px;
  flex-shrink: 0;
`;

const SectionTitle = styled.h3`
  margin: 0 0 8px 0;
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.7);
  text-transform: uppercase;
  letter-spacing: 1px;
`;

const UploadArea = styled.label`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 320px;
  height: 200px;
  border: 3px dashed rgba(167, 237, 254, 0.3);
  border-radius: 16px;
  cursor: pointer;
  transition: all 0.15s ease;
  color: rgba(167, 237, 254, 0.5);
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 600;
  gap: 8px;

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
  font-size: 36px;
`;

const PreviewContainer = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
`;

const PreviewCanvas = styled.canvas`
  border: 2px solid rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  image-rendering: pixelated;
  max-width: 400px;
  max-height: 300px;
`;

const PreviewInfo = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 13px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.6);
`;

const NameInput = styled.input`
  width: 280px;
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
  text-align: center;

  &:focus {
    border-color: #a7edfe;
    background: rgba(255, 255, 255, 0.12);
  }

  &::placeholder {
    color: rgba(255, 255, 255, 0.3);
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

const StampList = styled.div`
  flex: 1;
  overflow-y: auto;
  padding: 0 16px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;

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

const StampCard = styled.div`
  background: rgba(255, 255, 255, 0.05);
  border: 2px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 12px;
  display: flex;
  gap: 12px;
  align-items: flex-start;
`;

const StampPreview = styled.div`
  display: grid;
  gap: 0;
  flex-shrink: 0;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 4px;
  overflow: hidden;
  background: #000;

  img {
    width: 16px;
    height: 16px;
    image-rendering: pixelated;
    display: block;
  }
`;

const StampInfo = styled.div`
  flex: 1;
  min-width: 0;
`;

const StampName = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.85);
  margin-bottom: 4px;
`;

const StampMeta = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 11px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.4);
`;

const EmptyState = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.2);
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  font-weight: 600;
`;

const TILE_SIZE = 16;

const StampCreatorTab: React.FC = () => {
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [imagePreview, setImagePreview] = useState<HTMLImageElement | null>(null);
  const [stampName, setStampName] = useState("");
  const [uploading, setUploading] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; isError: boolean } | null>(null);
  const [stamps, setStamps] = useState<StampData[]>([]);
  const [loadingStamps, setLoadingStamps] = useState(false);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Load stamps on mount
  useEffect(() => {
    fetchStamps();
  }, []);

  const fetchStamps = async () => {
    setLoadingStamps(true);
    try {
      const resp = await fetch("/api/tiles/stamps");
      const data = await resp.json();
      if (data.success && Array.isArray(data.stamps)) {
        setStamps(data.stamps);
      }
    } catch {
      // Server may not be running
    } finally {
      setLoadingStamps(false);
    }
  };

  // Draw preview with grid overlay when image changes
  useEffect(() => {
    if (!imagePreview || !canvasRef.current) return;
    const canvas = canvasRef.current;
    const scale = Math.min(400 / imagePreview.width, 300 / imagePreview.height, 8);
    canvas.width = imagePreview.width * scale;
    canvas.height = imagePreview.height * scale;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.imageSmoothingEnabled = false;
    ctx.drawImage(imagePreview, 0, 0, canvas.width, canvas.height);

    // Draw grid lines
    ctx.strokeStyle = "rgba(167, 237, 254, 0.4)";
    ctx.lineWidth = 1;
    const tileW = TILE_SIZE * scale;
    const tileH = TILE_SIZE * scale;
    for (let x = tileW; x < canvas.width; x += tileW) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, canvas.height);
      ctx.stroke();
    }
    for (let y = tileH; y < canvas.height; y += tileH) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(canvas.width, y);
      ctx.stroke();
    }
  }, [imagePreview]);

  const widthTiles = imagePreview ? imagePreview.width / TILE_SIZE : 0;
  const heightTiles = imagePreview ? imagePreview.height / TILE_SIZE : 0;

  const handleFileChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.type.startsWith("image/")) {
      setStatusMsg({ text: "Please select an image file", isError: true });
      return;
    }

    const img = new Image();
    img.onload = () => {
      if (img.width % TILE_SIZE !== 0 || img.height % TILE_SIZE !== 0) {
        setStatusMsg({
          text: `Dimensions must be multiples of ${TILE_SIZE}px (got ${img.width}×${img.height})`,
          isError: true,
        });
        setImageFile(null);
        setImagePreview(null);
        URL.revokeObjectURL(img.src);
        return;
      }
      setImageFile(file);
      setImagePreview(img);
      setStatusMsg(null);
    };
    img.onerror = () => {
      setStatusMsg({ text: "Failed to load image", isError: true });
    };
    img.src = URL.createObjectURL(file);
  }, []);

  const handleCreate = useCallback(async () => {
    if (!imageFile) return;
    setUploading(true);
    setStatusMsg(null);

    try {
      const formData = new FormData();
      formData.append("name", stampName || "Unnamed Stamp");
      formData.append("image", imageFile);

      const resp = await fetch("/api/tiles/stamp", {
        method: "POST",
        body: formData,
      });

      const data = await resp.json();
      if (!data.success) {
        setStatusMsg({ text: data.error || "Upload failed", isError: true });
        return;
      }

      setStatusMsg({
        text: `Stamp created! ${data.width}×${data.height} tiles (${data.width * data.height} total)`,
        isError: false,
      });
      setImageFile(null);
      setImagePreview(null);
      setStampName("");
      if (fileInputRef.current) fileInputRef.current.value = "";

      // Refresh stamp list
      fetchStamps();
    } catch {
      setStatusMsg({ text: "Network error — is the server running?", isError: true });
    } finally {
      setUploading(false);
    }
  }, [imageFile, stampName]);

  const handleClear = useCallback(() => {
    setImageFile(null);
    setImagePreview(null);
    setStampName("");
    setStatusMsg(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }, []);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    e.stopPropagation();
  }, []);

  return (
    <Container>
      <CreatePanel>
        <SectionTitle>Create New Stamp</SectionTitle>

        {!imagePreview ? (
          <UploadArea>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/png,image/gif,image/jpeg,image/webp"
              onChange={handleFileChange}
            />
            <UploadIcon>+</UploadIcon>
            Upload image (multiples of 16×16)
          </UploadArea>
        ) : (
          <PreviewContainer>
            <PreviewCanvas ref={canvasRef} />
            <PreviewInfo>
              {imagePreview.width}×{imagePreview.height}px — {widthTiles}×{heightTiles} tiles ({widthTiles * heightTiles} total)
            </PreviewInfo>
          </PreviewContainer>
        )}

        {imagePreview && (
          <>
            <NameInput
              value={stampName}
              onChange={(e) => setStampName(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Stamp name (e.g. Pokécenter)"
              autoComplete="off"
            />

            <ButtonRow>
              <ActionButton onClick={handleCreate} disabled={uploading}>
                {uploading ? "Creating..." : "Create Stamp"}
              </ActionButton>
              <ActionButton $variant="danger" onClick={handleClear} disabled={uploading}>
                Clear
              </ActionButton>
            </ButtonRow>
          </>
        )}

        {statusMsg && (
          <StatusMessage $isError={statusMsg.isError}>{statusMsg.text}</StatusMessage>
        )}
      </CreatePanel>

      <StampListPanel>
        <PanelHeader>
          <SectionTitle>Saved Stamps ({stamps.length})</SectionTitle>
        </PanelHeader>

        <StampList>
          {loadingStamps ? (
            <EmptyState>Loading...</EmptyState>
          ) : stamps.length === 0 ? (
            <EmptyState>No stamps yet</EmptyState>
          ) : (
            stamps.map((stamp) => (
              <StampCard key={stamp.id}>
                <StampPreview
                  style={{
                    gridTemplateColumns: `repeat(${stamp.widthTiles}, 16px)`,
                  }}
                >
                  {stamp.tileImageIds.flat().map((tid, i) => (
                    <img
                      key={i}
                      src={getTileImageUrl(tid)}
                      alt=""
                    />
                  ))}
                </StampPreview>
                <StampInfo>
                  <StampName>{stamp.name}</StampName>
                  <StampMeta>
                    {stamp.widthTiles}×{stamp.heightTiles} tiles — ID #{stamp.id}
                  </StampMeta>
                </StampInfo>
              </StampCard>
            ))
          )}
        </StampList>
      </StampListPanel>
    </Container>
  );
};

export default StampCreatorTab;
