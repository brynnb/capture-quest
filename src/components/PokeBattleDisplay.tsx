import React, { useCallback, useEffect, useMemo, useState } from "react";
import styled, { keyframes } from "styled-components";
import usePokeBattleStore from "@stores/PokeBattleStore";
import type { BattleEvent } from "@stores/PokeBattleStore";
import useGameStatusStore from "@stores/GameStatusStore";
import useCQInventoryStore, {
  type CQInventoryItem,
  ITEM_TYPE_BATTLE_ITEM,
  ITEM_TYPE_MEDICINE,
} from "@stores/CQInventoryStore";
import type { PokemonDTO } from "@/net/generated/world_api";
import { WorldSocket, OpCodes } from "@/net";
import { sendCQInventoryRequest, sendSafariAction } from "@/phaser-game/services/PhaserNetworkService";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

// --- Type color map (Gen 1 palette-inspired) ---
const typeColors: Record<string, string> = {
  NORMAL: "#A8A878",
  FIRE: "#F08030",
  WATER: "#6890F0",
  ELECTRIC: "#F8D030",
  GRASS: "#78C850",
  ICE: "#98D8D8",
  FIGHTING: "#C03028",
  POISON: "#A040A0",
  GROUND: "#E0C068",
  FLYING: "#A890F0",
  PSYCHIC: "#F85888",
  BUG: "#A8B820",
  ROCK: "#B8A038",
  GHOST: "#705898",
  DRAGON: "#7038F8",
};

// --- Trainer class → sprite filename mapping ---
const trainerSpriteMap: Record<string, string> = {
  BUG_CATCHER: "bugcatcher",
  YOUNGSTER: "youngster",
  LASS: "lass",
  HIKER: "hiker",
  SUPER_NERD: "supernerd",
  POKEMANIAC: "pokemaniac",
  SAILOR: "sailor",
  BIKER: "biker",
  JR_TRAINER_M: "jr.trainerm",
  JR_TRAINER_F: "jr.trainerf",
  BEAUTY: "beauty",
  GENTLEMAN: "gentleman",
  SCIENTIST: "scientist",
  ROCKER: "rocker",
  JUGGLER: "juggler",
  TAMER: "tamer",
  BIRD_KEEPER: "birdkeeper",
  BLACKBELT: "blackbelt",
  PSYCHIC_TR: "psychic",
  CHANNELER: "channeler",
  ROCKET_GRUNT: "rocket",
  COOLTRAINER_M: "cooltrainerm",
  COOLTRAINER_F: "cooltrainerf",
  BURGLAR: "burglar",
  ENGINEER: "engineer",
  FISHER: "fisher",
  SWIMMER: "swimmer",
  CUE_BALL: "cueball",
  GAMBLER: "gambler",
  BROCK: "brock",
  MISTY: "misty",
  LT_SURGE: "lt.surge",
  ERIKA: "erika",
  KOGA: "koga",
  SABRINA: "sabrina",
  BLAINE: "blaine",
  GIOVANNI: "giovanni",
  LORELEI: "lorelei",
  BRUNO: "bruno",
  AGATHA: "agatha",
  LANCE: "lance",
};

function getTrainerSpritePath(trainerClass: string): string | null {
  const filename = trainerSpriteMap[trainerClass];
  return filename ? `/assets/trainers/${filename}.png` : null;
}

type BattleItemEntry = {
  kind: "ball" | "item";
  inventory: CQInventoryItem;
};

type BattleItemSection = {
  label: string;
  entries: BattleItemEntry[];
};

type MenuDirection = "up" | "down" | "left" | "right";

function getBattleItemSections(
  items: CQInventoryItem[],
  ballsOnly = false,
): BattleItemSection[] {
  const balls = items
    .filter((i) => i.item.ballModifier > 0 && i.instance.quantity > 0)
    .sort((a, b) => a.item.ballModifier - b.item.ballModifier)
    .map((inventory) => ({ kind: "ball" as const, inventory }));

  if (ballsOnly) {
    return [{ label: "Poké Balls", entries: balls }].filter(
      (section) => section.entries.length > 0,
    );
  }

  const medicine = items
    .filter(
      (i) =>
        i.item.isUsable &&
        i.item.ballModifier <= 0 &&
        i.instance.quantity > 0 &&
        i.item.itemType === ITEM_TYPE_MEDICINE &&
        (i.item.healAmount > 0 ||
          i.item.statusCure ||
          i.item.ppRestore > 0 ||
          i.item.revivePercent > 0),
    )
    .sort((a, b) => a.item.name.localeCompare(b.item.name))
    .map((inventory) => ({ kind: "item" as const, inventory }));

  const battleItems = items
    .filter(
      (i) =>
        i.item.isUsable &&
        i.item.ballModifier <= 0 &&
        i.instance.quantity > 0 &&
        (i.item.itemType === ITEM_TYPE_BATTLE_ITEM ||
          i.item.shortName === "POKE_FLUTE"),
    )
    .sort((a, b) => a.item.name.localeCompare(b.item.name))
    .map((inventory) => ({ kind: "item" as const, inventory }));

  return [
    { label: "Poké Balls", entries: balls },
    { label: "Medicine", entries: medicine },
    { label: "Battle Items", entries: battleItems },
  ].filter((section) => section.entries.length > 0);
}

function clampIndex(index: number, count: number): number {
  if (count <= 0) return 0;
  return Math.max(0, Math.min(count - 1, index));
}

function moveGridIndex(
  index: number,
  count: number,
  columns: number,
  direction: MenuDirection,
): number {
  if (count <= 1) return 0;
  const delta =
    direction === "left"
      ? -1
      : direction === "right"
        ? 1
        : direction === "up"
          ? -columns
          : columns;
  return clampIndex(index + delta, count);
}

function moveListIndex(
  index: number,
  count: number,
  direction: MenuDirection,
): number {
  if (count <= 1) return 0;
  const delta = direction === "up" || direction === "left" ? -1 : 1;
  return clampIndex(index + delta, count);
}

function firstSwitchablePartyIndex(
  party: PokemonDTO[],
  activeIndex: number,
): number {
  const index = party.findIndex(
    (pokemon, i) => i !== activeIndex && pokemon.curHp > 0,
  );
  return index >= 0 ? index : 0;
}

function keyboardDirection(event: KeyboardEvent): MenuDirection | null {
  const key = event.key.toLowerCase();
  if (key === "arrowup" || key === "w") return "up";
  if (key === "arrowdown" || key === "s") return "down";
  if (key === "arrowleft" || key === "a") return "left";
  if (key === "arrowright" || key === "d") return "right";
  return null;
}

