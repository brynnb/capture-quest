import styled from "styled-components";
import type { CompactChoice } from "./CompactChoiceCarousel";

interface CompactChoiceListProps {
  title: string;
  choices: CompactChoice[];
  selectedId: CompactChoice["id"] | null | undefined;
  onSelect: (id: CompactChoice["id"]) => void;
  showOnDesktop?: boolean;
  showOnMobile?: boolean;
}

const Shell = styled.section<{
  $showOnDesktop: boolean;
  $showOnMobile: boolean;
}>`
  display: ${({ $showOnDesktop }) => ($showOnDesktop ? "flex" : "none")};
  width: 100%;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  gap: 8px;

  ${({ $showOnDesktop }) =>
    $showOnDesktop &&
    `
      position: relative;
      height: calc(60px + var(--character-choice-height, clamp(420px, 66vh, 700px)));
      padding-top: 60px;
      box-sizing: border-box;

      > h2 {
        position: absolute;
        top: 15px;
        right: 0;
        left: 0;
      }
    `}

  @media (max-width: 900px), (pointer: coarse) {
    display: ${({ $showOnMobile }) => ($showOnMobile ? "flex" : "none")};
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
    flex: 1 1 auto;
    flex-direction: column;
    gap: 8px;
    padding-top: 0;

    > h2 {
      position: static;
    }
  }
`;

const Title = styled.h2`
  margin: 0;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-size: clamp(20px, 6vw, 27px);
  font-weight: 900;
  line-height: 1.1;
  text-align: center;

  @media (max-height: 650px) {
    font-size: 18px;
  }
`;

const ChoiceList = styled.div`
  display: flex;
  width: 100%;
  min-height: 0;
  padding: 10px 12px 14px;
  border-style: solid;
  border-width: 8px;
  border-image: url("/assets/pokemon_frame/frame-hd.png") 42 round;
  background: rgba(192, 193, 255, 0.3);
  image-rendering: pixelated;
  box-sizing: border-box;
  flex: 1 1 auto;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-color: #4a4ba6 rgba(192, 193, 255, 0.35);
  scrollbar-width: thin;

  @media (max-height: 650px) {
    padding: 7px 9px 10px;
    border-width: 7px;
    gap: 5px;
  }
`;

const ChoiceButton = styled.button<{ $selected: boolean }>`
  width: 100%;
  min-height: 80px;
  padding: 10px 12px;
  flex: 0 0 auto;
  color: #2e2f66;
  background: ${({ $selected }) =>
    $selected ? "rgba(167, 237, 254, 0.96)" : "rgba(255, 236, 241, 0.95)"};
  border: 3px solid ${({ $selected }) => ($selected ? "#4a4ba6" : "#ffccd9")};
  border-radius: 16px;
  box-shadow: 0 4px 0 rgba(74, 75, 166, 0.82);
  font-family: "Outfit", sans-serif;
  text-align: center;
  touch-action: manipulation;

  &:active {
    transform: translateY(2px);
    box-shadow: 0 2px 0 rgba(74, 75, 166, 0.82);
  }

  @media (max-height: 650px) {
    min-height: 66px;
    padding: 6px 9px;
    border-radius: 12px;
  }
`;

const ChoiceName = styled.strong`
  display: block;
  overflow: hidden;
  font-size: clamp(17px, 5vw, 22px);
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;

  @media (max-height: 650px) {
    font-size: 16px;
  }
`;

const Description = styled.span`
  display: -webkit-box;
  margin-top: 5px;
  overflow: hidden;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.25;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;

  @media (max-height: 650px) {
    margin-top: 2px;
    font-size: 11px;
  }
`;

const CompactChoiceList = ({
  title,
  choices,
  selectedId,
  onSelect,
  showOnDesktop = false,
  showOnMobile = true,
}: CompactChoiceListProps) => (
  <Shell
    aria-label={title}
    $showOnDesktop={showOnDesktop}
    $showOnMobile={showOnMobile}
  >
    <Title>{title}</Title>
    <ChoiceList data-testid="compact-choice-list">
      {choices.map((choice) => {
        const isSelected = choice.id === selectedId;
        return (
          <ChoiceButton
            key={choice.id}
            type="button"
            $selected={isSelected}
            aria-pressed={isSelected}
            onClick={() => onSelect(choice.id)}
          >
            <ChoiceName>{choice.label}</ChoiceName>
            {choice.description && (
              <Description>{choice.description}</Description>
            )}
          </ChoiceButton>
        );
      })}
    </ChoiceList>
  </Shell>
);

export default CompactChoiceList;
