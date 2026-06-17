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
`;

const ContentContainer = styled.div`
  display: grid;
  grid-template-columns: 840px 500px;
  gap: 10px;
  padding: 40px;
`;

const LeftColumn = styled.div`
  display: flex;
  flex-direction: column;
  gap: 15px;
`;

const CharacterList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 5px;
  flex: 1;
  align-items: center;
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
`;

const BottomButtons = styled.div`
  display: flex;
  flex-direction: row;
  margin-top: 10px;
  justify-content: space-between;
  width: 690px;
  align-self: center;
`;

const ButtonGroup = styled.div`
  display: flex;
  gap: 10px;
`;

const RightColumn = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
`;

const CharacterPreview = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
`;

const CharacterImageContainer = styled.div`
  position: relative;
  width: 500px;
  height: 750px;
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
  const { getClassById, loadStaticData } =
    useStaticDataStore();
  const gameStatus = useGameStatusStore();
  const { initializeMaps } = gameStatus;

  useEffect(() => {
    const loadData = async () => {
      await loadStaticData();
      await initializeMaps();
    };
    loadData();
  }, [loadStaticData, initializeMaps]);

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

  // Create array of 8 slots, filling empty ones with null
  const characterSlots = Array.from(
    { length: 8 },
    (_, i) => characters[i] || null,
  );

  return (
    <Wrapper>
      <ContentContainer>
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
