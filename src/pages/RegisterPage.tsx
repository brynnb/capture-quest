import { useState } from "react";
import styled from "styled-components";
import useGameScreenStore from "@stores/GameScreenStore";
import SelectionButton from "@components/Interface/SelectionButton";
import { registerAccount } from "@/services/authService";

const Wrapper = styled.div`
  height: 100%;
  width: 100%;
  background-image: url("/assets/animebgfull.jpg");
  background-size: cover;
  background-repeat: no-repeat;
  background-position: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  box-sizing: border-box;
  padding: 12px 0 92px;
  overflow-y: auto;
  overscroll-behavior: contain;

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    justify-content: flex-start;
    padding: max(10px, env(safe-area-inset-top, 0px)) 0
      calc(88px + env(safe-area-inset-bottom, 0px));
  }
`;

const BottomButtons = styled.div`
  position: absolute;
  bottom: 20px;
  left: 50px;
  right: 50px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  pointer-events: none;
  z-index: 20;

  & > * {
    pointer-events: auto;
    width: 350px;
  }

  @media (max-width: 700px) {
    right: 16px;
    bottom: calc(12px + env(safe-area-inset-bottom, 0px));
    left: 16px;
    gap: 10px;

    & > * {
      width: auto;
      flex: 1;
    }
  }

  @media (max-height: 500px) {
    bottom: calc(7px + env(safe-area-inset-bottom, 0px));
  }
`;

const CenterContent = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 24px;
  z-index: 10;
  width: min(540px, calc(100% - 24px));
  flex: 0 0 auto;

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    gap: 10px;
  }
`;

const Logo = styled.img`
  width: min(500px, calc(100vw - 40px));
  max-height: 20vh;
  height: auto;
  object-fit: contain;

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    max-height: 76px;
  }

  @media (max-height: 500px) and (min-width: 651px) {
    display: none;
  }
`;

const FormPanel = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: min(500px, calc(100vw - 24px));
  gap: 10px;
  padding: 40px;
  box-sizing: border-box;
  background: rgba(192, 193, 255, 0.4);
  backdrop-filter: blur(10px);
  border: 4px solid #4a4ba6;
  border-radius: 30px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.2);

  @media (max-width: 600px), (max-height: 760px) {
    padding: 20px 18px;
    border-width: 3px;
    border-radius: 18px;
  }

  @media (max-height: 500px) and (min-width: 651px) {
    gap: 4px;
    padding: 10px 16px;
  }
`;

const Title = styled.h1`
  font-family: 'Outfit', sans-serif;
  font-weight: 800;
  color: #2e2f66;
  font-size: 36px;
  margin-bottom: 20px;
  text-transform: none;

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    margin: 0 0 5px;
    font-size: 26px;
  }

  @media (max-height: 500px) and (min-width: 651px) {
    margin-bottom: 0;
    font-size: 23px;
  }
`;

const InputGroup = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  margin-bottom: 20px;

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    gap: 3px;
    margin-bottom: 4px;
  }

  @media (max-height: 500px) and (min-width: 651px) {
    gap: 3px;
    margin-bottom: 0;
  }
`;

const InputLabel = styled.label`
  font-family: 'Outfit', sans-serif;
  font-weight: 800;
  font-size: 16px;
  color: #2e2f66;
  text-transform: uppercase;
  width: 100%;
  text-align: center;

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    font-size: 12px;
  }

  @media (max-height: 500px) and (min-width: 651px) {
    padding: 6px 9px;
    font-size: 15px;
  }
`;

const TextInput = styled.input`
  width: 100%;
  padding: 12px 15px;
  font-family: 'Outfit', sans-serif;
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
  &:-webkit-autofill:active  {
    -webkit-box-shadow: 0 0 0 1000px #ffffff inset !important;
    -webkit-text-fill-color: #2e2f66 !important;
    transition: background-color 5000s ease-in-out 0s;
  }

  &:autofill {
    filter: none;
    box-shadow: 0 0 0 1000px #ffffff inset !important;
  }

  &::placeholder {
    color: #4a4ba6;
    opacity: 0.6;
  }

  @media (max-width: 700px), (max-height: 760px), (pointer: coarse) {
    padding: 8px 10px;
    font-size: 16px;
  }
`;

const StatusText = styled.p`
  font-family: 'Outfit', sans-serif;
  font-size: 18px;
  color: #ffaf84;
  text-align: center;
  line-height: 1.25;
  margin: 0;
  font-weight: 600;
`;

const SuccessText = styled(StatusText)`
  color: #4caf50;
`;

const StatusSlot = styled.div`
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: visible;

  @media (max-height: 500px) {
    height: 24px;
  }
`;

const RegisterPage = () => {
  const { setScreen } = useGameScreenStore();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleRegister = async () => {
    // Basic email validation
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!email.trim() || !emailRegex.test(email)) {
      setError("Please enter a valid email address");
      return;
    }

    // Password validation
    if (!password || password.length < 6) {
      setError("Password must be at least 6 characters long");
      return;
    }

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    setIsSubmitting(true);
    setError(null);
    setSuccess(null);

    try {
      await registerAccount(email, password);
      setSuccess("Account created successfully!");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Registration failed");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Wrapper>
      <CenterContent>
        <Logo src="/assets/capturequestlogo.png" alt="CaptureQuest" />
        <FormPanel>
          <Title>Create Account</Title>
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
            />
          </InputGroup>
          <InputGroup>
            <InputLabel>Confirm Password</InputLabel>
            <TextInput
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Confirm password"
              onKeyDown={(e) => e.key === "Enter" && handleRegister()}
            />
          </InputGroup>
        </FormPanel>
        <StatusSlot>
          {error && <StatusText>{error}</StatusText>}
          {!error && success && <SuccessText>{success}</SuccessText>}
        </StatusSlot>
      </CenterContent>

      <BottomButtons>
        <SelectionButton
          onClick={() => setScreen("title")}
          $isSelected={false}
          $isDisabled={false}
        >
          BACK
        </SelectionButton>
        <SelectionButton
          onClick={handleRegister}
          $isSelected={false}
          $isDisabled={isSubmitting}
          disabled={isSubmitting}
        >
          {isSubmitting ? "CREATING..." : "CREATE ACCOUNT"}
        </SelectionButton>
      </BottomButtons>
    </Wrapper>
  );
};

export default RegisterPage;
