import styled from "styled-components";

export const BADGE_NAMES = [
  "Boulder",
  "Cascade",
  "Thunder",
  "Rainbow",
  "Soul",
  "Marsh",
  "Volcano",
  "Earth",
];

export const BADGE_FLAGS = [
  "EVENT_GOT_BOULDERBADGE",
  "EVENT_GOT_CASCADEBADGE",
  "EVENT_GOT_THUNDERBADGE",
  "EVENT_GOT_RAINBOWBADGE",
  "EVENT_GOT_SOULBADGE",
  "EVENT_GOT_MARSHBADGE",
  "EVENT_GOT_VOLCANOBADGE",
  "EVENT_GOT_EARTHBADGE",
];

const ATLAS_COLUMNS: number = 4;
const ATLAS_ROWS: number = 2;
const BADGE_CELL_RATIO = 81 / ATLAS_ROWS / (150 / ATLAS_COLUMNS);

const BadgeImage = styled.span`
  display: inline-block;
  flex: 0 0 auto;
  background-image: url("/images/badgeatlas.png");
  background-repeat: no-repeat;
  background-size: ${ATLAS_COLUMNS * 100}% ${ATLAS_ROWS * 100}%;
  image-rendering: pixelated;
`;

const EmptyBadge = styled.span`
  display: inline-block;
  flex: 0 0 auto;
  box-sizing: border-box;
  border: 2px solid #7f7f7f;
  border-radius: 50%;
  background: #f8f0e0;
  box-shadow: inset 0 0 0 2px rgba(255, 255, 255, 0.65);
`;

type BadgeAtlasIconProps = {
  index: number;
  earned?: boolean;
  size?: number;
  title?: string;
};

function atlasPosition(index: number): string {
  const safeIndex = Math.max(0, Math.min(BADGE_NAMES.length - 1, index));
  const column = safeIndex % ATLAS_COLUMNS;
  const row = Math.floor(safeIndex / ATLAS_COLUMNS);
  const x = ATLAS_COLUMNS === 1 ? 0 : (column / (ATLAS_COLUMNS - 1)) * 100;
  const y = ATLAS_ROWS === 1 ? 0 : (row / (ATLAS_ROWS - 1)) * 100;
  return `${x}% ${y}%`;
}

export function BadgeAtlasIcon({
  index,
  earned = true,
  size = 32,
  title,
}: BadgeAtlasIconProps) {
  const name = title || BADGE_NAMES[index] || "Badge";

  if (!earned) {
    return (
      <EmptyBadge
        aria-label={`${name} not earned`}
        role="img"
        style={{
          width: `${size}px`,
          height: `${size}px`,
        }}
        title={name}
      />
    );
  }

  return (
    <BadgeImage
      aria-label={name}
      role="img"
      style={{
        width: `${size}px`,
        height: `${Math.round(size * BADGE_CELL_RATIO)}px`,
        backgroundPosition: atlasPosition(index),
      }}
      title={name}
    />
  );
}
