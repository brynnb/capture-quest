import React, { useMemo } from "react";
import useCharacterStore from "@stores/CharacterCreatorStore";

const CharacterDescription: React.FC = () => {
  const { selectedFaction, selectedClass, selectedHomeTown } =
    useCharacterStore();

  const description = useMemo(() => {
    if (!selectedFaction || !selectedClass) {
      return "Please select a faction and class to see your character description.";
    }

    const homeTownName = selectedHomeTown
      ? selectedHomeTown.name || "an unknown location"
      : "an unknown location";

    return `You are a ${selectedFaction.name} ${selectedClass.name}. Your journey begins in ${homeTownName}.`;
  }, [selectedFaction, selectedClass, selectedHomeTown]);

  return (
    <div className="character-description">
      <h2>Character Description</h2>
      <p>{description}</p>
    </div>
  );
};

export default CharacterDescription;
