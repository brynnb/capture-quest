import { useEffect, useState } from "react";
import styled from "styled-components";
import useGameScreenStore from "@stores/GameScreenStore";
import SelectionButton from "@components/Interface/SelectionButton";
import { WorldSocket } from "@/net";
import useCharacterSelectStore, {
  CharacterSelectEntry,
} from "@stores/CharacterSelectStore";
import usePlayerCharacterStore from "@stores/PlayerCharacterStore";
import useGameStatusStore from "@stores/GameStatusStore";
import useStaticDataStore from "@stores/StaticDataStore";
import PopupWindow from "@components/Interface/PopupWindow";

const Wrapper = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  overflow: auto;
`;

const ContentContainer = styled.div`
  display: grid;
  width: min(1360px, 100%);
  grid-template-columns: minmax(0, 1.68fr) minmax(320px, 1fr);
  gap: 10px;
  padding: clamp(20px, 3vw, 40px);
  box-sizing: border-box;

  @media (max-width: 900px), (pointer: coarse) {
    width: 100%;
    grid-template-columns: minmax(0, 1fr);
    padding: 64px 16px 20px;
    box-sizing: border-box;
  }

  @media (max-height: 650px) and (max-width: 900px),
    (max-height: 650px) and (pointer: coarse) {
    padding: 10px 10px 8px;
  }
`;

const DesktopOnly = styled.div`
  display: contents;

  @media (max-width: 900px), (pointer: coarse) {
    display: none;
  }
`;

const MobileCharacterSelect = styled.div`
  display: none;

  @media (max-width: 900px), (pointer: coarse) {
    display: flex;
    width: 100%;
    max-width: 440px;
    min-height: 0;
    margin: 0 auto;
    flex-direction: column;
    align-items: center;
    gap: 10px;
  }

  @media (max-height: 650px) and (max-width: 900px),
    (max-height: 650px) and (pointer: coarse) {
    gap: 5px;
  }
`;

const MobileTitle = styled.h2`
  margin: 0;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-size: clamp(25px, 8vw, 34px);
  font-weight: 900;
  line-height: 1;
  text-align: center;
  text-transform: uppercase;

  @media (max-height: 650px) {
    font-size: 24px;
  }
`;

const MobileCarousel = styled.div`
  position: relative;
  width: 100%;
`;

const MobilePreview = styled.div`
  position: relative;
  width: min(100%, 330px);
  height: clamp(220px, 38dvh, 320px);
  margin: 0 auto;
  overflow: hidden;
  background-image: url("/assets/charselectbg.png");
  background-position: center;
  background-size: cover;
  border: 3px solid #4a4ba6;
  border-radius: 20px;
  box-shadow:
    0 8px 0 rgba(74, 75, 166, 0.78),
    0 16px 34px rgba(0, 0, 0, 0.22);

  @media (max-height: 650px) {
    width: min(100%, 250px);
    height: 150px;
    border-radius: 14px;
    box-shadow: 0 5px 0 rgba(74, 75, 166, 0.78);
  }
`;

const MobileTrainerImage = styled.img`
  position: absolute;
  right: 10%;
  bottom: 0;
  left: 10%;
  width: 80%;
  height: 94%;
  object-fit: contain;
`;

const MobileArrow = styled.button<{ $side: "left" | "right" }>`
  position: absolute;
  top: 50%;
  ${({ $side }) => ($side === "left" ? "left: -2px;" : "right: -2px;")}
  z-index: 3;
  width: 50px;
  height: 58px;
  padding: 0;
  color: #2e2f66;
  background: rgba(192, 193, 255, 0.94);
  border: 3px solid #4a4ba6;
  border-radius: 15px;
  box-shadow: 0 4px 0 #4a4ba6;
  font:
    900 34px/1 "Outfit",
    sans-serif;
  transform: translateY(-50%);
  touch-action: manipulation;

  &:disabled {
    opacity: 0.35;
  }

  &:active:not(:disabled) {
    box-shadow: 0 2px 0 #4a4ba6;
    transform: translateY(calc(-50% + 2px));
  }

  @media (max-height: 650px) {
    width: 44px;
    height: 48px;
    border-radius: 12px;
    font-size: 29px;
  }
