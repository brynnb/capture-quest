import { useEffect } from "react";
import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import { ClassData } from "@/services/characterService";
import styled from "styled-components";
import { findDefaultCharacterClass } from "./classDefaults";
import CompactChoiceList from "./CompactChoiceList";

interface ClassSelectorProps {
  onClassSelect?: (classId: number) => void;
}

const ClassSelectorContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;

  @media (max-width: 900px), (pointer: coarse) {
    height: 100%;
    min-height: 0;
    flex: 1 1 auto;
  }
`;

const ClassSelector = ({ onClassSelect }: ClassSelectorProps) => {
  const { selectedClass, setSelectedClass } = useCharacterCreatorStore();
  const classes = useStaticDataStore((state) => state.classes);

  useEffect(() => {
    if (selectedClass) return;
    const defaultClass = findDefaultCharacterClass(classes);
    if (defaultClass) {
      setSelectedClass(defaultClass);
    }
  }, [classes, selectedClass, setSelectedClass]);

  const onSelectClass = (charClass: ClassData) => {
    setSelectedClass(charClass);
    if (onClassSelect) {
      onClassSelect(charClass.id);
    }
  };

  return (
    <ClassSelectorContainer>
      <CompactChoiceList
        title="Choose Your Class"
        choices={classes.map((classItem) => ({
          id: classItem.id,
          label: classItem.name,
          description:
            classItem.lore ||
            `A ${classItem.classType || "specialist"} trainer class.`,
        }))}
        selectedId={selectedClass?.id}
        showOnDesktop
        onSelect={(id) => {
          const charClass = classes.find((item) => item.id === Number(id));
          if (charClass) onSelectClass(charClass);
        }}
      />
    </ClassSelectorContainer>
  );
};

export default ClassSelector;
