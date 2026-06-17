import { useEffect, useState, useCallback } from "react";
import styled from "styled-components";
import SelectionButton from "@components/Interface/SelectionButton";
import { login } from "@/services/authService";
import useCharacterSelectStore from "@stores/CharacterSelectStore";
import PopupWindow from "@components/Interface/PopupWindow";
import { getApiUrl } from "@/config";
import AudioManager from "@/services/audio/AudioManager";
import useGameStatusStore from "@stores/GameStatusStore";
import useGameScreenStore from "@stores/GameScreenStore";
import { GUEST_ACCOUNT_WARNING, useGuestLogin } from "@/hooks/useGuestLogin";

const Wrapper = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  box-sizing: border-box;
  background-image: url("/assets/animebgfull.jpg");
  background-size: cover;
  background-repeat: no-repeat;
  background-position: center;
`;

const CenterColumn = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;

  .selection-button {
    width: 460px;
  }
`;

const Logo = styled.img`
  max-width: 500px;
  height: auto;
`;

const FormPanel = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 500px;
  gap: 10px;
  padding: 36px 40px;
  box-sizing: border-box;
  background: rgba(192, 193, 255, 0.4);
  backdrop-filter: blur(10px);
  border: 4px solid #4a4ba6;
  border-radius: 30px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.2);
`;

const Title = styled.h1`
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  color: #2e2f66;
  font-size: 36px;
  margin: 0 0 16px;
`;

const InputGroup = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  margin-bottom: 14px;
`;

const InputLabel = styled.label`
  font-family: "Outfit", sans-serif;
  font-weight: 800;
  font-size: 16px;
  color: #2e2f66;
  text-transform: uppercase;
  width: 100%;
  text-align: center;
`;

const TextInput = styled.input`
  width: 100%;
  padding: 12px 15px;
  font-family: "Outfit", sans-serif;
  font-size: 20px;
  background: rgba(255, 255, 255, 0.9);
  border: 3px solid #4a4ba6;
  border-radius: 12px;
  color: #2e2f66;
  outline: none;
  box-sizing: border-box;
  text-align: center;
  transition: all 0.2s ease;

  &:focus {
    border-color: #a7edfe;
    background: #ffffff;
    box-shadow: 0 0 10px rgba(167, 237, 254, 0.3);
  }

  &:-webkit-autofill,
  &:-webkit-autofill:hover,
  &:-webkit-autofill:focus,
  &:-webkit-autofill:active {
    -webkit-box-shadow: 0 0 0 1000px #ffffff inset !important;
    -webkit-text-fill-color: #2e2f66 !important;
    transition: background-color 5000s ease-in-out 0s;
  }

  &::placeholder {
    color: #4a4ba6;
    opacity: 0.6;
  }
`;

const SecondaryActions = styled.div`
  display: flex;
  gap: 16px;

  .selection-button {
    width: 222px;
  }
`;

const OfflineText = styled.p`
  font-family: "Outfit", Inter, sans-serif;
  font-size: 18px;
  color: #4a4ba6;
  text-align: center;
  line-height: 1.25;
  margin: 0;

  &.offline {
    color: #ffaf84;
  }
`;

const OfflineWarning = styled(OfflineText)`
  color: #ffaf84;
  text-align: center;
`;

const StatusSlot = styled.div`
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: visible;
`;

