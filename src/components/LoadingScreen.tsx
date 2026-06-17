import { useEffect, useState } from "react";
import styled from "styled-components";
import { LoadingJokeUtil } from "@utils/getRandomLoadingJoke";

const Wrapper = styled.div<{ $isGlobal?: boolean }>`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: absolute;
  top: ${props => props.$isGlobal ? '15px' : '0'};
  left: ${props => props.$isGlobal ? '15px' : '0'};
  right: ${props => props.$isGlobal ? '15px' : '0'};
  bottom: ${props => props.$isGlobal ? '21px' : '0'}; // Matches bottom of MainContainer
  width: ${props => props.$isGlobal ? '1440px' : '100%'};
  height: ${props => props.$isGlobal ? '1080px' : '100%'};
  grid-area: 1 / 1 / -1 / -1;
  margin: 0;
  z-index: 999999;
  background-image: url("/assets/animebgfull.jpg");
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
  pointer-events: auto;
`;




const LoadingContainer = styled.div`
  position: absolute;
  bottom: 80px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 15px;
  width: 100%;
`;

const LoadingBarContainer = styled.div`
  width: 700px;
  height: 48px;
  background: rgba(46, 47, 102, 0.4); /* Deep navy base */
  backdrop-filter: blur(8px);
  border: 4px solid #4a4ba6; /* Navy border */
  border-radius: 24px;
  overflow: hidden;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
  position: relative;
`;

const LoadingText = styled.div`
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 100%;
  text-align: center;
  font-family: 'Outfit', sans-serif;
  font-size: 20px;
  font-weight: 800;
  color: #ffffff;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.5);
  letter-spacing: 0.5px;
  z-index: 2;
  white-space: nowrap;
`;

const LoadingBarFill = styled.div.attrs<{ $progress: number }>(props => ({
  style: {
    width: `${props.$progress}%`,
  },
})) <{ $progress: number }>`
  height: 100%;
  background: linear-gradient(90deg, #a7edfe, #c0c1ff); /* Pastel Blue to Purple gradient */
  transition: width 0.3s ease-out;
  box-shadow: 0 0 20px rgba(167, 237, 254, 0.6);
  z-index: 1;
`;

interface LoadingScreenProps {
  /** Optional message (currently unused - loading jokes displayed instead) */
  message?: string;
  progress?: number;
  isIndeterminate?: boolean;
  isGlobal?: boolean;
}

const LoadingScreen = ({
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  message: _message,
  progress = 0,
  isIndeterminate = false,
  isGlobal = false,
}: LoadingScreenProps) => {

  const [animatedProgress, setAnimatedProgress] = useState(0);
  const [loadingJoke, setLoadingJoke] = useState(() =>
    LoadingJokeUtil.getRandomLoadingJoke()
  );
  const [dots, setDots] = useState("");

  // Rotate through random jokes every 3 seconds
  useEffect(() => {
    const jokeInterval = setInterval(() => {
      setLoadingJoke(LoadingJokeUtil.getRandomLoadingJoke());
    }, 3000);

    return () => clearInterval(jokeInterval);
  }, []);

  // Animate the ellipsis
  useEffect(() => {
    const dotsInterval = setInterval(() => {
      setDots((prev) => (prev.length >= 3 ? "" : prev + "."));
    }, 400);

    return () => clearInterval(dotsInterval);
  }, []);

  useEffect(() => {
    if (isIndeterminate) {
      // Animate progress bar back and forth for indeterminate loading
      const interval = setInterval(() => {
        setAnimatedProgress((prev) => {
          if (prev >= 100) return 0;
          return prev + 2;
        });
      }, 50);
      return () => clearInterval(interval);
    } else {
      setAnimatedProgress(progress);
    }
  }, [progress, isIndeterminate]);

  return (
    <Wrapper $isGlobal={isGlobal}>
      <LoadingContainer>

        <LoadingBarContainer>
          <LoadingBarFill $progress={animatedProgress} />
          <LoadingText>
            {loadingJoke}
            {dots}
          </LoadingText>
        </LoadingBarContainer>
      </LoadingContainer>
    </Wrapper>
  );
};

export default LoadingScreen;
