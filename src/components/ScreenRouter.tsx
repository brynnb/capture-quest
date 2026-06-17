import useGameScreenStore from "@stores/GameScreenStore";
import TitlePage from "@/pages/TitlePage";
import LoginPage from "@/pages/LoginPage";
import RegisterPage from "@/pages/RegisterPage";
import CharacterSelectPage from "@/pages/CharacterSelectPage";
import CharacterCreatorPage from "@/pages/CharacterCreatorPage";
import MainPage from "@/pages/MainPage";
import StaticDataGate from "./StaticDataGate";
import SceneDebugger from "./Interface/SceneDebugger";
import { IS_LOCAL_DEV } from "@/config";

const IS_TEST_MODE = import.meta.env.VITE_TEST_MODE === "true";
const SHOW_DEBUG_TOOLS = IS_TEST_MODE || IS_LOCAL_DEV;

const ScreenRouter = () => {
  const { currentScreen } = useGameScreenStore();

  let content;
  switch (currentScreen) {
    case "title":
      content = <TitlePage />;
      break;
    case "login":
      content = <LoginPage />;
      break;
    case "register":
      content = <RegisterPage />;
      break;
    case "characterSelect":
      content = (
        <StaticDataGate>
          <CharacterSelectPage />
        </StaticDataGate>
      );
      break;
    case "characterCreate":
      content = (
        <StaticDataGate>
          <CharacterCreatorPage />
        </StaticDataGate>
      );
      break;
    case "game":
      content = (
        <StaticDataGate>
          <MainPage />
        </StaticDataGate>
      );
      break;
    default:
      content = <TitlePage />;
      break;
  }

  const isAuthScreen =
    currentScreen === "title" ||
    currentScreen === "login" ||
    currentScreen === "register";

  return (
    <>
      {content}
      {SHOW_DEBUG_TOOLS && !isAuthScreen && (
        <>
          <SceneDebugger />
        </>
      )}
    </>
  );
};

export default ScreenRouter;
