import { useState } from "react";
import { IS_LOCAL_DEV } from "@/config";
import AudioManager from "@/services/audio/AudioManager";
import useGameStatusStore from "@stores/GameStatusStore";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import { getOrCreateGuestToken } from "@/utils/guestToken";

export const GUEST_ACCOUNT_WARNING =
  "Guest accounts are tied to your browser. You will lose your characters and progress if used in private windows, if you clear local storage, or change browsers. If you later register, your guest characters will transfer to your new account.";

const initializeAudioForLogin = () => {
  const { sfxVolume, ambientVolume, musicVolume, isMuted } =
    useGameStatusStore.getState();

  return AudioManager.initialize({
    sfx: sfxVolume,
    ambient: ambientVolume,
    music: musicVolume,
    muted: isMuted,
  }).catch((e) =>
    console.warn("[Login] Audio initialization failed (non-fatal):", e),
  );
};

export const useGuestLogin = () => {
  const isGuestConnecting = useCharacterSelectStore(state => state.accountOperation === "authenticate");
  const guestLoginError = useCharacterSelectStore(state => state.accountError);
  const [showGuestPopup, setShowGuestPopup] = useState(false);

  const performGuestLogin = async () => {
    if (useCharacterSelectStore.getState().accountOperation || useCharacterSelectStore.getState().isEnteringWorld) return;
    setShowGuestPopup(false);
    void initializeAudioForLogin();
    await useCharacterSelectStore.getState().authenticate({kind: "guest", token: getOrCreateGuestToken()});
  };
  const requestGuestLogin = async () => {
    if (useCharacterSelectStore.getState().accountOperation) return;
    if (IS_LOCAL_DEV) await performGuestLogin();
    else setShowGuestPopup(true);
  };
  return {
    isGuestConnecting, guestLoginError, showGuestPopup,
    requestGuestLogin, performGuestLogin,
    cancelGuestLogin: () => setShowGuestPopup(false),
  };
};
