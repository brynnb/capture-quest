import React, { useEffect, useRef } from 'react';
import Phaser from 'phaser';
import { createPhaserGame } from './main';
import styled from 'styled-components';
import { clearAllHandlers } from './services/PhaserNetworkService';
import { cancelActiveCutscene } from './services/CutsceneService';

const GameContainer = styled.div`
  width: 1440px;
  height: 1080px;
  position: absolute;
  top: 0;
  left: 0;
  z-index: 0;

  canvas {
    display: block;
    width: 100%;
    height: 100%;
  }
`;

interface PhaserEngineProps {
    isActive?: boolean;
}

/**
 * React wrapper component for Phaser game engine
 * Handles initialization and cleanup of the Phaser game instance
 */
const PhaserEngine: React.FC<PhaserEngineProps> = ({ isActive = true }) => {
    const gameRef = useRef<Phaser.Game | null>(null);
    const containerRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (!isActive) {
            // Destroy game if it exists and we're not active
            if (gameRef.current) {
                cancelActiveCutscene('phaser inactive');
                gameRef.current.destroy(true);
                gameRef.current = null;
            }
            return;
        }

        // Only create game if we don't have one
        if (!gameRef.current && containerRef.current) {
            console.log('[PhaserEngine] Initializing Phaser game');
            gameRef.current = createPhaserGame();
        }

        // Cleanup on unmount
        return () => {
            if (gameRef.current) {
                console.log('[PhaserEngine] Destroying Phaser game');
                cancelActiveCutscene('phaser destroy');
                // Clear all handlers synchronously to prevent crashes from late network messages
                clearAllHandlers();
                gameRef.current.destroy(true);
                gameRef.current = null;
            }
        };
    }, [isActive]);

    if (!isActive) {
        return null;
    }

    return (
        <GameContainer
            ref={containerRef}
            id="phaser-game-container"
        />
    );
};

export default PhaserEngine;
