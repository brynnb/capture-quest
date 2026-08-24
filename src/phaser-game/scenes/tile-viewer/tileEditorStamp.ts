export const MAX_CAPTURED_STAMP_TILES = 500;

export interface CapturedTileStamp {
  widthTiles: number;
  heightTiles: number;
  tileImageIds: number[][];
  populatedTiles: number;
}

export function captureTileStamp(
  start: { x: number; y: number },
  end: { x: number; y: number },
  getTileImageIdAt: (x: number, y: number) => number,
): CapturedTileStamp | null {
  const minX = Math.min(start.x, end.x);
  const minY = Math.min(start.y, end.y);
  const maxX = Math.max(start.x, end.x);
  const maxY = Math.max(start.y, end.y);
  const widthTiles = maxX - minX + 1;
  const heightTiles = maxY - minY + 1;
  if (widthTiles * heightTiles > MAX_CAPTURED_STAMP_TILES) return null;

  const tileImageIds: number[][] = [];
  let populatedTiles = 0;
  for (let y = minY; y <= maxY; y++) {
    const row: number[] = [];
    for (let x = minX; x <= maxX; x++) {
      const tileImageId = getTileImageIdAt(x, y);
      row.push(tileImageId);
      if (tileImageId > 0) populatedTiles++;
    }
    tileImageIds.push(row);
  }

  return { widthTiles, heightTiles, tileImageIds, populatedTiles };
}