function isKeyboardActivation(event: KeyboardEvent): boolean {
  return event.code === "Space" || event.key === " " || event.key === "Enter";
}

function isEditableKeyboardTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName.toLowerCase();
  return (
    tag === "input" ||
    tag === "textarea" ||
    tag === "select" ||
    target.isContentEditable
  );
}

// --- Styled Components ---

const BattleOverlay = styled.div`
  position: absolute;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  width: 480px;
  z-index: 2000;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  image-rendering: pixelated;
`;

const BattleWindow = styled.div`
  background: #f8f8f8;
  border: 4px solid #383838;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
`;

const BattleScene = styled.div`
  position: relative;
  height: 200px;
  background: linear-gradient(
    180deg,
    #88c8e8 0%,
    #88c8e8 60%,
    #a8d8a8 60%,
    #78b878 100%
  );
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
`;

const EnemySprite = styled.img`
  position: absolute;
  right: 40px;
  top: 20px;
  width: 96px;
  height: 96px;
  image-rendering: pixelated;
  filter: drop-shadow(2px 2px 0 rgba(0, 0, 0, 0.2));
`;

const PlayerSprite = styled.img`
  position: absolute;
  left: 40px;
  bottom: 10px;
  width: 96px;
  height: 96px;
  image-rendering: pixelated;
  filter: drop-shadow(2px 2px 0 rgba(0, 0, 0, 0.2));
`;

const EnemyInfo = styled.div`
  display: flex;
  flex-direction: column;
  align-self: flex-start;
  background: #f8f0e0;
  border: 2px solid #383838;
  border-radius: 4px;
  padding: 6px 10px;
  min-width: 200px;
`;

const PlayerInfo = styled.div`
  display: flex;
  flex-direction: column;
  align-self: flex-end;
  background: #f8f0e0;
  border: 2px solid #383838;
  border-radius: 4px;
  padding: 6px 10px;
  min-width: 220px;
`;

const PokemonNameRow = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
`;

const PokemonName = styled.span`
  font-size: 10px;
  color: #383838;
  text-transform: uppercase;
`;

const PokemonLevel = styled.span`
  font-size: 9px;
  color: #383838;
`;

const HPBarContainer = styled.div`
  display: flex;
  align-items: center;
  gap: 4px;
`;

const HPLabel = styled.span`
  font-size: 8px;
  color: #383838;
  font-weight: bold;
`;

const HPBarOuter = styled.div`
  flex: 1;
  height: 6px;
  background: #383838;
  border-radius: 3px;
  padding: 1px;
`;

const HPBarInner = styled.div`
  height: 100%;
  border-radius: 2px;
  transition:
    width 0.5s ease,
    background-color 0.5s ease;
`;

const HPText = styled.div`
  font-size: 8px;
  color: #383838;
  text-align: right;
  margin-top: 2px;
`;

const TextBox = styled.div`
  background: #f8f8f8;
  border-top: 4px solid #383838;
  padding: 12px 16px;
  min-height: 56px;
  display: flex;
  align-items: center;
  cursor: pointer;
  user-select: none;
`;

const MessageText = styled.p`
  font-size: 10px;
  color: #383838;
  line-height: 1.6;
  margin: 0;
  white-space: pre-line;
`;

const bounce = keyframes`
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-3px); }
`;

const TextArrow = styled.span`
  display: inline-block;
  margin-left: 8px;
  animation: ${bounce} 0.6s ease-in-out infinite;
  font-size: 10px;
`;

const ActionMenu = styled.div`
  background: #f8f8f8;
  border-top: 4px solid #383838;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
`;

const menuCursor = `
  &::before {
    content: ">";
    position: absolute;
    left: 8px;
    opacity: 0;
  }
`;

const selectedMenuCursor = `
  &::before {
    opacity: 1;
  }
`;

const ActionButton = styled.button<{ $selected?: boolean }>`
  position: relative;
  background: #f8f8f8;
  border: 2px solid #c8c8c8;
  padding: 10px 8px 10px 24px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 10px;
  color: #383838;
  cursor: pointer;
  text-transform: uppercase;
  transition: background 0.1s;

  &:hover {
    background: #e8e8e8;
  }

  &:active {
    background: #d0d0d0;
  }

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const MoveGrid = styled.div`
  background: #f8f8f8;
  border-top: 4px solid #383838;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
`;

const MoveButton = styled.button<{ $typeColor: string; $selected?: boolean }>`
  position: relative;
  background: ${(p) => p.$typeColor}22;
  border: 2px solid ${(p) => p.$typeColor};
  padding: 8px 6px 8px 22px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 8px;
  color: #383838;
  cursor: pointer;
  text-align: left;
  transition: background 0.1s;
  display: flex;
  flex-direction: column;
  gap: 2px;

  &:hover {
    background: ${(p) => p.$typeColor}44;
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  ${({ $selected, $typeColor }) =>
    $selected &&
    `
      background: ${$typeColor}44;
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const MovePP = styled.span`
  font-size: 7px;
  color: #686868;
`;

const MoveType = styled.span<{ $color: string }>`
  font-size: 7px;
  color: ${(p) => p.$color};
  font-weight: bold;
`;

const BackButton = styled.button<{ $selected?: boolean }>`
  position: relative;
  grid-column: 1 / -1;
  background: #f8f8f8;
  border: 2px solid #c8c8c8;
  padding: 6px 6px 6px 24px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 8px;
  color: #888;
  cursor: pointer;

  &:hover {
    background: #e8e8e8;
  }

  ${({ $selected }) =>
    $selected &&
    `
      background: #e8e8e8;
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

// --- Item Select Panel (item selection during battle) ---

const ItemGrid = styled.div`
  background: #f8f8f8;
  border-top: 4px solid #383838;
  display: flex;
  flex-direction: column;
  max-height: 260px;
  overflow: hidden;
`;

const ItemList = styled.div`
  flex: 1;
  overflow-y: auto;

  &::-webkit-scrollbar {
    width: 6px;
  }
  &::-webkit-scrollbar-thumb {
    background: #b0b0b0;
    border-radius: 3px;
  }
`;

const ItemCategoryLabel = styled.div`
  background: #e0e0e0;
  padding: 4px 16px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 7px;
  color: #686868;
  text-transform: uppercase;
  border-bottom: 1px solid #c8c8c8;
`;

