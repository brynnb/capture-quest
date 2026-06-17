import React, { useState } from "react";
import styled from "styled-components";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import * as PhaserNet from "@/phaser-game/services/PhaserNetworkService";
import type { PokemonDTO } from "@/net/generated/world_api";

const Overlay = styled.div`
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: all;
  background: rgba(0, 0, 0, 0.4);
`;

const PCWindow = styled.div`
  width: 640px;
  max-height: 600px;
  background: #f8f8f0;
  border: 4px solid #383838;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  font-family: "Press Start 2P", "Courier New", monospace;
  font-size: 11px;
  color: #383838;
  overflow: hidden;
`;

const PCHeader = styled.div`
  background: #3070c0;
  color: white;
  padding: 10px 16px;
  font-size: 13px;
  text-align: center;
  border-bottom: 3px solid #383838;
  text-shadow: 1px 1px 0 rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const BoxNav = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: #e8e8d8;
  border-bottom: 2px solid #c8c8b8;
  justify-content: center;
`;

const NavButton = styled.button`
  font-family: "Press Start 2P", "Courier New", monospace;
  font-size: 12px;
  padding: 4px 10px;
  border: 2px solid #383838;
  border-radius: 4px;
  cursor: pointer;
  background: #c0c0b0;
  color: #383838;
  &:hover {
    background: #d0d0c0;
  }
`;

const ContentArea = styled.div`
  display: flex;
  flex: 1;
  overflow: hidden;
`;

const BoxPanel = styled.div`
  flex: 1;
  padding: 8px;
  overflow-y: auto;
  border-right: 2px solid #c8c8b8;
`;

const PartyPanel = styled.div`
  width: 200px;
  padding: 8px;
  overflow-y: auto;
  background: #f0f0e8;
`;

const PanelTitle = styled.div`
  font-size: 10px;
  color: #606060;
  text-align: center;
  margin-bottom: 6px;
  padding-bottom: 4px;
  border-bottom: 1px solid #d0d0c0;
`;

const PokemonSlot = styled.div<{ $selected: boolean; $empty?: boolean }>`
  display: flex;
  align-items: center;
  padding: 6px 8px;
  margin: 2px 0;
  cursor: ${(p) => (p.$empty ? "default" : "pointer")};
  background: ${(p) =>
    p.$selected ? "#d0d0ff" : p.$empty ? "transparent" : "#fff"};
  border: ${(p) =>
    p.$selected ? "2px solid #4040c0" : "2px solid transparent"};
  border-radius: 6px;
  transition: background 0.1s;
  min-height: 28px;

  &:hover {
    background: ${(p) =>
      p.$empty ? "transparent" : p.$selected ? "#d0d0ff" : "#e8e8e0"};
  }
`;

const PokemonInfo = styled.div`
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
`;

const PokemonName = styled.span`
  font-size: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const PokemonLevel = styled.span`
  font-size: 8px;
  color: #808080;
`;

const Footer = styled.div`
  border-top: 3px solid #383838;
  padding: 10px 16px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  background: #e8e8d8;
`;

const PCButton = styled.button<{
  $variant?: "deposit" | "withdraw" | "release" | "cancel";
}>`
  font-family: "Press Start 2P", "Courier New", monospace;
  font-size: 10px;
  padding: 8px 14px;
  border: 2px solid #383838;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.1s;
  color: white;

  background: ${(p) => {
    switch (p.$variant) {
      case "deposit":
        return "#3070c0";
      case "withdraw":
        return "#40a040";
      case "release":
        return "#a04040";
      case "cancel":
        return "#808080";
      default:
        return "#808080";
    }
  }};

  &:hover {
    filter: brightness(1.15);
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    filter: none;
  }
