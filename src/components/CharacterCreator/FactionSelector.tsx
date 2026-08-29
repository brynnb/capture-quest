import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import { FactionData } from "@/services/characterService";
import styled from "styled-components";
import CompactChoiceCarousel from "./CompactChoiceCarousel";
import CompactChoiceList from "./CompactChoiceList";

const FactionSelectorContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
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

  const factionChoices = playableFactions.map((faction) => ({
    id: faction.id,
    label: faction.name,
    description: faction.lore,
  }));

  return (
    <FactionSelectorContainer>
      <CompactChoiceList
        title="Choose Your Faction"
        choices={factionChoices}
        selectedId={selectedFaction?.id}
        showOnDesktop
        showOnMobile={false}
        onSelect={(id) => {
          const faction = playableFactions.find(
            (item) => item.id === Number(id),
          );
          if (faction) onSelectFaction(faction);
        }}
      />
      <CompactChoiceCarousel
        title="Choose Your Faction"
        itemLabel="faction"
        choices={factionChoices}
        selectedId={selectedFaction?.id}
        onSelect={(id) => {
          const faction = playableFactions.find(
            (item) => item.id === Number(id),
          );
          if (faction) onSelectFaction(faction);
        }}
      />
    </FactionSelectorContainer>
  );
};

export default FactionSelector;