const ItemButton = styled.button<{ $isBall?: boolean; $selected?: boolean }>`
  position: relative;
  background: ${(p) => (p.$isBall ? "#fff8f0" : "#f8f8f8")};
  border: none;
  border-bottom: 1px solid #e0e0e0;
  padding: 8px 16px 8px 30px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 9px;
  color: #383838;
  cursor: pointer;
  text-align: left;
  display: flex;
  justify-content: space-between;
  align-items: center;
  transition: background 0.1s;

  &:hover {
    background: ${(p) => (p.$isBall ? "#f8ece0" : "#e8e8e8")};
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  ${({ $selected, $isBall }) =>
    $selected &&
    `
      background: ${$isBall ? "#f8ece0" : "#e8e8e8"};
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const ItemQty = styled.span`
  font-size: 8px;
  color: #686868;
`;

const ItemEmptyMsg = styled.div`
  padding: 16px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 9px;
  color: #888;
  text-align: center;
`;

const ItemSelectPanel: React.FC<{
  sections: BattleItemSection[];
  selectedIndex: number;
  onUseBall: (itemId: number) => void;
  onUseItem: (itemId: number) => void;
  onBack: () => void;
}> = ({ sections, selectedIndex, onUseBall, onUseItem, onBack }) => {
  const hasItems = sections.length > 0;
  let itemIndex = 0;

  return (
    <ItemGrid>
      <ItemList>
        {!hasItems && <ItemEmptyMsg>No usable items!</ItemEmptyMsg>}

        {sections.map((section) => (
          <React.Fragment key={section.label}>
            <ItemCategoryLabel>{section.label}</ItemCategoryLabel>
            {section.entries.map((entry) => {
              const currentIndex = itemIndex;
              itemIndex += 1;
              return (
                <ItemButton
                  key={entry.inventory.instance.id}
                  $isBall={entry.kind === "ball"}
                  $selected={selectedIndex === currentIndex}
                  data-testid={`battle-item-${currentIndex}`}
                  data-selected={selectedIndex === currentIndex ? "true" : "false"}
                  onClick={() =>
                    entry.kind === "ball"
                      ? onUseBall(entry.inventory.item.id)
                      : onUseItem(entry.inventory.item.id)
                  }
                >
                  <span>{entry.inventory.item.name}</span>
                  <ItemQty>×{entry.inventory.instance.quantity}</ItemQty>
                </ItemButton>
              );
            })}
          </React.Fragment>
        ))}
      </ItemList>

      <BackButton
        $selected={selectedIndex === itemIndex}
        data-testid="battle-item-back"
        data-selected={selectedIndex === itemIndex ? "true" : "false"}
        onClick={onBack}
      >
        ← BACK
      </BackButton>
    </ItemGrid>
  );
};

// --- Faint Switch Panel (party select after player Pokémon faints) ---

const FaintSwitchHeader = styled.div`
  background: #e84040;
  padding: 6px 16px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 8px;
  color: #fff;
  text-align: center;
`;

const PartySlotButton = styled.button<{ $isActive?: boolean; $selected?: boolean }>`
  position: relative;
  background: ${(p) => (p.$selected ? "#e8e8e8" : p.$isActive ? "#d0d0d0" : "#f8f8f8")};
  border: none;
  border-bottom: 1px solid #e0e0e0;
  padding: 8px 16px 8px 30px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 9px;
  color: #383838;
  cursor: pointer;
  text-align: left;
  display: flex;
  justify-content: space-between;
  align-items: center;
  transition: background 0.1s;

  &:hover:not(:disabled) {
    background: #e8e8e8;
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    color: #a0a0a0;
  }

  ${({ $selected }) =>
    $selected &&
    `
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const PartySlotInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const PartySlotName = styled.span`
  font-size: 9px;
`;

const PartySlotHP = styled.span`
  font-size: 7px;
  color: #686868;
`;

const FaintRunButton = styled.button<{ $selected?: boolean }>`
  position: relative;
  background: #fff0f0;
  border: none;
  border-top: 2px solid #e84040;
  padding: 10px 16px 10px 30px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 9px;
  color: #e84040;
  cursor: pointer;
  text-align: center;
  transition: background 0.1s;

  &:hover {
    background: #ffe0e0;
  }

  ${({ $selected }) =>
    $selected &&
    `
      background: #ffe0e0;
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const FaintSwitchPanel: React.FC<{
  selectedIndex: number;
  onSwitch: (partyIndex: number) => void;
  onRun: () => void;
  onBack?: () => void;
  isWild: boolean;
  activeIndex: number;
  party: PokemonDTO[];
}> = ({ selectedIndex, onSwitch, onRun, onBack, isWild, activeIndex, party }) => {
  return (
    <ItemGrid>
      <FaintSwitchHeader>Choose a POKéMON!</FaintSwitchHeader>
      {party.map((poke, i) => {
        const isFainted = poke.curHp <= 0;
        const isCurrent = i === activeIndex;
        return (
          <PartySlotButton
            key={i}
            $isActive={isCurrent}
            $selected={selectedIndex === i}
            data-testid={`battle-party-${i}`}
            data-selected={selectedIndex === i ? "true" : "false"}
            disabled={isFainted || isCurrent}
            onClick={() => onSwitch(i)}
          >
            <PartySlotInfo>
              <PartySlotName>
                {poke.name} Lv{poke.level}
              </PartySlotName>
              <PartySlotHP>
                HP {poke.curHp}/{poke.maxHp}
                {isFainted ? " (FNT)" : ""}
              </PartySlotHP>
            </PartySlotInfo>
          </PartySlotButton>
        );
      })}
      {isWild && (
        <FaintRunButton
          $selected={selectedIndex === party.length}
          data-testid="battle-party-run"
          data-selected={selectedIndex === party.length ? "true" : "false"}
          onClick={onRun}
        >
          RUN
        </FaintRunButton>
      )}
      {onBack && (
        <BackButton
          $selected={selectedIndex === party.length + (isWild ? 1 : 0)}
          data-testid="battle-party-back"
          data-selected={
            selectedIndex === party.length + (isWild ? 1 : 0)
              ? "true"
              : "false"
          }
          onClick={onBack}
        >
          ← BACK
        </BackButton>
      )}
    </ItemGrid>
  );
};

// --- Move Learn Panel (choose which move to forget or skip) ---

const MoveLearnHeader = styled.div`
  background: #4088e0;
  padding: 6px 16px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 8px;
  color: #fff;
  text-align: center;
`;

const MoveLearnInfo = styled.div`
  padding: 8px 16px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 7px;
  color: #383838;
  background: #f0f8ff;
  border-bottom: 1px solid #d0d8e0;
  text-align: center;
`;

