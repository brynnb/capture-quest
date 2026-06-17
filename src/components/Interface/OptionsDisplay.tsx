import React from "react";
import styled from "styled-components";
import useGameStatusStore from "@stores/GameStatusStore";
import ActionButton from "@components/Interface/ActionButton";

// Center viewport container - matching other display components
const CenterViewport = styled.div`
  width: 902px;
  height: 650px; /* Reduced slightly to clear chatbox */
  position: absolute;
  left: 50%;
  top: 30px;
  transform: translateX(-50%);
  background: rgba(192, 193, 255, 0.57);
  backdrop-filter: blur(12px);
  border: 4px solid #4a4ba6;
  border-radius: 30px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 40px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.3);
  transition: all 0.3s ease;
`;

const OptionsTitle = styled.h2`
  font-family: "Outfit", sans-serif;
  text-transform: uppercase;
  font-weight: 800;
  font-size: 32px;
  text-align: center;
  margin: 0 0 30px 0;
  color: #2e2f66;
`;

const OptionsGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 25px 40px;
  width: 800px;
  max-height: 420px;
  overflow-y: auto;
  padding: 20px;

  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.5) transparent;

  &::-webkit-scrollbar {
    width: 8px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background-color: rgba(255, 255, 255, 0.5);
    border-radius: 4px;
  }
`;

const OptionCard = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  cursor: pointer;
`;

const OptionLabel = styled.div`
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-size: 18px;
  font-weight: 800;
  text-transform: uppercase;
  text-align: center;
  line-height: 1.3;
  min-height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const ToggleButton = styled.button<{ $isOn: boolean }>`
  width: 80px;
  height: 32px;
  border: 3px solid #4a4ba6;
  cursor: pointer;
  font-family: "Outfit", sans-serif;
  font-size: 16px;
  font-weight: 800;
  text-transform: uppercase;
  color: #2e2f66;
  background-color: ${(props) => (props.$isOn ? "#a7edfe" : "#ffaf84")};
  border-radius: 12px;
  transition: all 0.15s ease;
  box-shadow: 0 2px 0 #4a4ba6;

  &:hover {
    filter: brightness(1.1);
    transform: translateY(-1px);
    box-shadow: 0 3px 0 #4a4ba6;
  }

  &:active {
    transform: translateY(1px);
    box-shadow: 0 1px 0 #4a4ba6;
  }
`;

const VolumeContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 15px;
  grid-column: span 3;
  margin-top: 10px;
  padding: 15px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 5px;
  border: 1px solid rgba(255, 255, 255, 0.1);
`;

const VolumeRow = styled.div`
  display: flex;
  align-items: center;
  gap: 20px;
  width: 100%;
`;

const VolumeLabel = styled.div`
  color: white;
  font-family: "Inter", sans-serif;
  font-size: 16px;
  font-weight: bold;
  text-transform: uppercase;
  width: 120px;
  flex-shrink: 0;
`;

const SliderWrapper = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  gap: 15px;
`;

const RangeInput = styled.input`
  flex: 1;
  cursor: pointer;
  accent-color: #d4c4a8;
  height: 6px;
  -webkit-appearance: none;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
  outline: none;

  &::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 18px;
    height: 18px;
    background: #ffffff;
    border: 2px solid #d4c4a8;
    border-radius: 50%;
    cursor: pointer;
    box-shadow: 0 0 5px rgba(0, 0, 0, 0.5);
  }

  &:hover::-webkit-slider-thumb {
    box-shadow: 0 0 8px rgba(255, 255, 255, 0.8);
  }
`;

const VolumeValue = styled.div`
  color: #d4c4a8;
  font-family: "Inter", sans-serif;
  font-size: 14px;
  font-weight: bold;
  width: 40px;
  text-align: right;
`;

const TooltipBox = styled.div`
  position: absolute;
  bottom: 20px;
  left: 20px;
  width: 380px;
  height: auto;
  min-height: 80px;
  background: rgba(255, 255, 255, 0.4);
  backdrop-filter: blur(8px);
  border: 3px solid #4a4ba6;
  border-radius: 16px;
  padding: 15px;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  font-size: 14px;
  line-height: 1.4;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
`;

const TooltipLabel = styled.div`
  color: #2e2f66;
  font-weight: 800;
  text-transform: uppercase;
  margin-bottom: 4px;
  font-size: 13px;
`;

const TooltipText = styled.div`
  color: #3e3f7a;
  font-weight: 500;
`;

const TooltipPlaceholder = styled.div`
  color: rgba(46, 47, 102, 0.5);
  font-style: italic;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
