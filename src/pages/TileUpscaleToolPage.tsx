import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import JSZip from "jszip";
import {
  FiDownload,
  FiGrid,
  FiImage,
  FiLink,
  FiPackage,
  FiTrash2,
  FiUpload,
} from "react-icons/fi";
import styled from "styled-components";
import { getTileImageUrl } from "@/phaser-game/api/tileService";

const TILE_COUNT = 826;
const TILE_SIZE = 16;
const SCALE = 10;
const SCALED_TILE_SIZE = TILE_SIZE * SCALE;
const ATLAS_COLUMNS = 32;
const ATLAS_ROWS = Math.ceil(TILE_COUNT / ATLAS_COLUMNS);
const ATLAS_WIDTH = ATLAS_COLUMNS * SCALED_TILE_SIZE;
const ATLAS_HEIGHT = ATLAS_ROWS * SCALED_TILE_SIZE;

type Mapping = {
  targetIndex: number;
  sourceIndex: number;
};

const Page = styled.main`
  min-height: 100vh;
  background: #11151b;
  color: #f5f2e8;
  font-family: Inter, "Segoe UI", sans-serif;
  display: flex;
  flex-direction: column;
`;

const Header = styled.header`
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: #1a2029;
  padding: 18px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
`;

const TitleBlock = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
`;

const Title = styled.h1`
  font-size: 24px;
  line-height: 1.2;
  margin: 0;
`;

const Meta = styled.div`
  color: #b7bdc7;
  font-size: 13px;
`;

const Toolbar = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
`;

const Button = styled.button<{ $variant?: "primary" | "danger" }>`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 38px;
  border: 1px solid
    ${({ $variant }) =>
      $variant === "danger"
        ? "rgba(255, 120, 120, 0.55)"
        : $variant === "primary"
          ? "rgba(126, 220, 255, 0.7)"
          : "rgba(255, 255, 255, 0.16)"};
  border-radius: 8px;
  background: ${({ $variant }) =>
    $variant === "danger"
      ? "rgba(255, 120, 120, 0.12)"
      : $variant === "primary"
        ? "rgba(126, 220, 255, 0.16)"
        : "rgba(255, 255, 255, 0.06)"};
  color: ${({ $variant }) => ($variant === "danger" ? "#ffb3b3" : "#f5f2e8")};
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  padding: 8px 12px;

  &:hover:not(:disabled) {
    background: ${({ $variant }) =>
      $variant === "danger"
        ? "rgba(255, 120, 120, 0.2)"
        : $variant === "primary"
          ? "rgba(126, 220, 255, 0.24)"
          : "rgba(255, 255, 255, 0.1)"};
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }
`;

const UploadLabel = styled.label`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 38px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  color: #f5f2e8;
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  padding: 8px 12px;

  &:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  input {
    display: none;
  }
`;

const StatusBar = styled.div`
  min-height: 34px;
  padding: 8px 24px;
  background: #151a21;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  color: #cfd6e0;
  font-size: 13px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
`;

const ZoomControl = styled.label`
  display: inline-flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;

  input {
    width: 160px;
  }
`;

const Workspace = styled.section`
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  grid-template-rows: minmax(360px, 1fr) minmax(300px, 0.78fr);
  gap: 12px;
  padding: 12px;

  @media (max-width: 1100px) {
    grid-template-columns: 1fr;
    grid-template-rows: repeat(3, minmax(320px, auto));
  }
`;

const Pane = styled.article`
  min-width: 0;
  min-height: 0;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: #1a2029;
  display: flex;
  flex-direction: column;
  overflow: hidden;
`;

const OutputPane = styled(Pane)`
  grid-column: 1 / span 2;

  @media (max-width: 1100px) {
    grid-column: auto;
  }
`;

const PaneHeader = styled.div`
  min-height: 48px;
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
`;

const PaneTitle = styled.h2`
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 15px;
`;

const SelectionLabel = styled.div`
  color: #b7bdc7;
  font-size: 13px;
`;

