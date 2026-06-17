import { StrictMode, Suspense, lazy, useEffect } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import Layout from "./pages/Layout";
import useGameStatusStore from "./stores/GameStatusStore";
import useGameScreenStore from "./stores/GameScreenStore";
import { WorldSocket } from "./net";
import { NetworkBridge } from "./net/NetworkBridge";

const TileUpscaleToolPage = lazy(() => import("./pages/TileUpscaleToolPage"));

const App = () => {
  const isTileUpscaleTool =
    window.location.pathname.replace(/\/+$/, "") === "/tile-upscale-tool";

  useEffect(() => {
    if (isTileUpscaleTool) return;

    // Initialize network bridge
    NetworkBridge.initialize();

    // Handle global disconnection by returning to the title screen.
    WorldSocket.onDisconnect = () => {
      console.log("Disconnected from the Pokémon world - returning to title");
      useGameScreenStore.getState().setScreen("title");
      useGameStatusStore.setState({ currentMap: null });
    };
  }, [isTileUpscaleTool]);

  if (isTileUpscaleTool) {
    return (
      <StrictMode>
        <Suspense fallback={null}>
          <TileUpscaleToolPage />
        </Suspense>
      </StrictMode>
    );
  }

  return (
    <StrictMode>
      <Layout />
    </StrictMode>
  );
};

const initializeStore = async () => {
  await useGameStatusStore.persist.rehydrate();

  createRoot(document.getElementById("root")!).render(<App />);
};

initializeStore();