const MoveSlotButton = styled.button<{ $selected?: boolean }>`
  position: relative;
  background: #f8f8f8;
  border: none;
  border-bottom: 1px solid #e0e0e0;
  padding: 8px 16px 8px 30px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 9px;
  color: #383838;
  cursor: pointer;
  text-align: left;
  display: flex;
  justify-content: space-between;
  align-items: center;
  transition: background 0.1s;

  &:hover {
    background: #ffe0e0;
  }

  ${({ $selected }) =>
    $selected &&
    `
      background: #ffe0e0;
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const MoveSlotType = styled.span<{ $color: string }>`
  font-size: 7px;
  color: ${(p) => p.$color};
`;

const SkipLearnButton = styled.button<{ $selected?: boolean }>`
  position: relative;
  background: #f8f8f8;
  border: none;
  border-top: 2px solid #4088e0;
  padding: 10px 16px 10px 30px;
  font-family: "Press Start 2P", "Pokemon GB", monospace;
  font-size: 9px;
  color: #4088e0;
  cursor: pointer;
  text-align: center;
  transition: background 0.1s;

  &:hover {
    background: #e0f0ff;
  }

  ${({ $selected }) =>
    $selected &&
    `
      background: #e0f0ff;
      box-shadow: inset 0 0 0 2px #383838;
    `}

  ${menuCursor}
  ${({ $selected }) => $selected && selectedMenuCursor}
`;

const MoveLearnPanel: React.FC<{
  moveName: string;
  currentMoves: { name: string; type: string; pp: number; maxPp: number }[];
  selectedIndex: number;
  onForget: (slot: number) => void;
  onSkip: () => void;
}> = ({ moveName, currentMoves, selectedIndex, onForget, onSkip }) => {
  return (
    <ItemGrid>
      <MoveLearnHeader>Forget a move?</MoveLearnHeader>
      <MoveLearnInfo>
        Trying to learn {moveName}
      </MoveLearnInfo>
      {currentMoves.map((move, i) => {
        const color = typeColors[move.type] || "#A8A878";
        return (
          <MoveSlotButton
            key={i}
            $selected={selectedIndex === i}
            onClick={() => onForget(i)}
          >
            <PartySlotInfo>
              <PartySlotName>{move.name}</PartySlotName>
              <PartySlotHP>
                PP {move.pp}/{move.maxPp}
              </PartySlotHP>
            </PartySlotInfo>
            <MoveSlotType $color={color}>{move.type}</MoveSlotType>
          </MoveSlotButton>
        );
      })}
      <SkipLearnButton
        $selected={selectedIndex === currentMoves.length}
        onClick={onSkip}
      >
        Don&apos;t learn {moveName}
      </SkipLearnButton>
    </ItemGrid>
  );
};

// --- Component ---

