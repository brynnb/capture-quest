import { useEffect, ReactNode } from "react";
import useStaticDataStore from "@stores/StaticDataStore";
import useGameScreenStore from "@stores/GameScreenStore";
import { WorldSocket } from "@/net";
import LoadingScreen from "./LoadingScreen";

interface StaticDataGateProps {
  children: ReactNode;
  fallback?: ReactNode;
}

const StaticDataGate = ({ children, fallback }: StaticDataGateProps) => {
  const setScreen = useGameScreenStore((state) => state.setScreen);
  const {
    isLoaded,
    isLoading,
    loadStaticData,
    areModelsPreloaded,
    setModelsPreloaded,
  } = useStaticDataStore();

  useEffect(() => {
    // If not connected, return to the title screen.
    if (!WorldSocket.isConnected) {
      setScreen("title");
      return;
    }

    if (!isLoaded && !isLoading) {
      loadStaticData();
    }
  }, [isLoaded, isLoading, loadStaticData, setScreen]);

  // Mark models as preloaded immediately since we're using 2D now
  // The Phaser renderer will handle its own asset loading
  useEffect(() => {
    if (isLoaded && !areModelsPreloaded) {
      console.log("[StaticDataGate] 2D mode - marking assets as preloaded");
      setModelsPreloaded(true);
    }
  }, [isLoaded, areModelsPreloaded, setModelsPreloaded]);

  // Debug logging
  useEffect(() => {
    console.log("[StaticDataGate] State:", {
      isConnected: WorldSocket.isConnected,
      isLoaded,
      isLoading,
      areModelsPreloaded,
    });
  }, [isLoaded, isLoading, areModelsPreloaded]);

  // Simple check without minimum loading time - we already have enough loading indicators
  const isReady = WorldSocket.isConnected && isLoaded && areModelsPreloaded;

  if (!isReady) {
    if (!WorldSocket.isConnected) {
      return <LoadingScreen message="Connecting..." isIndeterminate />;
    }

    const message = !isLoaded ? "Loading Game Data..." : "Preparing...";

    return fallback ? (
      <>{fallback}</>
    ) : (
      <LoadingScreen
        message={message}
        progress={0}
        isIndeterminate={true}
      />
    );
  }

  return <>{children}</>;
};

export default StaticDataGate;
