import ActionButton from "../Interface/ActionButton";
import useGameStatusStore from "@stores/GameStatusStore";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

interface SystemOptionsProps {
  showInventory?: boolean;
}

const SystemOptions = ({ showInventory = true }: SystemOptionsProps) => {
  const {
    isInventoryOpen,
    toggleInventory,
    isOptionsOpen,
    toggleOptions,
    isHelpOpen,
    toggleHelp,
    isMuted,
    toggleMute,
  } = useGameStatusStore();

  return (
    <>
      {showInventory && (
        <ActionButton
          text="Inventory"
          onClick={toggleInventory}
          isPressed={isInventoryOpen}
          isToggleable={true}
          marginBottom="4px"
          clickSound={
            isInventoryOpen
              ? sfxPathForConstant("SFX_TURN_OFF_PC")
              : sfxPathForConstant("SFX_TURN_ON_PC")
          }
        />
      )}
      <ActionButton
        text="Options"
        onClick={toggleOptions}
        isPressed={isOptionsOpen}
        isToggleable={true}
        marginBottom="4px"
      />
      <ActionButton
        text="Help"
        onClick={toggleHelp}
        isPressed={isHelpOpen}
        isToggleable={true}
        marginBottom="4px"
      />
      <ActionButton
        text="Mute"
        onClick={toggleMute}
        isPressed={isMuted}
        isToggleable={true}
        marginBottom="4px"
      />
    </>
  );
};

export default SystemOptions;
