import { useEffect, useMemo, useState } from "react";
import styled from "styled-components";

export interface CompactChoice {
  id: number | string;
  label: string;
  description?: string | null;
}

interface CompactChoiceCarouselProps {
  title: string;
  itemLabel: string;
  choices: CompactChoice[];
  selectedId: CompactChoice["id"] | null | undefined;
  onSelect: (id: CompactChoice["id"]) => void;
}

const Shell = styled.section`
  display: none;

  @media (max-width: 900px), (pointer: coarse) {
    display: flex;
    width: 100%;
    min-width: 0;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  @media (max-height: 650px) and (max-width: 900px),
    (max-height: 650px) and (pointer: coarse) {
    gap: 4px;
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

const Controls = styled.div`
  display: grid;
  width: 100%;
  grid-template-columns: 48px minmax(0, 1fr) 48px;
  gap: 8px;
  align-items: stretch;

  @media (max-height: 650px) {
    grid-template-columns: 44px minmax(0, 1fr) 44px;
    gap: 5px;
  }
`;

const ArrowButton = styled.button`
  min-width: 48px;
  min-height: 48px;
  padding: 0;
  color: #2e2f66;
  background: rgba(192, 193, 255, 0.94);
  border: 3px solid #4a4ba6;
  border-radius: 14px;
  box-shadow: 0 4px 0 #4a4ba6;
  font:
    900 32px/1 "Outfit",
    sans-serif;
  touch-action: manipulation;

  &:active {
    transform: translateY(2px);
    box-shadow: 0 2px 0 #4a4ba6;
  }

  @media (max-height: 650px) {
    min-width: 44px;
    min-height: 44px;
    border-radius: 11px;
    font-size: 27px;
  }
`;

const ChoiceCard = styled.button<{ $selected: boolean }>`
  min-width: 0;
  min-height: 92px;
  padding: 10px 12px;
  color: #2e2f66;
  background: ${({ $selected }) =>
    $selected ? "rgba(167, 237, 254, 0.96)" : "rgba(255, 236, 241, 0.95)"};
  border: 3px solid ${({ $selected }) => ($selected ? "#4a4ba6" : "#ffccd9")};
  border-radius: 16px;
  box-shadow: 0 4px 0 rgba(74, 75, 166, 0.82);
  font-family: "Outfit", sans-serif;
  text-align: center;
  touch-action: manipulation;

  @media (max-height: 650px) {
    min-height: 68px;
    padding: 5px 8px;
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
    -webkit-line-clamp: 1;
  }
`;

const Counter = styled.div`
  color: #4a4ba6;
  font:
    800 12px/1 "Outfit",
    sans-serif;
  letter-spacing: 0.06em;

  @media (max-height: 650px) {
    font-size: 10px;
  }
`;

const wrapIndex = (index: number, length: number) =>
  ((index % length) + length) % length;

const CompactChoiceCarousel = ({
  title,
  itemLabel,
  choices,
  selectedId,
  onSelect,
}: CompactChoiceCarouselProps) => {
  const selectedIndex = useMemo(
    () => choices.findIndex((choice) => choice.id === selectedId),
    [choices, selectedId],
  );
  const [cursorIndex, setCursorIndex] = useState(() =>
    selectedIndex >= 0 ? selectedIndex : 0,
  );

  useEffect(() => {
    if (selectedIndex >= 0) setCursorIndex(selectedIndex);
  }, [selectedIndex]);

  useEffect(() => {
    if (selectedIndex < 0 && choices.length > 0) {
      onSelect(choices[0].id);
    }
  }, [choices, onSelect, selectedIndex]);

  if (choices.length === 0) return null;

  const safeIndex = wrapIndex(cursorIndex, choices.length);
  const current = choices[safeIndex];
  const isSelected = current.id === selectedId;

  const cycle = (direction: -1 | 1) => {
    const nextIndex = wrapIndex(safeIndex + direction, choices.length);
    setCursorIndex(nextIndex);
    onSelect(choices[nextIndex].id);
  };

  return (
    <Shell aria-label={title}>
      <Title>{title}</Title>
      <Controls>
        <ArrowButton
          type="button"
          aria-label={`Previous ${itemLabel}`}
          onClick={() => cycle(-1)}
        >
          ‹
        </ArrowButton>
        <ChoiceCard
          type="button"
          $selected={isSelected}
          aria-pressed={isSelected}
          onClick={() => onSelect(current.id)}
        >
          <ChoiceName>{current.label}</ChoiceName>
          {current.description && (
            <Description>{current.description}</Description>
          )}
        </ChoiceCard>
        <ArrowButton
          type="button"
          aria-label={`Next ${itemLabel}`}
          onClick={() => cycle(1)}
        >
          ›
        </ArrowButton>
      </Controls>
      <Counter aria-live="polite">
        {safeIndex + 1} / {choices.length}
      </Counter>
    </Shell>
  );
};

export default CompactChoiceCarousel;
