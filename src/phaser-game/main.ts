import { TileViewer } from "./scenes/TileViewer";
import Phaser from "phaser";

// Phaser game configuration
// Adapted for integration with React
export const phaserConfig: Phaser.Types.Core.GameConfig = {
  type: Phaser.AUTO,
  width: 1440,
  height: 1080,
  backgroundColor: "#000000",
  // Keep sprites and magnified pixels crisp. Composite world textures opt into
  // mipmapped minification separately so zoomed-out terrain does not shimmer.
  pixelArt: true,
  scale: {
    mode: Phaser.Scale.RESIZE,
    autoCenter: Phaser.Scale.CENTER_BOTH,
  },
  input: {
    // Phaser enables one touch pointer by default. Two are required for pinch
    // zoom; the mouse pointer is managed separately.
    activePointers: 2,
  },
  loader: {
    // Local overview PNGs come from the API port. Explicit CORS keeps their
    // textures origin-clean under the app's cross-origin isolation headers.
    crossOrigin: "anonymous",
  },
  // Using TileViewer - Go backend is now configured for tile data via WebTransport
  scene: [TileViewer],
};

// Factory function to create a new game instance
// This is called by the React component
export function createPhaserGame(parent: HTMLElement): Phaser.Game {
  return new Phaser.Game({
    ...phaserConfig,
    parent,
    width: Math.max(parent.clientWidth, 320),
    height: Math.max(parent.clientHeight, 320),
  });
}
