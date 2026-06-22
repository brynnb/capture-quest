import React, { useEffect, useRef, useCallback, useState } from "react";
import styled from "styled-components";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import { suppressWorldInputFor } from "@/phaser-game/utils/worldInputGuard";

const TYPEWRITER_SPEED = 30; // ms per character

const DialogueOverlay = styled.div`
  position: absolute;
  bottom: 100%;
  left: 0;
  right: 0;
  z-index: 10;
  padding-bottom: 8px;
  pointer-events: none;
`;

const DialogueContainer = styled.div`
  width: 100%;
  pointer-events: auto;
`;

const DialogueBox = styled.div`
  background: #f8f8f8;
  border: 4px solid #383838;
  border-radius: 8px;
  padding: 16px 20px;
  min-height: 80px;
  max-height: 120px;
  position: relative;
  cursor: pointer;
  user-select: none;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  justify-content: center;

  &:hover {
    background: #fff;
  }
`;

const DialogueText = styled.div`
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 16px;
  line-height: 1.8;
  color: #383838;
  white-space: pre-wrap;
  word-wrap: break-word;
`;

const AdvanceIndicator = styled.div<{ $visible: boolean }>`
  position: absolute;
  bottom: 8px;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 6px;
  opacity: ${(props) => (props.$visible ? 1 : 0)};

  @keyframes bounce {
    0%,
    100% {
      transform: translateY(0);
    }
    50% {
      transform: translateY(-4px);
    }
  }
`;

const AdvanceArrow = styled.span`
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 12px;
  color: #383838;
  animation: bounce 0.6s infinite;
`;

const AdvanceHint = styled.span`
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 9px;
  color: #aaa;
`;

const ChoiceRow = styled.div`
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-top: 12px;
`;

const ChoiceButton = styled.button<{ $variant: "yes" | "no"; $selected: boolean }>`
  position: relative;
  font-family: "Pokemon GB", "Press Start 2P", monospace;
  font-size: 14px;
  padding: 8px 24px 8px 34px;
  border: 3px solid #383838;
  border-radius: 6px;
  cursor: pointer;
  background: ${(p) => (p.$variant === "yes" ? "#4caf50" : "#e53935")};
  color: #fff;
  transition: transform 0.1s ease;

  &:hover {
    transform: scale(1.05);
  }

  &:active {
    transform: scale(0.97);
  }

  &::before {
    content: ">";
    position: absolute;
    left: 12px;
    opacity: ${(p) => (p.$selected ? 1 : 0)};
  }

  ${(p) =>
    p.$selected &&
    `
      box-shadow: inset 0 0 0 2px #fff;
      transform: scale(1.04);
    `}
`;