`;

const MobileCharacterInfo = styled.div`
  width: 100%;
  padding: 9px 12px;
  box-sizing: border-box;
  color: #2e2f66;
  background: rgba(255, 236, 241, 0.94);
  border: 3px solid #ffccd9;
  border-radius: 16px;
  box-shadow: 0 4px 0 rgba(255, 204, 217, 0.86);
  font-family: "Outfit", sans-serif;
  text-align: center;

  @media (max-height: 650px) {
    padding: 6px 10px;
    border-width: 2px;
    border-radius: 12px;
    box-shadow: 0 2px 0 rgba(255, 204, 217, 0.86);
  }
`;

const MobileCharacterName = styled.strong`
  display: block;
  overflow: hidden;
  font-size: 24px;
  font-weight: 900;
  line-height: 1.1;
  text-overflow: ellipsis;
  white-space: nowrap;

  @media (max-height: 650px) {
    font-size: 20px;
  }
`;

const MobileCharacterMeta = styled.div`
  margin-top: 4px;
  font-size: 14px;
  font-weight: 800;
  line-height: 1.3;

  @media (max-height: 650px) {
    margin-top: 2px;
    font-size: 12px;
    line-height: 1.2;
  }
`;

const MobileCounter = styled.div`
  margin-top: 5px;
  color: #4a4ba6;
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.08em;
  text-transform: uppercase;

  @media (max-height: 650px) {
    margin-top: 3px;
    font-size: 10px;
  }
`;

const MobilePrimaryActions = styled.div`
  display: grid;
  width: 100%;
  grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
  gap: 8px;

  & > button {
    width: 100%;
    height: 52px;
    min-width: 0;
    padding: 0 8px;
    border-radius: 14px;
    font-size: clamp(14px, 4.5vw, 18px);
  }

  @media (max-height: 650px) {
    & > button {
      height: 46px;
      font-size: 14px;
    }
  }
`;

const MobileSingleAction = styled.div`
  width: 100%;

  & > button {
    width: 100%;
    height: 52px;
    min-width: 0;
    padding: 0 8px;
    border-radius: 14px;
    font-size: 18px;
  }

  @media (max-height: 650px) {
    & > button {
      height: 46px;
      font-size: 14px;
    }
  }
`;

const MobileSecondaryActions = styled.div`
  display: flex;
  width: 100%;
  justify-content: center;
  gap: 8px;

  & > button {
    width: auto;
    height: 42px;
    min-width: 96px;
    padding: 0 12px;
    border-width: 3px;
    border-radius: 12px;
    font-size: 14px;
  }

  @media (max-height: 650px) {
    & > button {
      height: 36px;
      font-size: 12px;
    }
  }
`;

const LeftColumn = styled.div`
  display: flex;
  flex-direction: column;
  gap: 15px;
  min-width: 0;
`;

const CharacterList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 5px;
  flex: 1;
  align-items: center;

  & > button {
    max-width: 100%;
  }
`;

const Title = styled.h2`
  font-family: "Outfit", sans-serif;
  text-transform: uppercase;
  font-weight: 900;
  font-size: 50px;
  text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.5);
  text-align: center;
  margin: 0 0 10px 0;
  color: #2e2f66;
  width: 100%;

  @media (max-width: 600px), (pointer: coarse) {
    font-size: 32px;
  }
`;

const BottomButtons = styled.div`
  display: flex;
  flex-direction: row;
  margin-top: 10px;
  justify-content: space-between;
  width: min(690px, 100%);
  align-self: center;

  @media (max-width: 900px), (pointer: coarse) {
    width: 100%;
    flex-wrap: wrap;
    gap: 8px;
  }
`;