const CanvasScroll = styled.div`
  flex: 1;
  min-height: 0;
  overflow: auto;
  background:
    linear-gradient(45deg, rgba(255, 255, 255, 0.035) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(255, 255, 255, 0.035) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(255, 255, 255, 0.035) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(255, 255, 255, 0.035) 75%);
  background-size: 24px 24px;
  background-position:
    0 0,
    0 12px,
    12px -12px,
    -12px 0;
`;

const CanvasWrap = styled.div<{ $zoom: number }>`
  position: relative;
  width: ${ATLAS_WIDTH}px;
  height: ${ATLAS_HEIGHT}px;
  transform: scale(${({ $zoom }) => $zoom});
  transform-origin: top left;
  margin-right: ${({ $zoom }) => ATLAS_WIDTH * ($zoom - 1)}px;
  margin-bottom: ${({ $zoom }) => ATLAS_HEIGHT * ($zoom - 1)}px;
`;

const AtlasCanvas = styled.canvas`
  display: block;
  width: ${ATLAS_WIDTH}px;
  height: ${ATLAS_HEIGHT}px;
  image-rendering: pixelated;
  background: rgba(0, 0, 0, 0.4);
  cursor: crosshair;
`;

const GridOverlay = styled.div`
  pointer-events: none;
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(to right, rgba(255, 255, 255, 0.12) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(255, 255, 255, 0.12) 1px, transparent 1px);
  background-size: ${SCALED_TILE_SIZE}px ${SCALED_TILE_SIZE}px;
`;

const Placeholder = styled.div`
  height: 100%;
  min-height: 240px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #8e98a6;
  font-size: 14px;
`;

const UploadPlaceholder = styled.div`
  pointer-events: none;
  position: absolute;
  left: 0;
  top: 0;
  width: min(100vw, 100%);
  height: min(70vh, 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #8e98a6;
  font-size: 14px;
`;

const TileHighlight = styled.div<{
  $index: number;
  $tone: "target" | "source" | "mapped";
}>`
  pointer-events: none;
  position: absolute;
  left: ${({ $index }) => ($index % ATLAS_COLUMNS) * SCALED_TILE_SIZE}px;
  top: ${({ $index }) => Math.floor($index / ATLAS_COLUMNS) * SCALED_TILE_SIZE}px;
  width: ${SCALED_TILE_SIZE}px;
  height: ${SCALED_TILE_SIZE}px;
  box-sizing: border-box;
  border: ${({ $tone }) => ($tone === "mapped" ? "4px solid rgba(120, 255, 170, 0.8)" : "5px solid #ffe063")};
  background: ${({ $tone }) =>
    $tone === "mapped"
      ? "rgba(120, 255, 170, 0.16)"
      : $tone === "source"
        ? "rgba(126, 220, 255, 0.18)"
        : "rgba(255, 224, 99, 0.18)"};
`;

const MappingPanel = styled.aside`
  width: min(430px, 42vw);
  min-width: 320px;
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;

  @media (max-width: 1100px) {
    width: 100%;
    min-width: 0;
    border-left: 0;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
  }
`;

const OutputBody = styled.div`
  flex: 1;
  min-height: 0;
  display: flex;

  @media (max-width: 1100px) {
    flex-direction: column;
  }
`;

const MappingActions = styled.div`
  padding: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
`;

const MappingList = styled.div`
  flex: 1;
  min-height: 0;
  overflow: auto;
`;

const MappingRow = styled.div`
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  color: #dbe2ec;
  font-size: 13px;
`;

const SmallIconButton = styled.button`
  width: 32px;
  height: 32px;
  border: 1px solid rgba(255, 120, 120, 0.45);
  border-radius: 8px;
  background: rgba(255, 120, 120, 0.1);
  color: #ffb3b3;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;

  &:hover {
    background: rgba(255, 120, 120, 0.18);
  }
`;

const tileLabel = (index: number | null) =>
  index === null ? "None" : `tile_${index}.png`;

const tilePosition = (index: number) => ({
  x: (index % ATLAS_COLUMNS) * SCALED_TILE_SIZE,
  y: Math.floor(index / ATLAS_COLUMNS) * SCALED_TILE_SIZE,
});

const loadImage = (url: string): Promise<HTMLImageElement> =>
  new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error(`Unable to load ${url}`));
    image.src = url;
  });

