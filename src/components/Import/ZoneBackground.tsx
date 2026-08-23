import React from 'react';
import styled from 'styled-components';

const ViewContainer = styled.div`
  overflow: hidden;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: -1;
  width: 100%;
  height: 100%;
  pointer-events: none;
`;

const ZoneBackground: React.FC = () => {
  return (
    <ViewContainer className="view" />
  );
};

export default ZoneBackground;
