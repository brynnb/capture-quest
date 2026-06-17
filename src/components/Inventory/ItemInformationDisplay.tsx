import React from "react";
import styled from "styled-components";
import { Item } from "@entities/Item";

const ItemDisplayContainer = styled.div.attrs({
  className: "item-display-container",
})`
  width: 902px;
  height: 300px;
  left: 266px;
  top: 722px;
  padding-top: 40px;
  padding-bottom: 20px;
  position: absolute;
  background-image: url("/images/chatbginspect.png");
  background-size: cover;
  font-size: 20px;
  line-height: 1.2;
  display: flex;
  font-family: Arial, sans-serif;
  color: black;
  font-weight: bold;

  p {
    margin: 5px 0;
    font-size: 16px;
    line-height: 1.2;
    color: black;
    font-weight: 600;
  }
`;

const ItemDisplayContent = styled.div`
  position: absolute;
  top: 50px;
  left: 120px;
  right: 60px;
  bottom: 60px;
  overflow: auto;
  word-wrap: break-word;
  white-space: pre-wrap;
  padding-right: 25px;
`;

interface ItemDisplayProps {
  item: Item | null;
  isVisible: boolean;
}

const ItemDisplay: React.FC<ItemDisplayProps> = ({ item, isVisible }) => {
  if (!item || !isVisible) return null;

  return (
    <ItemDisplayContainer>
      <ItemDisplayContent>
        <p>{item.name}</p>
        {item.loreText && <p>{item.loreText}</p>}
      </ItemDisplayContent>
    </ItemDisplayContainer>
  );
};

export default ItemDisplay;
