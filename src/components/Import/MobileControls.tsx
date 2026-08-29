import { useCallback, useEffect, useRef } from "react";
import nipplejs from "nipplejs";
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
  cardinalDirectionForJoystick,
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

const JoystickZone = styled.div<{ $hidden: boolean }>`
  position: absolute;
  bottom: calc(16px + env(safe-area-inset-bottom, 0px));
  left: calc(16px + env(safe-area-inset-left, 0px));
  width: 132px;
  height: 132px;
  opacity: ${(props) => (props.$hidden ? 0 : 1)};
  visibility: ${(props) => (props.$hidden ? "hidden" : "visible")};
  pointer-events: ${(props) => (props.$hidden ? "none" : "auto")};
  touch-action: none;
  user-select: none;
  -webkit-user-select: none;
  -webkit-tap-highlight-color: transparent;

  /* nipplejs renders these as inline-styled children. The important overrides
     retain its proven geometry while keeping the control visible against the
     Game Boy world's predominantly white tiles. */
  & .back {
    box-sizing: border-box;
    background: rgba(220, 221, 255, 0.78) !important;
    border: 3px solid rgba(74, 75, 166, 0.92);
    box-shadow: 0 7px 0 rgba(46, 47, 102, 0.72), 0 12px 26px rgba(0, 0, 0, 0.24);
    backdrop-filter: blur(9px);
  }

  & .front {
    box-sizing: border-box;
    background: rgba(167, 237, 254, 0.96) !important;
    border: 2px solid #4a4ba6;
    box-shadow: 0 4px 0 rgba(46, 47, 102, 0.72), 0 7px 15px rgba(0, 0, 0, 0.22);
  }
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

function emitMove(direction: MobileMovementDirection): void {
  window.dispatchEvent(
    new CustomEvent<MobileMovementDirection>(MOBILE_MOVE_EVENT, {
      detail: direction,
    }),
  );
}

const MOVEMENT_REPEAT_MS = 115;

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
      state.isArtStudioOpen ||
      !state.isCameraFollowEnabled ||
      state.isWarpMode,
  );
  const joystickZone = useRef<HTMLDivElement | null>(null);
  const heldDirection = useRef<MobileMovementDirection | null>(null);
  const movementInterval = useRef<ReturnType<typeof setInterval> | null>(null);

  const releaseDirection = useCallback(() => {
    if (movementInterval.current) {
      clearInterval(movementInterval.current);
      movementInterval.current = null;
    }
    heldDirection.current = null;
  }, []);

  const holdDirection = useCallback(
    (direction: MobileMovementDirection | null) => {
      if (!direction) {
        releaseDirection();
        return;
      }
      if (heldDirection.current === direction) return;

      releaseDirection();
      heldDirection.current = direction;
      emitMove(direction);
      movementInterval.current = setInterval(() => {
        if (heldDirection.current) emitMove(heldDirection.current);
      }, MOVEMENT_REPEAT_MS);
    },
    [releaseDirection],
  );

  useEffect(() => releaseDirection, [releaseDirection]);

  useEffect(() => {
    if (!joystickZone.current) return;

    // Keep this aligned with New Yokosuka's proven mobile stick dimensions and
    // behavior. Only the output mapping differs because CaptureQuest is a
    // cardinal, tile-based game rather than an analog 3D controller.
    const joystick = nipplejs.create({
      zone: joystickZone.current,
      mode: "static",
      position: { left: "50%", top: "50%" },
      size: 112,
      threshold: 0.1,
      color: "#f2f4f5",
      restOpacity: 0.72,
    });
    joystick.on("move", (event) => {
      holdDirection(cardinalDirectionForJoystick(event.data?.vector));
    });
    joystick.on("end", releaseDirection);

    return () => {
      releaseDirection();
      joystick.destroy();
    };
  }, [holdDirection, releaseDirection]);

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
      <JoystickZone
        ref={joystickZone}
        $hidden={dialogueOpen}
        role="application"
        aria-label="Movement joystick"
        aria-hidden={dialogueOpen}
      />
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
