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
  width: min(1400px, 100%);
  height: 100%;
  margin: 0 auto;
  padding: 20px 20px 84px;
  box-sizing: border-box;
  overflow-y: auto;
  overscroll-behavior: contain;

  @media (max-width: 1100px), (pointer: coarse) {
    width: 100%;
    gap: 12px;
    padding: 16px 12px 76px;
  }
`;

const MultiColumnLayout = styled.div<{ $twoColumns?: boolean }>`
  display: grid;
  grid-template-columns: ${(props) =>
    props.$twoColumns
      ? "minmax(260px, 350px) minmax(0, 1fr)"
      : "minmax(250px, 350px) minmax(380px, 500px) minmax(250px, 350px)"};
  gap: clamp(18px, 3vw, 45px);
  width: 100%;
  max-width: ${(props) => (props.$twoColumns ? "1290px" : "none")};
  justify-content: center;
  align-items: start;

  @media (max-width: 1100px), (pointer: coarse) {
    grid-template-columns: minmax(0, 1fr);
    gap: 14px;
    width: 100%;
  }
`;

const NavigationContainer = styled.div`
  position: absolute;
  bottom: 20px;
  right: 20px;
  display: flex;
  gap: 10px;

  @media (max-width: 900px), (pointer: coarse) {
    right: 12px;
    bottom: calc(10px + env(safe-area-inset-bottom, 0px));
    left: 12px;
    justify-content: flex-end;
    flex-wrap: wrap;

    & > button {
      width: auto;
      min-width: 0;
      flex: 1 1 138px;
    }
  }
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

  @media (max-width: 900px), (pointer: coarse) {
    min-height: 0;
    padding: 24px;
    font-size: 18px;
    border-radius: 18px;
  }
`;

const ViewportContainer = styled.div`
  position: relative;
  width: 100%;
  max-width: 500px;
  height: clamp(420px, 70vh, 750px);
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

  @media (max-width: 1100px), (pointer: coarse) {
    width: min(100%, 500px);
    height: min(48vh, 520px);
    align-self: center;
  }
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
  margin-top: 60px;

  min-width: 0;

  @media (max-width: 1100px), (pointer: coarse) {
    height: auto;
    margin-top: 0;
  }
`;

const IdentityColumn = styled.div`
  display: flex;
  flex-direction: column;
  gap: 20px;
  justify-content: flex-start;
  margin-top: 60px;
  min-width: 0;

  @media (max-width: 1100px), (pointer: coarse) {
    margin-top: 0;
  }
`;

const StoryColumn = styled.div`
  display: flex;
  width: 100%;
  min-width: 0;
  margin-top: 44px;
  flex-direction: column;
  gap: 20px;

  @media (max-width: 1100px), (pointer: coarse) {
    margin-top: 0;
  }
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
              <ViewportColumn>
                <ViewportContainer id="CharacterCreator__ViewportContainer">
                  <TrainerImage src={getTrainerImage()} alt="Trainer Preview" />
                </ViewportContainer>
              </ViewportColumn>
              <IdentityColumn>
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
              </IdentityColumn>
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
            <MultiColumnLayout $twoColumns>
              <ClassSelector onClassSelect={handleClassSelection} />
              <StoryColumn>
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
              </StoryColumn>
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
              As a {selectedFaction?.name} {selectedClass?.name}, you
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
