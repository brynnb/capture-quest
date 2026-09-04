package scriptcandidateimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

func loadExistingScripts(outputDir string) (existingScripts, error) {
	result := existingScripts{
		ByLabel:      map[string]existingScript{},
		ByTrigger:    map[string]existingScript{},
		ByMapSetFlag: map[string]existingScript{},
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return existingScripts{}, fmt.Errorf("read output dir %s: %w", outputDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return existingScripts{}, fmt.Errorf("read %s: %w", path, err)
		}
		var event scriptedevents.EventFile
		if err := json.Unmarshal(raw, &event); err != nil {
			return existingScripts{}, fmt.Errorf("decode %s: %w", path, err)
		}
		if event.ScriptLabel == "" {
			continue
		}
		record := existingScript{
			Path:        path,
			Source:      event.Trigger.Source,
			ScriptLabel: event.ScriptLabel,
		}
		result.ByLabel[event.ScriptLabel] = record
		if key := triggerKeyForEvent(event); key != "" {
			if current, ok := result.ByTrigger[key]; !ok || current.Source != capturequestSource || record.Source == capturequestSource {
				result.ByTrigger[key] = record
			}
		}
		for _, flag := range setFlagsForEvent(event) {
			if key := mapSetFlagKey(event.MapName, flag); key != "" {
				if current, ok := result.ByMapSetFlag[key]; !ok || current.Source != capturequestSource || record.Source == capturequestSource {
					result.ByMapSetFlag[key] = record
				}
			}
		}
	}
	return result, nil
}

func (scripts existingScripts) ownerForMapSetFlag(event scriptedevents.EventFile) (existingScript, string) {
	for _, flag := range setFlagsForEvent(event) {
		if current := scripts.ByMapSetFlag[mapSetFlagKey(event.MapName, flag)]; current.ScriptLabel != "" {
			return current, flag
		}
	}
	return existingScript{}, ""
}

func setFlagsForEvent(event scriptedevents.EventFile) []string {
	flags := append([]string{}, event.SetsFlags...)
	for _, raw := range event.Actions {
		var action candidateAction
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		flags = append(flags, setFlagsForAction(action)...)
	}
	return uniqueStrings(flags)
}

func setFlagsForAction(action candidateAction) []string {
	flags := []string{}
	switch action.Type {
	case "setFlag", "setEvent":
		flag := action.Flag
		if flag == "" {
			flag = action.Event
		}
		if flag != "" {
			flags = append(flags, flag)
		}
	case "startTrainerBattle", "startWildBattle":
		if action.WinFlag != "" {
			flags = append(flags, mapEventName(action.WinFlag))
		}
		for _, nested := range action.PostWinActions {
			flags = append(flags, setFlagsForAction(nested)...)
		}
	}
	return flags
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func triggerKeyForEvent(event scriptedevents.EventFile) string {
	if event.MapName == "" || event.Trigger.Type == "" || event.Trigger.Label == "" {
		return ""
	}
	return triggerKey(event.MapName, event.Trigger.Type, event.Trigger.Label)
}

func mapSetFlagKey(mapName, flag string) string {
	mapName = mapNameToUpperSnake(mapName)
	flag = strings.TrimSpace(flag)
	if mapName == "" || flag == "" {
		return ""
	}
	return mapName + "\x00" + flag
}

func triggerKey(mapName, triggerType, triggerLabel string) string {
	parts := []string{
		mapNameToUpperSnake(mapName),
		strings.ToLower(strings.TrimSpace(triggerType)),
		strings.TrimSpace(triggerLabel),
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, "\x00")
}

func writeEventFile(plan *outputPlan, path string, event scriptedevents.EventFile) (bool, error) {
	raw, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode %s: %w", event.ScriptLabel, err)
	}
	raw = append(raw, '\n')

	return plan.Stage(path, raw)
}

func scriptFileName(label string) string {
	return camelToSnake(label) + ".json"
}

func camelToUpperSnake(value string) string {
	return strings.ToUpper(camelToSnake(value))
}

func mapNameToUpperSnake(value string) string {
	converted := camelToUpperSnake(value)
	converted = floorSuffixPattern.ReplaceAllString(converted, "_${1}F")
	return basementFloorPattern.ReplaceAllString(converted, "_B${1}F")
}

var acronymBoundaryPattern = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
var lowerUpperBoundaryPattern = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var alphaDigitBoundaryPattern = regexp.MustCompile(`([A-Za-z])([0-9])`)
var nonIdentifierPattern = regexp.MustCompile(`[^A-Za-z0-9]+`)
var floorSuffixPattern = regexp.MustCompile(`_((?:B)?\d+)_F\b`)
var basementFloorPattern = regexp.MustCompile(`_B_(\d+)F\b`)

func camelToSnake(value string) string {
	value = strings.TrimSpace(value)
	value = nonIdentifierPattern.ReplaceAllString(value, "_")
	value = acronymBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = lowerUpperBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = alphaDigitBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = strings.Trim(value, "_")
	return strings.ToLower(value)
}

func DefaultSQLitePath() string {
	candidates := []string{
		filepath.Join("..", "public", "phaser", "pokemon.db"),
		filepath.Join("public", "phaser", "pokemon.db"),
		filepath.Join("..", "..", "public", "phaser", "pokemon.db"),
		filepath.Join("..", "..", "..", "public", "phaser", "pokemon.db"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func DefaultOutputDir() string {
	for _, candidate := range []string{
		filepath.Join("scripted_events", "scripts"),
		filepath.Join("server", "scripted_events", "scripts"),
		filepath.Join("..", "scripted_events", "scripts"),
		filepath.Join("..", "..", "server", "scripted_events", "scripts"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("scripted_events", "scripts")
}

func DefaultDiagnosticsPath(outputDir string) string {
	return filepath.Join(filepath.Dir(outputDir), "script_candidate_import_diagnostics.json")
}
