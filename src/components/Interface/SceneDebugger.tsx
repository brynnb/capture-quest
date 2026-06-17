import { useEffect, useMemo, useState } from "react";
import { FiCheck, FiCopy, FiMap, FiRefreshCcw, FiZap } from "react-icons/fi";
import styled from "styled-components";
import useDebugSceneStore, { type DebugSceneEntry } from "@/stores/DebugSceneStore";
import { WorldSocket, OpCodes } from "@/net";
import {
  PROCEDURAL_OVERWORLD_GENERATE_EVENT,
  PROCEDURAL_OVERWORLD_GENERATED_EVENT,
  type ProceduralOverworldGeneratedDetail,
} from "@/phaser-game/procedural/overworldProceduralTiles";

const ToggleButton = styled.button`
  position: fixed;
  bottom: 12px;
  left: 12px;
  width: 72px;
  height: 72px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.7);
  border: 2px solid #555;
  color: #ffcc00;
  font-size: 36px;
  font-weight: 700;
  cursor: pointer;
  z-index: 10001;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;

  &:hover {
    background: rgba(0, 0, 0, 0.9);
    border-color: #ffcc00;
    transform: scale(1.1);
  }
`;

const Panel = styled.div`
  position: fixed;
  bottom: 96px;
  left: 12px;
  width: min(920px, calc(100vw - 24px));
  max-height: min(1240px, calc(100vh - 120px));
  background: rgba(10, 10, 15, 0.95);
  border: 2px solid #444;
  border-radius: 10px;
  z-index: 10001;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
  font-family: "Inter", "Segoe UI", sans-serif;
  overflow: hidden;
`;

const FilterWrap = styled.div`
  padding: 20px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 14px;
`;

const FilterInput = styled.input`
  width: 100%;
  box-sizing: border-box;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid #3b3b45;
  border-radius: 6px;
  color: #eee;
  font-size: 24px;
  padding: 16px 20px;
  outline: none;

  &:focus {
    border-color: #ffcc00;
  }
`;

const CategoryRow = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
`;

const CategoryButton = styled.button<{ $active?: boolean }>`
  border: 1px solid ${({ $active }) => ($active ? "#ffcc00" : "rgba(255, 255, 255, 0.12)")};
  border-radius: 6px;
  background: ${({ $active }) => ($active ? "rgba(255, 204, 0, 0.16)" : "rgba(255, 255, 255, 0.06)")};
  color: ${({ $active }) => ($active ? "#ffcc00" : "#d8d8df")};
  cursor: pointer;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  padding: 10px 14px;
  transition:
    background 0.1s,
    border-color 0.1s,
    color 0.1s;

  &:hover {
    background: rgba(255, 204, 0, 0.12);
    border-color: #ffcc00;
    color: #ffcc00;
  }
`;

const PanelHeader = styled.div`
  padding: 20px 28px;
  background: rgba(30, 30, 40, 0.9);
  border-bottom: 1px solid #333;
  display: flex;
  align-items: center;
  justify-content: space-between;
`;

const PanelTitle = styled.span`
  color: #ffcc00;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: 0.5px;
  text-transform: uppercase;
`;

const HeaderActions = styled.div`
  display: flex;
  align-items: center;
  gap: 12px;
`;

const HeaderActionButton = styled.button`
  display: inline-flex;
  align-items: center;
  gap: 10px;
  border: 1px solid rgba(255, 204, 0, 0.45);
  border-radius: 8px;
  background: rgba(255, 204, 0, 0.1);
  color: #ffcc00;
  cursor: pointer;
  font-size: 22px;
  font-weight: 700;
  padding: 10px 14px;
  transition:
    background 0.1s,
    border-color 0.1s,
    color 0.1s;

  &:hover {
    background: rgba(255, 204, 0, 0.18);
    border-color: #ffcc00;
    color: #ffe680;
  }

  svg {
    font-size: 24px;
  }
