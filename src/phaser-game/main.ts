import { TileViewer } from "./scenes/TileViewer";
import Phaser from "phaser";

// Phaser game configuration
// Adapted for integration with React
export const phaserConfig: Phaser.Types.Core.GameConfig = {
  type: Phaser.AUTO,
  width: 1440, // Match capture-quest design width
  height: 1080, // Match capture-quest design height
  parent: "phaser-game-container",
  backgroundColor: "#000000",
  pixelArt: true, // Nearest-neighbor scaling, no antialias, roundPixels — eliminates tile tears
  scale: {
    mode: Phaser.Scale.NONE,
  },
  // Using TileViewer - Go backend is now configured for tile data via WebTransport
  scene: [TileViewer],
};

// Factory function to create a new game instance
// This is called by the React component
export function createPhaserGame(): Phaser.Game {
  return new Phaser.Game(phaserConfig);
}