const ButtonGroup = styled.div`
  display: flex;
  gap: 10px;

  @media (max-width: 600px), (pointer: coarse) {
    width: 100%;
    flex-wrap: wrap;

    & > button {
      min-width: 0;
      flex: 1;
    }
  }
`;

const RightColumn = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-width: 0;
`;

const CharacterPreview = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  width: 100%;
  max-width: 500px;
`;

const CharacterImageContainer = styled.div`
  position: relative;
  width: 100%;
  max-width: 500px;
  height: 750px;
  box-sizing: border-box;
  display: flex;
  align-items: flex-end; /* Align images to bottom */
  justify-content: center;
  border: 4px solid #4a4ba6;
  border-radius: 24px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.3);
  overflow: hidden;
  background-image: url("/assets/charselectbg.png");
  background-size: cover;
  background-position: center;

  @media (max-width: 900px), (pointer: coarse) {
    width: min(100%, 500px);
    height: min(44vh, 420px);
  }
`;

const CharacterImage = styled.img`
  max-width: 100%;
  max-height: 100%;
`;

const CharacterLabel = styled.div`
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  font-family: "Outfit", sans-serif;
  font-size: 24px;
  font-weight: 800;
  color: white;
  text-align: center;
  text-shadow: 2px 2px 8px rgba(0, 0, 0, 0.9);
  line-height: 1.4;
  z-index: 10;
  background: rgba(46, 47, 102, 0.6);
  padding: 8px 20px;
  border-radius: 12px;
  backdrop-filter: blur(4px);
  border: 1px solid rgba(255, 255, 255, 0.2);
`;

const LocationInfo = styled.div`
  font-family: "Outfit", sans-serif;
  font-size: 20px;
  color: #2e2f66;
  text-align: center;
  line-height: 1.6;
  font-weight: 800;
  margin-top: 20px;
`;

const EmptyMessage = styled.div`
  font-family: "Times New Roman", Times, serif;
  font-size: 18px;
  color: white;
  text-align: center;
  padding: 40px;
  text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.8);
`;

function getTrainerImage(gender?: number): string {
  switch (gender) {
    case 0:
      return "/assets/trainerm.png";
    case 1:
      return "/assets/trainerf.png";
    case 2:
      return "/assets/trainernb.png";
    default:
      return "/assets/trainernb.png";
  }
}

