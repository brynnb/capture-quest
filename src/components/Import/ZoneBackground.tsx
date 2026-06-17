import React from 'react';
import styled from 'styled-components';

const ViewContainer = styled.div`
  overflow: hidden;
  position: absolute;
  top: 0;
  left: 0;
  z-index: -1;
  width: 1440px;
  height: 720px;
  pointer-events: none;
`;

const ZoneBackground: React.FC = () => {
  return (
    <ViewContainer className="view" />
  );
};

export default ZoneBackground;
