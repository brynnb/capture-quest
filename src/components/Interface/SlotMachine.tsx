import React, { useEffect, useRef, useCallback, useState } from "react";
import styled, { keyframes, css } from "styled-components";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import {
  WHEELS,
  WHEEL_SIZE,
  getVisibleSymbols,
  SYMBOL_DISPLAY,
  PAYOUT_TABLE,
  SlotSymbol,
} from "@/game/SlotMachineEngine";
import {
  playSlotMachine,
  requestCoinBalance,
} from "@/phaser-game/services/PhaserNetworkService";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import { GameFrameOverlay } from "@/components/Interface/GameFrameOverlay";

const SPIN_INTERVAL_MS = 60;
const NUM_CHASE_LIGHTS = 24;
const NUM_WIN_PARTICLES = 30;

// --- Chase lights around the cabinet border ---
const chaseAnim = keyframes`
  0% { opacity: 0.3; transform: scale(0.8); }
  50% { opacity: 1; transform: scale(1.2); filter: brightness(1.5); }
  100% { opacity: 0.3; transform: scale(0.8); }
`;

const ChaseLight = styled.div<{ $index: number; $total: number; $winning: boolean }>`
  position: absolute;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: ${(p) => {
    const colors = ["#ff4444", "#ffd700", "#44ff44", "#4488ff", "#ff44ff", "#ffd700"];
    return p.$winning ? "#ffd700" : colors[p.$index % colors.length];
  }};
  box-shadow: ${(p) => p.$winning
    ? "0 0 12px #ffd700, 0 0 24px #ffd700"
    : `0 0 6px ${["#ff4444", "#ffd700", "#44ff44", "#4488ff", "#ff44ff", "#ffd700"][p.$index % 6]}`};
  animation: ${chaseAnim} ${(p) => p.$winning ? "0.3s" : "1.2s"} ease-in-out infinite;
  animation-delay: ${(p) => (p.$index / p.$total) * (p.$winning ? 0.3 : 1.2)}s;
  z-index: 11;
  pointer-events: none;
`;

// --- Lever (pull toward viewer: shaft shrinks, ball grows) ---
const armShrink = keyframes`
  0% { height: 140px; }
  30% { height: 60px; }
  100% { height: 140px; }
`;

const ballPull = keyframes`
  0% { transform: scale(1); }
  30% { transform: scale(1.35); }
  100% { transform: scale(1); }
`;

const LeverContainer = styled.div`
  position: absolute;
  right: -60px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  height: 200px;
  cursor: pointer;
  z-index: 12;
`;

const LeverArm = styled.div<{ $pulling: boolean }>`
  width: 12px;
  height: 140px;
  background: linear-gradient(90deg, #c0c0c0, #e8e8e8, #c0c0c0);
  border-radius: 6px;
  border: 2px solid #888;
  box-shadow: 2px 2px 8px rgba(0,0,0,0.3);
  ${(p) => p.$pulling && css`animation: ${armShrink} 0.6s ease-out;`}
`;

const LeverBall = styled.div<{ $pulling: boolean }>`
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: radial-gradient(circle at 35% 35%, #ff6666, #cc0000, #880000);
  border: 3px solid #ffd700;
  box-shadow: 0 4px 12px rgba(0,0,0,0.4), inset 0 -4px 8px rgba(0,0,0,0.3);
  margin-bottom: -4px;
  z-index: 1;
  ${(p) => p.$pulling
    ? css`animation: ${ballPull} 0.6s ease-out;`
    : css`
      transition: transform 0.1s;
      &:hover { transform: scale(1.1); }
    `}
`;

const LeverBase = styled.div`
  width: 30px;
  height: 20px;
  background: linear-gradient(180deg, #888, #666);
  border-radius: 0 0 8px 8px;
  border: 2px solid #555;
`;

// --- Win celebration ---
const coinFall = keyframes`
  0% { transform: translateY(-40px) rotate(0deg) scale(0); opacity: 0; }
  15% { opacity: 1; transform: translateY(0) rotate(45deg) scale(1); }
  100% { transform: translateY(600px) rotate(720deg) scale(0.5); opacity: 0; }
`;

