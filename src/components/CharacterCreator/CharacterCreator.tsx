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
  padding: 20px;
  box-sizing: border-box;
  overflow-y: auto;
  overscroll-behavior: contain;

  @media (max-width: 1100px), (pointer: coarse) {
    width: 100%;
    gap: 8px;
    padding: 8px 10px max(8px, env(safe-area-inset-bottom, 0px));
  }

  @media (max-height: 650px) and (max-width: 1100px),
    (max-height: 650px) and (pointer: coarse) {
    gap: 4px;
    padding: 4px 8px max(4px, env(safe-area-inset-bottom, 0px));
  }
`;

const CreatorShell = styled.div`
  --character-choice-height: clamp(420px, 66vh, 700px);
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 60px;
  box-sizing: border-box;
  overflow: hidden;

  @media (max-width: 1100px), (pointer: coarse), (max-height: 700px) {
    padding-top: max(6px, env(safe-area-inset-top, 0px));
  }
`;

const MultiColumnLayout = styled.div<{
  $twoColumns?: boolean;
  $trainerStep?: boolean;
}>`
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
    gap: 8px;
    width: 100%;
    min-height: 0;
    flex: 1 1 auto;
    align-items: stretch;
  }

  @media (max-height: 650px) and (min-width: 600px) and (pointer: coarse),
    (max-height: 650px) and (min-width: 600px) and (max-width: 1100px) {
    ${(props) =>
      props.$trainerStep &&
      `
        grid-template-areas:
          "preview identity"
          "faction identity";
        grid-template-columns: minmax(180px, 0.7fr) minmax(280px, 1.3fr);
        grid-template-rows: 100px auto;
        column-gap: 12px;
        row-gap: 4px;
      `}
  }
`;

const FactionColumn = styled.div`
  min-width: 0;

  @media (max-width: 1100px), (pointer: coarse) {
    order: 2;
  }

  @media (max-height: 650px) and (min-width: 600px) {
    grid-area: faction;
  }
`;

const NavigationContainer = styled.div<{
  $inset?: boolean;
  $centered?: boolean;
  $wideActions?: boolean;
}>`
  position: relative;
  z-index: 10;
  display: flex;
  width: 100%;
  min-width: 0;
  margin-top: auto;
  padding-right: ${(props) => (props.$inset ? "clamp(20px, 4vw, 52px)" : "0")};
  box-sizing: border-box;
  gap: 10px;
  justify-content: ${(props) => (props.$centered ? "center" : "flex-end")};

  & > button {
    flex-shrink: 0;
  }

  ${(props) =>
    props.$wideActions &&
    `
      & > button:first-child {
        flex: 0 0 230px;
      }

      & > button:last-child {
        width: 340px;
        min-width: 0;
        flex: 0 0 340px;
      }
    `}

  @media (max-width: 900px), (pointer: coarse) {
    display: grid;
    width: 100%;
    margin-top: auto;
    padding-right: 0;
    grid-template-columns: repeat(2, minmax(0, 1fr));

    & > button {
      width: 100%;
      max-width: 100%;
      height: 52px;
      min-width: 0;
      padding: 0 8px;
      font-size: clamp(13px, 4vw, 17px);
      line-height: 1.1;
      white-space: normal;
    }
  }

  @media (max-height: 650px) and (max-width: 900px),
    (max-height: 650px) and (pointer: coarse) {
    & > button {
      height: 46px;
      font-size: 13px;
    }
  }
`;

const DesktopNavigationLabel = styled.span`
  @media (max-width: 900px), (pointer: coarse) {
    display: none;
  }
`;

const MobileNavigationLabel = styled.span`
  display: none;

  @media (max-width: 900px), (pointer: coarse) {
    display: inline;
  }
`;

const StepIndicator = styled.div`
  color: #4a4ba6;
  font:
    900 12px/1 "Outfit",
    sans-serif;
  letter-spacing: 0.1em;
  text-transform: uppercase;

  @media (min-width: 901px) and (pointer: fine) {
    display: none;
  }
`;

const ConfirmationContent = styled.div`
  display: flex;
  width: 100%;
  min-height: 0;
  flex: 1 1 auto;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
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
    padding: 18px;
    font-size: 17px;
    line-height: 1.45;
    border-width: 3px;
    border-radius: 16px;
  }
`;

const ViewportContainer = styled.div`
  position: relative;
  width: 100%;
  max-width: 500px;
  height: var(--character-choice-height);
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
  box-sizing: border-box;
  border: 4px solid #4a4ba6;
  border-radius: 24px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.3);
  background-image: url("/assets/charselectbg.png");
  background-size: cover;
  background-position: center;

  @media (max-width: 1100px), (pointer: coarse) {
    width: min(100%, 300px);
    height: clamp(150px, 24dvh, 210px);
    align-self: center;
    border-width: 3px;
    border-radius: 16px;
  }

  @media (max-height: 650px) and (max-width: 1100px),
    (max-height: 650px) and (pointer: coarse) {
    width: min(100%, 220px);
    height: 100px;
    border-radius: 12px;
  }
