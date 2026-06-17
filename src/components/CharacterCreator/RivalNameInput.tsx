import React from "react";
import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import SelectionButton from "@components/Interface/SelectionButton";
import styled from "styled-components";

const rivalNames = ["Gary", "Blue", "Trace", "Ash", "Leaf", "Green"];

const RivalNameInputContainer = styled.div`
  width: 100%;
  padding: 12px;
  box-sizing: border-box;
`;

const StyledRivalNameInput = styled.input`
  width: 100%;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.9);
  border: 3px solid #4a4ba6;
  border-radius: 12px;
  padding: 15px;
  color: #2e2f66;
  font-family: 'Outfit', sans-serif;
  font-size: 24px;
  outline: none;
  text-align: center;
  transition: all 0.2s ease;

  &:focus {
    border-color: #a7edfe;
    background: #ffffff;
    box-shadow: 0 0 10px rgba(167, 237, 254, 0.3);
  }

  &::placeholder {
    color: #4a4ba6;
    opacity: 0.5;
  }
`;

const RandomNameButtonContainer = styled.div`
  display: flex;
  justify-content: center;
  margin-top: 15px;
`;

const ValidationMessage = styled.div<{ $isError: boolean }>`
  color: ${(props) => (props.$isError ? "#ffaf84" : "#4a4ba6")};
  font-family: 'Outfit', sans-serif;
  font-weight: 700;
  font-size: 24px;
  text-align: center;
  margin-top: 12px;
  min-height: 28px;
`;

function formatRivalName(value: string): string {
  const lettersOnly = value.replace(/[^a-zA-Z]/g, "").slice(0, 12);
  return lettersOnly.charAt(0).toUpperCase() + lettersOnly.slice(1).toLowerCase();
}

const RivalNameInput: React.FC = () => {
  const { rivalName, setRivalName } = useCharacterCreatorStore();
  const isValid = rivalName.trim().length > 0;

  const handleNameChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setRivalName(formatRivalName(event.target.value));
  };

  const handleRandomName = () => {
    const nextName = rivalNames[Math.floor(Math.random() * rivalNames.length)];
    setRivalName(nextName);
  };

  return (
    <RivalNameInputContainer>
      <StyledRivalNameInput
        type="text"
        value={rivalName}
        onChange={handleNameChange}
        maxLength={12}
        placeholder="Enter rival name"
      />
      <RandomNameButtonContainer>
        <SelectionButton onClick={handleRandomName} $isSelected={false}>
          Random Rival
        </SelectionButton>
      </RandomNameButtonContainer>
      <ValidationMessage $isError={!isValid}>
        {isValid ? "" : "Rival name is required"}
      </ValidationMessage>
    </RivalNameInputContainer>
  );
};

export default RivalNameInput;
