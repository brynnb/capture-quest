import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useGameScreenStore from "@stores/GameScreenStore";
import useStaticDataStore from "@stores/StaticDataStore";
import FactionSelector from "./FactionSelector";
import ClassSelector from "./ClassSelector";

import NameInput from "./NameInput";
import RivalNameInput from "./RivalNameInput";
import HomeTownSelector from "./HomeTownSelector";
import SubmitCharacter from "./SubmitCharacter";
import styled from "styled-components";
import SelectionButton from "../Interface/SelectionButton";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

const MainContainer = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  width: 1400px;
  margin: 0 auto;
`;

const MultiColumnLayout = styled.div`
  display: grid;
  grid-template-columns: 350px 500px 350px;
  gap: 45px;
  width: 100%;
  justify-content: center;
  align-items: start;
`;

const NavigationContainer = styled.div`
  position: absolute;
  bottom: 20px;
  right: 20px;
  display: flex;
  gap: 10px;
`;

const StoryText = styled.div`
  font-family: "Outfit", sans-serif;
  background-color: rgba(255, 236, 241, 0.95);
  backdrop-filter: blur(15px);
  border: 4px solid #ffccd9;
  border-radius: 30px;
  padding: 60px;
  color: #2e2f66;
  font-size: 28px;
  line-height: 1.6;
  max-width: 900px;
  text-align: left;
  white-space: pre-wrap;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.2);
  font-weight: 500;
  min-height: 400px;
  display: flex;
  align-items: flex-start;
  justify-content: flex-start;
`;

const ViewportContainer = styled.div`
  position: relative;
  width: 500px;
  height: 750px;
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
  border: 4px solid #4a4ba6;
  border-radius: 24px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.3);
  background-image: url("/assets/charselectbg.png");
  background-size: cover;
  background-position: center;
`;

const TrainerImage = styled.img`
  max-height: 90%;
  max-width: 90%;
  object-fit: contain;
  transform: translateY(45px);
`;

const ViewportColumn = styled.div`
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 20px;
`;

const GenderSelectorContainer = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-top: 25px;
  background: rgba(192, 193, 255, 0.4);
  backdrop-filter: blur(10px);
  border: 4px solid #4a4ba6;
  border-radius: 20px;
  padding: 15px;
  width: 100%;
  box-sizing: border-box;
`;

const GenderOption = styled.div`
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 15px;
  cursor: pointer;
  user-select: none;
  font-family: "Outfit", sans-serif;
  text-transform: uppercase;
  font-weight: 800;
  font-size: 24px;
  color: #2e2f66;
  width: 100%;
  transition: all 0.2s ease;
  &:hover {
    transform: scale(1.02);
  }
`;

const RadioButton = styled.div<{ $isActive: boolean }>`
  width: 24px;
  height: 24px;
  background-color: ${({ $isActive }) =>
    $isActive ? "#ffccd9" : "rgba(255, 255, 255, 0.5)"};
  border: 3px solid #4a4ba6;
  border-radius: 50%;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;

  &::after {
    content: "";
    width: 12px;
    height: 12px;
    background-color: #2e2f66;
    border-radius: 50%;
    display: ${({ $isActive }) => ($isActive ? "block" : "none")};
  }
`;