`;

const ResetAllButton = styled(HeaderActionButton)``;

const PowerPokemonButton = styled(HeaderActionButton)`
  border-color: rgba(119, 221, 153, 0.45);
  background: rgba(119, 221, 153, 0.1);
  color: #77dd99;

  &:hover {
    background: rgba(119, 221, 153, 0.18);
    border-color: #77dd99;
    color: #b8ffd0;
  }
`;

const ProceduralTerrainButton = styled(HeaderActionButton)`
  border-color: rgba(120, 190, 255, 0.45);
  background: rgba(120, 190, 255, 0.1);
  color: #8fc8ff;

  &:hover {
    background: rgba(120, 190, 255, 0.18);
    border-color: #8fc8ff;
    color: #cce8ff;
  }
`;

const CloseBtn = styled.button`
  background: none;
  border: none;
  color: #888;
  font-size: 32px;
  cursor: pointer;
  padding: 0 8px;
  &:hover {
    color: #fff;
  }
`;

const SceneList = styled.div`
  overflow-y: auto;
  flex: 1;
  max-height: 880px;

  &::-webkit-scrollbar {
    width: 12px;
  }
  &::-webkit-scrollbar-track {
    background: transparent;
  }
  &::-webkit-scrollbar-thumb {
    background: #444;
    border-radius: 6px;
  }
`;

const SceneRow = styled.div`
  display: flex;
  align-items: flex-start;
  gap: 16px;
  width: 100%;
  padding: 0;
  background: transparent;
  border: none;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  transition: background 0.1s;

  &:hover {
    background: rgba(255, 204, 0, 0.08);
  }

  &:active {
    background: rgba(255, 204, 0, 0.15);
  }
`;

const SceneJumpButton = styled.button`
  display: flex;
  align-items: flex-start;
  gap: 20px;
  min-width: 0;
  flex: 1;
  padding: 16px 0 16px 28px;
  background: transparent;
  border: none;
  cursor: pointer;
  text-align: left;
`;

const CopyButton = styled.button<{ $copied?: boolean }>`
  flex-shrink: 0;
  width: 56px;
  height: 56px;
  margin: 16px 24px 16px 0;
  border-radius: 8px;
  border: 1px solid ${({ $copied }) => ($copied ? "#77dd99" : "rgba(255, 255, 255, 0.12)")};
  background: ${({ $copied }) => ($copied ? "rgba(119, 221, 153, 0.14)" : "rgba(255, 255, 255, 0.06)")};
  color: ${({ $copied }) => ($copied ? "#77dd99" : "#d8d8df")};
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  transition:
    background 0.1s,
    border-color 0.1s,
    color 0.1s;

  &:hover {
    background: rgba(255, 204, 0, 0.12);
    border-color: #ffcc00;
    color: #ffcc00;
  }
`;

const SeqBadge = styled.span`
  flex-shrink: 0;
  width: 56px;
  height: 56px;
  border-radius: 6px;
  background: rgba(255, 204, 0, 0.15);
  color: #ffcc00;
  font-size: 24px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const SceneInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
`;

const SceneLabel = styled.span`
  color: #ddd;
  font-size: 24px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`;

const SceneDesc = styled.span`
  color: #888;
  font-size: 22px;
  line-height: 1.3;
`;

const SceneMeta = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
`;

const MetaBadge = styled.span`
  color: #b8b8c0;
  background: rgba(255, 255, 255, 0.07);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 4px;
  padding: 4px 10px;
  font-size: 20px;
  line-height: 1.2;
`;

const EmptyState = styled.div`
  padding: 40px;
  color: #666;
  text-align: center;
  font-size: 24px;
`;

const DebugMessage = styled.div`
  padding: 14px 24px;
  color: #77dd99;
  background: rgba(119, 221, 153, 0.08);
  border-bottom: 1px solid rgba(119, 221, 153, 0.16);
  font-size: 22px;
  line-height: 1.3;
`;

