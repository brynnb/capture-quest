import React from 'react';
import styled from 'styled-components';

const ViewContainer = styled.div`
  overflow: hidden;
  position: absolute;
  top: 0;
  left: 0;
  z-index: -2;
  width: 100%;
  height: 100%;
`;

const CharacterSelectBackground: React.FC = () => {
    return <ViewContainer />;
};

export default CharacterSelectBackground;
