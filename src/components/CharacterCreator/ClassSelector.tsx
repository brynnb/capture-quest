import { useEffect } from "react";
import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import { ClassData } from "@/services/characterService";
import styled from "styled-components";
import SelectionButton from "../Interface/SelectionButton";
import { findDefaultCharacterClass } from "./classDefaults";

interface ClassSelectorProps {
  onClassSelect?: (classId: number) => void;
}

const ClassSelectorContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
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
      <Title>Choose Your Class</Title>
      {classes.map((classItem) => (
        <SelectionButton
          key={classItem.id}
          onClick={() => onSelectClass(classItem)}
          $isSelected={selectedClass?.id === classItem.id}
          $width="100%"
          $height="58px"
        >
          <span style={{ fontSize: "22px" }}>{classItem.name}</span>
        </SelectionButton>
      ))}
    </ClassSelectorContainer>
  );
};

export default ClassSelector;
