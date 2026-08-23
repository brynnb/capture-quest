import React, { useEffect, useRef, useState } from "react";
import styled from "styled-components";
import useChatStore, { MessageType } from "@stores/ChatStore";

const ChatContainer = styled.div.attrs({ className: "chat-container" })`
  width: 100%;
  height: 100%;
  background: rgba(244, 244, 255, 0.84);
  backdrop-filter: blur(14px);
  border: 3px solid #4a4ba6;
  border-radius: 16px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 12px 36px rgba(20, 21, 67, 0.28);
  z-index: 5;
  font-family: 'Outfit', sans-serif;
  font-weight: 500;
  transition: all 0.3s ease;
  min-width: 0;
  position: relative;

  @media (max-width: 850px), (pointer: coarse) {
    background: rgba(236, 237, 255, 0.96);
  }

  .blue-vibrant-text {
    color: #008BF1;
  }

  .black-text {
    color: #281e16;
  }

  .blue-text {
    color: #150a6e;
  }

  .red-text {
    color: #cf081b;
  }

  .green-text {
    color: #55873a;
  }

  .purple-text {
    color: #7e476b;
  }

  .teal-text {
    color: #80e8de;
  }

  .name-text {
    color: #d1d2d3;
  }

  .gray-text {
    color: #808080;
  }

  .pink-text {
    color: #ff00cb;
  }
`;

const ChatHeader = styled.header`
  display: flex;
  min-height: 35px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 11px;
  color: #4a4ba6;
  background: rgba(192, 193, 255, 0.42);
  border-bottom: 2px solid rgba(74, 75, 166, 0.25);
  font-family: "Outfit", sans-serif;
  font-size: 9px;
  font-weight: 900;
  letter-spacing: 0.12em;
  text-transform: uppercase;
`;

const ConnectionStatus = styled.span<{ $connected: boolean }>`
  color: ${(props) => (props.$connected ? "#357b4d" : "#956171")};
`;

const ChatContent = styled.div.attrs({ className: "chat-content" })`
  width: 100%;
  flex: 1;
  min-height: 0;
  padding: 9px 13px;
  box-sizing: border-box;
  overflow-y: auto;
  overscroll-behavior: contain;

  &::-webkit-scrollbar {
    width: 8px;
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: #4a4ba6;
    border-radius: 4px;
  }

  &::-webkit-scrollbar-track {
    background: rgba(0, 0, 0, 0.05);
    border-radius: 4px;
  }
`;

const ChatMessage = styled.div`
  margin-bottom: 3px;
  font-size: 13px;
  line-height: 1.32;
  overflow-wrap: anywhere;
`;

const ChatInputForm = styled.form`
  width: 100%;
  padding: 8px 9px 9px;
  background: rgba(192, 193, 255, 0.2);
  border-top: 1px solid rgba(74, 75, 166, 0.16);
  box-sizing: border-box;
  flex-shrink: 0;
`;

const ChatInput = styled.input`
  width: 100%;
  min-height: 38px;
  padding: 8px 11px;
  box-sizing: border-box;
  background: rgba(255, 255, 255, 0.7);
  border: 2px solid #4a4ba6;
  border-radius: 9px;
  font-family: 'Outfit', sans-serif;
  font-weight: 650;
  font-size: 13px;
  color: #281e16;
  outline: none;
  transition: border-color 0.2s ease, background 0.2s ease;

  &:focus {
    border-color: #6a6bf6;
    background: rgba(255, 255, 255, 0.9);
  }

  &::placeholder {
    color: #999;
    font-weight: 400;
  }
`;

const getMessageClass = (type: MessageType): string => {
  switch (type) {
    case MessageType.GENERAL_CHAT:
    case MessageType.LOOT:
      return "black-text";
    case MessageType.DEATH:
      return "red-text";
    case MessageType.QUEST_UPDATE:
    case MessageType.SYSTEM:
    case MessageType.ZONE_ANNOUNCEMENT:
    case MessageType.RESURRECTION:
    case MessageType.ZONE_ENTRY:
      return "blue-vibrant-text";
    case MessageType.SYSTEM_ERROR:
      return "red-text";
    default:
      return "pink-text";
  }
};

const MAX_CHAT_LENGTH = 256;

