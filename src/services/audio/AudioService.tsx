import { useEffect } from "react";
import useGameStatusStore from "@stores/GameStatusStore";
import useGameScreenStore from "@stores/GameScreenStore";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import useStaticDataStore from "@stores/StaticDataStore";
import AudioManager from "./AudioManager";
import {
  battleMusicTrackForState,
  bikeMusicTrack,
  DEFAULT_WORLD_MUSIC,
  musicTrackForMapId,
  surfingMusicTrack,
  TITLE_MUSIC,
} from "./pokemonMusic";
import usePokeBattleStore from "@stores/PokeBattleStore";
import useAudioActivityStore from "@stores/AudioActivityStore";

/**
 * AudioService bridges the Zustand stores with the static AudioManager.
 * It's implemented as a "headless" component that lives at the top level of the app.
 */
const AudioService = () => {
  const currentMapId = useGameStatusStore((state) => state.currentMap);
  const getMapNameById = useGameStatusStore((state) => state.getMapNameById);
  const isMuted = useGameStatusStore((state) => state.isMuted);
  const sfxVolume = useGameStatusStore((state) => state.sfxVolume);
  const ambientVolume = useGameStatusStore((state) => state.ambientVolume);
  const musicVolume = useGameStatusStore((state) => state.musicVolume);

  const currentScreen = useGameScreenStore((state) => state.currentScreen);
  const isLoadingCharacters = useCharacterSelectStore(
    (state) => state.isLoading,
  );
  const isMapLoading = useGameStatusStore((state) => state.isMapLoading);
  const isStaticDataLoaded = useStaticDataStore((state) => state.isLoaded);
  const isCharCreateDataLoaded = useStaticDataStore(
    (state) => state.isCharCreateLoaded,
  );
  const areModelsPreloaded = useStaticDataStore(
    (state) => state.areModelsPreloaded,
  );
  const isInBattle = usePokeBattleStore((state) => state.isInBattle);
  const battleType = usePokeBattleStore((state) => state.battleType);
  const trainerClass = usePokeBattleStore((state) => state.trainerClass);
  const isSurfing = useAudioActivityStore((state) => state.isSurfing);
  const isBicycleActive = useAudioActivityStore((state) => state.isBicycleActive);
  const travelMapId = useAudioActivityStore((state) => state.travelMapId);
  const battleVictoryTrack = useAudioActivityStore(
    (state) => state.battleVictoryTrack,
  );
  const setBattleVictoryTrack = useAudioActivityStore(
    (state) => state.setBattleVictoryTrack,
  );
  const resetTravelAudio = useAudioActivityStore((state) => state.resetTravelAudio);

  // Handle Loading and Character Select Music
  useEffect(() => {
    // Don't try to play music until AudioManager is initialized (by user interaction on Welcome modal)
    if (!AudioManager.isInitialized()) {
      return;
    }

    // State 1: "Preparing..." Phase (Initial and Asset Preloading)
    // We play loading.mp3 as long as we are waiting for the server OR preloading 3D assets.
    // For character select, we only need static data and models.
    // For character create, we also need char create data.
    const isPreparingCharSelect =
      currentScreen === "characterSelect" &&
      (!isStaticDataLoaded || !areModelsPreloaded);
    const isPreparingCharCreate =
      currentScreen === "characterCreate" &&
      (!isStaticDataLoaded || !isCharCreateDataLoaded || !areModelsPreloaded);

    if (isLoadingCharacters || isPreparingCharSelect || isPreparingCharCreate) {
      AudioManager.playMusic(TITLE_MUSIC);
      return;
    }

    // State 2: Actual Selection Phase + "Enter World" Loading Screen
    // Once assets are ready, we switch to character select music.
    // It should persist until the zone is fully loaded and we are in-game.
    // For zone loading: only continue title.mp3 if it's already playing (initial enter world).
    // Don't restart it for in-game zone changes.
    const currentTrack = AudioManager.getRequestedMusicTrack();
    const isEnteringWorldFromSelect =
      currentScreen === "game" &&
      isMapLoading &&
      currentTrack === TITLE_MUSIC;

    const isSelectingOrEnteringWorld =
      currentScreen === "characterSelect" ||
      currentScreen === "characterCreate" ||
      isEnteringWorldFromSelect;

    if (isSelectingOrEnteringWorld) {
      AudioManager.playMusic(TITLE_MUSIC);
      return;
    }

    // State 3: Enter Game World (loading finished)
    // Switch from character select music to the world theme
    if (currentScreen === "game" && !isMapLoading) {
      if (AudioManager.getRequestedMusicTrack() === TITLE_MUSIC) {
        AudioManager.playMusic(DEFAULT_WORLD_MUSIC);
      }
    }

    // State 4: Default title/auth screens - title music plays continuously
    if (
      currentScreen === "title" ||
      currentScreen === "login" ||
      currentScreen === "register"
    ) {
      if (!isLoadingCharacters) {
        resetTravelAudio();
        AudioManager.playMusic(TITLE_MUSIC);
        AudioManager.stopAllAmbients();
      }
    }
  }, [
    currentScreen,
    isLoadingCharacters,
    isMapLoading,
    isStaticDataLoaded,
    isCharCreateDataLoaded,
    areModelsPreloaded,
    resetTravelAudio,
  ]);

  useEffect(() => {
    if (!AudioManager.isInitialized()) return;
    if (currentScreen !== "game" || !isInBattle) return;

    AudioManager.playMusic(battleMusicTrackForState(battleType, trainerClass));
  }, [battleType, currentScreen, isInBattle, trainerClass]);

  // Handle Zone Transitions (Ambient/Music)
  useEffect(() => {
    if (currentMapId === null) {
      AudioManager.stopAllAmbients();
      // Don't stop music here anymore, the loading logic above handles it
      return;
    }

    const mapName = getMapNameById(currentMapId);
    if (mapName) {
      console.log(
        `[AudioService] Map changed to ${mapName}, loading assets...`,
      );
      AudioManager.loadZone(mapName.toLowerCase());
    }
  }, [currentMapId, getMapNameById]);

  // Handle travel/map music from imported Red/Blue metadata.
  useEffect(() => {
    if (!AudioManager.isInitialized()) return;
    if (currentScreen !== "game" || isMapLoading || currentMapId === null || isInBattle) {
      return;
    }

    if (battleVictoryTrack) {
      AudioManager.playMusic(battleVictoryTrack);
      const timeout = window.setTimeout(() => {
        setBattleVictoryTrack(null);
      }, 5000);
      return () => window.clearTimeout(timeout);
    }

    if (isSurfing) {
      AudioManager.playMusic(surfingMusicTrack());
      return;
    }

    if (isBicycleActive && currentMapId === 9999) {
      AudioManager.playMusic(bikeMusicTrack());
      return;
    }

    const effectiveMapId = travelMapId ?? currentMapId;
    AudioManager.playMusic(musicTrackForMapId(effectiveMapId));
  }, [
    battleVictoryTrack,
    currentMapId,
    currentScreen,
    isBicycleActive,
    isInBattle,
    isMapLoading,
    isSurfing,
    setBattleVictoryTrack,
    travelMapId,
  ]);

  // Handle volume and mute sync
  useEffect(() => {
    AudioManager.setMuted(isMuted);
  }, [isMuted]);

  useEffect(() => {
    AudioManager.setSFXVolume(sfxVolume);
  }, [sfxVolume]);

  useEffect(() => {
    AudioManager.setAmbientVolume(ambientVolume);
  }, [ambientVolume]);

  useEffect(() => {
    AudioManager.setMusicVolume(musicVolume);
  }, [musicVolume]);

  return null; // Headless component
};

export default AudioService;