const canvasToBlob = (canvas: HTMLCanvasElement): Promise<Blob> =>
  new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) resolve(blob);
      else reject(new Error("Canvas export failed"));
    }, "image/png");
  });

const downloadBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const tileIndexFromPointer = (
  event: React.MouseEvent<HTMLCanvasElement>,
): number | null => {
  const canvas = event.currentTarget;
  const rect = canvas.getBoundingClientRect();
  const x = ((event.clientX - rect.left) / rect.width) * canvas.width;
  const y = ((event.clientY - rect.top) / rect.height) * canvas.height;
  const col = Math.floor(x / SCALED_TILE_SIZE);
  const row = Math.floor(y / SCALED_TILE_SIZE);
  const index = row * ATLAS_COLUMNS + col;
  return index >= 0 && index < TILE_COUNT ? index : null;
};

const TileUpscaleToolPage: React.FC = () => {
  const baseCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const uploadedCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const outputCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const [isBaseReady, setIsBaseReady] = useState(false);
  const [isUploadedReady, setIsUploadedReady] = useState(false);
  const [selectedTarget, setSelectedTarget] = useState<number | null>(null);
  const [selectedSource, setSelectedSource] = useState<number | null>(null);
  const [mappings, setMappings] = useState<Mapping[]>([]);
  const [zoom, setZoom] = useState(0.25);
  const [busyLabel, setBusyLabel] = useState<string | null>(null);
  const [status, setStatus] = useState("Building 10x map tile atlas...");

  const mappedTargets = useMemo(
    () => new Set(mappings.map((mapping) => mapping.targetIndex)),
    [mappings],
  );

  const buildBaseAtlas = useCallback(async () => {
    const canvas = baseCanvasRef.current;
    if (!canvas) return;
    setBusyLabel("Building atlas");
    setStatus("Building 10x map tile atlas...");
    setIsBaseReady(false);

    canvas.width = ATLAS_WIDTH;
    canvas.height = ATLAS_HEIGHT;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, ATLAS_WIDTH, ATLAS_HEIGHT);

    for (let index = 0; index < TILE_COUNT; index += 1) {
      const image = await loadImage(getTileImageUrl(index + 1));
      const { x, y } = tilePosition(index);
      ctx.drawImage(image, x, y, SCALED_TILE_SIZE, SCALED_TILE_SIZE);
      if (index % 50 === 0) {
        setStatus(`Building 10x map tile atlas... ${index + 1}/${TILE_COUNT}`);
        await new Promise((resolve) => window.setTimeout(resolve, 0));
      }
    }

    setIsBaseReady(true);
    setBusyLabel(null);
    setStatus(`Ready: ${TILE_COUNT} tiles, ${ATLAS_WIDTH}x${ATLAS_HEIGHT}px`);
  }, []);

  const redrawOutputAtlas = useCallback(() => {
    const baseCanvas = baseCanvasRef.current;
    const uploadedCanvas = uploadedCanvasRef.current;
    const outputCanvas = outputCanvasRef.current;
    if (!baseCanvas || !outputCanvas || !isBaseReady) return;

    outputCanvas.width = ATLAS_WIDTH;
    outputCanvas.height = ATLAS_HEIGHT;
    const ctx = outputCanvas.getContext("2d");
    if (!ctx) return;
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, ATLAS_WIDTH, ATLAS_HEIGHT);
    ctx.drawImage(baseCanvas, 0, 0);

    if (uploadedCanvas && isUploadedReady) {
      for (const mapping of mappings) {
        const target = tilePosition(mapping.targetIndex);
        const source = tilePosition(mapping.sourceIndex);
        ctx.drawImage(
          uploadedCanvas,
          source.x,
          source.y,
          SCALED_TILE_SIZE,
          SCALED_TILE_SIZE,
          target.x,
          target.y,
          SCALED_TILE_SIZE,
          SCALED_TILE_SIZE,
        );
      }
    }

  }, [isBaseReady, isUploadedReady, mappings]);

  useEffect(() => {
    void buildBaseAtlas();
  }, [buildBaseAtlas]);

  useEffect(() => {
    redrawOutputAtlas();
  }, [redrawOutputAtlas]);

  const addMapping = useCallback(() => {
    if (selectedTarget === null || selectedSource === null) return;
    setMappings((current) => {
      const next = current.filter(
        (mapping) => mapping.targetIndex !== selectedTarget,
      );
      next.push({ targetIndex: selectedTarget, sourceIndex: selectedSource });
      next.sort((a, b) => a.targetIndex - b.targetIndex);
      return next;
    });
    setStatus(`${tileLabel(selectedSource)} mapped to ${tileLabel(selectedTarget)}`);
  }, [selectedSource, selectedTarget]);

  const mapAllOneToOne = useCallback(() => {
    setMappings(
      Array.from({ length: TILE_COUNT }, (_, index) => ({
        targetIndex: index,
        sourceIndex: index,
      })),
    );
    setStatus(`Mapped all ${TILE_COUNT} tiles 1:1`);
  }, []);

  const removeMapping = useCallback((targetIndex: number) => {
    setMappings((current) =>
      current.filter((mapping) => mapping.targetIndex !== targetIndex),
    );
  }, []);

  const clearMappings = useCallback(() => {
    setMappings([]);
    setStatus("Mappings cleared");
  }, []);

  const handleBaseClick = useCallback((event: React.MouseEvent<HTMLCanvasElement>) => {
    const index = tileIndexFromPointer(event);
    if (index === null) return;
    setSelectedTarget(index);
  }, []);

  const handleUploadClick = useCallback((event: React.MouseEvent<HTMLCanvasElement>) => {
    const index = tileIndexFromPointer(event);
    if (index === null) return;
    setSelectedSource(index);
  }, []);

  const handleUpload = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      event.target.value = "";
      if (!file) return;

      const url = URL.createObjectURL(file);
      try {
        const image = await loadImage(url);
        if (image.naturalWidth !== ATLAS_WIDTH || image.naturalHeight !== ATLAS_HEIGHT) {
          setStatus(
            `Upload rejected: expected ${ATLAS_WIDTH}x${ATLAS_HEIGHT}px, got ${image.naturalWidth}x${image.naturalHeight}px`,
          );
          setIsUploadedReady(false);
          return;
        }

        const canvas = uploadedCanvasRef.current;
        if (!canvas) return;
        canvas.width = ATLAS_WIDTH;
        canvas.height = ATLAS_HEIGHT;
        const ctx = canvas.getContext("2d");
        if (!ctx) return;
        ctx.imageSmoothingEnabled = false;
        ctx.clearRect(0, 0, ATLAS_WIDTH, ATLAS_HEIGHT);
        ctx.drawImage(image, 0, 0);
        setIsUploadedReady(true);
        setStatus(`Uploaded ${file.name}`);
      } finally {
        URL.revokeObjectURL(url);
      }
    },
    [],
  );

  const downloadBaseAtlas = useCallback(async () => {
    const canvas = baseCanvasRef.current;
    if (!canvas || !isBaseReady) return;
    setBusyLabel("Exporting atlas");
    downloadBlob(await canvasToBlob(canvas), "capturequest-map-tiles-10x.png");
    setBusyLabel(null);
  }, [isBaseReady]);

  const downloadOutputAtlas = useCallback(async () => {
    const canvas = outputCanvasRef.current;
    if (!canvas || !isBaseReady) return;
    setBusyLabel("Exporting mapped atlas");
    redrawOutputAtlas();
    downloadBlob(
      await canvasToBlob(canvas),
      "capturequest-map-tiles-10x-mapped.png",
    );
    setBusyLabel(null);
  }, [isBaseReady, redrawOutputAtlas]);

  const canvasTileBlob = useCallback(
    async (canvas: HTMLCanvasElement, index: number): Promise<Blob> => {
      const { x, y } = tilePosition(index);
      const tileCanvas = document.createElement("canvas");
      tileCanvas.width = SCALED_TILE_SIZE;
      tileCanvas.height = SCALED_TILE_SIZE;
      const ctx = tileCanvas.getContext("2d");
      if (!ctx) throw new Error("Unable to create tile canvas");
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(
        canvas,
        x,
        y,
        SCALED_TILE_SIZE,
        SCALED_TILE_SIZE,
        0,
        0,
        SCALED_TILE_SIZE,
        SCALED_TILE_SIZE,
      );
      return canvasToBlob(tileCanvas);
    },
    [],
  );

  const downloadBaseTilesZip = useCallback(async () => {
    const canvas = baseCanvasRef.current;
    if (!canvas || !isBaseReady) return;
    setBusyLabel("Zipping base tiles");
    const zip = new JSZip();

    for (let index = 0; index < TILE_COUNT; index += 1) {
      zip.file(`tile_${index}.png`, await canvasTileBlob(canvas, index));
      if (index % 40 === 0) {
        setStatus(`Zipping 10x individual tiles... ${index + 1}/${TILE_COUNT}`);
        await new Promise((resolve) => window.setTimeout(resolve, 0));
      }
    }

    const blob = await zip.generateAsync({ type: "blob" });
    downloadBlob(blob, "capturequest-map-tiles-10x-individual.zip");
    setBusyLabel(null);
    setStatus(`Ready: ${TILE_COUNT} tiles, ${ATLAS_WIDTH}x${ATLAS_HEIGHT}px`);
  }, [canvasTileBlob, isBaseReady]);

  const downloadReplacementZip = useCallback(async () => {
    const uploadedCanvas = uploadedCanvasRef.current;
    if (!uploadedCanvas || !isUploadedReady || mappings.length === 0) return;
    setBusyLabel("Zipping replacements");
    const zip = new JSZip();
    const manifest = {
      tileSize: SCALED_TILE_SIZE,
      scale: SCALE,
      mappings: mappings.map((mapping) => ({
        targetFile: `tile_${mapping.targetIndex}.png`,
        sourceTile: mapping.sourceIndex,
        targetTile: mapping.targetIndex,
      })),
    };
    zip.file("tile_replacements.json", JSON.stringify(manifest, null, 2));

    for (let i = 0; i < mappings.length; i += 1) {
      const mapping = mappings[i];
      zip.file(
        `tile_${mapping.targetIndex}.png`,
        await canvasTileBlob(uploadedCanvas, mapping.sourceIndex),
      );
      if (i % 20 === 0) {
        setStatus(`Zipping mapped replacement tiles... ${i + 1}/${mappings.length}`);
        await new Promise((resolve) => window.setTimeout(resolve, 0));
      }
    }

    const blob = await zip.generateAsync({ type: "blob" });
    downloadBlob(blob, "capturequest-map-tile-replacements-10x.zip");
    setBusyLabel(null);
    setStatus(`Exported ${mappings.length} mapped replacement tiles`);
  }, [canvasTileBlob, isUploadedReady, mappings]);

  return (
    <Page>
      <Header>
        <TitleBlock>
          <Title>Map Tile Upscale Tool</Title>
          <Meta>
            {TILE_COUNT} map tiles, {ATLAS_COLUMNS} columns, {SCALE}x scale,{" "}
            {ATLAS_WIDTH}x{ATLAS_HEIGHT}px atlas
          </Meta>
        </TitleBlock>
        <Toolbar>
          <Button onClick={buildBaseAtlas} disabled={Boolean(busyLabel)}>
            <FiGrid /> Rebuild
          </Button>
          <Button onClick={downloadBaseAtlas} disabled={!isBaseReady || Boolean(busyLabel)}>
            <FiDownload /> Combined
          </Button>
          <Button onClick={downloadBaseTilesZip} disabled={!isBaseReady || Boolean(busyLabel)}>
            <FiPackage /> Individual
          </Button>
          <UploadLabel>
            <FiUpload /> Upload
            <input accept="image/png,image/jpeg,image/webp" type="file" onChange={handleUpload} />
          </UploadLabel>
          <Button
            $variant="primary"
            onClick={downloadReplacementZip}
            disabled={!isUploadedReady || mappings.length === 0 || Boolean(busyLabel)}
          >
            <FiPackage /> Replacements
          </Button>
          <Button onClick={downloadOutputAtlas} disabled={!isBaseReady || Boolean(busyLabel)}>
            <FiDownload /> Mapped Sheet
          </Button>
        </Toolbar>
      </Header>

      <StatusBar>
        <span>{busyLabel ? `${busyLabel}...` : status}</span>
        <ZoomControl>
          Zoom
          <input
            min="0.1"
            max="1"
            step="0.05"
            type="range"
            value={zoom}
            onChange={(event) => setZoom(Number(event.target.value))}
          />
          {Math.round(zoom * 100)}%
        </ZoomControl>
      </StatusBar>

      <Workspace>
        <Pane>
          <PaneHeader>
            <PaneTitle>
              <FiGrid /> Existing Map Tiles
            </PaneTitle>
            <SelectionLabel>{tileLabel(selectedTarget)}</SelectionLabel>
          </PaneHeader>
          <CanvasScroll>
            <CanvasWrap $zoom={zoom}>
              <AtlasCanvas ref={baseCanvasRef} onClick={handleBaseClick} />
              <GridOverlay />
              {selectedTarget !== null && (
                <TileHighlight $index={selectedTarget} $tone="target" />
              )}
              {Array.from(mappedTargets).map((index) => (
                <TileHighlight key={index} $index={index} $tone="mapped" />
              ))}
            </CanvasWrap>
          </CanvasScroll>
        </Pane>

        <Pane>
          <PaneHeader>
            <PaneTitle>
              <FiImage /> Uploaded Sheet
            </PaneTitle>
            <SelectionLabel>{tileLabel(selectedSource)}</SelectionLabel>
          </PaneHeader>
          <CanvasScroll>
            <CanvasWrap $zoom={zoom}>
              <AtlasCanvas ref={uploadedCanvasRef} onClick={handleUploadClick} />
              <GridOverlay />
              {!isUploadedReady && (
                <UploadPlaceholder>
                  Upload {ATLAS_WIDTH}x{ATLAS_HEIGHT}px
                </UploadPlaceholder>
              )}
              {selectedSource !== null && (
                <TileHighlight $index={selectedSource} $tone="source" />
              )}
            </CanvasWrap>
          </CanvasScroll>
        </Pane>

        <OutputPane>
          <PaneHeader>
            <PaneTitle>
              <FiLink /> Mapped Output
            </PaneTitle>
            <SelectionLabel>{mappings.length} mapped</SelectionLabel>
          </PaneHeader>
          <OutputBody>
            <CanvasScroll>
              <CanvasWrap $zoom={zoom}>
                <AtlasCanvas ref={outputCanvasRef} />
                <GridOverlay />
                {mappings.map((mapping) => (
                  <TileHighlight
                    key={mapping.targetIndex}
                    $index={mapping.targetIndex}
                    $tone="mapped"
                  />
                ))}
              </CanvasWrap>
            </CanvasScroll>
            <MappingPanel>
              <MappingActions>
                <Button
                  $variant="primary"
                  onClick={addMapping}
                  disabled={
                    !isUploadedReady ||
                    selectedTarget === null ||
                    selectedSource === null
                  }
                >
                  <FiLink /> Map Pair
                </Button>
                <Button onClick={mapAllOneToOne} disabled={!isUploadedReady}>
                  <FiGrid /> 1:1
                </Button>
                <Button $variant="danger" onClick={clearMappings} disabled={mappings.length === 0}>
                  <FiTrash2 /> Clear
                </Button>
              </MappingActions>
              <MappingList>
                {mappings.length === 0 ? (
                  <Placeholder>No mappings</Placeholder>
                ) : (
                  mappings.map((mapping) => (
                    <MappingRow key={mapping.targetIndex}>
                      <span>
                        {tileLabel(mapping.sourceIndex)} → {tileLabel(mapping.targetIndex)}
                      </span>
                      <SmallIconButton
                        aria-label={`Remove ${tileLabel(mapping.targetIndex)}`}
                        onClick={() => removeMapping(mapping.targetIndex)}
                      >
                        <FiTrash2 />
                      </SmallIconButton>
                    </MappingRow>
                  ))
                )}
              </MappingList>
            </MappingPanel>
          </OutputBody>
        </OutputPane>
      </Workspace>
    </Page>
  );
};

export default TileUpscaleToolPage;