const ChatBox: React.FC = () => {
  const {
    messages,
    addMessage,
    sendMessage,
    initializeWebTransport,
    isConnected,
    connectionError,
  } = useChatStore();
  const chatContentRef = useRef<HTMLDivElement>(null);
  const chatInputRef = useRef<HTMLInputElement>(null);
  const [inputValue, setInputValue] = useState("");
  const lastManualScrollTime = useRef<number>(0);
  const isAutoScrolling = useRef<boolean>(false);

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = inputValue.trim();
    if (trimmed) {
      sendMessage(trimmed);
    }
    setInputValue("");
    // Return focus to the game after sending
    chatInputRef.current?.blur();
  };

  const handleScroll = () => {
    if (isAutoScrolling.current) return;
    const el = chatContentRef.current;
    if (el) {
      const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 10;
      if (isAtBottom) {
        lastManualScrollTime.current = 0;
        return;
      }
    }
    lastManualScrollTime.current = Date.now();
  };

  // Ensure the chat input starts blurred so WASD/arrow keys control the game
  useEffect(() => {
    chatInputRef.current?.blur();
  }, []);

  // Global Enter key: focus chat input when not already focused
  // Global click: blur chat input when clicking outside the chat container
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      const interactiveTarget = (e.target as HTMLElement | null)?.closest?.(
        "input, button, select, textarea, [contenteditable='true']",
      );
      if (
        e.key === "Enter" &&
        document.activeElement !== chatInputRef.current &&
        !interactiveTarget &&
        chatInputRef.current &&
        chatInputRef.current.offsetParent !== null
      ) {
        e.preventDefault();
        chatInputRef.current?.focus();
      }
    };

    const handleGlobalClick = (e: PointerEvent) => {
      const chatContainer = chatInputRef.current?.closest(".chat-container");
      if (chatContainer && !chatContainer.contains(e.target as Node)) {
        chatInputRef.current?.blur();
      }
    };

    window.addEventListener("keydown", handleGlobalKeyDown);
    window.addEventListener("pointerdown", handleGlobalClick);
    return () => {
      window.removeEventListener("keydown", handleGlobalKeyDown);
      window.removeEventListener("pointerdown", handleGlobalClick);
    };
  }, []);

  useEffect(() => {
    const initializeConnection = async () => {
      try {
        await initializeWebTransport();
      } catch (error) {
        console.error("Failed to initialize WebTransport:", error);
        addMessage(`Failed to connect to chat server: ${error}`, MessageType.SYSTEM_ERROR);
      }
    };
    initializeConnection();
  }, [initializeWebTransport, addMessage]);

  useEffect(() => {
    if (isConnected) {
      console.log("Connected to chat server");
    }
  }, [isConnected]);

  useEffect(() => {
    if (connectionError) {
      addMessage(connectionError, MessageType.SYSTEM_ERROR);
    }
  }, [connectionError, addMessage]);

  useEffect(() => {
    if (chatContentRef.current) {
      const el = chatContentRef.current;
      const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 10;
      const timeSinceManualScroll = Date.now() - lastManualScrollTime.current;
      if (timeSinceManualScroll > 5000 || isAtBottom) {
        isAutoScrolling.current = true;
        el.scrollTop = el.scrollHeight;
        setTimeout(() => {
          isAutoScrolling.current = false;
        }, 50);
      }
    }
  }, [messages]);

  return (
    <ChatContainer>
      <ChatHeader>
        <span>Trainer Chat</span>
        <ConnectionStatus $connected={isConnected}>
          {isConnected ? "Online" : "Offline"}
        </ConnectionStatus>
      </ChatHeader>
      <ChatContent
        ref={chatContentRef}
        onScroll={handleScroll}
        onWheel={(event) => event.stopPropagation()}
        role="log"
        aria-live="polite"
        aria-relevant="additions"
      >
        {messages.map((message) => (
          <ChatMessage
            key={message.id}
            className={getMessageClass(message.type)}
            data-testid="chat-message"
          >
            {typeof message.text === "string" ? message.text.replace(/_/g, " ") : String(message.text)}
          </ChatMessage>
        ))}
      </ChatContent>

      <ChatInputForm onSubmit={handleSend}>
        <ChatInput
          ref={chatInputRef}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value.slice(0, MAX_CHAT_LENGTH))}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault();
              setInputValue("");
              chatInputRef.current?.blur();
            }
            e.stopPropagation();
          }}
          placeholder={isConnected ? "Press Enter to chat…" : "Chat is offline"}
          maxLength={MAX_CHAT_LENGTH}
          autoComplete="off"
          disabled={!isConnected}
          data-testid="chat-input"
        />
      </ChatInputForm>
    </ChatContainer>
  );
};

export default ChatBox;
