import React, { useEffect, useState } from "react";
import styled from "styled-components";
import { getTMHMMoveById } from "@/constants/tmhmMoveData";
import useCQInventoryStore, {
  ITEM_TYPE_BATTLE_ITEM,
  ITEM_TYPE_EVOLUTION_STONE,
  ITEM_TYPE_HM,
  ITEM_TYPE_MEDICINE,
  ITEM_TYPE_POKEBALL,
  ITEM_TYPE_TM,
  type CQInventoryItem,
} from "@/stores/CQInventoryStore";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";
import { sendCQInventoryRequest } from "@/phaser-game/services/PhaserNetworkService";
import type { Item } from "@/entities/Item";

const FilterBar = styled.div`
  width: 232px;
  height: 24px;
  position: absolute;
  right: 20px;
  top: 540px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  z-index: 2500;
  pointer-events: auto;
`;

const FilterButton = styled.button<{ $active: boolean }>`
  border: 0;
  background: transparent;
  color: #ffffff;
  cursor: pointer;
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 8px;
  line-height: 1;
  padding: 4px 2px;
  text-decoration: ${(props) => (props.$active ? "underline" : "none")};
  text-underline-offset: 3px;
  text-shadow: 1px 1px 2px rgba(31, 32, 107, 0.9);

  &:hover {
    color: #ffffff;
  }
`;

const ItemListContainer = styled.div`
  width: 232px;
  height: 430px;
  background: rgba(192, 193, 255, 0.4);
  backdrop-filter: blur(10px);
  border: 4px solid #4a4ba6;
  border-radius: 20px;
  position: absolute;
  right: 20px;
  top: 570px;
  padding: 8px 0;
  box-sizing: border-box;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  z-index: 2500;
  overflow-y: auto;
  overscroll-behavior: contain;
  pointer-events: auto;

  scrollbar-width: thin;
  scrollbar-color: rgba(74, 75, 166, 0.4) transparent;

  &::-webkit-scrollbar {
    width: 6px;
  }
  &::-webkit-scrollbar-track {
    background: transparent;
  }
  &::-webkit-scrollbar-thumb {
    background-color: rgba(74, 75, 166, 0.4);
    border-radius: 3px;
  }
`;

const ItemRow = styled.div<{ $isSelected?: boolean }>`
  position: relative;
  display: flex;
  align-items: center;
  padding: 6px 14px;
  cursor: pointer;
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 11px;
  color: #2e2f66;
  transition: background 0.1s ease;
  gap: 8px;

  background: ${(props) =>
    props.$isSelected ? "rgba(255, 255, 255, 0.35)" : "transparent"};

  &:hover {
    background: rgba(255, 255, 255, 0.25);
  }

  &:active {
    background: rgba(255, 255, 255, 0.4);
  }
`;

const ItemName = styled.span`
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const ItemQuantity = styled.span`
  flex-shrink: 0;
  color: #4a4ba6;
  font-weight: 700;
`;

const MoveTooltip = styled.span`
  position: absolute;
  inset: 4px 8px;
  display: flex;
  align-items: center;
  background: rgba(31, 32, 107, 0.96);
  border: 1px solid rgba(255, 255, 255, 0.8);
  border-radius: 4px;
  color: #ffffff;
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 8px;
  line-height: 1.25;
  padding: 0 8px;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.1s ease;
  z-index: 2;

  ${ItemRow}:hover & {
    opacity: 1;
  }
`;

const EmptyMessage = styled.div`
  text-align: center;
  padding: 30px 14px;
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 10px;
  color: #6e6faa;
  opacity: 0.7;
