import React from "react";
import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import SelectionButton from "../Interface/SelectionButton";

const SubmitCharacter: React.FC = () => {
  const { accountOperation, createError } = useCharacterSelectStore();
  const { characterName, rivalName, selectedFaction, selectedClass, selectedHomeTown, createCharacter } = useCharacterCreatorStore();
  const disabled = !characterName || !rivalName.trim() || !selectedFaction || !selectedClass || !selectedHomeTown || accountOperation !== null;
  return <>
    {createError && <p role="alert">{createError}</p>}
    <SelectionButton onClick={() => { void createCharacter(); }} disabled={disabled} $isSelected={false} $isDisabled={disabled}>
      {accountOperation === "create" ? "CREATING..." : "Create"}
    </SelectionButton>
  </>;
};

export default SubmitCharacter;