`;

interface OptionConfig {
  id: string;
  label: string;
  tooltip: string;
  value: boolean;
  onChange: () => void;
}

interface OptionItemProps {
  option: OptionConfig;
  onHover: (tooltip: string | null, label: string | null) => void;
}

const OptionItem: React.FC<OptionItemProps> = ({ option, onHover }) => {
  return (
    <OptionCard
      onMouseEnter={() => onHover(option.tooltip, option.label)}
      onMouseLeave={() => onHover(null, null)}
    >
      <OptionLabel>{option.label}</OptionLabel>
      <ToggleButton $isOn={option.value} onClick={option.onChange}>
        {option.value ? "ON" : "OFF"}
      </ToggleButton>
    </OptionCard>
  );
};

interface VolumeSliderProps {
  label: string;
  value: number;
  tooltip: string;
  onChange: (value: number) => void;
  onHover: (tooltip: string | null, label: string | null) => void;
}

const VolumeSlider: React.FC<VolumeSliderProps> = ({
  label,
  value,
  tooltip,
  onChange,
  onHover,
}) => {
  return (
    <VolumeRow
      onMouseEnter={() => onHover(tooltip, label)}
      onMouseLeave={() => onHover(null, null)}
    >
      <VolumeLabel>{label}</VolumeLabel>
      <SliderWrapper>
        <RangeInput
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={value}
          onChange={(e) => onChange(parseFloat(e.target.value))}
        />
        <VolumeValue>{Math.round(value * 100)}%</VolumeValue>
      </SliderWrapper>
    </VolumeRow>
  );
};

const OptionsDisplay: React.FC = () => {
  const {
    toggleOptions,
    isMuted,
    toggleMute,
    sfxVolume,
    setSFXVolume,
    ambientVolume,
    setAmbientVolume,
    musicVolume,
    setMusicVolume,
    allowTrainerRebattles,
    toggleAllowTrainerRebattles,
  } = useGameStatusStore();

  // Hover state for tooltip
  const [hoveredTooltip, setHoveredTooltip] = React.useState<string | null>(
    null,
  );
  const [hoveredLabel, setHoveredLabel] = React.useState<string | null>(null);

  const handleHover = (tooltip: string | null, label: string | null) => {
    setHoveredTooltip(tooltip);
    setHoveredLabel(label);
  };

  const options: OptionConfig[] = [
    {
      id: "mute",
      label: "Mute Audio",
      tooltip: "Mutes all game audio and music.",
      value: isMuted,
      onChange: toggleMute,
    },
    {
      id: "allow-trainer-rebattles",
      label: "Trainer Re-battles",
      tooltip:
        "When enabled, defeated trainers will challenge you again when you walk into their sight range. In the future, trainers will automatically reset after healing at a Pok\u00e9mon Center.",
      value: allowTrainerRebattles,
      onChange: toggleAllowTrainerRebattles,
    },
  ];

  return (
    <CenterViewport>
      <OptionsTitle>OPTIONS</OptionsTitle>
      <OptionsGrid>
        {options.map((option) => (
          <OptionItem key={option.id} option={option} onHover={handleHover} />
        ))}

        {/* Separate Volume Section */}
        <VolumeContainer>
          <VolumeSlider
            label="Music"
            value={musicVolume}
            tooltip="Adjust global music volume level."
            onChange={setMusicVolume}
            onHover={handleHover}
          />
          <VolumeSlider
            label="SFX"
            value={sfxVolume}
            tooltip="Adjust Pokémon battle and interface sound effect volume."
            onChange={setSFXVolume}
            onHover={handleHover}
          />
          <VolumeSlider
            label="Ambient"
            value={ambientVolume}
            tooltip="Adjust zone background sounds and environmental audio."
            onChange={setAmbientVolume}
            onHover={handleHover}
          />
        </VolumeContainer>
      </OptionsGrid>

      {/* Permanent tooltip box in bottom left */}
      <TooltipBox>
        {hoveredTooltip ? (
          <>
            <TooltipLabel>{hoveredLabel}</TooltipLabel>
            <TooltipText>{hoveredTooltip}</TooltipText>
          </>
        ) : (
          <TooltipPlaceholder>
            Hover over an option to see its description
          </TooltipPlaceholder>
        )}
      </TooltipBox>

      <ActionButton
        text="Done"
        onClick={toggleOptions}
        customCSS={`position: absolute; bottom: 30px; right: 30px; width: 120px;`}
      />
    </CenterViewport>
  );
};

export default OptionsDisplay;
