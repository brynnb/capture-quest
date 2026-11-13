package scriptcandidateimport

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// The extractor preserves text macro invocations in its IR even when it has no
// portable candidate for the macro. Translate the complete family here rather
// than maintaining map-specific event files. Source: macros/scripts/text.asm and
// engine/events/pokecenter.asm in the negotiated extractor source tree.
func loadTextMacroCandidates(ctx context.Context, database *sql.DB) ([]scriptCandidate, error) {
	exists, err := sqliteTableExists(ctx, database, "script_event_ir_blocks")
	if err != nil || !exists {
		return nil, err
	}
	rows, err := database.QueryContext(ctx, `SELECT id,map_name,map_id,label,kind,raw_asm FROM script_event_ir_blocks WHERE instr(raw_asm,'script_pokecenter_nurse')>0 ORDER BY map_name,label,id`)
	if err != nil {
		return nil, fmt.Errorf("query nurse text macros: %w", err)
	}
	type block struct {
		id, mapID                 int
		mapName, label, kind, raw string
	}
	var blocks []block
	for rows.Next() {
		var b block
		if err := rows.Scan(&b.id, &b.mapName, &b.mapID, &b.label, &b.kind, &b.raw); err != nil {
			rows.Close()
			return nil, err
		}
		blocks = append(blocks, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(blocks) == 0 {
		return nil, err
	}

	texts := make(map[string][]string)
	for _, label := range []string{"_PokemonCenterWelcomeText", "_ShallWeHealYourPokemonText", "_NeedYourPokemonText", "_PokemonFightingFitText", "_PokemonCenterFarewellText"} {
		var count int
		var text string
		if err := database.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(dialogue),'') FROM dialogue_text WHERE label=?`, label).Scan(&count, &text); err != nil {
			return nil, fmt.Errorf("read nurse dialogue %s: %w", label, err)
		}
		if count != 1 || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("nurse dialogue %s requires one nonempty source record; found %d", label, count)
		}
		texts[label] = strings.Split(text, "\n")
	}
	seen := make(map[string]bool)
	var candidates []scriptCandidate
	for _, b := range blocks {
		var commands []string
		for _, line := range strings.Split(b.raw, "\n") {
			line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
			if line != "" {
				commands = append(commands, line)
			}
		}
		if b.kind != "text" || b.label == "" || len(commands) != 2 || commands[0] != b.label+":" || commands[1] != "script_pokecenter_nurse" {
			return nil, fmt.Errorf("nurse IR row %d (%s/%s) has unsupported macro body", b.id, b.mapName, b.label)
		}
		var mapCount, pointerCount int
		var textConstant string
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM maps WHERE id=? AND name=?`, b.mapID, mapNameToUpperSnake(b.mapName)).Scan(&mapCount); err != nil {
			return nil, err
		}
		if err := database.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(text_constant),'') FROM text_pointers WHERE map_name=? AND map_id=? AND local_label=? AND is_trainer=0`, b.mapName, b.mapID, b.label).Scan(&pointerCount, &textConstant); err != nil {
			return nil, fmt.Errorf("nurse IR row %d text pointer: %w", b.id, err)
		}
		key := b.mapName + "/" + textConstant
		if mapCount != 1 || pointerCount != 1 || textConstant == "" || seen[key] {
			return nil, fmt.Errorf("nurse IR row %d (%s/%s) has missing, duplicate or inconsistent map/text identity", b.id, b.mapName, b.label)
		}
		seen[key] = true
		candidates = append(candidates, scriptCandidate{
			Version: supportedCandidateSchemaVersion, Kind: "scriptEventCandidate", MapName: b.mapName, ScriptLabel: b.label,
			Trigger: candidateTrigger{Type: "npc_click", Label: textConstant}, Confidence: "source_macro",
			Actions: []candidateAction{
				{Type: "lockInput"},
				{Type: "dialogue", Lines: texts["_PokemonCenterWelcomeText"]},
				{Type: "choice", Prompt: strings.Join(texts["_ShallWeHealYourPokemonText"], "\n"), YesLines: texts["_NeedYourPokemonText"], NoLines: texts["_PokemonCenterFarewellText"]},
				{Type: "healParty", HealingPolicy: "center"},
				{Type: "playSFX", SFXConstant: "SFX_HEALING_MACHINE"},
				{Type: "dialogue", Lines: texts["_PokemonFightingFitText"]},
				{Type: "dialogue", Lines: texts["_PokemonCenterFarewellText"]},
				{Type: "unlockInput"},
			},
		})
	}
	return candidates, nil
}
