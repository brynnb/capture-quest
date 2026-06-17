import React, { useState, useCallback, useEffect } from "react";
import styled from "styled-components";
import { getTMHMMoveById } from "@/constants/tmhmMoveData";
import InventoryItemList from "./InventoryItemList";
import TMHMTeachModal from "./TMHMTeachModal";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";
import ActionButton from "@components/Interface/ActionButton";
import useGameStatusStore from "@stores/GameStatusStore";
import ItemInformationDisplay from "@components/Inventory/ItemInformationDisplay";

import StatInfoBar from "./StatInfoSidebar";
import PartyPokemonHUD, {
  type CursorItemUse,
  type PendingItemUse,
} from "@components/Interface/PartyPokemonHUD";
import useCQInventoryStore, { type CQInventoryItem } from "@/stores/CQInventoryStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import { WorldSocket, OpCodes } from "@/net";

const InventorySidebarContainer = styled.div`
  position: absolute;
  right: 0;
  top: 0;
  width: 272px;
  height: 1080px;
  background: rgba(192, 193, 255, 0.4);
  backdrop-filter: blur(10px);
  border-left: 4px solid #4a4ba6;
  box-shadow: -8px 0 32px rgba(0, 0, 0, 0.2);
  display: flex;
  flex-direction: column;
  z-index: 5;
`;

const inventoryPartyStyle: React.CSSProperties = {
  position: 'absolute',
  right: 15,
  top: 75,
  bottom: 'auto',
  width: 242,
};

const directUseShortNames = new Set([
  "BICYCLE",
  "ESCAPE_ROPE",
  "EXP_ALL",
  "GOOD_ROD",
  "ITEMFINDER",
  "MAX_REPEL",
  "OLD_ROD",
  "POKE_FLUTE",
  "POKEDEX",
  "REPEL",
  "SUPER_REPEL",
  "SUPER_ROD",
  "TOWN_MAP",
]);

function isDirectUseItem(item: CQInventoryItem): boolean {
  return directUseShortNames.has(item.item.shortName.toUpperCase());
}