`;

const TrainerImage = styled.img`
  max-height: 90%;
  max-width: 90%;
  object-fit: contain;
  transform: translateY(45px);

  @media (max-width: 1100px), (pointer: coarse) {
    transform: translateY(18px);
  }
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
    gap: 8px;
    order: 1;
  }

  @media (max-height: 650px) and (min-width: 600px) {
    grid-area: preview;
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
    gap: 6px;
    order: 3;
  }

  @media (max-height: 650px) and (max-width: 1100px),
    (max-height: 650px) and (pointer: coarse) {
    gap: 2px;
  }

  @media (max-height: 650px) and (min-width: 600px) {
    grid-area: identity;
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
    display: none;
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

  @media (max-width: 1100px), (pointer: coarse) {
    flex-direction: row;
    gap: 4px;
    margin-top: 0;
    padding: 6px;
    border-width: 3px;
    border-radius: 14px;
  }
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

  @media (max-width: 1100px), (pointer: coarse) {
    min-width: 0;
    flex: 1 1 0;
    flex-direction: column;
    justify-content: center;
    gap: 3px;
    font-size: clamp(10px, 3vw, 13px);
    line-height: 1.1;
    text-align: center;
    white-space: nowrap;
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

  @media (max-width: 1100px), (pointer: coarse) {
    width: 16px;
    height: 16px;
    border-width: 2px;

    &::after {
      width: 8px;
      height: 8px;
    }
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
          <MainContainer data-testid="character-creator-step">
            <StepIndicator>Step 1 of 4 · Trainer</StepIndicator>
            <MultiColumnLayout $trainerStep>
              <FactionColumn>
                <FactionSelector />
              </FactionColumn>
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
            <NavigationContainer
              $inset
              data-testid="character-creation-navigation"
            >
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
                <DesktopNavigationLabel>
                  Next Step: Class
                </DesktopNavigationLabel>
                <MobileNavigationLabel>Next</MobileNavigationLabel>
              </SelectionButton>
            </NavigationContainer>
          </MainContainer>
        );
      case 2:
        return (
          <MainContainer data-testid="character-creator-step">
            <StepIndicator>Step 2 of 4 · Class</StepIndicator>
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
            <NavigationContainer
              $wideActions
              data-testid="character-creation-navigation"
            >
              <SelectionButton onClick={handleBack} $isSelected={false}>
                Back
              </SelectionButton>
              <SelectionButton
                onClick={handleNext}
                disabled={!canProceedToNextStep()}
                $isSelected={false}
                $isDisabled={!canProceedToNextStep()}
              >
                <DesktopNavigationLabel>
                  Next Step: Home City
                </DesktopNavigationLabel>
                <MobileNavigationLabel>Next</MobileNavigationLabel>
              </SelectionButton>
            </NavigationContainer>
          </MainContainer>
        );
      case 3:
        return (
          <MainContainer data-testid="character-creator-step">
            <StepIndicator>Step 3 of 4 · Home</StepIndicator>
            <HomeTownSelector />
            <NavigationContainer data-testid="character-creation-navigation">
              <SelectionButton onClick={handleBack} $isSelected={false}>
                Back
              </SelectionButton>
              <SelectionButton
                onClick={handleNext}
                disabled={!canProceedToNextStep()}
                $isSelected={false}
                $isDisabled={!canProceedToNextStep()}
              >
                <DesktopNavigationLabel>
                  Next Step: Confirm
                </DesktopNavigationLabel>
                <MobileNavigationLabel>Next</MobileNavigationLabel>
              </SelectionButton>
            </NavigationContainer>
          </MainContainer>
        );
      case 4:
        return (
          <MainContainer data-testid="character-creator-step">
            <ConfirmationContent data-testid="confirmation-content">
              <StepIndicator>Step 4 of 4 · Confirm</StepIndicator>
              <StoryText>
                As a {selectedFaction?.name} {selectedClass?.name}, you begin
                your journey in{" "}
                {selectedHomeTown
                  ? selectedHomeTown.name || "your new home"
                  : "your new home"}
                . Your rival {rivalName || "Gary"} will be watching every step.
                Are you ready to begin?
              </StoryText>
            </ConfirmationContent>
            <NavigationContainer
              $centered
              data-testid="character-creation-navigation"
            >
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

  return <CreatorShell>{renderStep()}</CreatorShell>;
};

export default CharacterCreator;
