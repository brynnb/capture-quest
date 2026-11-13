import { WorldSocket } from "@/net";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import type { GameplayStateResponse } from "@/net/generated/world_api";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import { applyGameplayResourceSnapshot, readCurrentGameplayState } from "./GameplayRecoveryService";
import { runInventoryRequest, watchInteractionPosition } from "./InventoryCommandService";

// Coins and ordinary money are read from one owned snapshot. The caller's view
// can retire a read, but no balance packet is applied globally.
export function showCoinCaseBalance(instanceId: number): Promise<void> {
  if (!Number.isSafeInteger(instanceId) || instanceId <= 0 || !useGameStatusStore.getState().isInventoryOpen) return Promise.resolve();
  return runInventoryRequest<GameplayStateResponse>({
    mutation: false,
    read: readCurrentGameplayState,
    watchPresentation: retire => {
      const stopBag = useGameStatusStore.subscribe(state => { if (!state.isInventoryOpen) retire(); });
      const stopItem = useCQInventoryStore.subscribe(state => { if (!state.items.some(entry => entry.instance.id === instanceId)) retire(); });
      return () => { stopBag(); stopItem(); };
    },
    validate: snapshot => {
      if (!snapshot.inventory.some(entry => entry.instance.id === instanceId && entry.instance.quantity > 0 && entry.item.shortName.toUpperCase() === "COIN_CASE")) throw new Error("Coin Case is no longer owned");
    },
    apply: applyGameplayResourceSnapshot,
    present: snapshot => useChatStore.getState().addMessage(`You have ${snapshot.coins} coin${snapshot.coins === 1 ? "" : "s"}.`, MessageType.SYSTEM),
    readError: "Could not read your Coin Case. Please try again.",
  });
}

export async function refreshSlotCoinBalance(signal: AbortSignal): Promise<void> {
  const view = useSlotMachineStore.getState().openVersion;
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  const transport = WorldSocket.sessionGeneration;
  const source = useGameStatusStore.getState().playerTileContext;
  const current = () => {
    const position = useGameStatusStore.getState().playerTileContext;
    return !signal.aborted && useGameScreenStore.getState().currentScreen === "game"
      && usePlayerCharacterStore.getState().characterProfile.id === characterId && WorldSocket.sessionGeneration === transport
      && position?.mapId === source?.mapId && position?.x === source?.x && position?.y === source?.y
      && useSlotMachineStore.getState().isOpen && useSlotMachineStore.getState().openVersion === view;
  };
  if (!current() || useSlotMachineStore.getState().coinsLoading) return;
  let loaded = false;
  useSlotMachineStore.setState({ coinsLoading: true, coinsReady: false, message: "Checking your coins..." });
  try {
    await runInventoryRequest<GameplayStateResponse>({
      mutation: false,
      read: readCurrentGameplayState,
      watchPresentation: retire => {
        signal.addEventListener("abort", retire, { once: true });
        const stopPosition = watchInteractionPosition(retire);
        const stopPanel = useSlotMachineStore.subscribe(state => { if (!state.isOpen || state.openVersion !== view) retire(); });
        if (signal.aborted) retire();
        return () => { signal.removeEventListener("abort", retire); stopPosition(); stopPanel(); };
      },
      apply: applyGameplayResourceSnapshot,
      present: snapshot => {
        if (!current()) return;
        loaded = true;
        useSlotMachineStore.setState({ coins: snapshot.coins, coinsReady: true, coinsLoading: false, message: "Insert coins and pull the lever!" });
      },
      readError: "Could not read your coins. Please try again.",
    });
  } finally {
    if (current() && !loaded) useSlotMachineStore.setState({ coinsLoading: false, message: "Could not read your coins. Select RETRY." });
  }
}