const CoinParticle = styled.div<{ $delay: number; $x: number; $size: number }>`
  position: absolute;
  top: 0;
  left: ${(p) => p.$x}%;
  width: ${(p) => p.$size}px;
  height: ${(p) => p.$size}px;
  border-radius: 50%;
  background: radial-gradient(circle at 35% 35%, #ffe066, #ffd700, #cc9900);
  border: 2px solid #b8860b;
  box-shadow: 0 0 8px rgba(255, 215, 0, 0.6);
  animation: ${coinFall} 2s ease-in forwards;
  animation-delay: ${(p) => p.$delay}s;
  pointer-events: none;
  z-index: 20;
  &::after {
    content: "";
    position: absolute;
    top: 25%;
    left: 25%;
    width: 50%;
    height: 50%;
    border-radius: 50%;
    border: 1px solid #b8860b;
  }
`;

const winGlow = keyframes`
  0%, 100% { box-shadow: 0 0 20px rgba(255, 215, 0, 0.3), inset 0 0 20px rgba(255, 215, 0, 0.1); }
  50% { box-shadow: 0 0 50px rgba(255, 215, 0, 0.7), inset 0 0 40px rgba(255, 215, 0, 0.3); }
`;

const WinBanner = styled.div`
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 48px;
  font-weight: bold;
  color: #ffd700;
  text-shadow: 0 0 20px #ffd700, 0 0 40px #ff8c00, 2px 2px 0 #8b0000;
  z-index: 25;
  pointer-events: none;
  animation: ${keyframes`
    0% { transform: translate(-50%, -50%) scale(0) rotate(-10deg); opacity: 0; }
    50% { transform: translate(-50%, -50%) scale(1.3) rotate(3deg); opacity: 1; }
    100% { transform: translate(-50%, -50%) scale(1) rotate(0deg); opacity: 1; }
  `} 0.5s ease-out;
`;

const sparkle = keyframes`
  0%, 100% { text-shadow: 2px 2px 0 rgba(255,255,255,0.4); }
  25% { text-shadow: 2px 2px 0 rgba(255,255,255,0.4), 0 0 20px rgba(255,215,0,0.6); }
  50% { text-shadow: 2px 2px 0 rgba(255,255,255,0.4), 0 0 40px rgba(255,215,0,0.8), 0 0 60px rgba(255,215,0,0.4); }
  75% { text-shadow: 2px 2px 0 rgba(255,255,255,0.4), 0 0 20px rgba(255,215,0,0.6); }
`;

const Cabinet = styled.div<{ $winning: boolean }>`
  width: 840px;
  background: linear-gradient(180deg, #c41e3a 0%, #8b0000 100%);
  border: 6px solid #ffd700;
  border-radius: 20px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.6), inset 0 3px 6px rgba(255, 255, 255, 0.2);
  font-family: "Outfit", monospace, sans-serif;
  overflow: visible;
  user-select: none;
  position: relative;
  ${(p) => p.$winning && css`animation: ${winGlow} 0.8s ease-in-out infinite;`}
`;

const Header = styled.div<{ $winning: boolean }>`
  background: linear-gradient(180deg, #ffd700 0%, #daa520 100%);
  border-bottom: 4px solid #8b6914;
  padding: 16px 32px;
  text-align: center;
  font-size: 36px;
  font-weight: bold;
  color: #8b0000;
  letter-spacing: 4px;
  border-radius: 14px 14px 0 0;
  ${(p) => p.$winning
    ? css`animation: ${sparkle} 1s ease-in-out infinite;`
    : css`text-shadow: 2px 2px 0 rgba(255, 255, 255, 0.4);`}
`;

const CoinDisplay = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 32px;
  font-size: 26px;
  color: #ffd700;
  font-weight: bold;
`;

const CoinBox = styled.div`
  background: #1a1a2e;
  border: 3px solid #ffd700;
  border-radius: 8px;
  padding: 8px 24px;
  min-width: 160px;
  text-align: center;
