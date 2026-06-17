import { useState } from "react";
import { loginGuest } from "@/services/authService";
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

const waitForAudioOrContinue = (audioInitPromise: Promise<void>) =>
  Promise.race([
    audioInitPromise,
    new Promise((resolve) => setTimeout(resolve, 2000)),
  ]);

export const useGuestLogin = () => {
  const [isGuestConnecting, setIsGuestConnecting] = useState(false);
  const [guestLoginError, setGuestLoginError] = useState<string | null>(null);
  const [showGuestPopup, setShowGuestPopup] = useState(false);
  const { setIsLoading } = useCharacterSelectStore();

  const performGuestLogin = async () => {
    setShowGuestPopup(false);
    setIsGuestConnecting(true);
    setGuestLoginError(null);
    setIsLoading(true);

    const loginTimeout = setTimeout(() => {
      setGuestLoginError("Connection timed out. Please try again.");
      setIsGuestConnecting(false);
      setIsLoading(false);
    }, 15000);

    try {
      const guestToken = getOrCreateGuestToken();
      console.log(
        "[Login] Starting guest login with token:",
        guestToken.substring(0, 8) + "...",
      );

      const audioInitPromise = initializeAudioForLogin();
      await loginGuest(guestToken);
      await waitForAudioOrContinue(audioInitPromise);

      clearTimeout(loginTimeout);
      console.log("[Login] Guest login successful");
    } catch (err) {
      clearTimeout(loginTimeout);
      console.error("Login failed:", err);
      setGuestLoginError(err instanceof Error ? err.message : "Login failed");
      setIsGuestConnecting(false);
      setIsLoading(false);
    }
  };

  const requestGuestLogin = async () => {
    if (IS_LOCAL_DEV) {
      await performGuestLogin();
      return;
    }
    setShowGuestPopup(true);
  };

  return {
    isGuestConnecting,
    guestLoginError,
    showGuestPopup,
    requestGuestLogin,
    performGuestLogin,
    cancelGuestLogin: () => setShowGuestPopup(false),
  };
};
