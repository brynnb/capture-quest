import { useCallback, useEffect, useRef } from "react";
import styled from "styled-components";
import usePokeBattleStore from "@stores/PokeBattleStore";
import usePokemonDialogueStore from "@stores/PokemonDialogueStore";
import useGameStatusStore from "@stores/GameStatusStore";
import useCQInventoryStore from "@stores/CQInventoryStore";
import usePokemonPCStore from "@stores/PokemonPCStore";
import useSlotMachineStore from "@stores/SlotMachineStore";
import {
  MOBILE_INTERACT_EVENT,
  MOBILE_MOVE_EVENT,
  type MobileMovementDirection,
} from "@/phaser-game/mobileControls";

const ControlsLayer = styled.div<{ $hidden: boolean }>`
  display: none;

  @media (max-width: 850px), (pointer: coarse) {
    position: absolute;
    inset: 0;
    z-index: 30;
    display: block;
    pointer-events: none;
    opacity: ${(props) => (props.$hidden ? 0 : 1)};
    visibility: ${(props) => (props.$hidden ? "hidden" : "visible")};
    transition: opacity 140ms ease;
  }
`;

const DPad = styled.div<{ $hidden: boolean }>`
  position: absolute;
  bottom: calc(16px + env(safe-area-inset-bottom, 0px));
  left: calc(16px + env(safe-area-inset-left, 0px));
  display: grid;
  width: 132px;
  height: 132px;
  grid-template: repeat(3, 1fr) / repeat(3, 1fr);
  opacity: ${(props) => (props.$hidden ? 0 : 1)};
  visibility: ${(props) => (props.$hidden ? "hidden" : "visible")};
  pointer-events: ${(props) => (props.$hidden ? "none" : "auto")};
  touch-action: none;
  user-select: none;
`;

const DirectionButton = styled.button<{ $area: string }>`
  grid-area: ${(props) => props.$area};
  min-width: 44px;
  min-height: 44px;
  padding: 0;
  color: #292b69;
  background: rgba(220, 221, 255, 0.82);
  border: 2px solid rgba(74, 75, 166, 0.9);
  border-radius: 10px;
  box-shadow: 0 5px 0 rgba(46, 47, 102, 0.82), 0 9px 20px rgba(0, 0, 0, 0.2);
  backdrop-filter: blur(9px);
  font-family: "Outfit", sans-serif;
  font-size: 20px;
  font-weight: 900;
  line-height: 1;
  touch-action: none;
  -webkit-tap-highlight-color: transparent;

  &:active {
    transform: translateY(3px);
    box-shadow: 0 2px 0 rgba(46, 47, 102, 0.82);
  }
`;

const DPadCenter = styled.div`
  grid-area: 2 / 2;
  margin: 4px;
  background: rgba(74, 75, 166, 0.72);
  border-radius: 50%;
  box-shadow: inset 0 0 0 5px rgba(192, 193, 255, 0.42);
`;

const ActionCluster = styled.div`
  position: absolute;
  right: calc(18px + env(safe-area-inset-right, 0px));
  bottom: calc(22px + env(safe-area-inset-bottom, 0px));
  display: flex;
  align-items: flex-end;
  gap: 14px;
  pointer-events: auto;
  touch-action: none;
  user-select: none;
`;

const RoundButton = styled.button<{ $secondary?: boolean }>`
  width: ${(props) => (props.$secondary ? "58px" : "78px")};
  height: ${(props) => (props.$secondary ? "58px" : "78px")};
  padding: 0;
  color: ${(props) => (props.$secondary ? "#5d3768" : "#23485c")};
  background: ${(props) =>
    props.$secondary ? "rgba(255, 204, 217, 0.88)" : "rgba(167, 237, 254, 0.88)"};
  border: 3px solid #4a4ba6;
  border-radius: 50%;
  box-shadow: 0 6px 0 #2e2f66, 0 12px 24px rgba(0, 0, 0, 0.24);
  backdrop-filter: blur(10px);
  font-family: "Pokemon GB", "Outfit", sans-serif;
  font-size: ${(props) => (props.$secondary ? "16px" : "22px")};
  font-weight: 900;
  touch-action: manipulation;
  -webkit-tap-highlight-color: transparent;

  &:active {
    transform: translateY(4px);
    box-shadow: 0 2px 0 #2e2f66;
  }
`;

interface HeldDirection {
  pointerId: number;
  interval: ReturnType<typeof setInterval>;
}

function emitMove(direction: MobileMovementDirection): void {
  window.dispatchEvent(
    new CustomEvent<MobileMovementDirection>(MOBILE_MOVE_EVENT, {
      detail: direction,
    }),
  );
}

