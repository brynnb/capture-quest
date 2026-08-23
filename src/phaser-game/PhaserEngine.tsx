import React, { useEffect, useRef } from 'react';
import Phaser from 'phaser';
import { createPhaserGame } from './main';
import styled from 'styled-components';
import { clearAllHandlers } from './services/PhaserNetworkService';
import { cancelActiveCutscene } from './services/CutsceneService';
import useGameStatusStore from '@/stores/GameStatusStore';

const GameContainer = styled.div<{ $sidebarCollapsed: boolean }>`
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: ${({ $sidebarCollapsed }) => ($sidebarCollapsed ? "64px" : "272px")};
  z-index: 0;
  overflow: hidden;
  background: #101329;
  transition: left 180ms ease;

  canvas {
    display: block;
    width: 100%;
    height: 100%;
    touch-action: none;
  }

  @media (max-width: 850px), (pointer: coarse) {
    left: 0;
    transition: none;
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
    const sidebarCollapsed = useGameStatusStore((state) => state.isHudSidebarCollapsed);

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
            gameRef.current = createPhaserGame(containerRef.current);
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

    useEffect(() => {
        if (!isActive || !containerRef.current || !gameRef.current) return;
        const container = containerRef.current;
        const resizeGame = () => {
            const width = Math.max(Math.round(container.clientWidth), 320);
            const height = Math.max(Math.round(container.clientHeight), 320);
            gameRef.current?.scale.resize(width, height);
        };
        const observer = new ResizeObserver(resizeGame);
        observer.observe(container);
        resizeGame();
        return () => observer.disconnect();
    }, [isActive, sidebarCollapsed]);

    if (!isActive) {
        return null;
    }

    return (
        <GameContainer
            ref={containerRef}
            id="phaser-game-container"
            $sidebarCollapsed={sidebarCollapsed}
        />
    );
};

export default PhaserEngine;