const InventorySidebar: React.FC = () => {
  const { hoveredItem } = usePlayerCharacterStore();
  const { toggleInventory } = useGameStatusStore();
  const cqItems = useCQInventoryStore((s) => s.items);
  const pendingTMHM = useCQInventoryStore((s) => s.pendingTMHM);
  const setPendingTMHM = useCQInventoryStore((s) => s.setPendingTMHM);
  const party = usePokemonPartyStore((s) => s.party);
  const [pendingPPRestore, setPendingPPRestore] = useState<PendingItemUse | null>(null);
  const [selectedCQItem, setSelectedCQItem] = useState<CQInventoryItem | null>(null);
  const [selectedTMHMItem, setSelectedTMHMItem] = useState<CQInventoryItem | null>(null);

  useEffect(() => {
    if (
      selectedCQItem &&
      !cqItems.some(
        (item) =>
          item.instance.id === selectedCQItem.instance.id &&
          item.instance.quantity > 0,
      )
    ) {
      setSelectedCQItem(null);
    }
  }, [cqItems, selectedCQItem]);

  useEffect(() => {
    if (
      selectedTMHMItem &&
      !cqItems.some(
        (item) =>
          item.instance.id === selectedTMHMItem.instance.id &&
          item.instance.quantity > 0,
      )
    ) {
      setSelectedTMHMItem(null);
    }
  }, [cqItems, selectedTMHMItem]);

  useEffect(() => {
    if (!selectedCQItem) return;

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) {
        setSelectedCQItem(null);
        return;
      }
      if (target.closest("[data-cq-party-entry='true']")) {
        return;
      }
      setSelectedCQItem(null);
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setSelectedCQItem(null);
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [selectedCQItem]);

  // Merge PP restore (local) and TM/HM (store) into a single pending item for the HUD
  const activePending: PendingItemUse | null = pendingPPRestore
    ? pendingPPRestore
    : pendingTMHM
      ? { instanceId: pendingTMHM.instanceId, partySlot: pendingTMHM.partySlot, itemName: pendingTMHM.itemName }
      : null;

  const activeBannerText = pendingPPRestore
    ? `Use ${pendingPPRestore.itemName} \u2014 select a move`
    : pendingTMHM
      ? `${pendingTMHM.message}`
      : undefined;

  const handleItemUseOnPartySlot = useCallback((partySlot: number, instanceId: number) => {
    const inv = cqItems.find((i) => i.instance.id === instanceId);
    if (!inv) return;

    // PP restore and PP Up items need a move slot selection via the modal
    if (
      inv.item.ppRestore > 0 ||
      inv.item.shortName === "PP_UP" ||
      inv.item.shortName === "PP_UP_2"
    ) {
      setPendingPPRestore({
        instanceId: inv.instance.id,
        partySlot,
        itemName: inv.item.name,
      });
      setSelectedCQItem(null);
      return;
    }

    // TM/HM items: send with moveSlot -1 first; server will respond
    // with needsMoveSlot if 4 moves are known
    WorldSocket.sendJsonMessage(OpCodes.CQItemUseRequest, {
      instanceId: inv.instance.id,
      partySlot,
      moveSlot: -1,
    });
    setSelectedCQItem(null);
  }, [cqItems]);

  const handleCursorItemUse = useCallback((partySlot: number) => {
    if (!selectedCQItem) return;
    handleItemUseOnPartySlot(partySlot, selectedCQItem.instance.id);
  }, [selectedCQItem, handleItemUseOnPartySlot]);

  const handleCQItemSelect = useCallback((item: CQInventoryItem) => {
    if (isDirectUseItem(item)) {
      WorldSocket.sendJsonMessage(OpCodes.CQItemUseRequest, {
        instanceId: item.instance.id,
      });
      setSelectedCQItem(null);
      return;
    }

    if (selectedCQItem) {
      setSelectedCQItem(null);
      return;
    }
    setSelectedCQItem(item);
  }, [selectedCQItem]);

  const handleTMHMSelect = useCallback((item: CQInventoryItem) => {
    setSelectedCQItem(null);
    setSelectedTMHMItem(item);
  }, []);

  const handleTMHMPokemonSelected = useCallback((partySlot: number) => {
    if (!selectedTMHMItem) return;
    WorldSocket.sendJsonMessage(OpCodes.CQItemUseRequest, {
      instanceId: selectedTMHMItem.instance.id,
      partySlot,
      moveSlot: -1,
    });
    setSelectedTMHMItem(null);
  }, [selectedTMHMItem]);

  const handleMoveSlotSelected = useCallback((moveSlot: number) => {
    if (pendingPPRestore) {
      WorldSocket.sendJsonMessage(OpCodes.CQItemUseRequest, {
        instanceId: pendingPPRestore.instanceId,
        partySlot: pendingPPRestore.partySlot,
        moveSlot,
      });
      setPendingPPRestore(null);
      setSelectedCQItem(null);
    } else if (pendingTMHM) {
      const pokemon = party[pendingTMHM.partySlot];
      const forgottenMove = pokemon?.moves?.[moveSlot];
      if (forgottenMove) {
        const pokemonName = pokemon?.name?.toLowerCase() || "this Pokémon";
        const moveName = (pendingTMHM.moveName || pendingTMHM.itemName).toLowerCase();
        const oldMoveName = forgottenMove.name.toLowerCase();
        const confirmed = window.confirm(
          `${pokemonName} will forget ${oldMoveName} and learn ${moveName}.\n\nIs that OK?`,
        );
        if (!confirmed) {
          return;
        }
      }
      WorldSocket.sendJsonMessage(OpCodes.CQItemUseRequest, {
        instanceId: pendingTMHM.instanceId,
        partySlot: pendingTMHM.partySlot,
        moveSlot,
      });
      setPendingTMHM(null);
      setSelectedCQItem(null);
    }
  }, [party, pendingPPRestore, pendingTMHM, setPendingTMHM]);

  const handleCancelPending = useCallback(() => {
    setPendingPPRestore(null);
    setPendingTMHM(null);
  }, [setPendingTMHM]);

  const selectedTMHMMove = selectedTMHMItem
    ? getTMHMMoveById(selectedTMHMItem.item.moveId)
    : undefined;

  return (
    <>
      <InventorySidebarContainer />

      <PartyPokemonHUD
        containerStyle={inventoryPartyStyle}
        cursorItemUse={
          selectedCQItem
            ? ({
                instanceId: selectedCQItem.instance.id,
                itemName: selectedCQItem.item.name,
              } satisfies CursorItemUse)
            : null
        }
        onCursorItemUse={handleCursorItemUse}
        pendingItemUse={activePending}
        pendingBannerText={activeBannerText}
        onMoveSlotSelected={handleMoveSlotSelected}
        onCancelPendingItem={handleCancelPending}
      />
      <StatInfoBar />

      <InventoryItemList
        selectedInstanceId={selectedCQItem?.instance.id ?? null}
        onItemSelect={handleCQItemSelect}
        onTMHMSelect={handleTMHMSelect}
        onClearSelection={() => setSelectedCQItem(null)}
      />

      {selectedTMHMItem && selectedTMHMMove && (
        <TMHMTeachModal
          inventoryItem={selectedTMHMItem}
          move={selectedTMHMMove}
          party={party}
          onChoosePokemon={handleTMHMPokemonSelected}
          onClose={() => setSelectedTMHMItem(null)}
        />
      )}

      <ActionButton
        text="Done"
        onClick={toggleInventory}
        customCSS={`position: absolute; bottom: 15px; right: 75px; z-index: 1000; width: 120px;`}
      />

      <ItemInformationDisplay
        item={hoveredItem}
        isVisible={true}
      />


    </>
  );
};

export default InventorySidebar;