const CharacterSelectPage = () => {
  const { setScreen } = useGameScreenStore();
  const [deleteTarget, setDeleteTarget] = useState<CharacterSelectEntry | null>(
    null,
  );
  const { characters, selectedCharacter, setSelectedCharacter, isLoading } =
    useCharacterSelectStore();
  usePlayerCharacterStore();
  const { getClassById, loadStaticData } = useStaticDataStore();
  const gameStatus = useGameStatusStore();
  const { initializeMaps } = gameStatus;

  useEffect(() => {
    const loadData = async () => {
      await loadStaticData();
      await initializeMaps();
    };
    loadData();
  }, [loadStaticData, initializeMaps]);

  useEffect(() => {
    if (characters.length === 0) return;

    const selectionStillExists = characters.some(
      (character) => character.name === selectedCharacter?.name,
    );
    if (!selectionStillExists) {
      setSelectedCharacter(characters[0]);
    }
  }, [characters, selectedCharacter?.name, setSelectedCharacter]);

  const getClassName = (classId: number) => {
    const charClass = getClassById(classId);
    return charClass?.name || "Unknown";
  };

  const handleSelectCharacter = (character: CharacterSelectEntry) => {
    setSelectedCharacter(character);
  };

  const handleEnterWorld = async () => {
    if (!selectedCharacter) return;
    useGameStatusStore.getState().setIsMapLoading(true);
    const success = await useCharacterSelectStore
      .getState()
      .enterWorld(selectedCharacter.name);
    if (!success) {
      useGameStatusStore.getState().setIsMapLoading(false);
      alert("Could not enter world. Please try again.");
    }
  };

  const handleCreateNew = () => {
    setScreen("characterCreate");
  };

  const handleLogout = () => {
    // Navigate home, logout handled by GameScreenManager or similar
    setScreen("title");
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;

    // Use centralized store action which also handles local removal
    const success = await useCharacterSelectStore
      .getState()
      .deleteCharacter(deleteTarget.name);
    if (!success) {
      alert("Failed to delete character.");
    }
    setDeleteTarget(null);
  };

  const handleCancelDelete = () => {
    setDeleteTarget(null);
  };

  // Redirect to login if not connected
  useEffect(() => {
    if (!WorldSocket.isConnected) {
      setScreen("title");
    }
  }, [setScreen]);

  if (isLoading) {
    return (
      <Wrapper>
        <EmptyMessage>Loading Characters...</EmptyMessage>
      </Wrapper>
    );
  }

  const getMapName = (mapId?: number) => {
    if (typeof mapId !== "number") return "Unknown";
    return gameStatus.getMapNameById(mapId) || `Map ${mapId}`;
  };

  const hasNoCharacters = characters.length === 0;
  const selectedCharacterIndex = selectedCharacter
    ? characters.findIndex(
        (character) => character.name === selectedCharacter.name,
      )
    : -1;

  const cycleCharacter = (direction: -1 | 1) => {
    if (characters.length === 0) return;
    const currentIndex =
      selectedCharacterIndex >= 0 ? selectedCharacterIndex : 0;
    const nextIndex =
      (currentIndex + direction + characters.length) % characters.length;
    handleSelectCharacter(characters[nextIndex]);
  };

  // Create array of 8 slots, filling empty ones with null
  const characterSlots = Array.from(
    { length: 8 },
    (_, i) => characters[i] || null,
  );

  return (
    <Wrapper>
      <ContentContainer>
        <DesktopOnly>
          {/* Left Column - Character Selection */}
          <LeftColumn>
            <CharacterList>
              <Title>SELECT A CHARACTER</Title>
              {characterSlots.map((character, index) => (
                <SelectionButton
                  key={character?.name || `empty-${index}`}
                  $isSelected={
                    character !== null &&
                    selectedCharacter?.name === character?.name
                  }
                  onClick={() => {
                    if (character) {
                      handleSelectCharacter(character);
                    } else {
                      handleCreateNew();
                    }
                  }}
                  $width="690px"
                >
                  {character ? character.name : "CREATE NEW CHARACTER"}
                </SelectionButton>
              ))}
            </CharacterList>

            <BottomButtons>
              <ButtonGroup>
                <SelectionButton
                  onClick={handleLogout}
                  $isSelected={false}
                  $width="110px"
                >
                  QUIT
                </SelectionButton>
                <SelectionButton
                  onClick={() => {
                    if (selectedCharacter) {
                      setDeleteTarget(selectedCharacter);
                    }
                  }}
                  $isSelected={false}
                  $isDisabled={!selectedCharacter}
                  disabled={!selectedCharacter}
                  $width="110px"
                >
                  DELETE
                </SelectionButton>
              </ButtonGroup>
              <SelectionButton
                onClick={handleEnterWorld}
                $isSelected={false}
                $isDisabled={!selectedCharacter}
                disabled={!selectedCharacter}
                $width="300px"
              >
                ENTER WORLD
              </SelectionButton>
            </BottomButtons>
          </LeftColumn>

          {/* Right Column - Character Preview */}
          <RightColumn>
            {selectedCharacter ? (
              <>
                <CharacterPreview>
                  <CharacterImageContainer>
                    {/* Trainer Portrait (Layer 2) */}
                    <CharacterImage
                      src={getTrainerImage(selectedCharacter.gender)}
                      alt="Trainer Preview"
                      style={{
                        zIndex: 5,
                        position: "absolute",
                        bottom: "40px",
                        height: "80%",
                        objectFit: "contain",
                      }}
                    />

                    <CharacterLabel>
                      {getClassName(selectedCharacter.class)}
                    </CharacterLabel>
                  </CharacterImageContainer>
                </CharacterPreview>
                <LocationInfo>
                  CURRENT LOCATION
                  <br />
                  {getMapName(selectedCharacter.mapId)}
                </LocationInfo>
              </>
            ) : hasNoCharacters ? (
              <CharacterPreview>
                <CharacterImageContainer>
                  <CharacterImage
                    src={getTrainerImage()}
                    alt="Trainer Preview"
                    style={{
                      zIndex: 5,
                      position: "absolute",
                      bottom: "40px",
                      height: "80%",
                      objectFit: "contain",
                    }}
                  />
                </CharacterImageContainer>
              </CharacterPreview>
            ) : (
              <EmptyMessage>Select a character to view details</EmptyMessage>
            )}
          </RightColumn>
        </DesktopOnly>

        <MobileCharacterSelect>
          <MobileTitle>Select a Character</MobileTitle>
          <MobileCarousel>
            <MobilePreview>
              <MobileTrainerImage
                src={getTrainerImage(selectedCharacter?.gender)}
                alt={
                  selectedCharacter
                    ? `${selectedCharacter.name} trainer preview`
                    : "Trainer preview"
                }
              />
            </MobilePreview>
            {!hasNoCharacters && (
              <>
                <MobileArrow
                  type="button"
                  $side="left"
                  aria-label="Previous character"
                  disabled={characters.length <= 1}
                  onClick={() => cycleCharacter(-1)}
                >
                  ‹
                </MobileArrow>
                <MobileArrow
                  type="button"
                  $side="right"
                  aria-label="Next character"
                  disabled={characters.length <= 1}
                  onClick={() => cycleCharacter(1)}
                >
                  ›
                </MobileArrow>
              </>
            )}
          </MobileCarousel>

          <MobileCharacterInfo>
            <MobileCharacterName>
              {selectedCharacter?.name || "Your adventure starts here"}
            </MobileCharacterName>
            <MobileCharacterMeta>
              {selectedCharacter ? (
                <>
                  {getClassName(selectedCharacter.class)}
                  <br />
                  Current location: {getMapName(selectedCharacter.mapId)}
                </>
              ) : (
                "Create your first trainer to begin."
              )}
            </MobileCharacterMeta>
            {!hasNoCharacters && (
              <MobileCounter aria-live="polite">
                Character {selectedCharacterIndex + 1} of {characters.length}
              </MobileCounter>
            )}
          </MobileCharacterInfo>

          {selectedCharacter ? (
            <MobilePrimaryActions>
              <SelectionButton onClick={handleCreateNew} $isSelected={false}>
                Create New
              </SelectionButton>
              <SelectionButton onClick={handleEnterWorld} $isSelected={false}>
                Enter World
              </SelectionButton>
            </MobilePrimaryActions>
          ) : (
            <MobileSingleAction>
              <SelectionButton onClick={handleCreateNew} $isSelected={false}>
                Create New Character
              </SelectionButton>
            </MobileSingleAction>
          )}

          <MobileSecondaryActions>
            <SelectionButton onClick={handleLogout} $isSelected={false}>
              Quit
            </SelectionButton>
            <SelectionButton
              onClick={() => {
                if (selectedCharacter) setDeleteTarget(selectedCharacter);
              }}
              $isSelected={false}
              $isDisabled={!selectedCharacter}
              disabled={!selectedCharacter}
            >
              Delete
            </SelectionButton>
          </MobileSecondaryActions>
        </MobileCharacterSelect>
      </ContentContainer>

      {/* Delete Confirmation Modal */}
      <PopupWindow
        isOpen={deleteTarget !== null}
        title="Delete Character?"
        message={`Are you sure you want to delete ${deleteTarget?.name}? This action cannot be undone.`}
        okText="DELETE"
        cancelText="CANCEL"
        onOk={handleConfirmDelete}
        onCancel={handleCancelDelete}
      />
    </Wrapper>
  );
};

export default CharacterSelectPage;
