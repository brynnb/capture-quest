import { useEffect, useRef } from "react";
import CharacterCreator from "@components/CharacterCreator/CharacterCreator";
import useCharacterCreatorStore from "@stores/CharacterCreatorStore";
import useStaticDataStore from "@stores/StaticDataStore";
import LoadingScreen from "@components/LoadingScreen";
import { useMinimumLoadingTime } from "@hooks/useMinimumLoadingTime";

const CharacterCreatorPage = () => {
  const resetStore = useCharacterCreatorStore((state) => state.resetStore);
  const initializeDefaults = useCharacterCreatorStore(
    (state) => state.initializeDefaults,
  );
  const loadCharCreateData = useStaticDataStore(
    (state) => state.loadCharCreateData,
  );
  const isCharCreateLoaded = useStaticDataStore(
    (state) => state.isCharCreateLoaded,
  );
  const isLoadingCharCreate = useStaticDataStore(
    (state) => state.isLoadingCharCreate,
  );
  const hasReset = useRef(false);

  useEffect(() => {
    loadCharCreateData();
  }, [loadCharCreateData]);

  // Reset the character creator store when this page mounts (only once)
  // Then initialize with null faction/class to force selection
  useEffect(() => {
    if (isCharCreateLoaded && !hasReset.current) {
      hasReset.current = true;
      resetStore();
      initializeDefaults();
    }
  }, [isCharCreateLoaded, resetStore, initializeDefaults]);

  const showLoading = useMinimumLoadingTime(
    isLoadingCharCreate || !isCharCreateLoaded,
  );

  if (showLoading) {
    return (
      <LoadingScreen isIndeterminate message="Entering the Pokémon World..." />
    );
  }

  return (
    <>
      <CharacterCreator />
    </>
  );
};

export default CharacterCreatorPage;
