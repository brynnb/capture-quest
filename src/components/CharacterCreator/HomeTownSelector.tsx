import { useEffect } from "react";
import useCharacterStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import styled from "styled-components";
import AudioManager from "@/services/audio/AudioManager";
import { sfxPathForConstant } from "@/services/audio/pokemonMusic";
import CompactChoiceList from "./CompactChoiceList";

const SelectorShell = styled.div`
  display: flex;
  width: 100%;
  max-width: 1290px;
  min-width: 0;
  flex-direction: column;

  @media (max-width: 900px), (pointer: coarse) {
    height: 100%;
    min-height: 0;
    flex: 1 1 auto;
  }
`;

const TownGrid = styled.div`
  display: grid;
  width: 100%;
  min-width: 0;
  grid-template-columns: 350px minmax(0, 1fr);
  gap: 45px;

  @media (max-width: 900px), (pointer: coarse) {
    grid-template-columns: minmax(0, 1fr);
    gap: 14px;
  }
`;

const DesktopTownContent = styled.div`
  display: contents;

  @media (max-width: 900px), (pointer: coarse) {
    display: none;
  }
`;

const MobileOnly = styled.div`
  display: none;

  @media (max-width: 900px), (pointer: coarse) {
    display: flex;
    width: 100%;
    height: 100%;
    min-height: 0;
    flex: 1 1 auto;
  }
`;

const ScrollableZones = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 350px;
  max-height: 800px;
  overflow-y: auto;
  padding-right: 10px;

  @media (max-width: 900px), (pointer: coarse) {
    min-width: 0;
    width: 100%;
    max-height: 240px;
    padding-right: 4px;
  }

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

  @media (max-width: 900px), (pointer: coarse) {
    width: 100%;
    min-height: 44px;
    height: auto;
    padding: 8px 10px;
    font-size: 18px;
  }

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

  @media (max-width: 900px), (pointer: coarse) {
    margin-bottom: 12px;
    font-size: 28px;
  }
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

  @media (max-width: 900px), (pointer: coarse) {
    min-height: 180px;
    padding: 18px;
    font-size: 16px;
  }
`;

const ZoneText = styled.div`
  font-size: 24px;
  text-align: left;
  color: #2e2f66;

  @media (max-width: 900px), (pointer: coarse) {
    font-size: 17px;
  }
`;

const TownName = styled.div`
  padding-bottom: 10px;
  color: #4a4ba6;
  border-bottom: 4px solid #ffccd9;
  font-size: 24px;
  font-weight: bold;
  text-transform: uppercase;

  @media (max-width: 900px), (pointer: coarse) {
    font-size: 19px;
  }
`;

const EmptyDescription = styled.div`
  padding-top: 100px;
  opacity: 0.5;
  text-align: center;

  @media (max-width: 900px), (pointer: coarse) {
    padding-top: 42px;
  }
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
    <SelectorShell>
      <DesktopTownContent>
        <Title>Choose Your Home City</Title>

        <TownGrid>
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

          <DescriptionBox>
            {selectedHomeTown ? (
              <div
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: "20px",
                }}
              >
                <TownName>{selectedHomeTown.name}</TownName>
                <ZoneText>
                  {selectedHomeTown.description ||
                    "A bustling city in the heart of the region, full of mystery and opportunity."}
                </ZoneText>
              </div>
            ) : (
              <EmptyDescription>
                Select a home city to view details
              </EmptyDescription>
            )}
          </DescriptionBox>
        </TownGrid>
      </DesktopTownContent>
      <MobileOnly>
        <CompactChoiceList
          title="Choose Your Home City"
          choices={uniqueHomeTowns.map((town) => ({
            id: town.mapId,
            label: town.name,
            description:
              town.description || "A new home full of mystery and opportunity.",
          }))}
          selectedId={selectedHomeTown?.mapId}
          onSelect={(id) => onSelectMap(Number(id))}
        />
      </MobileOnly>
    </SelectorShell>
  );
};

export default HomeTownSelector;
