import styled, { keyframes } from "styled-components";
import ActionButton from "./Interface/ActionButton";

const activity = keyframes`
  from { transform: translateX(-110%); }
  to { transform: translateX(390%); }
`;

const Wrapper = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  grid-area: 1 / 1 / -1 / -1;
  margin: 0;
  z-index: 999999;
  background-color: #101329;
  background-image: url("/assets/animebgfull.jpg");
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
  pointer-events: auto;
`;

const LoadingContainer = styled.div`
  position: absolute;
  bottom: clamp(24px, 10vh, 80px);
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 16px;
  width: min(560px, calc(100% - 32px));
  box-sizing: border-box;
  padding: 24px;
  color: #ffffff;
  text-align: center;
  background: rgba(16, 19, 41, 0.94);
  border: 1px solid rgba(192, 193, 255, 0.6);
  border-radius: 20px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.4);
`;

const LoadingTitle = styled.h2`
  margin: 0;
  font-family: "Outfit", sans-serif;
  font-size: clamp(22px, 4vw, 28px);
  font-weight: 800;
`;

const LoadingText = styled.p`
  margin: 0;
  font-family: "Outfit", sans-serif;
  font-size: 16px;
  line-height: 1.5;
  overflow-wrap: anywhere;
`;

const LoadingBarContainer = styled.div`
  width: 100%;
  height: 12px;
  background: rgba(192, 193, 255, 0.16);
  border: 1px solid #6263b7;
  border-radius: 12px;
  overflow: hidden;
`;

const LoadingBarFill = styled.div.attrs<{ $progress: number; $indeterminate: boolean }>(props => ({
  style: { width: props.$indeterminate ? "30%" : `${props.$progress}%` },
}))<{ $progress: number; $indeterminate: boolean }>`
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #a7edfe, #c0c1ff);
  animation: ${({ $indeterminate }) => $indeterminate ? activity : "none"} 1.6s ease-in-out infinite;
  transition: width 0.3s ease-out;

  @media (prefers-reduced-motion: reduce) {
    animation: none;
    width: 100% !important;
    opacity: 0.75;
  }
`;

interface LoadingScreenProps {
  message?: string;
  progress?: number;
  isIndeterminate?: boolean;
  isGlobal?: boolean;
  onRetry?: () => void;
}

const LoadingScreen = ({
  message = "Preparing your adventure...",
  progress = 0,
  isIndeterminate = false,
  isGlobal = false,
  onRetry,
}: LoadingScreenProps) => {
  const boundedProgress = Math.max(0, Math.min(100, progress));
  return (
    <Wrapper
      role="status"
      aria-live="polite"
      aria-busy={!onRetry}
      data-testid={isGlobal ? "world-loading-screen" : undefined}
    >
      <LoadingContainer>
        <LoadingTitle>{onRetry ? "Loading interrupted" : isGlobal ? "Loading the world" : "Loading"}</LoadingTitle>
        <LoadingText>{message}</LoadingText>
        {!onRetry && (
          <LoadingBarContainer
            role="progressbar"
            aria-label="Loading progress"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={isIndeterminate ? undefined : boundedProgress}
          >
            <LoadingBarFill $progress={boundedProgress} $indeterminate={isIndeterminate} />
          </LoadingBarContainer>
        )}
        {onRetry && <ActionButton text="Retry" onClick={onRetry} />}
      </LoadingContainer>
    </Wrapper>
  );
};

export default LoadingScreen;