`;

const reelGlow = keyframes`
  0%, 100% { border-color: #ffd700; }
  50% { border-color: #fff; box-shadow: inset 0 0 30px rgba(255,215,0,0.3); }
`;

const ReelWindow = styled.div<{ $winning: boolean }>`
  display: flex;
  justify-content: center;
  gap: 8px;
  padding: 16px 32px;
  background: #1a1a2e;
  margin: 0 32px;
  border: 4px solid #ffd700;
  border-radius: 14px;
  position: relative;
  ${(p) => p.$winning && css`animation: ${reelGlow} 0.6s ease-in-out infinite;`}
`;

const ReelColumn = styled.div<{ $stopped: boolean }>`
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 200px;
  cursor: ${(p) => (p.$stopped ? "default" : "pointer")};
  transition: opacity 0.1s;
  &:hover {
    opacity: ${(p) => (p.$stopped ? 1 : 0.85)};
  }
`;

const flash = keyframes`
  0%, 100% { background: #2a2a4e; }
  50% { background: #4a4a8e; }
`;

const SymbolCell = styled.div<{ $highlight: boolean; $isMiddle: boolean }>`
  width: 180px;
  height: 110px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 60px;
  background: ${(p) => (p.$highlight ? "#4a4a8e" : "#2a2a4e")};
  border: 3px solid ${(p) => (p.$isMiddle ? "#ffd700" : "#444")};
  border-radius: 8px;
  margin: 3px 0;
  ${(p) =>
    p.$highlight &&
    css`
      animation: ${flash} 0.5s ease-in-out infinite;
    `}
`;

const LineIndicators = styled.div`
  position: absolute;
  left: 8px;
  top: 16px;
  bottom: 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-around;
  align-items: center;
  width: 20px;
`;

const LineDot = styled.div<{ $active: boolean }>`
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: ${(p) => (p.$active ? "#ffd700" : "#555")};
  box-shadow: ${(p) => (p.$active ? "0 0 8px #ffd700" : "none")};
`;

const Controls = styled.div`
  display: flex;
  justify-content: center;
  gap: 16px;
  padding: 20px 32px;
`;

const pulseGreen = keyframes`
  0%, 100% { box-shadow: 0 0 8px rgba(34, 139, 34, 0.4); }
  50% { box-shadow: 0 0 20px rgba(34, 139, 34, 0.8), 0 0 40px rgba(34, 139, 34, 0.3); }
`;

const Button = styled.button<{ $variant?: string }>`
  padding: 16px 40px;
  font-size: 26px;
  font-weight: bold;
  font-family: "Outfit", monospace, sans-serif;
  border: 3px solid #ffd700;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s;
  color: white;
  background: ${(p) =>
    p.$variant === "spin"
      ? "linear-gradient(180deg, #228b22 0%, #006400 100%)"
      : p.$variant === "stop"
        ? "linear-gradient(180deg, #cc7722 0%, #8b4513 100%)"
        : "linear-gradient(180deg, #555 0%, #333 100%)"};
  ${(p) => p.$variant === "spin" && css`animation: ${pulseGreen} 2s ease-in-out infinite;`}
  &:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  }
  &:active:not(:disabled) {
    transform: translateY(1px);
  }
  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    animation: none;
  }
`;

const BetSelector = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 32px 8px;
  justify-content: center;
  color: #ffd700;
  font-size: 24px;
  font-weight: bold;
`;

const BetButton = styled.button<{ $active: boolean }>`
  width: 64px;
  height: 48px;
  font-size: 24px;
  font-weight: bold;
  font-family: "Outfit", monospace, sans-serif;
  border: 3px solid ${(p) => (p.$active ? "#ffd700" : "#666")};
  border-radius: 8px;
  background: ${(p) => (p.$active ? "#ffd700" : "#333")};
  color: ${(p) => (p.$active ? "#8b0000" : "#999")};
  cursor: pointer;
  &:hover {
    border-color: #ffd700;
  }
`;

const PayoutTable = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 24px;
  padding: 12px 32px 20px;
  font-size: 20px;
  color: #ddd;
`;

const PayoutRow = styled.div`
  display: flex;
  justify-content: space-between;
  padding: 4px 8px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 4px;
`;

const MessageBar = styled.div<{ $winning: boolean }>`
  text-align: center;
  padding: 10px 32px;
  font-size: 22px;
  font-weight: bold;
  color: ${(p) => p.$winning ? "#ffd700" : "#ccc"};
  background: #1a1a2e;
  margin: 8px 32px;
  border-radius: 8px;
  border: 2px solid ${(p) => p.$winning ? "#ffd700" : "#333"};
  min-height: 32px;
  ${(p) => p.$winning && css`
    text-shadow: 0 0 10px #ffd700;
  `}
`;

const CloseButton = styled.button`
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 10;
  width: 48px;
  height: 48px;
  border: 3px solid #ffd700;
  border-radius: 50%;
  background: #8b0000;
  color: #ffd700;
  font-size: 24px;
  font-weight: bold;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.4);
  &:hover {
    background: #c41e3a;
    transform: scale(1.1);
  }
  transition: all 0.15s;
`;

// Server response shape from GameCornerSlotResultResponse
interface SlotResultEvent {
  detail: {
    success: boolean;
    reelPositions?: number[];
    payout?: number;
    matchLine?: string;
    coins?: number;
    bet?: number;
    error?: string;
  };
}

// Server response shape from GameCornerCoinBalanceResponse
interface CoinBalanceEvent {
  detail: {
    coins: number;
  };
}

const SlotMachine: React.FC = () => {
  const {
    isOpen,
    coins,
    bet,
    isSpinning,
    reelPositions,
    reelStopped,
    payout,
    message,
    matchLine,
    closeSlotMachine,
    setBet,
    isLuckyMachine,
  } = useSlotMachineStore();

  const spinTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const serverResultRef = useRef<SlotResultEvent["detail"] | null>(null);
  const cabinetRef = useRef<HTMLDivElement>(null);
  const [leverPulling, setLeverPulling] = useState(false);
  const [showWinCelebration, setShowWinCelebration] = useState(false);
  const [cabinetHeight, setCabinetHeight] = useState(700);
  const winTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Measure actual cabinet height for chase lights
  useEffect(() => {
    if (isOpen && cabinetRef.current) {
      setCabinetHeight(cabinetRef.current.offsetHeight);
    }
  }, [isOpen, payout]);

  const cleanup = useCallback(() => {
    if (spinTimerRef.current) {
      clearInterval(spinTimerRef.current);
      spinTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    return cleanup;
  }, [cleanup]);

  // Request coin balance when opening
  useEffect(() => {
    if (isOpen) {
      requestCoinBalance();
    }
  }, [isOpen]);

  // Listen for server coin balance responses
  useEffect(() => {
    const handler = (e: Event) => {
      const evt = e as unknown as CoinBalanceEvent;
      if (evt.detail?.coins !== undefined) {
        useSlotMachineStore.getState().setCoins(evt.detail.coins);
      }
    };
    window.addEventListener("gameCornerCoinBalance", handler);
    return () => window.removeEventListener("gameCornerCoinBalance", handler);
  }, []);

  // Listen for server slot result responses
  useEffect(() => {
    const handler = (e: Event) => {
      const evt = e as unknown as SlotResultEvent;
      const data = evt.detail;
      if (!data) return;

      const store = useSlotMachineStore.getState();

      if (!data.success) {
        // Error (e.g. not enough coins)
        useChatStore.getState().addMessage(data.error || "Error!", MessageType.SYSTEM);
        store.setMessage("");
        store.setIsSpinning(false);
        if (data.coins !== undefined) store.setCoins(data.coins);
        cleanup();
        return;
      }

      // Store the server result — the animation loop will use it
      serverResultRef.current = data;

      // Update coins immediately (server already deducted the bet)
      if (data.coins !== undefined) {
        // The server returns final coins (after payout), but during animation
        // we show the pre-payout amount. We'll update to final after animation.
        const prePayoutCoins = data.coins - (data.payout || 0);
        store.setCoins(prePayoutCoins >= 0 ? prePayoutCoins : 0);
      }
    };
    window.addEventListener("gameCornerSlotResult", handler);
    return () => window.removeEventListener("gameCornerSlotResult", handler);
  }, [cleanup]);

  // Spin animation loop — reels spin until server result arrives, then stop sequentially
  useEffect(() => {
    if (!isSpinning) {
      cleanup();
      return;
    }

    if (!spinTimerRef.current) {
      serverResultRef.current = null;
      let ticksSinceResult = 0;
      const TICKS_BETWEEN_STOPS = 8; // frames between each reel stopping

      spinTimerRef.current = setInterval(() => {
        const store = useSlotMachineStore.getState();
        const pos = [...store.reelPositions] as [number, number, number];
        const stopped = [...store.reelStopped] as [boolean, boolean, boolean];
        const result = serverResultRef.current;

        // If we have the server result, start stopping reels sequentially
        if (result && result.reelPositions) {
          ticksSinceResult++;

          for (let i = 0; i < 3; i++) {
            if (!stopped[i]) {
              // Stop this reel after enough ticks
              if (ticksSinceResult > (i + 1) * TICKS_BETWEEN_STOPS) {
                pos[i] = result.reelPositions[i];
                stopped[i] = true;
              } else {
                pos[i] = (pos[i] + 1) % WHEEL_SIZE;
              }
            }
          }
        } else {
          // No result yet — keep all reels spinning
          for (let i = 0; i < 3; i++) {
            if (!stopped[i]) {
              pos[i] = (pos[i] + 1) % WHEEL_SIZE;
            }
          }
        }

        store.setReelPositions(pos);
        if (stopped[0] !== store.reelStopped[0] ||
            stopped[1] !== store.reelStopped[1] ||
            stopped[2] !== store.reelStopped[2]) {
          store.setReelStopped(stopped);
        }

        // All stopped — show result
        if (stopped[0] && stopped[1] && stopped[2]) {
          if (spinTimerRef.current) {
            clearInterval(spinTimerRef.current);
            spinTimerRef.current = null;
          }

          if (result) {
            const payout = result.payout || 0;
            store.setPayout(payout);
            store.setMatchLine(result.matchLine || "");
            if (result.coins !== undefined) store.setCoins(result.coins);

            const chat = useChatStore.getState();
            if (payout > 0) {
              chat.addMessage(`Won ${payout} coins!`, MessageType.LOOT);
              store.setMessage(`Won ${payout} coins!`);
              setShowWinCelebration(true);
              if (winTimerRef.current) clearTimeout(winTimerRef.current);
              winTimerRef.current = setTimeout(() => setShowWinCelebration(false), 3000);
            } else {
              store.setMessage("Not this time...");
            }
          } else {
            store.setMessage("Not this time...");
          }
          store.setIsSpinning(false);
          serverResultRef.current = null;
        }
      }, SPIN_INTERVAL_MS);
    }
  }, [isSpinning, cleanup]);

  const handleSpin = useCallback(() => {
    if (isSpinning) return;
    const store = useSlotMachineStore.getState();
    if (store.coins < store.bet) {
      useChatStore.getState().addMessage("Not enough coins!", MessageType.SYSTEM);
      return;
    }
    // Start local animation
    setShowWinCelebration(false);
    setLeverPulling(true);
    setTimeout(() => setLeverPulling(false), 600);
    store.setIsSpinning(true);
    store.setReelStopped([false, false, false]);
    store.setPayout(0);
    store.setMatchLine("");
    store.setMessage("Spinning...");
    // Send request to server — result will arrive via event listener
    playSlotMachine(store.bet, store.isLuckyMachine);
  }, [isSpinning]);

  const handleClose = useCallback(() => {
    cleanup();
    closeSlotMachine();
  }, [cleanup, closeSlotMachine]);

  // Keyboard handler
  useEffect(() => {
    if (!isOpen) return;

    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (!isSpinning) handleClose();
        return;
      }
      if (e.key === " " || e.key === "Enter") {
        e.preventDefault();
        if (!isSpinning) {
          handleSpin();
        }
      }
    };

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isOpen, isSpinning, handleSpin, handleClose]);

  if (!isOpen) return null;

  // Get visible symbols for each reel
  const visibleSymbols = WHEELS.map((wheel, i) =>
    getVisibleSymbols(wheel, reelPositions[i]),
  );

  // Determine which line won for highlighting (using server-provided matchLine)
  const getHighlight = (reelIdx: number, rowIdx: number): boolean => {
    if (isSpinning || payout <= 0 || !matchLine) return false;
    const engineRow = 2 - rowIdx; // convert display row to engine row
    switch (matchLine) {
      case "middle":
        return engineRow === 1;
      case "top":
        return engineRow === 2;
      case "bottom":
        return engineRow === 0;
      case "diagonal-up":
        return (
          (reelIdx === 0 && engineRow === 0) ||
          (reelIdx === 1 && engineRow === 1) ||
          (reelIdx === 2 && engineRow === 2)
        );
      case "diagonal-down":
        return (
          (reelIdx === 0 && engineRow === 2) ||
          (reelIdx === 1 && engineRow === 1) ||
          (reelIdx === 2 && engineRow === 0)
        );
      default:
        return false;
    }
  };

  const isWinning = showWinCelebration && payout > 0;

  // Pre-compute chase light positions around the cabinet perimeter
  const chaseLightPositions = Array.from({ length: NUM_CHASE_LIGHTS }, (_, i) => {
    const t = i / NUM_CHASE_LIGHTS;
    const w = 840, h = cabinetHeight;
    const perimeter = 2 * (w + h);
    const dist = t * perimeter;
    let x = 0, y = 0;
    if (dist < w) { x = dist; y = -8; }
    else if (dist < w + h) { x = w + 2; y = dist - w; }
    else if (dist < 2 * w + h) { x = w - (dist - w - h); y = h + 2; }
    else { x = -8; y = h - (dist - 2 * w - h); }
    return { x, y };
  });

  // Pre-compute win particle data
  const winParticles = Array.from({ length: NUM_WIN_PARTICLES }, () => ({
    x: Math.random() * 100,
    delay: Math.random() * 0.8,
    size: 16 + Math.random() * 20,
  }));

  return (
    <GameFrameOverlay
      $tint="rgba(0, 0, 0, 0.6)"
      $zIndex={5000}
      data-testid="slot-machine-overlay"
      onClick={(e) => e.target === e.currentTarget && !isSpinning && handleClose()}
    >
      <Cabinet $winning={isWinning} ref={cabinetRef} data-testid="slot-machine">
        {/* Chase lights around the border */}
        {chaseLightPositions.map((pos, i) => (
          <ChaseLight
            key={i}
            $index={i}
            $total={NUM_CHASE_LIGHTS}
            $winning={isWinning}
            style={{ left: pos.x, top: pos.y }}
          />
        ))}

        {/* Lever on the right side */}
        <LeverContainer
          onClick={() => { if (!isSpinning && coins >= bet) handleSpin(); }}
          title="Pull to spin!"
        >
          <LeverBall $pulling={leverPulling} />
          <LeverArm $pulling={leverPulling} />
          <LeverBase />
        </LeverContainer>

        {/* Win celebration: coin rain */}
        {isWinning && winParticles.map((p, i) => (
          <CoinParticle key={i} $x={p.x} $delay={p.delay} $size={p.size} />
        ))}

        {/* Win banner */}
        {isWinning && (
          <WinBanner>
            +{payout} COINS!
          </WinBanner>
        )}

        <CloseButton onClick={handleClose} title="Close (Esc)">
          ✕
        </CloseButton>
        <Header $winning={isWinning}>
          GAME CORNER{isLuckyMachine ? " ★" : ""}
        </Header>

        <CoinDisplay>
          <CoinBox data-testid="slot-machine-coins">COINS: {coins}</CoinBox>
          <CoinBox data-testid="slot-machine-payout">PAYOUT: {payout}</CoinBox>
        </CoinDisplay>

        <ReelWindow $winning={isWinning}>
          <LineIndicators>
            <LineDot $active={bet >= 3} title="Diagonal" />
            <LineDot $active={bet >= 2} title="Top row" />
            <LineDot $active={true} title="Middle row" />
            <LineDot $active={bet >= 2} title="Bottom row" />
            <LineDot $active={bet >= 3} title="Diagonal" />
          </LineIndicators>
          {visibleSymbols.map((symbols, reelIdx) => (
            <ReelColumn
              key={reelIdx}
              $stopped={reelStopped[reelIdx]}
              onClick={() => {}}
            >
              {[2, 1, 0].map((engineRow, displayRow) => (
                <SymbolCell
                  key={displayRow}
                  $highlight={getHighlight(reelIdx, displayRow)}
                  $isMiddle={displayRow === 1}
                >
                  {SYMBOL_DISPLAY[symbols[engineRow]].emoji}
                </SymbolCell>
              ))}
            </ReelColumn>
          ))}
        </ReelWindow>

        <MessageBar $winning={isWinning}>
          {message || (isSpinning ? "Spinning..." : "Insert coins and pull the lever!")}
        </MessageBar>

        <BetSelector>
          BET:
          {[1, 2, 3].map((b) => (
            <BetButton
              key={b}
              data-testid={`slot-machine-bet-${b}`}
              $active={bet === b}
              onClick={() => setBet(b)}
              disabled={isSpinning}
            >
              ×{b}
            </BetButton>
          ))}
        </BetSelector>

        <Controls>
          <Button
            data-testid="slot-machine-spin"
            $variant="spin"
            onClick={handleSpin}
            disabled={isSpinning || coins < bet}
          >
            {isSpinning ? "SPINNING..." : `SPIN (${bet} coin${bet > 1 ? "s" : ""})`}
          </Button>
          <Button
            data-testid="slot-machine-quit"
            onClick={handleClose}
            disabled={isSpinning}
          >
            QUIT
          </Button>
        </Controls>

        <PayoutTable>
          {Object.entries(PAYOUT_TABLE).map(([sym, pay]) => {
            const s = Number(sym) as SlotSymbol;
            return (
              <PayoutRow key={sym}>
                <span>
                  {SYMBOL_DISPLAY[s].emoji} {SYMBOL_DISPLAY[s].name}
                </span>
                <span>{pay}</span>
              </PayoutRow>
            );
          })}
        </PayoutTable>
      </Cabinet>
    </GameFrameOverlay>
  );
};

export default SlotMachine;
