import React from "react";
import styled from "styled-components";

const Container = styled.div.attrs({ className: "page-selection" })`
  display: flex;
  align-items: center;
  margin-bottom: 15px;
  gap: 8px;
`;

const PageButton = styled.button.attrs({ className: "page-button" }) <{
  $isPressed: boolean;
  $direction: "left" | "right";
}>`
  width: 40px;
  height: 40px;
  background-color: ${({ $isPressed }) => ($isPressed ? "#a7edfe" : "#c0c1ff")};
  border: 3px solid #4a4ba6;
  border-radius: 50%;
  cursor: pointer;
  outline: none;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #2e2f66;
  font-size: 20px;
  font-weight: 900;
  box-shadow: 0 4px 0 #4a4ba6;
  transition: all 0.1s ease-in-out;
  padding: 0;

  &:hover:not(:disabled) {
    background-color: #d1d2ff;
    transform: translateY(-2px);
    box-shadow: 0 6px 0 #4a4ba6;
  }

  &:active:not(:disabled) {
    transform: translateY(2px);
    box-shadow: 0 2px 0 #4a4ba6;
  }

  &::before {
    content: '${({ $direction }) => ($direction === "left" ? "‹" : "›")}';
    font-size: 32px;
    line-height: 1;
    margin-top: -4px;
  }

  &:focus {
    outline: none;
  }
`;

const PageName = styled.p.attrs({ className: "page-name" }) <{
  $isNumeric?: boolean;
  $useAttributeBackground?: boolean;
}>`
  margin: 0;
  font-family: 'Outfit', sans-serif;
  font-size: 22px;
  font-weight: 800;
  color: #2e2f66;
  min-width: ${({ $isNumeric }) => ($isNumeric ? "40px" : "120px")};
  text-align: center;
  user-select: none;

  ${({ $useAttributeBackground }) =>
    $useAttributeBackground &&
    `
    background: rgba(255, 255, 255, 0.5);
    border: 2px solid #4a4ba6;
    border-radius: 12px;
    padding: 4px 12px;
    box-shadow: inset 0 2px 4px rgba(0,0,0,0.1);
  `}
`;

interface PageSelectionProps {
  pages: string[];
  currentPage: string;
  onPageChange: (direction: "left" | "right") => void;
  useAttributeBackground?: boolean;
}

const PageSelection: React.FC<PageSelectionProps> = ({
  currentPage,
  onPageChange,
  useAttributeBackground = false,
}) => {
  const [pressedPageButtons, setPressedPageButtons] = React.useState({
    left: false,
    right: false,
  });

  const handlePageButtonPress = (direction: "left" | "right") => {
    setPressedPageButtons((prev) => ({ ...prev, [direction]: true }));
    onPageChange(direction);
  };

  const handlePageButtonRelease = (direction: "left" | "right") => {
    setPressedPageButtons((prev) => ({ ...prev, [direction]: false }));
  };

  const isNumeric = !isNaN(Number(currentPage));

  return (
    <Container>
      <PageButton
        $isPressed={pressedPageButtons.left}
        $direction="left"
        onMouseDown={() => handlePageButtonPress("left")}
        onMouseUp={() => handlePageButtonRelease("left")}
        onMouseLeave={() => handlePageButtonRelease("left")}
      />
      <PageName
        $isNumeric={isNumeric}
        $useAttributeBackground={useAttributeBackground}
      >
        {currentPage}
      </PageName>
      <PageButton
        $isPressed={pressedPageButtons.right}
        $direction="right"
        onMouseDown={() => handlePageButtonPress("right")}
        onMouseUp={() => handlePageButtonRelease("right")}
        onMouseLeave={() => handlePageButtonRelease("right")}
      />
    </Container>
  );
};

export default PageSelection;
