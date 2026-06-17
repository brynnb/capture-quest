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

const ButtonStack = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
`;

const Logo = styled.img`
  max-width: 520px;
  height: auto;
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