const PokemonDialogueBox: React.FC = () => {
  const {
    isOpen,
    lines,
    currentLineIndex,
    displayedText,
    isTyping,
    isChoicePending,
    advanceLine,
    setDisplayedText,
    setIsTyping,
    closeDialogue,
    resolveChoice,
  } = usePokemonDialogueStore();

  const typewriterRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const charIndexRef = useRef(0);
  const [selectedChoice, setSelectedChoice] = useState<"yes" | "no">("yes");

  const consumePointerEvent = useCallback(
    (
      e: React.PointerEvent | React.MouseEvent | PointerEvent,
      options: { preventDefault?: boolean } = {},
    ) => {
      suppressWorldInputFor();
      if (options.preventDefault ?? true) {
        e.preventDefault();
      }
      e.stopPropagation();
      if ("nativeEvent" in e) {
        e.nativeEvent.stopImmediatePropagation?.();
      } else {
        e.stopImmediatePropagation?.();
      }
    },
    [],
  );

  // Typewriter effect
  useEffect(() => {
    if (!isOpen || !isTyping || currentLineIndex >= lines.length) return;

    const fullText = lines[currentLineIndex];
    charIndexRef.current = 0;
    setDisplayedText("");

    const tick = () => {
      charIndexRef.current++;
      if (charIndexRef.current <= fullText.length) {
        setDisplayedText(fullText.slice(0, charIndexRef.current));
        typewriterRef.current = setTimeout(tick, TYPEWRITER_SPEED);
      } else {
        setIsTyping(false);
      }
    };

    typewriterRef.current = setTimeout(tick, TYPEWRITER_SPEED);

    return () => {
      if (typewriterRef.current) {
        clearTimeout(typewriterRef.current);
        typewriterRef.current = null;
      }
    };
  }, [
    isOpen,
    isTyping,
    currentLineIndex,
    lines,
    setDisplayedText,
    setIsTyping,
  ]);

  useEffect(() => {
    if (isChoicePending) {
      setSelectedChoice("yes");
    }
  }, [isChoicePending]);

  const consumeKeyboardEvent = useCallback((e: KeyboardEvent) => {
    e.preventDefault();
    e.stopPropagation();
    e.stopImmediatePropagation?.();
  }, []);

  // Keyboard handler (A key, Enter, Space, or Z to advance)
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (!isOpen) return;
      if (isChoicePending) {
        if (e.key === "y" || e.key === "Y") {
          consumeKeyboardEvent(e);
          resolveChoice(true);
        } else if (e.key === "n" || e.key === "N" || e.key === "Escape") {
          consumeKeyboardEvent(e);
          resolveChoice(false);
        } else if (
          e.key === "ArrowLeft" ||
          e.key === "ArrowUp" ||
          e.key === "a" ||
          e.key === "A" ||
          e.key === "w" ||
          e.key === "W"
        ) {
          consumeKeyboardEvent(e);
          setSelectedChoice("yes");
        } else if (
          e.key === "ArrowRight" ||
          e.key === "ArrowDown" ||
          e.key === "d" ||
          e.key === "D" ||
          e.key === "s" ||
          e.key === "S"
        ) {
          consumeKeyboardEvent(e);
          setSelectedChoice("no");
        } else if (e.key === "Enter" || e.key === " ") {
          consumeKeyboardEvent(e);
          resolveChoice(selectedChoice === "yes");
        }
        return;
      }
      if (
        e.key === "z" ||
        e.key === "Z" ||
        e.key === "Enter" ||
        e.key === " " ||
        e.key === "a" ||
        e.key === "A"
      ) {
        consumeKeyboardEvent(e);
        advanceLine();
      }
      if (e.key === "Escape") {
        consumeKeyboardEvent(e);
        closeDialogue();
      }
    },
    [
      isOpen,
      isChoicePending,
      advanceLine,
      closeDialogue,
      consumeKeyboardEvent,
      resolveChoice,
      selectedChoice,
    ],
  );

  // Global click-anywhere-to-close when on last line and done typing
  const isLastLine = currentLineIndex >= lines.length - 1;
  const canDismiss = isOpen && !isChoicePending && !isTyping && isLastLine;

  const handleGlobalClick = useCallback((e: PointerEvent) => {
    const state = usePokemonDialogueStore.getState();
    const currentlyDismissable =
      state.isOpen &&
      !state.isChoicePending &&
      !state.isTyping &&
      state.currentLineIndex >= state.lines.length - 1;

    if (currentlyDismissable) {
      consumePointerEvent(e);
      state.closeDialogue();
    }
  }, [consumePointerEvent]);

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [handleKeyDown]);

  useEffect(() => {
    if (!canDismiss) return;
    // Small delay so the click that triggered the last advance doesn't immediately close
    const timer = setTimeout(() => {
      window.addEventListener("pointerdown", handleGlobalClick, true);
    }, 100);
    return () => {
      clearTimeout(timer);
      window.removeEventListener("pointerdown", handleGlobalClick, true);
    };
  }, [canDismiss, handleGlobalClick]);

  const handleDialoguePointerDown = useCallback(
    (e: React.PointerEvent) => {
      consumePointerEvent(e, { preventDefault: false });
    },
    [consumePointerEvent],
  );

  const handleDialogueClick = useCallback(
    (e: React.MouseEvent) => {
      consumePointerEvent(e);
      advanceLine();
    },
    [advanceLine, consumePointerEvent],
  );

  const handleChoiceClick = useCallback(
    (choice: boolean, e: React.MouseEvent) => {
      consumePointerEvent(e);
      resolveChoice(choice);
    },
    [consumePointerEvent, resolveChoice],
  );

  if (!isOpen) return null;

  const hasMoreLines = !isLastLine;
  const showIndicator = !isTyping && hasMoreLines;
  const showCloseHint = canDismiss;

  return (
    <DialogueOverlay>
      <DialogueContainer>
        <DialogueBox
          data-testid="pokemon-dialogue-box"
          onPointerDown={handleDialoguePointerDown}
          onClick={isChoicePending ? undefined : handleDialogueClick}
        >
          <DialogueText>{displayedText}</DialogueText>
          {isChoicePending ? (
            <ChoiceRow>
              <ChoiceButton
                $variant="yes"
                $selected={selectedChoice === "yes"}
                data-testid="dialogue-choice-yes"
                data-selected={selectedChoice === "yes" ? "true" : "false"}
                onClick={(e) => handleChoiceClick(true, e)}
              >
                YES
              </ChoiceButton>
              <ChoiceButton
                $variant="no"
                $selected={selectedChoice === "no"}
                data-testid="dialogue-choice-no"
                data-selected={selectedChoice === "no" ? "true" : "false"}
                onClick={(e) => handleChoiceClick(false, e)}
              >
                NO
              </ChoiceButton>
            </ChoiceRow>
          ) : (
            <>
              <AdvanceIndicator $visible={showIndicator}>
                <AdvanceHint>Click to continue</AdvanceHint>
                <AdvanceArrow>▼</AdvanceArrow>
              </AdvanceIndicator>
              <AdvanceIndicator $visible={showCloseHint}>
                <AdvanceHint>Click anywhere to close</AdvanceHint>
              </AdvanceIndicator>
            </>
          )}
        </DialogueBox>
      </DialogueContainer>
    </DialogueOverlay>
  );
};

export default PokemonDialogueBox;
