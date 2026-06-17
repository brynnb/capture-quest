import React, { useState } from "react";
import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useGameScreenStore from "@stores/GameScreenStore";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import SelectionButton from "../Interface/SelectionButton";
const SubmitCharacter: React.FC = () => {
  const { setScreen } = useGameScreenStore();
  const [loading, setLoading] = useState(false);
  const { setPendingSelectName } = useCharacterSelectStore();
  const {
    characterName,
    rivalName,
    selectedFaction,
    selectedClass,
    selectedHomeTown,
    resetStore,
    createCharacter,
  } = useCharacterCreatorStore();

  const handleSubmit = async () => {
    setLoading(true);

    try {
      const success = await createCharacter();
      setLoading(false);

      if (success) {
        console.log("Character created successfully");
        setPendingSelectName(characterName);
        resetStore();
        setScreen("characterSelect");
      } else {
        window.alert(
          "Failed to create character. Please choose a different name or try again.",
        );
      }
    } catch (error) {
      console.error("Failed to create character:", error);
      setLoading(false);
    }
  };

  const disabled =
    !characterName ||
    !rivalName.trim() ||
    !selectedFaction ||
    !selectedClass ||
    !selectedHomeTown ||
    loading;

  return (
    <SelectionButton
      onClick={handleSubmit}
      disabled={disabled}
      $isSelected={false}
      $isDisabled={disabled}
    >
      Create
    </SelectionButton>
  );
};

export default SubmitCharacter;
