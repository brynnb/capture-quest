import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import { FactionData } from "@/services/characterService";
import styled from "styled-components";
import SelectionButton from "../Interface/SelectionButton";

const FactionSelectorContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
`;

const Title = styled.div`
  font-family: "Outfit", sans-serif;
  text-transform: uppercase;
  font-weight: 800;
  font-size: 24px;
  text-align: center;
  color: #2e2f66;
  margin-bottom: 12px;
`;

const FactionSelector = () => {
  const { selectedFaction, setSelectedFaction } = useCharacterCreatorStore(
    (state) => ({
      selectedFaction: state.selectedFaction,
      setSelectedFaction: state.setSelectedFaction,
    }),
  );

  const factions = useStaticDataStore((state) => state.factions);
  const playableFactions = factions.filter((f) => f.isPlayable && f.isStarting);

  const onSelectFaction = (faction: FactionData) => {
    setSelectedFaction(faction);
  };

  return (
    <FactionSelectorContainer>
      <Title>Choose Your Faction</Title>
      {playableFactions.map((faction) => (
        <SelectionButton
          key={faction.id}
          onClick={() => onSelectFaction(faction)}
          $isSelected={selectedFaction?.id === faction.id}
          $width="100%"
        >
          {faction.name}
        </SelectionButton>
      ))}
    </FactionSelectorContainer>
  );
};

export default FactionSelector;
