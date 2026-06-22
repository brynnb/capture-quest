import { useEffect } from "react";
import useCharacterStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import styled from "styled-components";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";

const ScrollableZones = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 350px;
  max-height: 800px;
  overflow-y: auto;
  padding-right: 10px;

  /* Scrollbar styling */
  &::-webkit-scrollbar {
    width: 6px;
  }
  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.05);
  }
  &::-webkit-scrollbar-thumb {
    background: #4a4ba6;
    border-radius: 3px;
  }
`;

const ZoneButton = styled.button<{
  $isSelected: boolean;
  $isDisabled?: boolean;
}>`
  width: 345px;
  height: 50px;
  background-color: ${({ $isSelected }) =>
    $isSelected ? "#a7edfe" : "#c0c1ff"};
  border: 3px solid #4a4ba6;
  border-radius: 12px;
  cursor: ${({ $isDisabled }) => ($isDisabled ? "not-allowed" : "pointer")};
  outline: none;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
  text-transform: none;
  opacity: ${({ $isDisabled }) => ($isDisabled ? 0.5 : 1)};
  white-space: nowrap;
  font-size: 24px;
  text-overflow: ellipsis;
  overflow: hidden;
  box-shadow: 0 4px 0 #4a4ba6;
  transition: all 0.1s ease-in-out;
  margin-bottom: 6px;

  &:focus {
    outline: none;
  }

  &:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 6px 0 #4a4ba6;
    background-color: #d1d2ff;
  }

  &:active:not(:disabled) {
    transform: translateY(2px);
    box-shadow: 0 2px 0 #4a4ba6;
  }
`;

const Title = styled.h2`
  font-family: "Outfit", sans-serif;
  text-transform: none;
  font-weight: 800;
  font-size: 42px;
  text-align: center;
  margin: 0 0 20px 0;
  color: #2e2f66;
  width: 100%;
`;

const DescriptionBox = styled.div`
  flex: 1;
  background-color: rgba(255, 236, 241, 0.9);
  backdrop-filter: blur(10px);
  border: 4px solid #ffccd9;
  border-radius: 20px;
  padding: 30px;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-size: 20px;
  line-height: 1.6;
  min-height: 400px;
  overflow-y: auto;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
`;

const ZoneText = styled.div`
  font-size: 24px;
  text-align: left;
  color: #2e2f66;
`;

const HomeTownSelector = () => {
  const { selectedHomeTown, setSelectedHomeTown } = useCharacterStore();
  const { homeTowns } = useStaticDataStore();

  const onSelectMap = (mapId: number) => {
    const sfx = sfxPathForConstant("SFX_PRESS_AB");
    if (sfx) AudioManager.playSFX(sfx);
    const homeTown = homeTowns.find((town) => town.mapId === mapId);
    if (homeTown) {
      setSelectedHomeTown(homeTown);
    }
  };

  useEffect(() => {
    if (!selectedHomeTown && homeTowns.length > 0) {
      setSelectedHomeTown(homeTowns[0]);
    }
  }, [homeTowns, selectedHomeTown, setSelectedHomeTown]);

  const uniqueHomeTowns = homeTowns.filter(
    (town, idx, arr) => arr.findIndex((t) => t.mapId === town.mapId) === idx,
  );

  return (
    <div
      style={{
        width: "100%",
        maxWidth: "1290px",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Title>Choose Your Home City</Title>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "350px 1fr",
          gap: "45px",
          width: "100%",
        }}
      >
        <ScrollableZones>
          {uniqueHomeTowns.map((town) => (
            <ZoneButton
              key={town.mapId}
              onClick={() => onSelectMap(town.mapId)}
              $isSelected={selectedHomeTown?.mapId === town.mapId}
            >
              {town.name}
            </ZoneButton>
          ))}
        </ScrollableZones>

        <DescriptionBox style={{ marginTop: "0px" }}>
          {selectedHomeTown ? (
            <div
              style={{ display: "flex", flexDirection: "column", gap: "20px" }}
            >
              <div
                style={{
                  fontWeight: "bold",
                  fontSize: "24px",
                  textTransform: "uppercase",
                  color: "#4a4ba6",
                  borderBottom: "4px solid #ffccd9",
                  paddingBottom: "10px",
                }}
              >
                {selectedHomeTown.name}
              </div>
              <ZoneText>
                {selectedHomeTown.description ||
                  "A bustling city in the heart of the region, full of mystery and opportunity."}
              </ZoneText>
            </div>
          ) : (
            <div
              style={{ opacity: 0.5, textAlign: "center", paddingTop: "100px" }}
            >
              Select a home city to view details
            </div>
          )}
        </DescriptionBox>
      </div>
    </div>
  );
};

export default HomeTownSelector;