`;

const PokemonPC: React.FC = () => {
  const {
    isOpen,
    currentBox,
    boxCount,
    boxPokemon,
    party,
    closePC,
  } = usePokemonPCStore();

  const [selectedPartySlot, setSelectedPartySlot] = useState<number | null>(
    null,
  );
  const [selectedBoxSlot, setSelectedBoxSlot] = useState<number | null>(null);

  if (!isOpen) return null;

  const handlePrevBox = () => {
    const prev = currentBox <= 0 ? boxCount - 1 : currentBox - 1;
    PhaserNet.sendPokemonPCSwitchBox(prev);
    setSelectedBoxSlot(null);
  };

  const handleNextBox = () => {
    const next = currentBox >= boxCount - 1 ? 0 : currentBox + 1;
    PhaserNet.sendPokemonPCSwitchBox(next);
    setSelectedBoxSlot(null);
  };

  const handleDeposit = () => {
    if (selectedPartySlot === null) return;
    if (party.length <= 1) return;
    PhaserNet.sendPokemonPCDeposit(selectedPartySlot, currentBox);
    setSelectedPartySlot(null);
  };

  const handleWithdraw = () => {
    if (selectedBoxSlot === null) return;
    if (party.length >= 6) return;
    const pokemon = boxPokemon[selectedBoxSlot];
    if (!pokemon) return;
    PhaserNet.sendPokemonPCWithdraw(currentBox, pokemon.boxSlot);
    setSelectedBoxSlot(null);
  };

  const handleRelease = () => {
    if (selectedBoxSlot === null) return;
    const pokemon = boxPokemon[selectedBoxSlot];
    if (!pokemon) return;
    if (!window.confirm(`Release ${pokemon.name}? This cannot be undone!`))
      return;
    PhaserNet.sendPokemonPCRelease(currentBox, pokemon.boxSlot);
    setSelectedBoxSlot(null);
  };

  const handleClose = () => {
    closePC();
    setSelectedPartySlot(null);
    setSelectedBoxSlot(null);
  };

  return (
    <Overlay onClick={handleClose}>
      <PCWindow onClick={(e) => e.stopPropagation()}>
        <PCHeader>
          <span>BILL&apos;s PC</span>
          <span style={{ fontSize: "10px", opacity: 0.8 }}>
            BOX {currentBox + 1} / {boxCount}
          </span>
        </PCHeader>

        <BoxNav>
          <NavButton onClick={handlePrevBox}>◀</NavButton>
          <span>BOX {currentBox + 1}</span>
          <NavButton onClick={handleNextBox}>▶</NavButton>
        </BoxNav>

        <ContentArea>
          <BoxPanel>
            <PanelTitle>PC BOX {currentBox + 1}</PanelTitle>
            {boxPokemon.length === 0 ? (
              <PokemonSlot $selected={false} $empty>
                <PokemonName style={{ color: "#a0a0a0", fontStyle: "italic" }}>
                  Empty
                </PokemonName>
              </PokemonSlot>
            ) : (
              boxPokemon.map((p: PokemonDTO, i: number) => (
                <PokemonSlot
                  key={`box-${i}`}
                  $selected={selectedBoxSlot === i}
                  onClick={() => {
                    setSelectedBoxSlot(i);
                    setSelectedPartySlot(null);
                  }}
                >
                  <PokemonInfo>
                    <PokemonName>{p.name}</PokemonName>
                    <PokemonLevel>
                      Lv.{p.level} &nbsp; {p.curHp}/{p.maxHp} HP
                    </PokemonLevel>
                  </PokemonInfo>
                </PokemonSlot>
              ))
            )}
          </BoxPanel>

          <PartyPanel>
            <PanelTitle>PARTY ({party.length}/6)</PanelTitle>
            {party.map((p: PokemonDTO, i: number) => (
              <PokemonSlot
                key={`party-${i}`}
                $selected={selectedPartySlot === i}
                onClick={() => {
                  setSelectedPartySlot(i);
                  setSelectedBoxSlot(null);
                }}
              >
                <PokemonInfo>
                  <PokemonName>{p.name}</PokemonName>
                  <PokemonLevel>
                    Lv.{p.level} &nbsp; {p.curHp}/{p.maxHp} HP
                  </PokemonLevel>
                </PokemonInfo>
              </PokemonSlot>
            ))}
          </PartyPanel>
        </ContentArea>

        <Footer>
          <PCButton
            $variant="deposit"
            disabled={selectedPartySlot === null || party.length <= 1}
            onClick={handleDeposit}
          >
            DEPOSIT
          </PCButton>
          <PCButton
            $variant="withdraw"
            disabled={selectedBoxSlot === null || party.length >= 6}
            onClick={handleWithdraw}
          >
            WITHDRAW
          </PCButton>
          <PCButton
            $variant="release"
            disabled={selectedBoxSlot === null}
            onClick={handleRelease}
          >
            RELEASE
          </PCButton>
          <PCButton $variant="cancel" onClick={handleClose}>
            CLOSE
          </PCButton>
        </Footer>
      </PCWindow>
    </Overlay>
  );
};

export default PokemonPC;