const LoginPage = () => {
  const { setScreen } = useGameScreenStore();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isConnecting, setIsConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [serverStatus, setServerStatus] = useState<
    "checking" | "online" | "offline"
  >("checking");
  const { setIsLoading } = useCharacterSelectStore();
  const {
    isGuestConnecting,
    guestLoginError,
    showGuestPopup,
    requestGuestLogin,
    performGuestLogin,
    cancelGuestLogin,
  } = useGuestLogin();
  const isBusy = isConnecting || isGuestConnecting;
  const statusMessage =
    serverStatus === "offline" ? "Server is offline" : error || guestLoginError;

  const checkServerHealth = useCallback(async (opts: { silent?: boolean } = {}) => {
    const silent = opts.silent ?? false;
    if (!silent) {
      setServerStatus("checking");
    }
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 1500);

    try {
      const res = await fetch(getApiUrl("/online"), {
        method: "GET",
        cache: "no-store",
        signal: controller.signal,
      });
      setServerStatus(res.ok ? "online" : "offline");
    } catch {
      setServerStatus("offline");
    } finally {
      clearTimeout(timeout);
    }
  }, []);

  useEffect(() => {
    checkServerHealth();
    const interval = setInterval(() => {
      // Only do a silent check to avoid UI flicker during polling.
      // We still want the UI to transition online/offline quickly.
      void checkServerHealth({ silent: true });
    }, 5000);
    return () => clearInterval(interval);
  }, [checkServerHealth]);

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

  const handleAccountLogin = async () => {
    if (serverStatus !== "online") {
      setError("Server is offline");
      return;
    }

    if (!email.trim() || !password) {
      setError("Please enter your email and password");
      return;
    }

    setIsConnecting(true);
    setError(null);
    setIsLoading(true);

    const loginTimeout = setTimeout(() => {
      setError("Connection timed out. Please try again.");
      setIsConnecting(false);
      setIsLoading(false);
    }, 15000);

    try {
      const audioInitPromise = initializeAudioForLogin();
      await login(email.trim(), password);
      await waitForAudioOrContinue(audioInitPromise);
      clearTimeout(loginTimeout);
      console.log("[Login] Account login successful");
    } catch (err) {
      clearTimeout(loginTimeout);
      console.error("Login failed:", err);
      setError(err instanceof Error ? err.message : "Login failed");
      setIsConnecting(false);
      setIsLoading(false);
    }
  };

  const handleGuestLogin = async () => {
    if (serverStatus !== "online") {
      setError("Server is offline");
      return;
    }
    setError(null);
    await requestGuestLogin();
  };


  return (
    <Wrapper>
      <CenterColumn>
        <Logo src="/assets/capturequestlogo.png" alt="CaptureQuest" />
        <FormPanel>
          <Title>Login</Title>
          <InputGroup>
            <InputLabel>Email</InputLabel>
            <TextInput
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="Enter email address"
            />
          </InputGroup>
          <InputGroup>
            <InputLabel>Password</InputLabel>
            <TextInput
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter password"
              onKeyDown={(e) => e.key === "Enter" && handleAccountLogin()}
            />
          </InputGroup>
          <SelectionButton
            onClick={handleAccountLogin}
            $isSelected={false}
            $isDisabled={isBusy || serverStatus !== "online"}
            disabled={isBusy || serverStatus !== "online"}
          >
            {isConnecting ? "CONNECTING..." : "LOGIN"}
          </SelectionButton>
        </FormPanel>

        <SecondaryActions>
          <SelectionButton
            onClick={() => setScreen("title")}
            $isSelected={false}
            $isDisabled={isBusy}
            disabled={isBusy}
          >
            BACK
          </SelectionButton>
          <SelectionButton
            onClick={handleGuestLogin}
            $isSelected={false}
            $isDisabled={isBusy || serverStatus !== "online"}
            disabled={isBusy || serverStatus !== "online"}
          >
            {isGuestConnecting ? "CONNECTING..." : "GUEST"}
          </SelectionButton>
        </SecondaryActions>

        <StatusSlot>
          {statusMessage && <OfflineWarning>{statusMessage}</OfflineWarning>}
        </StatusSlot>
      </CenterColumn>

      <PopupWindow
        isOpen={showGuestPopup}
        title="Guest Account Warning"
        message={GUEST_ACCOUNT_WARNING}
        onOk={performGuestLogin}
        onCancel={cancelGuestLogin}
        okText="OK"
        cancelText="CANCEL"
      />
    </Wrapper>
  );
};

export default LoginPage;
