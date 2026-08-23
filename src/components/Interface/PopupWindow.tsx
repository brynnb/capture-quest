import styled from "styled-components";
import SelectionButton from "./SelectionButton";

interface PopupWindowProps {
  title?: string;
  message: string | React.ReactNode;
  messageAlign?: "left" | "center" | "right";
  okText?: string;
  cancelText?: string;
  onOk: () => void;
  onCancel?: () => void;
  isOpen: boolean;
}

const Overlay = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999999;
  padding: max(12px, env(safe-area-inset-top, 0px))
    max(12px, env(safe-area-inset-right, 0px))
    max(12px, env(safe-area-inset-bottom, 0px))
    max(12px, env(safe-area-inset-left, 0px));
  box-sizing: border-box;
`;

const WindowContainer = styled.div`
  width: 600px;
  max-width: 100%;
  min-height: 350px;
  max-height: calc(var(--cq-viewport-height, 100dvh) - 24px);
  background: rgba(192, 193, 255, 0.95);
  backdrop-filter: blur(10px);
  border: 4px solid #4a4ba6;
  border-radius: 30px;
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 40px;
  box-sizing: border-box;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.3);

  @media (max-width: 650px), (pointer: coarse), (max-height: 600px) {
    min-height: min(350px, calc(var(--cq-viewport-height, 100dvh) - 24px));
    padding: 22px 18px;
    border-radius: 18px;
  }
`;

const Title = styled.h2`
  text-align: center;
  margin: 0 0 20px 0;
  font-family: 'Outfit', sans-serif;
  font-weight: 800;
  font-size: 32px;
  text-transform: none;
  color: #2e2f66;

  @media (max-width: 650px), (pointer: coarse) {
    margin-bottom: 12px;
    font-size: 24px;
  }
`;

const MessageContainer = styled.div<{ $align?: "left" | "center" | "right" }>`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: ${(props) =>
    props.$align === "center"
      ? "center"
      : props.$align === "right"
        ? "flex-end"
        : "flex-start"};
  text-align: ${(props) => props.$align || "left"};
  font-family: 'Outfit', sans-serif;
  font-weight: 500;
  font-size: 20px;
  color: #2e2f66;
  line-height: 1.4;
  overflow-y: auto;
  padding-bottom: 20px;

  @media (max-width: 650px), (pointer: coarse) {
    font-size: 16px;
  }
`;

const ButtonRow = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  margin-top: auto;
  padding-top: 10px;

  & > * {
    pointer-events: auto;
  }

  @media (max-width: 420px) {
    gap: 8px;

    & > button {
      min-width: 0;
      width: auto;
      flex: 1;
    }
  }
`;

const Spacer = styled.div`
  flex: 1;
`;

const PopupWindow = ({
  title,
  message,
  messageAlign,
  okText = "OK",
  cancelText,
  onOk,
  onCancel,
  isOpen,
}: PopupWindowProps) => {
  if (!isOpen) return null;

  return (
    <Overlay onClick={(e) => e.stopPropagation()}>
      <WindowContainer>
        {title && <Title>{title}</Title>}
        <MessageContainer $align={messageAlign}>{message}</MessageContainer>
        <ButtonRow>
          {onCancel ? (
            <SelectionButton
              $isSelected={false}
              $width="140px"
              $height="55px"
              onClick={onCancel}
            >
              {cancelText || "CANCEL"}
            </SelectionButton>
          ) : (
            <Spacer />
          )}

          <SelectionButton
            $isSelected={false}
            $width="140px"
            $height="55px"
            onClick={onOk}
          >
            {okText}
          </SelectionButton>
        </ButtonRow>
      </WindowContainer>
    </Overlay>
  );
};

export default PopupWindow;
