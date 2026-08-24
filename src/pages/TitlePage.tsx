import styled from "styled-components";
import SelectionButton from "@components/Interface/SelectionButton";
import useGameScreenStore from "@stores/GameScreenStore";
import PopupWindow from "@components/Interface/PopupWindow";
import { GUEST_ACCOUNT_WARNING, useGuestLogin } from "@/hooks/useGuestLogin";

const Wrapper = styled.div`
  position: relative;
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
  padding: 12px 0;
  overflow-y: auto;
  overscroll-behavior: contain;

  @media (max-height: 560px), (pointer: coarse) {
    justify-content: flex-start;
    padding: calc(30px + env(safe-area-inset-top, 0px)) 0
      calc(92px + env(safe-area-inset-bottom, 0px));
  }
`;

const CenterColumn = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
  width: min(520px, calc(100% - 32px));
  flex: 0 0 auto;

  .selection-button {
    width: min(460px, calc(100vw - 40px));
  }

  @media (max-height: 560px), (pointer: coarse) {
    gap: 14px;
  }
`;

const ButtonStack = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;

  @media (max-height: 560px), (pointer: coarse) {
    gap: 8px;
  }
`;

const Logo = styled.img`
  width: min(520px, calc(100vw - 40px));
  max-height: 34vh;
  height: auto;
  object-fit: contain;

  @media (max-height: 560px), (pointer: coarse) {
    max-height: 90px;
  }
`;

const StatusText = styled.p`
  margin: 0;
  font-family: "Outfit", Inter, sans-serif;
  font-size: 18px;
  font-weight: 700;
  color: #ffaf84;
  text-align: center;
`;

const TitlePage = () => {
  const { setScreen } = useGameScreenStore();
  const {
    isGuestConnecting,
    guestLoginError,
    showGuestPopup,
    requestGuestLogin,
    performGuestLogin,
    cancelGuestLogin,
  } = useGuestLogin();

  return (
    <Wrapper>
      <CenterColumn>
        <Logo src="/assets/capturequestlogo.png" alt="CaptureQuest" />
        <ButtonStack>
          <SelectionButton
            onClick={() => setScreen("login")}
            $isSelected={false}
            $isDisabled={false}
          >
            LOGIN
          </SelectionButton>
          <SelectionButton
            onClick={() => setScreen("register")}
            $isSelected={false}
            $isDisabled={false}
          >
            REGISTER
          </SelectionButton>
          <SelectionButton
            onClick={requestGuestLogin}
            $isSelected={false}
            $isDisabled={isGuestConnecting}
            disabled={isGuestConnecting}
          >
            {isGuestConnecting ? "CONNECTING..." : "PLAY AS GUEST"}
          </SelectionButton>
        </ButtonStack>
        {guestLoginError && <StatusText>{guestLoginError}</StatusText>}
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

export default TitlePage;