const MobileControls = () => {
  const isInBattle = usePokeBattleStore((state) => state.isInBattle);
  const dialogueOpen = usePokemonDialogueStore((state) => state.isOpen);
  const shopOpen = useCQInventoryStore((state) => state.shopOpen);
  const pcOpen = usePokemonPCStore((state) => state.isOpen);
  const slotMachineOpen = useSlotMachineStore((state) => state.isOpen);
  const hudBlocked = useGameStatusStore(
    (state) =>
      state.isMobileChatOpen ||
      !state.isHudSidebarCollapsed ||
      state.isInventoryOpen ||
      state.isPokedexOpen ||
      state.isTrainerCardOpen ||
      state.isOptionsOpen ||
      state.isHelpOpen ||
      state.isGroupOpen ||
      state.isTileManagerOpen ||
      state.isArtStudioOpen,
  );
  const heldDirection = useRef<HeldDirection | null>(null);

  const releaseDirection = useCallback((pointerId?: number) => {
    if (!heldDirection.current) return;
    if (
      pointerId !== undefined &&
      heldDirection.current.pointerId !== pointerId
    ) {
      return;
    }
    clearInterval(heldDirection.current.interval);
    heldDirection.current = null;
  }, []);

  const pressDirection = useCallback(
    (direction: MobileMovementDirection, event: React.PointerEvent<HTMLButtonElement>) => {
      event.preventDefault();
      event.stopPropagation();
      releaseDirection();
      event.currentTarget.setPointerCapture?.(event.pointerId);
      emitMove(direction);
      heldDirection.current = {
        pointerId: event.pointerId,
        interval: setInterval(() => emitMove(direction), 115),
      };
    },
    [releaseDirection],
  );

  useEffect(() => releaseDirection, [releaseDirection]);

  useEffect(() => {
    if (
      isInBattle ||
      hudBlocked ||
      dialogueOpen ||
      shopOpen ||
      pcOpen ||
      slotMachineOpen
    ) {
      releaseDirection();
    }
  }, [
    dialogueOpen,
    hudBlocked,
    isInBattle,
    pcOpen,
    releaseDirection,
    shopOpen,
    slotMachineOpen,
  ]);

  const releasePointer = (event: React.PointerEvent<HTMLButtonElement>) => {
    releaseDirection(event.pointerId);
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const interact = (event: React.PointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.stopPropagation();
    window.dispatchEvent(new CustomEvent(MOBILE_INTERACT_EVENT));
  };

  const cancel = (event: React.PointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.stopPropagation();
    window.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Escape",
        code: "Escape",
        bubbles: true,
        cancelable: true,
      }),
    );
  };

  return (
    <ControlsLayer
      $hidden={isInBattle || hudBlocked || shopOpen || pcOpen || slotMachineOpen}
      aria-hidden={isInBattle || hudBlocked || shopOpen || pcOpen || slotMachineOpen}
    >
      <DPad $hidden={dialogueOpen} aria-label="Movement controls">
        <DirectionButton
          $area="1 / 2"
          aria-label="Move up"
          onPointerDown={(event) => pressDirection("UP", event)}
          onPointerUp={releasePointer}
          onPointerCancel={releasePointer}
          onLostPointerCapture={releasePointer}
        >
          ▲
        </DirectionButton>
        <DirectionButton
          $area="2 / 1"
          aria-label="Move left"
          onPointerDown={(event) => pressDirection("LEFT", event)}
          onPointerUp={releasePointer}
          onPointerCancel={releasePointer}
          onLostPointerCapture={releasePointer}
        >
          ◀
        </DirectionButton>
        <DPadCenter />
        <DirectionButton
          $area="2 / 3"
          aria-label="Move right"
          onPointerDown={(event) => pressDirection("RIGHT", event)}
          onPointerUp={releasePointer}
          onPointerCancel={releasePointer}
          onLostPointerCapture={releasePointer}
        >
          ▶
        </DirectionButton>
        <DirectionButton
          $area="3 / 2"
          aria-label="Move down"
          onPointerDown={(event) => pressDirection("DOWN", event)}
          onPointerUp={releasePointer}
          onPointerCancel={releasePointer}
          onLostPointerCapture={releasePointer}
        >
          ▼
        </DirectionButton>
      </DPad>
      <ActionCluster aria-label="Action controls">
        <RoundButton $secondary aria-label="Cancel" onPointerDown={cancel}>
          B
        </RoundButton>
        <RoundButton aria-label="Interact" onPointerDown={interact}>
          A
        </RoundButton>
      </ActionCluster>
    </ControlsLayer>
  );
};

export default MobileControls;
