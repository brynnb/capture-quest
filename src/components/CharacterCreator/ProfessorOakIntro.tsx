import styled from "styled-components";
import type { FactionData } from "@/services/characterService";

const OakPanel = styled.div`
  width: 100%;
  box-sizing: border-box;
  background: rgba(255, 236, 241, 0.95);
  backdrop-filter: blur(15px);
  border: 4px solid #ffccd9;
  border-radius: 24px;
  padding: 22px;
  color: #2e2f66;
  font-family: "Outfit", sans-serif;
  box-shadow: 0 16px 38px rgba(0, 0, 0, 0.18);
`;

const OakHeader = styled.div`
  display: grid;
  grid-template-columns: 86px 1fr;
  gap: 16px;
  align-items: center;
`;

const OakPortraitFrame = styled.div`
  width: 86px;
  height: 86px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ffffff;
  border: 3px solid #4a4ba6;
  border-radius: 14px;
  box-shadow: inset 0 -5px 0 rgba(74, 75, 166, 0.12);
`;

const OakPortrait = styled.img`
  width: 72px;
  height: 72px;
  image-rendering: pixelated;
`;

const OakName = styled.div`
  font-weight: 900;
  font-size: 22px;
  text-transform: uppercase;
  color: #4a4ba6;
`;

const OakSpeech = styled.div`
  margin-top: 4px;
  font-size: 17px;
  line-height: 1.45;
  font-weight: 700;
`;

const StarterStrip = styled.div`
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-top: 18px;
`;

const PixelAsset = styled.img`
  width: 100%;
  aspect-ratio: 1;
  object-fit: contain;
  image-rendering: pixelated;
  background: rgba(255, 255, 255, 0.58);
  border: 2px solid rgba(74, 75, 166, 0.55);
  border-radius: 10px;
  padding: 6px;
  box-sizing: border-box;
`;

const FactionNote = styled.div`
  margin-top: 16px;
  font-size: 15px;
  line-height: 1.45;
  font-weight: 600;
`;

interface ProfessorOakIntroProps {
  selectedFaction: FactionData | null;
  characterName: string;
  rivalName: string;
}

function oakMessage(
  selectedFaction: FactionData | null,
  characterName: string,
  rivalName: string,
): string {
  const player = characterName || "new trainer";
  const rival = rivalName || "Gary";

  if (!selectedFaction) {
    return "Hello there! Welcome to the world of Pokémon. My name is Oak, and this world is inhabited by creatures called Pokémon.";
  }

  return `${player}, your story is about to unfold. ${rival} may call it a race, but every good trainer knows the journey matters too.`;
}

const ProfessorOakIntro = ({
  selectedFaction,
  characterName,
  rivalName,
}: ProfessorOakIntroProps) => {
  const factionLore = (selectedFaction as { lore?: string } | null)?.lore;

  return (
    <OakPanel>
      <OakHeader>
        <OakPortraitFrame>
          <OakPortrait src="/assets/trainers/prof.oak.png" alt="Professor Oak" />
        </OakPortraitFrame>
        <div>
          <OakName>Prof. Oak</OakName>
          <OakSpeech>
            {oakMessage(selectedFaction, characterName, rivalName)}
          </OakSpeech>
        </div>
      </OakHeader>

      <StarterStrip>
        <PixelAsset src="/assets/pokemon/front/1.png" alt="Bulbasaur" />
        <PixelAsset src="/assets/pokemon/front/4.png" alt="Charmander" />
        <PixelAsset src="/assets/pokemon/front/7.png" alt="Squirtle" />
        <PixelAsset src="/phaser/sprites/pokedex.png" alt="Pokédex" />
      </StarterStrip>

      {factionLore && <FactionNote>{factionLore}</FactionNote>}
    </OakPanel>
  );
};

export default ProfessorOakIntro;