`;

type InventoryFilter = "ALL" | "KEY" | "MOVES" | "STAT" | "BALLS";

const filters: InventoryFilter[] = ["ALL", "KEY", "MOVES", "STAT", "BALLS"];

const hiddenShortNames = new Set([
  "POKEDEX",
  "????",
  "?????",
]);

function isHiddenInventoryItem(invItem: CQInventoryItem): boolean {
  const shortName = invItem.item.shortName.toUpperCase();
  const name = invItem.item.name.toUpperCase();
  return (
    hiddenShortNames.has(shortName) ||
    /^\?+$/.test(shortName) ||
    /^\?+$/.test(name) ||
    shortName.includes("BADGE") ||
    name.includes("BADGE")
  );
}

function isMoveItem(invItem: CQInventoryItem): boolean {
  const item = invItem.item;
  return item.itemType === ITEM_TYPE_TM || item.itemType === ITEM_TYPE_HM;
}

function isStatItem(invItem: CQInventoryItem): boolean {
  const item = invItem.item;
  const shortName = item.shortName.toUpperCase();
  return (
    item.itemType === ITEM_TYPE_MEDICINE ||
    item.itemType === ITEM_TYPE_EVOLUTION_STONE ||
    item.itemType === ITEM_TYPE_BATTLE_ITEM ||
    item.healAmount > 0 ||
    !!item.statusCure ||
    item.revivePercent > 0 ||
    item.ppRestore > 0 ||
    shortName === "RARE_CANDY" ||
    shortName === "PP_UP" ||
    shortName === "PP_UP_2" ||
    ["HP_UP", "PROTEIN", "IRON", "CARBOS", "CALCIUM"].includes(shortName) ||
    item.bonusHp > 0 ||
    item.bonusAttack > 0 ||
    item.bonusDefense > 0 ||
    item.bonusSpeed > 0 ||
    item.bonusSpecial > 0 ||
    item.bonusAccuracy > 0 ||
    item.bonusEvasion > 0 ||
    item.bonusCrit > 0
  );
}

function isBallItem(invItem: CQInventoryItem): boolean {
  const item = invItem.item;
  return item.itemType === ITEM_TYPE_POKEBALL || item.ballModifier > 0;
}

function itemMatchesFilter(invItem: CQInventoryItem, filter: InventoryFilter): boolean {
  switch (filter) {
    case "KEY":
      return invItem.item.isKeyItem;
    case "MOVES":
      return isMoveItem(invItem);
    case "STAT":
      return isStatItem(invItem);
    case "BALLS":
      return isBallItem(invItem);
    case "ALL":
    default:
      return true;
  }
}

interface InventoryItemListProps {
  selectedInstanceId?: number | null;
  onItemSelect?: (item: CQInventoryItem) => void;
  onTMHMSelect?: (item: CQInventoryItem) => void;
  onClearSelection?: () => void;
}

const InventoryItemList: React.FC<InventoryItemListProps> = ({
  selectedInstanceId = null,
  onItemSelect,
  onTMHMSelect,
  onClearSelection,
}) => {
  const cqItems = useCQInventoryStore((s) => s.items);
  const { setHoveredItem } = usePlayerCharacterStore();
  const [activeFilter, setActiveFilter] = useState<InventoryFilter>("ALL");

  useEffect(() => {
    sendCQInventoryRequest();
  }, []);

  // Sort items alphabetically for a predictable inventory scan.
  const sortedItems = [...cqItems]
    .filter(
      (item) =>
        item.instance.quantity > 0 &&
        !isHiddenInventoryItem(item) &&
        itemMatchesFilter(item, activeFilter),
    )
    .sort((a, b) => {
      const nameCompare = a.item.name.localeCompare(b.item.name, undefined, {
        sensitivity: "base",
      });
      if (nameCompare !== 0) return nameCompare;
      if (a.item.id !== b.item.id) return a.item.id - b.item.id;
      return a.instance.id - b.instance.id;
    });

  const handleClick = (item: CQInventoryItem) => {
    if (isMoveItem(item) && onTMHMSelect) {
      onTMHMSelect(item);
      return;
    }
    if (onItemSelect) {
      onItemSelect(item);
    }
  };

  const renderItemRow = (invItem: CQInventoryItem) => {
    const move = getTMHMMoveById(invItem.item.moveId);
    const teachesMove = isMoveItem(invItem) && invItem.item.moveId;
    const moveTooltip = teachesMove ? move?.name : undefined;

    return (
      <ItemRow
        key={`item-${invItem.instance.id}`}
        $isSelected={selectedInstanceId === invItem.instance.id}
        title={moveTooltip}
        onPointerDown={(e) => {
          if (selectedInstanceId !== null && onItemSelect) {
            e.preventDefault();
            e.stopPropagation();
            onItemSelect(invItem);
          }
        }}
        onClick={(e) => {
          if (selectedInstanceId !== null && onItemSelect) {
            e.preventDefault();
            return;
          }
          handleClick(invItem);
        }}
        onMouseEnter={() => {
          const details = invItem.item;
          if (details) {
            const hoverItem: Item = {
              id: details.id,
              name: details.name,
              icon: details.icon,
              stackSize: details.stackSize,
            };
            setHoveredItem(hoverItem);
          }
        }}
        onMouseLeave={() => setHoveredItem(null)}
      >
        <ItemName>{invItem.item.name}</ItemName>
        {invItem.item.stackable && invItem.instance.quantity > 1 && (
          <ItemQuantity>&times;{invItem.instance.quantity}</ItemQuantity>
        )}
        {moveTooltip && <MoveTooltip>{moveTooltip}</MoveTooltip>}
      </ItemRow>
    );
  };

  return (
    <>
      <FilterBar data-cq-inventory-ui="true">
        {filters.map((filter) => (
          <FilterButton
            key={filter}
            $active={activeFilter === filter}
            onPointerDown={() => {
              if (selectedInstanceId !== null) {
                onClearSelection?.();
              }
            }}
            onClick={() => setActiveFilter(filter)}
          >
            {filter}
          </FilterButton>
        ))}
      </FilterBar>
      <ItemListContainer
        data-cq-inventory-ui="true"
        onWheel={(e) => {
          e.preventDefault();
          e.stopPropagation();
          e.currentTarget.scrollTop += e.deltaY;
        }}
      >
        {sortedItems.length === 0 ? (
          <EmptyMessage>No items</EmptyMessage>
        ) : (
          sortedItems.map(renderItemRow)
        )}
      </ItemListContainer>
    </>
  );
};

export default InventoryItemList;
