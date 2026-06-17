import React, { useEffect, useState } from "react";
import styled from "styled-components";
import useCQInventoryStore, {
  type CQMerchantItem,
} from "@/stores/CQInventoryStore";
import * as PhaserNet from "@/phaser-game/services/PhaserNetworkService";

const Overlay = styled.div`
  position: absolute;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: all;
  box-sizing: border-box;
`;

const ShopWindow = styled.div`
  width: 420px;
  max-width: calc(100% - 48px);
  max-height: 520px;
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

const ShopHeader = styled.div`
  background: #e03030;
  color: white;
  padding: 10px 16px;
  font-size: 13px;
  text-align: center;
  border-bottom: 3px solid #383838;
  text-shadow: 1px 1px 0 rgba(0, 0, 0, 0.3);
`;

const MoneyBar = styled.div`
  background: #e8e8d8;
  padding: 8px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 2px solid #c8c8b8;
  font-size: 10px;
`;

const ItemList = styled.div`
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
  max-height: 340px;
`;

const ItemRow = styled.div<{ $selected: boolean }>`
  display: flex;
  align-items: center;
  padding: 8px 16px;
  cursor: pointer;
  background: ${(p) => (p.$selected ? "#d0d0ff" : "transparent")};
  border-left: ${(p) => (p.$selected ? "4px solid #4040c0" : "4px solid transparent")};
  transition: background 0.1s;

  &:hover {
    background: ${(p) => (p.$selected ? "#d0d0ff" : "#e8e8e0")};
  }
`;

const ItemName = styled.span`
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const ItemPrice = styled.span`
  color: #606060;
  margin-left: 12px;
  white-space: nowrap;
`;

const Footer = styled.div`
  border-top: 3px solid #383838;
  padding: 10px 16px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  background: #e8e8d8;
`;

const ShopButton = styled.button<{ $variant?: "buy" | "cancel" }>`
  font-family: "Press Start 2P", "Courier New", monospace;
  font-size: 10px;
  padding: 8px 16px;
  border: 2px solid #383838;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.1s;

  background: ${(p) =>
    p.$variant === "buy"
      ? "#40a040"
      : p.$variant === "cancel"
        ? "#a04040"
        : "#808080"};
  color: white;

  &:hover {
    filter: brightness(1.15);
  }

  &:active {
    transform: scale(0.97);
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    filter: none;
  }
`;

const QuantityControl = styled.div`
  display: flex;
  align-items: center;
  gap: 6px;
  margin-right: 8px;
`;

const QtyButton = styled.button`
  font-family: "Press Start 2P", "Courier New", monospace;
  font-size: 10px;
  width: 24px;
  height: 24px;
  border: 2px solid #383838;
  border-radius: 4px;
  background: #d8d8d0;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;

  &:hover {
    background: #c0c0b8;
  }
`;

const QtyDisplay = styled.span`
  min-width: 20px;
  text-align: center;
  font-size: 11px;
`;

const PokeMartShop: React.FC = () => {
  const { shopOpen, shopName, shopMerchantId, shopItems, money, closeShop } =
    useCQInventoryStore();
  const [selectedIdx, setSelectedIdx] = useState(0);
  const [quantity, setQuantity] = useState(1);
  const [buying, setBuying] = useState(false);

  useEffect(() => {
    if (!shopOpen) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      closeShop();
    };

    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [closeShop, shopOpen]);

  if (!shopOpen) return null;

  const selectedItem: CQMerchantItem | undefined = shopItems[selectedIdx];
  const itemPrice = selectedItem
    ? (selectedItem.priceOverride ?? selectedItem.item.price)
    : 0;
  const totalCost = itemPrice * quantity;
  const canAfford = money >= totalCost;

  const handleBuy = () => {
    if (!selectedItem || !canAfford || !shopMerchantId || buying) return;
    setBuying(true);
    PhaserNet.sendCQMerchantBuy(shopMerchantId, selectedItem.itemId, quantity);
    setTimeout(() => setBuying(false), 300);
    setQuantity(1);
  };

  const handleSelectItem = (idx: number) => {
    setSelectedIdx(idx);
    setQuantity(1);
  };

  const formatMoney = (val: number) => `¥${val.toLocaleString()}`;

  return (
    <Overlay onClick={closeShop}>
      <ShopWindow onClick={(e) => e.stopPropagation()}>
        <ShopHeader>{shopName || "POKé MART"}</ShopHeader>
        <MoneyBar>
          <span>Your Money:</span>
          <span style={{ fontWeight: "bold" }}>{formatMoney(money)}</span>
        </MoneyBar>
        <ItemList>
          {shopItems.map((mi, idx) => (
            <ItemRow
              key={mi.itemId}
              $selected={idx === selectedIdx}
              onClick={() => handleSelectItem(idx)}
            >
              <ItemName>{mi.item.name}</ItemName>
              <ItemPrice>
                {formatMoney(mi.priceOverride ?? mi.item.price)}
              </ItemPrice>
            </ItemRow>
          ))}
          {shopItems.length === 0 && (
            <div style={{ padding: "16px", textAlign: "center", color: "#888" }}>
              No items for sale.
            </div>
          )}
        </ItemList>
        <Footer>
          {selectedItem && (
            <QuantityControl>
              <QtyButton
                onClick={() => setQuantity(Math.max(1, quantity - 1))}
                disabled={quantity <= 1}
              >
                -
              </QtyButton>
              <QtyDisplay>{quantity}</QtyDisplay>
              <QtyButton
                onClick={() => setQuantity(Math.min(99, quantity + 1))}
              >
                +
              </QtyButton>
              <span style={{ fontSize: "9px", color: "#606060" }}>
                = {formatMoney(totalCost)}
              </span>
            </QuantityControl>
          )}
          <ShopButton
            $variant="buy"
            onClick={handleBuy}
            disabled={!selectedItem || !canAfford || buying}
          >
            BUY
          </ShopButton>
          <ShopButton $variant="cancel" onClick={closeShop}>
            EXIT
          </ShopButton>
        </Footer>
      </ShopWindow>
    </Overlay>
  );
};

export default PokeMartShop;