const PokeBattleDisplay: React.FC = () => {
  const {
    isInBattle,
    phase,
    playerPokemon,
    enemyPokemon,
    eventQueue,
    currentEventIndex,
    battleResult,
    advanceEvent,
    closeBattle,
    setPhase,
    trainerClass,
    faintSwitchParty,
    faintSwitchActive,
    battleType,
    pendingMoveLearn,
    isSafari,
    safariBallsLeft,
    sentToPC,
    sentToPCBox,
    lossMessage,
    allowedActions,
    guaranteedCatch,
  } = usePokeBattleStore();

  const [displayMessage, setDisplayMessage] = useState("");
  const [selectedActionIndex, setSelectedActionIndex] = useState(0);
  const [selectedMoveIndex, setSelectedMoveIndex] = useState(0);
  const [selectedItemIndex, setSelectedItemIndex] = useState(0);
  const [selectedPokemonIndex, setSelectedPokemonIndex] = useState(0);
  const [selectedMoveLearnIndex, setSelectedMoveLearnIndex] = useState(0);
  const inventoryItems = useCQInventoryStore((s) => s.items);

  const battleItemSections = useMemo(
    () => getBattleItemSections(inventoryItems, guaranteedCatch),
    [guaranteedCatch, inventoryItems],
  );

  const battleItemEntries = useMemo(
    () => battleItemSections.flatMap((section) => section.entries),
    [battleItemSections],
  );

  const firstUsableMoveIndex = useMemo(() => {
    const moves = playerPokemon?.moves ?? [];
    const index = moves.findIndex((move) => move.pp > 0);
    return index >= 0 ? index : 0;
  }, [playerPokemon?.moves]);

  const isFaintSwitchWild = battleType === "wild" || trainerClass === null;
  const canRunFromFaintSwitch = phase === "faint_switch" && isFaintSwitchWild;
  const hasPokemonMenuBack = phase === "pokemon_select";
  const pokemonMenuCount =
    faintSwitchParty.length +
    (canRunFromFaintSwitch ? 1 : 0) +
    (hasPokemonMenuBack ? 1 : 0);

  // Auto-close battle on win (prize money message was the last thing shown)
  useEffect(() => {
    if (phase === "battle_end" && battleResult === "win") {
      closeBattle();
    }
  }, [phase, battleResult, closeBattle]);

  useEffect(() => {
    if (!isInBattle) return;

    switch (phase) {
      case "action_select":
        setSelectedActionIndex(0);
        break;
      case "move_select":
        setSelectedMoveIndex(firstUsableMoveIndex);
        break;
      case "item_select":
        setSelectedItemIndex(0);
        break;
      case "pokemon_select":
      case "faint_switch":
        setSelectedPokemonIndex(
          firstSwitchablePartyIndex(faintSwitchParty, faintSwitchActive),
        );
        break;
      case "move_learn_prompt":
        setSelectedMoveLearnIndex(0);
        break;
      default:
        break;
    }
  }, [
    battleItemEntries.length,
    faintSwitchActive,
    faintSwitchParty,
    firstUsableMoveIndex,
    isInBattle,
    phase,
    playerPokemon?.moves.length,
    pokemonMenuCount,
  ]);

  // Update display message when animating events
  useEffect(() => {
    if (phase === "animating" && eventQueue.length > 0) {
      const event = eventQueue[currentEventIndex];
      if (event) {
        setDisplayMessage(getEventMessage(event));
        if (event.type === "move_used" && event.moveSfx) {
          const sfxPath = sfxPathForConstant(event.moveSfx);
          if (sfxPath) {
            void AudioManager.playSFX(sfxPath, 0.75);
          } else {
            void AudioManager.playGeneratedSFX("battleStart", 0.25);
          }
        }
        // On evolution event, immediately update the player sprite to the evolved form
        if (event.type === "evolution" && event.evolvedSpeciesId) {
          const current = usePokeBattleStore.getState().playerPokemon;
          if (current && current.id !== event.evolvedSpeciesId) {
            usePokeBattleStore.setState({
              playerPokemon: {
                ...current,
                id: event.evolvedSpeciesId,
                name: event.evolvedName ?? current.name,
              },
            });
          }
        }
      }
    }
  }, [phase, eventQueue, currentEventIndex]);

  // Compute per-event HP snapshots so bars drain progressively.
  // Walk events up to currentEventIndex; any damage_dealt event with
  // targetHp/targetMaxHp updates the corresponding side's HP.
  const { targetPlayerHpPercent, targetEnemyHpPercent, targetDisplayPlayerHp } =
    useMemo(() => {
      if (!playerPokemon || !enemyPokemon) {
        return {
          targetPlayerHpPercent: 0,
          targetEnemyHpPercent: 0,
          targetDisplayPlayerHp: 0,
        };
      }

      // Start from the stored (pre-turn) HP
      let pHp = playerPokemon.curHp;
      let pMax = playerPokemon.maxHp;
      let eHp = enemyPokemon.curHp;
      let eMax = enemyPokemon.maxHp;

      if (phase === "animating" && eventQueue.length > 0) {
        // Walk through events up to the current one, applying HP snapshots.
        // On move_used events, look ahead to the next damage_dealt so the
        // HP bar drains when the user reads "X used Y!" rather than on
        // the follow-up "took N damage" message.
        const applyHpSnapshot = (ev: BattleEvent) => {
          if (!ev.targetName || !ev.targetMaxHp) return;
          if (ev.targetSide === "player") {
            pHp = ev.targetHp ?? 0;
            pMax = ev.targetMaxHp ?? pMax;
          } else if (ev.targetSide === "enemy") {
            eHp = ev.targetHp ?? 0;
            eMax = ev.targetMaxHp ?? eMax;
          } else if (ev.targetName === playerPokemon.name && ev.targetName !== enemyPokemon.name) {
            pHp = ev.targetHp ?? 0;
            pMax = ev.targetMaxHp ?? pMax;
          } else if (ev.targetName === enemyPokemon.name && ev.targetName !== playerPokemon.name) {
            eHp = ev.targetHp ?? 0;
            eMax = ev.targetMaxHp ?? eMax;
          }
        };

        for (let i = 0; i <= currentEventIndex && i < eventQueue.length; i++) {
          const ev = eventQueue[i];
          if (ev.targetName && ev.targetMaxHp) {
            applyHpSnapshot(ev);
          } else if (ev.type === "move_used" && i === currentEventIndex) {
            // Look ahead for the damage result of this move
            for (let j = i + 1; j < eventQueue.length; j++) {
              const next = eventQueue[j];
              if (next.targetName && next.targetMaxHp) {
                applyHpSnapshot(next);
                break;
              }
              // Stop looking if we hit another move_used (different attack)
              if (next.type === "move_used") break;
            }
          }
        }
      }

      return {
        targetPlayerHpPercent: pMax > 0 ? Math.max(0, (pHp / pMax) * 100) : 0,
        targetEnemyHpPercent: eMax > 0 ? Math.max(0, (eHp / eMax) * 100) : 0,
        targetDisplayPlayerHp: Math.max(0, pHp),
      };
    }, [playerPokemon, enemyPokemon, phase, eventQueue, currentEventIndex]);

  // Defer HP bar updates by one animation frame so the browser paints the
  // old value first, allowing the CSS transition to animate the change.
  const [playerHpPercent, setPlayerHpPercent] = useState(targetPlayerHpPercent);
  const [enemyHpPercent, setEnemyHpPercent] = useState(targetEnemyHpPercent);
  const [displayPlayerHp, setDisplayPlayerHp] = useState(targetDisplayPlayerHp);

  useEffect(() => {
    const raf = requestAnimationFrame(() => {
      setPlayerHpPercent(targetPlayerHpPercent);
      setEnemyHpPercent(targetEnemyHpPercent);
      setDisplayPlayerHp(targetDisplayPlayerHp);
    });
    return () => cancelAnimationFrame(raf);
  }, [targetPlayerHpPercent, targetEnemyHpPercent, targetDisplayPlayerHp]);

  const blackoutWarp = usePokeBattleStore((s) => s.blackoutWarp);

  const handleTextClick = useCallback(() => {
    if (phase === "animating") {
      advanceEvent();
    } else if (phase === "battle_end") {
      if (battleResult === "lose" && blackoutWarp) {
        useGameStatusStore
          .getState()
          .triggerBlackoutWarp(
            blackoutWarp.mapId,
            blackoutWarp.x,
            blackoutWarp.y,
          );
      }
      closeBattle();
    }
  }, [phase, battleResult, blackoutWarp, advanceEvent, closeBattle]);

  const handleFight = useCallback(() => {
    setPhase("move_select");
  }, [setPhase]);

  const handleItems = useCallback(() => {
    // Request fresh inventory so we know what balls the player has
    sendCQInventoryRequest();
    setPhase("item_select");
  }, [setPhase]);

  const handlePokemon = useCallback(() => {
    setPhase("pokemon_select");
  }, [setPhase]);

  const handleUseBall = useCallback(
    (itemId: number) => {
      WorldSocket.sendJsonMessage(OpCodes.PokeBattleActionRequest, {
        action: "item",
        moveSlot: 0,
        itemId,
      });
      setPhase("animating");
    },
    [setPhase],
  );

  const handleUseItem = useCallback(
    (itemId: number) => {
      WorldSocket.sendJsonMessage(OpCodes.PokeBattleActionRequest, {
        action: "item",
        moveSlot: 0,
        itemId,
        targetSlot: -1,
      });
      setPhase("animating");
    },
    [setPhase],
  );

  const handleRun = useCallback(() => {
    WorldSocket.sendJsonMessage(OpCodes.PokeBattleActionRequest, {
      action: "run",
      moveSlot: 0,
    });
  }, []);

  const isBattleActionAllowed = useCallback(
    (action: string) =>
      allowedActions.length === 0 || allowedActions.includes(action),
    [allowedActions],
  );

  const normalBattleActions = useMemo(
    () =>
      [
        { id: "fight", label: "FIGHT", handler: handleFight },
        { id: "item", label: "ITEMS", handler: handleItems },
        { id: "switch", label: "POKéMON", handler: handlePokemon },
        { id: "run", label: "RUN", handler: handleRun },
      ].filter((action) => isBattleActionAllowed(action.id)),
    [handleFight, handleItems, handlePokemon, handleRun, isBattleActionAllowed],
  );

  const handleMoveSelect = useCallback(
    (slot: number) => {
      WorldSocket.sendJsonMessage(OpCodes.PokeBattleActionRequest, {
        action: "fight",
        moveSlot: slot,
      });
      setPhase("animating");
    },
    [setPhase],
  );

  const handleBack = useCallback(() => {
    setPhase("action_select");
  }, [setPhase]);

  const handleFaintSwitch = useCallback(
    (partyIndex: number) => {
      WorldSocket.sendJsonMessage(OpCodes.PokeBattleSwitchRequest, {
        partyIndex,
        action: "switch",
      });
      setPhase("animating");
    },
    [setPhase],
  );

  const handleBattleSwitch = useCallback(
    (partyIndex: number) => {
      WorldSocket.sendJsonMessage(OpCodes.PokeBattleActionRequest, {
        action: "switch",
        moveSlot: partyIndex,
      });
      setPhase("animating");
    },
    [setPhase],
  );

  const handleFaintRun = useCallback(() => {
    WorldSocket.sendJsonMessage(OpCodes.PokeBattleSwitchRequest, {
      partyIndex: 0,
      action: "run",
    });
    // Server will respond with events (run_success or run_fail)
    // updateBattleState handles setting the phase
  }, []);

  const handleForgetMove = useCallback(
    (slot: number) => {
      WorldSocket.sendJsonMessage(OpCodes.PokeMoveLearnRequest, {
        forgetSlot: slot,
      });
    },
    [],
  );

  const handleSkipLearn = useCallback(() => {
    WorldSocket.sendJsonMessage(OpCodes.PokeMoveLearnRequest, {
      forgetSlot: -1,
    });
  }, []);

  // Safari Zone action handlers
  const handleSafariBall = useCallback(() => {
    sendSafariAction("ball");
    setPhase("animating");
  }, [setPhase]);

  const handleSafariBait = useCallback(() => {
    sendSafariAction("bait");
    setPhase("animating");
  }, [setPhase]);

  const handleSafariRock = useCallback(() => {
    sendSafariAction("rock");
    setPhase("animating");
  }, [setPhase]);

  const handleSafariRun = useCallback(() => {
    sendSafariAction("run");
    setPhase("animating");
  }, [setPhase]);

  const navigateBattleMenu = useCallback(
    (direction: MenuDirection) => {
      switch (phase) {
        case "action_select":
          setSelectedActionIndex((current) =>
            moveGridIndex(
              current,
              isSafari ? 4 : normalBattleActions.length,
              2,
              direction,
            ),
          );
          break;
        case "move_select":
          setSelectedMoveIndex((current) =>
            moveGridIndex(
              current,
              (playerPokemon?.moves.length ?? 0) + 1,
              2,
              direction,
            ),
          );
          break;
        case "item_select":
          setSelectedItemIndex((current) =>
            moveListIndex(current, battleItemEntries.length + 1, direction),
          );
          break;
        case "pokemon_select":
        case "faint_switch":
          setSelectedPokemonIndex((current) =>
            moveListIndex(current, pokemonMenuCount, direction),
          );
          break;
        case "move_learn_prompt":
          setSelectedMoveLearnIndex((current) =>
            moveListIndex(
              current,
              (playerPokemon?.moves.length ?? 0) + 1,
              direction,
            ),
          );
          break;
        default:
          break;
      }
    },
    [
      battleItemEntries.length,
      normalBattleActions.length,
      phase,
      playerPokemon?.moves.length,
      pokemonMenuCount,
      isSafari,
    ],
  );

  const activateBattleMenu = useCallback(() => {
    switch (phase) {
      case "animating":
      case "battle_end":
        handleTextClick();
        break;
      case "action_select": {
        const safariActions = [
          handleSafariBall,
          handleSafariBait,
          handleSafariRock,
          handleSafariRun,
        ];
        const actions = isSafari
          ? safariActions
          : normalBattleActions.map((action) => action.handler);
        actions[clampIndex(selectedActionIndex, actions.length)]?.();
        break;
      }
      case "move_select": {
        if (!playerPokemon) return;
        if (selectedMoveIndex >= playerPokemon.moves.length) {
          handleBack();
          return;
        }
        const move = playerPokemon.moves[selectedMoveIndex];
        if (move && move.pp > 0) {
          handleMoveSelect(selectedMoveIndex);
        }
        break;
      }
      case "item_select": {
        if (selectedItemIndex >= battleItemEntries.length) {
          handleBack();
          return;
        }
        const entry = battleItemEntries[selectedItemIndex];
        if (!entry) return;
        if (entry.kind === "ball") {
          handleUseBall(entry.inventory.item.id);
        } else {
          handleUseItem(entry.inventory.item.id);
        }
        break;
      }
      case "pokemon_select":
      case "faint_switch": {
        const selectedPokemon = faintSwitchParty[selectedPokemonIndex];
        if (selectedPokemon) {
          const canSwitch =
            selectedPokemonIndex !== faintSwitchActive &&
            selectedPokemon.curHp > 0;
          if (!canSwitch) return;
          if (phase === "pokemon_select") {
            handleBattleSwitch(selectedPokemonIndex);
          } else {
            handleFaintSwitch(selectedPokemonIndex);
          }
          return;
        }

        const runIndex = faintSwitchParty.length;
        const backIndex = runIndex + (canRunFromFaintSwitch ? 1 : 0);
        if (
          phase === "faint_switch" &&
          canRunFromFaintSwitch &&
          selectedPokemonIndex === runIndex
        ) {
          handleFaintRun();
        } else if (
          phase === "pokemon_select" &&
          selectedPokemonIndex === backIndex
        ) {
          handleBack();
        }
        break;
      }
      case "move_learn_prompt":
        if (!playerPokemon) return;
        if (selectedMoveLearnIndex >= playerPokemon.moves.length) {
          handleSkipLearn();
        } else {
          handleForgetMove(selectedMoveLearnIndex);
        }
        break;
      default:
        break;
    }
  }, [
    battleItemEntries,
    faintSwitchActive,
    faintSwitchParty,
    handleBack,
    handleBattleSwitch,
    handleFaintRun,
    handleFaintSwitch,
    handleForgetMove,
    handleMoveSelect,
    handleSafariBait,
    handleSafariBall,
    handleSafariRock,
    handleSafariRun,
    handleSkipLearn,
    handleTextClick,
    handleUseBall,
    handleUseItem,
    canRunFromFaintSwitch,
    isFaintSwitchWild,
    isSafari,
    normalBattleActions,
    phase,
    playerPokemon,
    selectedActionIndex,
    selectedItemIndex,
    selectedMoveIndex,
    selectedMoveLearnIndex,
    selectedPokemonIndex,
  ]);

  useEffect(() => {
    if (!isInBattle) return;

    const handleBattleKeyDown = (event: KeyboardEvent) => {
      if (isEditableKeyboardTarget(event.target)) return;

      const direction = keyboardDirection(event);
      if (direction) {
        event.preventDefault();
        event.stopPropagation();
        navigateBattleMenu(direction);
        return;
      }

      if (isKeyboardActivation(event)) {
        event.preventDefault();
        event.stopPropagation();
        activateBattleMenu();
      }
    };

    window.addEventListener("keydown", handleBattleKeyDown, {
      capture: true,
    });
    return () => {
      window.removeEventListener("keydown", handleBattleKeyDown, {
        capture: true,
      });
    };
  }, [activateBattleMenu, isInBattle, navigateBattleMenu]);

  // Determine if we should show trainer sprite (during intro) or Pokémon sprites.
  // Show trainer sprite if: trainer battle + animating + no move_used event seen yet
  // up to the current event index (i.e. we're still in the intro phase).
  const trainerSpritePath = trainerClass
    ? getTrainerSpritePath(trainerClass)
    : null;
  const showTrainerSprite = useMemo(() => {
    if (!trainerSpritePath || phase !== "animating") return false;
    // Check if any event up to current index is a move_used — if so, intro is over
    for (let i = 0; i <= currentEventIndex && i < eventQueue.length; i++) {
      if (eventQueue[i].type === "move_used") return false;
    }
    // Also check: if we're past the intro events (turnNumber > 0), don't show trainer
    return true;
  }, [trainerSpritePath, phase, currentEventIndex, eventQueue]);

  if (!isInBattle || !playerPokemon || !enemyPokemon) return null;

  return (
    <BattleOverlay data-testid="battle-overlay">
      <BattleWindow>
        <BattleScene>
          {/* Enemy info (top-left) — hidden during trainer intro */}
          {!showTrainerSprite && (
            <EnemyInfo>
              <PokemonNameRow>
                <PokemonName>{enemyPokemon.name}</PokemonName>
                <PokemonLevel>Lv{enemyPokemon.level}</PokemonLevel>
              </PokemonNameRow>
              <HPBarContainer>
                <HPLabel>HP</HPLabel>
                <HPBarOuter>
                  <HPBarInner
                    style={{
                      width: `${enemyHpPercent}%`,
                      backgroundColor:
                        enemyHpPercent > 50
                          ? "#48d048"
                          : enemyHpPercent > 20
                            ? "#f8c030"
                            : "#e84040",
                    }}
                  />
                </HPBarOuter>
              </HPBarContainer>
            </EnemyInfo>
          )}

          {/* Sprites: trainer intro shows trainer + player back; safari shows player back; battle shows Pokémon */}
          {showTrainerSprite ? (
            <>
              <EnemySprite src={trainerSpritePath!} alt="Trainer" />
              <PlayerSprite
                src="/assets/trainers/player_back.png"
                alt="Player"
              />
            </>
          ) : isSafari ? (
            <>
              <EnemySprite
                src={`/assets/pokemon/front/${enemyPokemon.id}.png`}
                alt={enemyPokemon.name}
              />
              <PlayerSprite
                src="/assets/trainers/player_back.png"
                alt="Player"
              />
            </>
          ) : (
            <>
              <EnemySprite
                src={`/assets/pokemon/front/${enemyPokemon.id}.png`}
                alt={enemyPokemon.name}
              />
              <PlayerSprite
                src={`/assets/pokemon/back/${playerPokemon.id}.png`}
                alt={playerPokemon.name}
              />
            </>
          )}

          {/* Player info (bottom-right) — hidden during trainer intro and safari */}
          {!showTrainerSprite && !isSafari && (
            <PlayerInfo>
              <PokemonNameRow>
                <PokemonName>{playerPokemon.name}</PokemonName>
                <PokemonLevel>Lv{playerPokemon.level}</PokemonLevel>
              </PokemonNameRow>
              <HPBarContainer>
                <HPLabel>HP</HPLabel>
                <HPBarOuter>
                  <HPBarInner
                    style={{
                      width: `${playerHpPercent}%`,
                      backgroundColor:
                        playerHpPercent > 50
                          ? "#48d048"
                          : playerHpPercent > 20
                            ? "#f8c030"
                            : "#e84040",
                    }}
                  />
                </HPBarOuter>
              </HPBarContainer>
              <HPText>
                {displayPlayerHp}/{playerPokemon.maxHp}
              </HPText>
            </PlayerInfo>
          )}
          {/* Safari ball count (bottom-right) */}
          {isSafari && (
            <PlayerInfo>
              <PokemonName>SAFARI BALLs</PokemonName>
              <HPText style={{ fontSize: "12px", marginTop: "4px" }}>× {safariBallsLeft}</HPText>
            </PlayerInfo>
          )}
        </BattleScene>

        {/* Text / Events display */}
        {phase === "animating" && (
          <TextBox data-testid="battle-text-box" onClick={handleTextClick}>
            <MessageText>
              {displayMessage}
              <TextArrow>▼</TextArrow>
            </MessageText>
          </TextBox>
        )}

        {/* Action select menu */}
        {phase === "action_select" && !isSafari && (
          <ActionMenu data-testid="battle-action-menu">
            {normalBattleActions.map((action, index) => (
              <ActionButton
                key={action.id}
                $selected={selectedActionIndex === index}
                data-testid={`battle-action-${action.id}`}
                data-selected={selectedActionIndex === index ? "true" : "false"}
                onClick={action.handler}
              >
                {action.label}
              </ActionButton>
            ))}
          </ActionMenu>
        )}

        {/* Safari Zone action menu */}
        {phase === "action_select" && isSafari && (
          <ActionMenu data-testid="battle-action-menu">
            <ActionButton
              $selected={selectedActionIndex === 0}
              data-testid="battle-action-safari-ball"
              data-selected={selectedActionIndex === 0 ? "true" : "false"}
              onClick={handleSafariBall}
            >
              SAFARI BALL
            </ActionButton>
            <ActionButton
              $selected={selectedActionIndex === 1}
              data-testid="battle-action-bait"
              data-selected={selectedActionIndex === 1 ? "true" : "false"}
              onClick={handleSafariBait}
            >
              BAIT
            </ActionButton>
            <ActionButton
              $selected={selectedActionIndex === 2}
              data-testid="battle-action-rock"
              data-selected={selectedActionIndex === 2 ? "true" : "false"}
              onClick={handleSafariRock}
            >
              ROCK
            </ActionButton>
            <ActionButton
              $selected={selectedActionIndex === 3}
              data-testid="battle-action-run"
              data-selected={selectedActionIndex === 3 ? "true" : "false"}
              onClick={handleSafariRun}
            >
              RUN
            </ActionButton>
          </ActionMenu>
        )}

        {/* Item select menu (Poké Balls) */}
        {phase === "item_select" && (
          <ItemSelectPanel
            sections={battleItemSections}
            selectedIndex={selectedItemIndex}
            onUseBall={handleUseBall}
            onUseItem={handleUseItem}
            onBack={handleBack}
          />
        )}

        {/* Normal switch — consumes the turn when a replacement is selected */}
        {phase === "pokemon_select" && (
          <FaintSwitchPanel
            selectedIndex={selectedPokemonIndex}
            onSwitch={handleBattleSwitch}
            onRun={() => undefined}
            onBack={handleBack}
            isWild={false}
            activeIndex={faintSwitchActive}
            party={faintSwitchParty}
          />
        )}

        {/* Faint switch — choose replacement Pokémon (or run in wild battles) */}
        {phase === "faint_switch" && (
          <FaintSwitchPanel
            selectedIndex={selectedPokemonIndex}
            onSwitch={handleFaintSwitch}
            onRun={handleFaintRun}
            isWild={battleType === "wild" || trainerClass === null}
            activeIndex={faintSwitchActive}
            party={faintSwitchParty}
          />
        )}

        {/* Move learn prompt — choose which move to forget or skip */}
        {phase === "move_learn_prompt" && pendingMoveLearn && playerPokemon && (
          <MoveLearnPanel
            moveName={pendingMoveLearn.moveName}
            currentMoves={playerPokemon.moves}
            selectedIndex={selectedMoveLearnIndex}
            onForget={handleForgetMove}
            onSkip={handleSkipLearn}
          />
        )}

        {/* Move select menu */}
        {phase === "move_select" && (
          <MoveGrid data-testid="battle-move-menu">
            {playerPokemon.moves.map((move, i) => {
              const color = typeColors[move.type] || "#A8A878";
              return (
                <MoveButton
                  key={move.id}
                  $typeColor={color}
                  $selected={selectedMoveIndex === i}
                  data-testid={`battle-move-${i}`}
                  data-selected={selectedMoveIndex === i ? "true" : "false"}
                  onClick={() => handleMoveSelect(i)}
                  disabled={move.pp <= 0}
                >
                  <span>{move.name}</span>
                  <MovePP>
                    PP {move.pp}/{move.maxPp}
                  </MovePP>
                  <MoveType $color={color}>{move.type}</MoveType>
                </MoveButton>
              );
            })}
            <BackButton
              $selected={selectedMoveIndex === playerPokemon.moves.length}
              data-testid="battle-move-back"
              data-selected={selectedMoveIndex === playerPokemon.moves.length ? "true" : "false"}
              onClick={handleBack}
            >
              ← BACK
            </BackButton>
          </MoveGrid>
        )}

        {/* Battle end — show as clickable text box for fled/lose/caught; win auto-closes */}
        {phase === "battle_end" && battleResult && battleResult !== "win" && (
          <TextBox onClick={handleTextClick}>
            <MessageText>
              {battleResult === "fled"
                ? "Got away safely!"
                : battleResult === "caught"
                  ? sentToPC
                    ? `${enemyPokemon.name} was transferred to\nBill's PC (BOX ${sentToPCBox}).`
                    : `${enemyPokemon.name}'s data was added to the POKéDEX!`
                  : lossMessage || (blackoutWarp ? "You blacked out!" : "You lost!")}
              <TextArrow>▼</TextArrow>
            </MessageText>
          </TextBox>
        )}
      </BattleWindow>
    </BattleOverlay>
  );
};

