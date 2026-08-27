export function canPresentTileManager(isLocalDevelopment: boolean, gmLevel: number): boolean {
  return isLocalDevelopment && gmLevel > 0;
}