const buildScenarioClipboardText = (scene: DebugSceneEntry) => {
  if (scene.scenarioJson) {
    return scene.scenarioJson;
  }
  return JSON.stringify(
    {
      name: scene.scenarioName,
      description: scene.description,
      triggerType: scene.triggerType,
      mapName: scene.mapName,
      scriptLabel: scene.scriptLabel,
    },
    null,
    2
  );
};

const copyTextToClipboard = async (text: string) => {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  document.execCommand("copy");
  document.body.removeChild(textarea);
};

type SceneFilterCategory = "all" | "trade" | "field";

const getSceneCategory = (scene: DebugSceneEntry): SceneFilterCategory | "" => {
  if (scene.category === "trade" || scene.category === "field") {
    return scene.category;
  }
  return scene.triggerType === "field_move_permission" ? "field" : "";
};

const SceneDebugger = () => {
  const {
    scenes,
    isOpen,
    powerPokemonMessage,
    toggleOpen,
    setOpen,
    setPowerPokemonMessage,
  } = useDebugSceneStore();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<SceneFilterCategory>("all");
  const [copiedScenarioName, setCopiedScenarioName] = useState<string | null>(null);
  const [proceduralTerrainMessage, setProceduralTerrainMessage] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      WorldSocket.sendJsonMessage(OpCodes.DebugSceneListRequest, {});
    }
  }, [isOpen]);

  const filteredScenes = useMemo(() => {
    const q = query.trim().toLowerCase();
    return scenes.filter((scene) => {
      const sceneCategory = getSceneCategory(scene);
      if (category !== "all" && sceneCategory !== category) {
        return false;
      }
      if (!q) {
        return true;
      }
      return [
        scene.label,
        scene.description,
        scene.scenarioName,
        scene.triggerType,
        scene.mapName,
        scene.scriptLabel || "",
        sceneCategory,
      ]
        .join(" ")
        .toLowerCase()
        .includes(q);
    });
  }, [category, query, scenes]);

  const tradeSceneCount = useMemo(
    () => scenes.filter((scene) => getSceneCategory(scene) === "trade").length,
    [scenes]
  );
  const fieldMoveSceneCount = useMemo(
    () => scenes.filter((scene) => getSceneCategory(scene) === "field").length,
    [scenes]
  );

  const handleJump = (seqNum: number, scenarioName: string) => {
    WorldSocket.sendJsonMessage(OpCodes.DebugSceneJumpRequest, { seqNum, scenarioName });
    setOpen(false);
  };

  const handleResetAll = () => {
    WorldSocket.sendJsonMessage(OpCodes.DebugSceneJumpRequest, { resetAll: true });
    setOpen(false);
  };

  const handleGivePowerPokemon = () => {
    WorldSocket.sendJsonMessage(OpCodes.DebugGivePowerPokemonRequest, {});
  };

  const handleGenerateProceduralTerrain = () => {
    window.dispatchEvent(
      new CustomEvent(PROCEDURAL_OVERWORLD_GENERATE_EVENT, {
        detail: { seed: Math.floor(Date.now() % 0x7fffffff) },
      }),
    );
  };

  const handleCopy = async (scene: DebugSceneEntry) => {
    try {
      await copyTextToClipboard(buildScenarioClipboardText(scene));
      setCopiedScenarioName(scene.scenarioName);
      window.setTimeout(() => {
        setCopiedScenarioName((current) => (current === scene.scenarioName ? null : current));
      }, 1500);
    } catch (error) {
      console.error("[DebugScene] Failed to copy scenario", error);
    }
  };

  useEffect(() => {
    if (!powerPokemonMessage) return;
    const timer = window.setTimeout(() => setPowerPokemonMessage(null), 3000);
    return () => window.clearTimeout(timer);
  }, [powerPokemonMessage, setPowerPokemonMessage]);

  useEffect(() => {
    const handleGenerated = (event: Event) => {
      const detail = (event as CustomEvent<ProceduralOverworldGeneratedDetail>).detail;
      if (detail.error) {
        setProceduralTerrainMessage(detail.error);
        return;
      }

      setProceduralTerrainMessage(
        `Generated ${detail.renderedTileCount.toLocaleString()} preview tiles ` +
          `(${detail.rockTileCount.toLocaleString()} rock, ` +
          `${detail.grassPatchTileCount.toLocaleString()} grass patch, ` +
          `${detail.waterTileCount.toLocaleString()} water, seed ${detail.seed}).`,
      );
    };

    window.addEventListener(PROCEDURAL_OVERWORLD_GENERATED_EVENT, handleGenerated);
    return () => {
      window.removeEventListener(PROCEDURAL_OVERWORLD_GENERATED_EVENT, handleGenerated);
    };
  }, []);

  return (
    <>
      <ToggleButton onClick={toggleOpen} title="Scenario Debugger">
        S
      </ToggleButton>

      {isOpen && (
        <Panel>
          <PanelHeader>
            <PanelTitle>Scenario Debugger</PanelTitle>
            <HeaderActions>
              <ProceduralTerrainButton onClick={handleGenerateProceduralTerrain} title="Generate client-only overworld preview terrain">
                <FiMap />
                Gen Terrain
              </ProceduralTerrainButton>
              <PowerPokemonButton onClick={handleGivePowerPokemon} title="Add a level 100 Mewtwo to party or PC">
                <FiZap />
                Power Mon
              </PowerPokemonButton>
              <ResetAllButton onClick={handleResetAll} title="Reset character to fresh-world state">
                <FiRefreshCcw />
                Reset All
              </ResetAllButton>
              <CloseBtn onClick={() => setOpen(false)}>x</CloseBtn>
            </HeaderActions>
          </PanelHeader>
          <FilterWrap>
            <CategoryRow>
              <CategoryButton $active={category === "all"} onClick={() => setCategory("all")}>
                All
              </CategoryButton>
              <CategoryButton $active={category === "trade"} onClick={() => setCategory("trade")}>
                Trades {tradeSceneCount}
              </CategoryButton>
              <CategoryButton $active={category === "field"} onClick={() => setCategory("field")}>
                Field Moves {fieldMoveSceneCount}
              </CategoryButton>
            </CategoryRow>
            <FilterInput
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Filter scenarios"
            />
          </FilterWrap>
          {powerPokemonMessage && <DebugMessage>{powerPokemonMessage}</DebugMessage>}
          {proceduralTerrainMessage && <DebugMessage>{proceduralTerrainMessage}</DebugMessage>}
          <SceneList>
            {scenes.length === 0 && (
              <EmptyState>Loading scenes...</EmptyState>
            )}
            {filteredScenes.map((scene) => (
              <SceneRow key={scene.scenarioName}>
                <SceneJumpButton onClick={() => handleJump(scene.seqNum, scene.scenarioName)}>
                  <SeqBadge>{scene.seqNum}</SeqBadge>
                  <SceneInfo>
                    <SceneLabel>{scene.label}</SceneLabel>
                    <SceneMeta>
                      <MetaBadge>{scene.triggerType}</MetaBadge>
                      <MetaBadge>{scene.mapName}</MetaBadge>
                      {scene.scriptLabel && <MetaBadge>{scene.scriptLabel}</MetaBadge>}
                    </SceneMeta>
                    <SceneDesc>{scene.description}</SceneDesc>
                  </SceneInfo>
                </SceneJumpButton>
                <CopyButton
                  $copied={copiedScenarioName === scene.scenarioName}
                  aria-label={`Copy ${scene.label} scenario`}
                  title="Copy scenario JSON"
                  onClick={() => handleCopy(scene)}
                >
                  {copiedScenarioName === scene.scenarioName ? <FiCheck /> : <FiCopy />}
                </CopyButton>
              </SceneRow>
            ))}
          </SceneList>
        </Panel>
      )}
    </>
  );
};

export default SceneDebugger;