const CharacterCreator = () => {
  const { setScreen } = useGameScreenStore();
  const {
    currentStep,
    setCurrentStep,
    canProceedToNextStep,
    selectedFaction,
    selectedClass,
    selectedHomeTown,
    rivalName,
    selectedGender,
    setSelectedGender,
    resetStore,
  } = useCharacterCreatorStore();

  const classes = useStaticDataStore((state) => state.classes);

  const handleClassSelection = (selectedClassId: number) => {
    const foundClass = classes.find((c) => c.id === selectedClassId);
    if (foundClass) {
      useCharacterCreatorStore.getState().setSelectedClass(foundClass);
    }
  };

  const handleNext = () => {
    setCurrentStep(currentStep + 1);
  };

  const handleBack = () => {
    if (currentStep > 1) {
      setCurrentStep(currentStep - 1);
    }
  };

  const handleBackToCharacterSelect = () => {
    resetStore();
    setScreen("characterSelect");
  };

  const getTrainerImage = () => {
    switch (selectedGender) {
      case 0:
        return "/assets/trainerm.png";
      case 1:
        return "/assets/trainerf.png";
      case 2:
        return "/assets/trainernb.png";
      default:
        return "/assets/trainerm.png";
    }
  };

  const renderStep = () => {
    switch (currentStep) {
      case 1:
        return (
          <MainContainer>
            <MultiColumnLayout>
              <div>
                <FactionSelector />
              </div>
              <ViewportColumn style={{ marginTop: "60px" }}>
                <ViewportContainer id="CharacterCreator__ViewportContainer">
                  <TrainerImage src={getTrainerImage()} alt="Trainer Preview" />
                </ViewportContainer>
              </ViewportColumn>
              <div
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: "20px",
                  justifyContent: "flex-start",
                  marginTop: "60px",
                }}
              >
                <GenderSelectorContainer style={{ marginTop: 0 }}>
                  <GenderOption
                    onClick={() => {
                      const sfx = sfxPathForConstant("SFX_PRESS_AB");
                      if (sfx) AudioManager.playSFX(sfx);
                      setSelectedGender(0);
                    }}
                  >
                    MASCULINE <RadioButton $isActive={selectedGender === 0} />
                  </GenderOption>
                  <GenderOption
                    onClick={() => {
                      const sfx = sfxPathForConstant("SFX_PRESS_AB");
                      if (sfx) AudioManager.playSFX(sfx);
                      setSelectedGender(1);
                    }}
                  >
                    FEMININE <RadioButton $isActive={selectedGender === 1} />
                  </GenderOption>
                  <GenderOption
                    onClick={() => {
                      const sfx = sfxPathForConstant("SFX_PRESS_AB");
                      if (sfx) AudioManager.playSFX(sfx);
                      setSelectedGender(2);
                    }}
                  >
                    NON-BINARY <RadioButton $isActive={selectedGender === 2} />
                  </GenderOption>
                </GenderSelectorContainer>
                <div style={{ marginTop: "0px" }}>
                  <NameInput />
                </div>
                <div style={{ marginTop: "0px" }}>
                  <RivalNameInput />
                </div>
              </div>
            </MultiColumnLayout>
            <NavigationContainer>
              <SelectionButton
                onClick={handleBackToCharacterSelect}
                $isSelected={false}
              >
                Cancel
              </SelectionButton>
              <SelectionButton
                onClick={handleNext}
                disabled={!canProceedToNextStep()}
                $isSelected={false}
                $isDisabled={!canProceedToNextStep()}
              >
                Next Step: Class
              </SelectionButton>
            </NavigationContainer>
          </MainContainer>
        );
      case 2:
        return (
          <MainContainer>
            <MultiColumnLayout
              style={{ gridTemplateColumns: "350px 1fr", maxWidth: "1290px" }}
            >
              <ClassSelector onClassSelect={handleClassSelection} />
              <div
                style={{
                  marginTop: "44px",
                  width: "100%",
                  display: "flex",
                  flexDirection: "column",
                  gap: "20px",
                }}
              >
                <StoryText
                  style={{
                    minHeight: "220px",
                    padding: "30px",
                    fontSize: "18px",
                    width: "100%",
                    boxSizing: "border-box",
                  }}
                >
                  {selectedClass ? (
                    <div style={{ width: "100%" }}>
                      <div
                        style={{
                          fontWeight: "bold",
                          marginBottom: "5px",
                          textTransform: "uppercase",
                          color: "#4a4ba6",
                        }}
                      >
                        {selectedClass.name} —{" "}
                        {selectedClass.classType || "Specialist"} Type
                      </div>
                      {selectedClass.lore ||
                        "A specialized trainer pursuing excellence in their chosen field."}
                    </div>
                  ) : (
                    <div
                      style={{
                        width: "100%",
                        opacity: 0.5,
                        textAlign: "center",
                      }}
                    >
                      Select a Class to view details
                    </div>
                  )}
                </StoryText>
              </div>
            </MultiColumnLayout>
            <NavigationContainer>
              <SelectionButton onClick={handleBack} $isSelected={false}>
                Back
              </SelectionButton>
              <SelectionButton
                onClick={handleNext}
                disabled={!canProceedToNextStep()}
                $isSelected={false}
                $isDisabled={!canProceedToNextStep()}
              >
                Next Step: Home City
              </SelectionButton>
            </NavigationContainer>
          </MainContainer>
        );
      case 3:
        return (
          <MainContainer>
            <HomeTownSelector />
            <NavigationContainer>
              <SelectionButton onClick={handleBack} $isSelected={false}>
                Back
              </SelectionButton>
              <SelectionButton
                onClick={handleNext}
                disabled={!canProceedToNextStep()}
                $isSelected={false}
                $isDisabled={!canProceedToNextStep()}
              >
                Next Step: Confirm
              </SelectionButton>
            </NavigationContainer>
          </MainContainer>
        );
      case 4:
        return (
          <MainContainer>
            <StoryText>
              As a {(selectedFaction as any)?.name} {selectedClass?.name}, you
              begin your journey in{" "}
              {selectedHomeTown
                ? selectedHomeTown.name || "your new home"
                : "your new home"}
              . Your rival {rivalName || "Gary"} will be watching every step.
              Are you ready to begin?
            </StoryText>
            <NavigationContainer>
              <SelectionButton onClick={handleBack} $isSelected={false}>
                Back
              </SelectionButton>
              <SubmitCharacter />
            </NavigationContainer>
          </MainContainer>
        );
      default:
        return null;
    }
  };

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "center",
        paddingTop: "60px",
      }}
    >
      {renderStep()}
    </div>
  );
};

export default CharacterCreator;