// --- Helpers ---

function getEventMessage(event: BattleEvent): string {
  switch (event.type) {
    case "move_used":
      return `${event.attackerName} used ${event.moveName}!`;
    case "damage_dealt": {
      let msg = event.message || `${event.targetName} took ${event.damage} damage!`;
      if (event.effectiveness && event.effectiveness >= 200) {
        msg += "\nIt's super effective!";
      } else if (
        event.effectiveness &&
        event.effectiveness > 0 &&
        event.effectiveness < 100
      ) {
        msg += "\nIt's not very effective...";
      }
      if (event.isCritical) {
        msg += "\nA critical hit!";
      }
      return msg;
    }
    case "critical_hit":
      return "A critical hit!";
    case "super_effective":
      return "It's super effective!";
    case "not_effective":
      return "It's not very effective...";
    case "immune":
      return event.message || "It doesn't affect the foe...";
    case "missed":
      return event.message || "The attack missed!";
    case "fainted":
      return `${event.faintedName} fainted!`;
    case "run_success":
      return "Got away safely!";
    case "run_fail":
      return event.message || "Can't escape!";
    case "battle_win":
      return "";
    case "battle_lose":
      return event.message || "You blacked out!";
    case "exp_gained":
      return event.message || "";
    case "catch_attempt":
    case "safari_ball":
      return event.message || "You threw a POKé BALL!";
    case "catch_success":
      return event.message || "Gotcha! The POKéMON was caught!";
    case "catch_fail":
      return event.message || "Oh no! The POKéMON broke free!";
    case "bait_thrown":
      return event.message || "You threw some BAIT!";
    case "rock_thrown":
      return event.message || "You threw a ROCK!";
    case "wild_fled":
      return event.message || "The wild POKéMON fled!";
    case "wild_watching":
      return event.message || "The wild POKéMON is watching carefully!";
    case "out_of_balls":
      return event.message || "You have no SAFARI BALLs left!";
    case "move_learned":
      return event.message || `Learned ${event.newMoveName}!`;
    case "move_learn_prompt":
      return event.message || "Wants to learn a new move!";
    case "evolution":
      return event.message || "Your Pokémon evolved!";
    case "message":
      return event.message || "";
    default:
      return event.message || "";
  }
}

export default PokeBattleDisplay;
