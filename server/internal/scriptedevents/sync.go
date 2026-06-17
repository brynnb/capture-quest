package scriptedevents

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"capturequest/internal/pokebattle"
)

const (
	defaultRootDir              = "scripted_events"
	scriptsDirName              = "scripts"
	manualScriptsDirName        = "manual_scripts"
	visibilityFileName          = "object_visibility.json"
	generatedVisibilityFileName = "object_visibility.generated.json"
	eventTilesFileName          = "event_tile_overrides.json"
	generatedEventTilesFileName = "event_tile_overrides.generated.json"
	conditionalDialogueFileName = "conditional_dialogue.generated.json"
	captureQuestSource          = "capturequest"
	extractorSource             = "extractor"
	sourceMissableLabel         = "SourceMissableInitial:%"
)

// EventFile is the repo-owned source format for a scripted event.
type EventFile struct {
	ScriptLabel            string            `json:"scriptLabel"`
	MapName                string            `json:"mapName"`
	Trigger                EventTrigger      `json:"trigger"`
	RequiresFlag           string            `json:"requiresFlag,omitempty"`
	RequiresFlagAbsent     string            `json:"requiresFlagAbsent,omitempty"`
	RequiresFlags          []string          `json:"requiresFlags,omitempty"`
	RequiresFlagsAbsent    []string          `json:"requiresFlagsAbsent,omitempty"`
	RequiresItemID         *int              `json:"requiresItemId,omitempty"`
	RequiresItemName       string            `json:"requiresItemName,omitempty"`
	RequiresItemAbsentID   *int              `json:"requiresItemAbsentId,omitempty"`
	RequiresItemAbsentName string            `json:"requiresItemAbsentName,omitempty"`
	RequiresPokedexCaught  *int              `json:"requiresPokedexCaught,omitempty"`
	RequiresMoney          *int              `json:"requiresMoney,omitempty"`
	RequiresMoneyBelow     *int              `json:"requiresMoneyBelow,omitempty"`
	RequiresCoins          *int              `json:"requiresCoins,omitempty"`
	RequiresCoinsBelow     *int              `json:"requiresCoinsBelow,omitempty"`
	RequiresPlayerFacing   string            `json:"requiresPlayerFacing,omitempty"`
	SetsFlags              []string          `json:"setsFlags,omitempty"`
	Actions                []json.RawMessage `json:"actions"`
	Warp                   *EventWarp        `json:"warp,omitempty"`
}

type EventTrigger struct {
	Type        string            `json:"type"`
	Source      string            `json:"source,omitempty"`
	Label       string            `json:"label,omitempty"`
	Coordinates []EventCoordinate `json:"coordinates,omitempty"`
}

type EventCoordinate struct {
	MapName string `json:"mapName"`
	MapID   int    `json:"mapId"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

type EventWarp struct {
	MapID int `json:"mapId"`
	X     int `json:"x"`
	Y     int `json:"y"`
}

type ObjectVisibilityRule struct {
	MapID              int    `json:"mapId"`
	MapName            string `json:"mapName"`
	ObjectName         string `json:"objectName"`
	Visible            bool   `json:"visible"`
	RequiresFlag       string `json:"requiresFlag,omitempty"`
	RequiresFlagAbsent string `json:"requiresFlagAbsent,omitempty"`
	Label              string `json:"label,omitempty"`
}

type EventTileOverrideFile struct {
	Palettes map[string][]EventTileCell `json:"palettes,omitempty"`
	Blocks   []EventTileOverrideBlock   `json:"blocks,omitempty"`
	Tiles    []EventTileOverrideRule    `json:"tiles,omitempty"`
}

type EventTileOverrideBlock struct {
	MapID              int    `json:"mapId"`
	MapName            string `json:"mapName"`
	BlockX             int    `json:"blockX"`
	BlockY             int    `json:"blockY"`
	Palette            string `json:"palette"`
	RequiresFlag       string `json:"requiresFlag,omitempty"`
	RequiresFlagAbsent string `json:"requiresFlagAbsent,omitempty"`
	LabelPrefix        string `json:"labelPrefix"`
}

type EventTileCell struct {
	DX            int `json:"dx"`
	DY            int `json:"dy"`
	TileImageID   int `json:"tileImageId"`
	CollisionType int `json:"collisionType"`
}

type EventTileOverrideRule struct {
	MapID              int    `json:"mapId"`
	MapName            string `json:"mapName"`
	X                  int    `json:"x"`
	Y                  int    `json:"y"`
	TileImageID        int    `json:"tileImageId"`
	CollisionType      int    `json:"collisionType"`
	RequiresFlag       string `json:"requiresFlag,omitempty"`
	RequiresFlagAbsent string `json:"requiresFlagAbsent,omitempty"`
	Label              string `json:"label,omitempty"`
}

type ConditionalDialogueFile struct {
	Rows []ConditionalDialogueRule `json:"rows"`
}

type ConditionalDialogueRule struct {
	TextConstant        string         `json:"textConstant"`
	Priority            int            `json:"priority"`
	RequiresFlag        string         `json:"requiresFlag,omitempty"`
	RequiresFlagAbsent  string         `json:"requiresFlagAbsent,omitempty"`
	RequiresFlags       []string       `json:"requiresFlags,omitempty"`
	RequiresFlagsAbsent []string       `json:"requiresFlagsAbsent,omitempty"`
	DialogueLabels      []string       `json:"dialogueLabels"`
	Source              map[string]any `json:"source,omitempty"`
}

type syncStats struct {
	ScriptFiles                int
	ScriptsChanged             int
	StaleScriptsDeleted        int
	OwnedTriggerLabels         int
	TriggerRowsChanged         bool
	VisibilityRules            int
	VisibilityChanged          bool
	EventTileRules             int
	EventTilesChanged          bool
	ConditionalDialogueRules   int
	ConditionalDialogueChanged bool
}

// SyncDefault syncs scripted-event definitions from the default repo directory
// into the runtime DB. The DB remains the hot path for existing managers.
func SyncDefault(db pokebattle.DBTX) error {
	root, err := findRootDir()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("[ScriptedEvents] No %s directory found; skipping file sync", defaultRootDir)
			return nil
		}
		return err
	}
	stats, err := Sync(db, root)
	if err != nil {
		return err
	}
	log.Printf("[ScriptedEvents] Synced %d script files (%d changed), %d owned coord labels (changed=%t), %d visibility rules (changed=%t), %d event tile rules (changed=%t), %d conditional dialogue rules (changed=%t)",
		stats.ScriptFiles, stats.ScriptsChanged, stats.OwnedTriggerLabels, stats.TriggerRowsChanged, stats.VisibilityRules, stats.VisibilityChanged,
		stats.EventTileRules, stats.EventTilesChanged, stats.ConditionalDialogueRules, stats.ConditionalDialogueChanged)
	if stats.StaleScriptsDeleted > 0 {
		log.Printf("[ScriptedEvents] Removed %d stale extractor trigger script rows", stats.StaleScriptsDeleted)
	}
	return nil
}

// Sync applies all scripted-event files under root.
func Sync(db pokebattle.DBTX, root string) (syncStats, error) {
	if err := ensureCutsceneRequirementColumns(db); err != nil {
		return syncStats{}, err
	}
	if err := ensureConditionalDialogueColumns(db); err != nil {
		return syncStats{}, err
	}

	events, err := LoadEvents(root)
	if err != nil {
		return syncStats{}, err
	}

	stats := syncStats{ScriptFiles: len(events)}
	for _, event := range events {
		changed, err := syncEvent(db, event)
		if err != nil {
			return stats, err
		}
		if changed {
			stats.ScriptsChanged++
		}
	}
	deleted, err := deleteStaleExtractorTriggerScripts(db, events)
	if err != nil {
		return stats, err
	}
	stats.StaleScriptsDeleted = deleted

	coords := ownedCoordinates(events)
	stats.OwnedTriggerLabels = len(coords)
	if len(coords) > 0 {
		changed, err := syncOwnedCoordinates(db, coords)
		if err != nil {
			return stats, err
		}
		stats.TriggerRowsChanged = changed
	}

	rules, err := LoadObjectVisibility(root)
	if err != nil {
		return stats, err
	}
	stats.VisibilityRules = len(rules)
	if rules != nil {
		changed, err := syncObjectVisibility(db, rules)
		if err != nil {
			return stats, err
		}
		stats.VisibilityChanged = changed
	}

	tileRules, err := LoadEventTileOverrides(root)
	if err != nil {
		return stats, err
	}
	stats.EventTileRules = len(tileRules)
	if tileRules != nil {
		changed, err := syncEventTileOverrides(db, tileRules)
		if err != nil {
			return stats, err
		}
		stats.EventTilesChanged = changed
	}

	dialogueRules, err := LoadConditionalDialogue(root)
	if err != nil {
		return stats, err
	}
	stats.ConditionalDialogueRules = len(dialogueRules)
	if dialogueRules != nil {
		changed, err := syncConditionalDialogue(db, dialogueRules)
		if err != nil {
			return stats, err
		}
		stats.ConditionalDialogueChanged = changed
	}

	return stats, nil
}

func ensureCutsceneRequirementColumns(db pokebattle.DBTX) error {
	for _, statement := range []string{
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_money INTEGER DEFAULT NULL`,
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_money_below INTEGER DEFAULT NULL`,
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_coins INTEGER DEFAULT NULL`,
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_coins_below INTEGER DEFAULT NULL`,
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_player_facing VARCHAR(10) DEFAULT NULL`,
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_flags JSONB DEFAULT NULL`,
		`ALTER TABLE phaser_cutscene_scripts ADD COLUMN requires_flags_absent JSONB DEFAULT NULL`,
	} {
		if _, err := db.Exec(statement); err != nil && !isDuplicateColumnError(err) {
			return fmt.Errorf("ensure cutscene requirement column: %w", err)
		}
	}
	return nil
}

func ensureConditionalDialogueColumns(db pokebattle.DBTX) error {
	for _, statement := range []string{
		`ALTER TABLE phaser_conditional_dialogue ADD COLUMN requires_flags JSONB DEFAULT NULL`,
		`ALTER TABLE phaser_conditional_dialogue ADD COLUMN requires_flags_absent JSONB DEFAULT NULL`,
		`ALTER TABLE phaser_conditional_dialogue ADD COLUMN dialogue_labels JSONB DEFAULT NULL`,
		`ALTER TABLE phaser_conditional_dialogue ADD COLUMN source VARCHAR(100) DEFAULT 'manual'`,
	} {
		if _, err := db.Exec(statement); err != nil && !isDuplicateColumnError(err) {
			return fmt.Errorf("ensure conditional dialogue column: %w", err)
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate column") ||
		strings.Contains(message, "already exists")
}

type loadedEventRef struct {
	path   string
	index  int
	source string
}

func LoadEvents(root string) ([]EventFile, error) {
	scriptDirs := []struct {
		path          string
		source        string
		allowOverride bool
	}{
		{
			path:   filepath.Join(root, scriptsDirName),
			source: extractorSource,
		},
		{
			path:          filepath.Join(root, manualScriptsDirName),
			source:        captureQuestSource,
			allowOverride: true,
		},
	}
	events := []EventFile{}
	seen := make(map[string]loadedEventRef)
	loadedDir := false
	for _, scriptsDir := range scriptDirs {
		exists, err := loadEventDir(scriptsDir.path, scriptsDir.source, scriptsDir.allowOverride, seen, &events)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		loadedDir = true
	}
	if !loadedDir {
		dirNames := make([]string, 0, len(scriptDirs))
		for _, scriptsDir := range scriptDirs {
			dirNames = append(dirNames, scriptsDir.path)
		}
		return nil, fmt.Errorf("read scripted event dirs %s: no directory found", strings.Join(dirNames, ", "))
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].ScriptLabel < events[j].ScriptLabel
	})
	return events, nil
}

func loadEventDir(scriptsDir, source string, allowOverride bool, seen map[string]loadedEventRef, events *[]EventFile) (bool, error) {
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read scripted event dir %s: %w", scriptsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(scriptsDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return false, fmt.Errorf("read %s: %w", path, err)
		}
		var event EventFile
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&event); err != nil {
			return false, fmt.Errorf("decode %s: %w", path, err)
		}
		if err := validateEvent(event, path); err != nil {
			return false, err
		}
		if prev, ok := seen[event.ScriptLabel]; ok {
			if !allowOverride || prev.source == source {
				return false, fmt.Errorf("duplicate scriptLabel %s in %s and %s", event.ScriptLabel, prev.path, path)
			}
			(*events)[prev.index] = event
			seen[event.ScriptLabel] = loadedEventRef{path: path, index: prev.index, source: source}
			continue
		}
		seen[event.ScriptLabel] = loadedEventRef{path: path, index: len(*events), source: source}
		*events = append(*events, event)
	}

	return true, nil
}

func LoadObjectVisibility(root string) ([]ObjectVisibilityRule, error) {
	paths := []string{
		filepath.Join(root, visibilityFileName),
		filepath.Join(root, generatedVisibilityFileName),
	}
	rules := []ObjectVisibilityRule{}
	seen := make(map[string]ObjectVisibilityRule)
	seenPath := make(map[string]string)
	loaded := false
	for _, path := range paths {
		fileRules, exists, err := loadObjectVisibilityFile(path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		loaded = true
		for i, rule := range fileRules {
			key := objectVisibilityConditionKey(rule)
			if previous, ok := seen[key]; ok {
				if previous.Visible != rule.Visible {
					return nil, fmt.Errorf("%s rule[%d] conflicts with object visibility key %s already loaded from %s", path, i, key, seenPath[key])
				}
				continue
			}
			seen[key] = rule
			seenPath[key] = path
			rules = append(rules, rule)
		}
	}
	if !loaded {
		return nil, nil
	}
	sortVisibilityRules(rules)
	return rules, nil
}

func loadObjectVisibilityFile(path string) ([]ObjectVisibilityRule, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	var rules []ObjectVisibilityRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, false, fmt.Errorf("decode %s: %w", path, err)
	}
	for i, rule := range rules {
		if rule.MapID == 0 && rule.MapName == "" {
			return nil, false, fmt.Errorf("%s[%d] missing mapId/mapName", path, i)
		}
		if rule.ObjectName == "" {
			return nil, false, fmt.Errorf("%s[%d] missing objectName", path, i)
		}
	}
	return rules, true, nil
}

func objectVisibilityConditionKey(rule ObjectVisibilityRule) string {
	return fmt.Sprintf("%d|%s|%s|%s", rule.MapID, rule.ObjectName, rule.RequiresFlag, rule.RequiresFlagAbsent)
}

func LoadEventTileOverrides(root string) ([]EventTileOverrideRule, error) {
	paths := []string{
		filepath.Join(root, eventTilesFileName),
		filepath.Join(root, generatedEventTilesFileName),
	}
	rules := []EventTileOverrideRule{}
	seen := make(map[string]string)
	loaded := false
	for _, path := range paths {
		fileRules, exists, err := loadEventTileOverridesFile(path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		loaded = true
		for i, rule := range fileRules {
			key := eventTileRuleKey(rule)
			if previous, ok := seen[key]; ok {
				return nil, fmt.Errorf("%s rule[%d] duplicates event tile key %s already loaded from %s", path, i, key, previous)
			}
			seen[key] = path
			rules = append(rules, rule)
		}
	}
	if !loaded {
		return nil, nil
	}
	return rules, nil
}

func LoadConditionalDialogue(root string) ([]ConditionalDialogueRule, error) {
	path := filepath.Join(root, conditionalDialogueFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var file ConditionalDialogueFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	for i, rule := range file.Rows {
		if strings.TrimSpace(rule.TextConstant) == "" {
			return nil, fmt.Errorf("%s.rows[%d] missing textConstant", path, i)
		}
		if len(rule.DialogueLabels) == 0 {
			return nil, fmt.Errorf("%s.rows[%d] missing dialogueLabels", path, i)
		}
	}
	sortConditionalDialogueRules(file.Rows)
	return file.Rows, nil
}

// LoadEventTileOverridesFile expands one repo-owned event tile override file.
func LoadEventTileOverridesFile(path string) ([]EventTileOverrideRule, error) {
	rules, _, err := loadEventTileOverridesFile(path)
	return rules, err
}

func loadEventTileOverridesFile(path string) ([]EventTileOverrideRule, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}

	var file EventTileOverrideFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, true, fmt.Errorf("decode %s: %w", path, err)
	}

	rules := append([]EventTileOverrideRule(nil), file.Tiles...)
	for i, block := range file.Blocks {
		if block.MapID == 0 && block.MapName == "" {
			return nil, true, fmt.Errorf("%s.blocks[%d] missing mapId/mapName", path, i)
		}
		if block.Palette == "" {
			return nil, true, fmt.Errorf("%s.blocks[%d] missing palette", path, i)
		}
		if block.LabelPrefix == "" {
			return nil, true, fmt.Errorf("%s.blocks[%d] missing labelPrefix", path, i)
		}
		palette, ok := file.Palettes[block.Palette]
		if !ok {
			return nil, true, fmt.Errorf("%s.blocks[%d] references unknown palette %q", path, i, block.Palette)
		}
		for _, cell := range palette {
			rules = append(rules, EventTileOverrideRule{
				MapID:              block.MapID,
				MapName:            block.MapName,
				X:                  block.BlockX*2 + cell.DX,
				Y:                  block.BlockY*2 + cell.DY,
				TileImageID:        cell.TileImageID,
				CollisionType:      cell.CollisionType,
				RequiresFlag:       block.RequiresFlag,
				RequiresFlagAbsent: block.RequiresFlagAbsent,
				Label:              fmt.Sprintf("%s_%d_%d", block.LabelPrefix, cell.DX, cell.DY),
			})
		}
	}

	seen := make(map[string]int)
	for i, rule := range rules {
		if rule.MapID == 0 && rule.MapName == "" {
			return nil, true, fmt.Errorf("%s rule[%d] missing mapId/mapName", path, i)
		}
		if rule.TileImageID == 0 {
			return nil, true, fmt.Errorf("%s rule[%d] missing tileImageId", path, i)
		}
		key := eventTileRuleKey(rule)
		if previous, ok := seen[key]; ok {
			return nil, true, fmt.Errorf("%s duplicate event tile key %s in rules %d and %d", path, key, previous, i)
		}
		seen[key] = i
	}

	return rules, true, nil
}

func validateEvent(event EventFile, path string) error {
	if event.ScriptLabel == "" {
		return fmt.Errorf("%s missing scriptLabel", path)
	}
	if event.MapName == "" {
		return fmt.Errorf("%s missing mapName", path)
	}
	if event.Trigger.Type == "" {
		return fmt.Errorf("%s missing trigger.type", path)
	}
	switch event.Trigger.Type {
	case "coord", "map_script", "npc_click":
	default:
		return fmt.Errorf("%s has unsupported trigger.type %q", path, event.Trigger.Type)
	}
	if event.Trigger.Source == captureQuestSource && len(event.Trigger.Coordinates) == 0 {
		return fmt.Errorf("%s capturequest trigger missing coordinates", path)
	}
	if event.RequiresPlayerFacing != "" && normalizeEventDirection(event.RequiresPlayerFacing) == "" {
		return fmt.Errorf("%s has invalid requiresPlayerFacing %q", path, event.RequiresPlayerFacing)
	}
	if _, err := canonicalRawArray(event.Actions); err != nil {
		return fmt.Errorf("%s has invalid actions: %w", path, err)
	}
	return nil
}

func normalizeEventDirection(direction string) string {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "UP", "DOWN", "LEFT", "RIGHT":
		return strings.ToUpper(strings.TrimSpace(direction))
	default:
		return ""
	}
}

func syncEvent(db pokebattle.DBTX, event EventFile) (bool, error) {
	setsFlagsJSON, err := canonicalValue(event.SetsFlags)
	if err != nil {
		return false, fmt.Errorf("%s setsFlags: %w", event.ScriptLabel, err)
	}
	requiresFlagsJSON, err := canonicalValue(normalizedFlagList(event.RequiresFlags))
	if err != nil {
		return false, fmt.Errorf("%s requiresFlags: %w", event.ScriptLabel, err)
	}
	requiresFlagsAbsentJSON, err := canonicalValue(normalizedFlagList(event.RequiresFlagsAbsent))
	if err != nil {
		return false, fmt.Errorf("%s requiresFlagsAbsent: %w", event.ScriptLabel, err)
	}
	actionsJSON, err := canonicalRawArray(event.Actions)
	if err != nil {
		return false, fmt.Errorf("%s actions: %w", event.ScriptLabel, err)
	}
	requiresItemID, requiresItemAbsentID, err := resolveEventItemRequirements(db, event)
	if err != nil {
		return false, fmt.Errorf("%s item requirements: %w", event.ScriptLabel, err)
	}

	target := eventDBRow{
		ScriptLabel:           event.ScriptLabel,
		MapName:               event.MapName,
		TriggerType:           event.Trigger.Type,
		TriggerLabel:          event.Trigger.Label,
		RequiresFlag:          event.RequiresFlag,
		RequiresFlagAbsent:    event.RequiresFlagAbsent,
		RequiresFlagsJSON:     requiresFlagsJSON,
		RequiresFlagsAbstJSON: requiresFlagsAbsentJSON,
		RequiresItemID:        requiresItemID,
		RequiresItemAbsentID:  requiresItemAbsentID,
		RequiresPokedexCaught: event.RequiresPokedexCaught,
		RequiresMoney:         event.RequiresMoney,
		RequiresMoneyBelow:    event.RequiresMoneyBelow,
		RequiresCoins:         event.RequiresCoins,
		RequiresCoinsBelow:    event.RequiresCoinsBelow,
		RequiresPlayerFacing:  normalizeEventDirection(event.RequiresPlayerFacing),
		SetsFlagsJSON:         setsFlagsJSON,
		ActionsJSON:           actionsJSON,
	}
	if event.Warp != nil {
		target.WarpToMapID = &event.Warp.MapID
		target.WarpToX = &event.Warp.X
		target.WarpToY = &event.Warp.Y
	}

	existing, err := loadEventRow(db, event.ScriptLabel)
	if err != nil {
		return false, err
	}
	if existing != nil && existing.equal(target) {
		return false, nil
	}

	if err := upsertEventRow(db, target); err != nil {
		return false, err
	}
	return true, nil
}

type extractorTriggerKey struct {
	MapName      string
	TriggerType  string
	TriggerLabel string
}

func deleteStaleExtractorTriggerScripts(db pokebattle.DBTX, events []EventFile) (int, error) {
	labelsByTrigger := make(map[extractorTriggerKey][]string)
	hasExtractorTrigger := make(map[extractorTriggerKey]bool)
	for _, event := range events {
		if event.Trigger.Label == "" {
			continue
		}
		key := extractorTriggerKey{
			MapName:      event.MapName,
			TriggerType:  event.Trigger.Type,
			TriggerLabel: event.Trigger.Label,
		}
		labelsByTrigger[key] = append(labelsByTrigger[key], event.ScriptLabel)
		if event.Trigger.Source == extractorSource {
			hasExtractorTrigger[key] = true
		}
	}

	total := 0
	for key, labels := range labelsByTrigger {
		if !hasExtractorTrigger[key] {
			continue
		}
		query, args := staleExtractorDeleteQuery(key, labels)
		result, err := db.Exec(query, args...)
		if err != nil {
			return total, fmt.Errorf("delete stale scripts for %s/%s/%s: %w", key.MapName, key.TriggerType, key.TriggerLabel, err)
		}
		affected, _ := result.RowsAffected()
		total += int(affected)
	}
	return total, nil
}

func staleExtractorDeleteQuery(key extractorTriggerKey, labels []string) (string, []any) {
	args := []any{key.MapName, key.TriggerType, key.TriggerLabel}
	placeholders := make([]string, 0, len(labels))
	for _, label := range labels {
		args = append(args, label)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	query := fmt.Sprintf(`
		DELETE FROM phaser_cutscene_scripts
		WHERE map_name = $1
			AND trigger_type = $2
			AND trigger_label = $3
			AND script_label NOT IN (%s)`, strings.Join(placeholders, ", "))
	return query, args
}

func resolveEventItemRequirements(db pokebattle.DBTX, event EventFile) (*int, *int, error) {
	requiresItemID := event.RequiresItemID
	if event.RequiresItemName != "" {
		id, err := resolveEventItemID(db, event.RequiresItemName)
		if err != nil {
			return nil, nil, err
		}
		requiresItemID = &id
	}

	requiresItemAbsentID := event.RequiresItemAbsentID
	if event.RequiresItemAbsentName != "" {
		id, err := resolveEventItemID(db, event.RequiresItemAbsentName)
		if err != nil {
			return nil, nil, err
		}
		requiresItemAbsentID = &id
	}
	return requiresItemID, requiresItemAbsentID, nil
}

func resolveEventItemID(db pokebattle.DBTX, itemName string) (int, error) {
	var id int
	err := db.QueryRow(
		`SELECT id FROM cq_items WHERE name = $1 OR short_name = $2 LIMIT 1`,
		itemName, itemName,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("lookup item %s: %w", itemName, err)
	}
	return id, nil
}

type eventDBRow struct {
	ScriptLabel           string
	MapName               string
	TriggerType           string
	TriggerLabel          string
	RequiresFlag          string
	RequiresFlagAbsent    string
	RequiresFlagsJSON     []byte
	RequiresFlagsAbstJSON []byte
	RequiresItemID        *int
	RequiresItemAbsentID  *int
	RequiresPokedexCaught *int
	RequiresMoney         *int
	RequiresMoneyBelow    *int
	RequiresCoins         *int
	RequiresCoinsBelow    *int
	RequiresPlayerFacing  string
	SetsFlagsJSON         []byte
	ActionsJSON           []byte
	WarpToMapID           *int
	WarpToX               *int
	WarpToY               *int
}

func loadEventRow(db pokebattle.DBTX, label string) (*eventDBRow, error) {
	var row eventDBRow
	var triggerLabel, reqFlag, reqFlagAbsent, reqPlayerFacing sql.NullString
	var reqItemID, reqItemAbsentID, reqCaught, reqMoney, reqMoneyBelow, reqCoins, reqCoinsBelow, warpMap, warpX, warpY sql.NullInt64
	var requiresFlagsRaw, requiresFlagsAbsentRaw, setsFlagsRaw, actionsRaw []byte
	err := db.QueryRow(`
			SELECT script_label, map_name, trigger_type, trigger_label,
				requires_flag, requires_flag_absent, requires_flags, requires_flags_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
				requires_money, requires_money_below, requires_coins, requires_coins_below, requires_player_facing,
				sets_flags, actions, warp_to_map_id, warp_to_x, warp_to_y
			FROM phaser_cutscene_scripts
			WHERE script_label = $1`, label).Scan(
		&row.ScriptLabel, &row.MapName, &row.TriggerType, &triggerLabel,
		&reqFlag, &reqFlagAbsent, &requiresFlagsRaw, &requiresFlagsAbsentRaw, &reqItemID, &reqItemAbsentID, &reqCaught, &reqMoney, &reqMoneyBelow, &reqCoins, &reqCoinsBelow,
		&reqPlayerFacing, &setsFlagsRaw, &actionsRaw, &warpMap, &warpX, &warpY)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load script %s: %w", label, err)
	}
	row.TriggerLabel = nullStringValue(triggerLabel)
	row.RequiresFlag = nullStringValue(reqFlag)
	row.RequiresFlagAbsent = nullStringValue(reqFlagAbsent)
	var errJSON error
	row.RequiresFlagsJSON, errJSON = canonicalRawJSON(requiresFlagsRaw)
	if errJSON != nil {
		return nil, fmt.Errorf("canonical requires_flags for %s: %w", label, errJSON)
	}
	row.RequiresFlagsAbstJSON, errJSON = canonicalRawJSON(requiresFlagsAbsentRaw)
	if errJSON != nil {
		return nil, fmt.Errorf("canonical requires_flags_absent for %s: %w", label, errJSON)
	}
	row.RequiresItemID = nullIntValue(reqItemID)
	row.RequiresItemAbsentID = nullIntValue(reqItemAbsentID)
	row.RequiresPokedexCaught = nullIntValue(reqCaught)
	row.RequiresMoney = nullIntValue(reqMoney)
	row.RequiresMoneyBelow = nullIntValue(reqMoneyBelow)
	row.RequiresCoins = nullIntValue(reqCoins)
	row.RequiresCoinsBelow = nullIntValue(reqCoinsBelow)
	row.RequiresPlayerFacing = nullStringValue(reqPlayerFacing)
	row.WarpToMapID = nullIntValue(warpMap)
	row.WarpToX = nullIntValue(warpX)
	row.WarpToY = nullIntValue(warpY)
	row.SetsFlagsJSON, errJSON = canonicalRawJSON(setsFlagsRaw)
	if errJSON != nil {
		return nil, fmt.Errorf("canonical sets_flags for %s: %w", label, errJSON)
	}
	row.ActionsJSON, errJSON = canonicalRawJSON(actionsRaw)
	if errJSON != nil {
		return nil, fmt.Errorf("canonical actions for %s: %w", label, errJSON)
	}
	return &row, nil
}

func (row eventDBRow) equal(other eventDBRow) bool {
	return row.ScriptLabel == other.ScriptLabel &&
		row.MapName == other.MapName &&
		row.TriggerType == other.TriggerType &&
		row.TriggerLabel == other.TriggerLabel &&
		row.RequiresFlag == other.RequiresFlag &&
		row.RequiresFlagAbsent == other.RequiresFlagAbsent &&
		bytes.Equal(row.RequiresFlagsJSON, other.RequiresFlagsJSON) &&
		bytes.Equal(row.RequiresFlagsAbstJSON, other.RequiresFlagsAbstJSON) &&
		intPtrEqual(row.RequiresItemID, other.RequiresItemID) &&
		intPtrEqual(row.RequiresItemAbsentID, other.RequiresItemAbsentID) &&
		intPtrEqual(row.RequiresPokedexCaught, other.RequiresPokedexCaught) &&
		intPtrEqual(row.RequiresMoney, other.RequiresMoney) &&
		intPtrEqual(row.RequiresMoneyBelow, other.RequiresMoneyBelow) &&
		intPtrEqual(row.RequiresCoins, other.RequiresCoins) &&
		intPtrEqual(row.RequiresCoinsBelow, other.RequiresCoinsBelow) &&
		row.RequiresPlayerFacing == other.RequiresPlayerFacing &&
		bytes.Equal(row.SetsFlagsJSON, other.SetsFlagsJSON) &&
		bytes.Equal(row.ActionsJSON, other.ActionsJSON) &&
		intPtrEqual(row.WarpToMapID, other.WarpToMapID) &&
		intPtrEqual(row.WarpToX, other.WarpToX) &&
		intPtrEqual(row.WarpToY, other.WarpToY)
}

func upsertEventRow(db pokebattle.DBTX, row eventDBRow) error {
	query := `
		INSERT INTO phaser_cutscene_scripts (
			script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_flags, requires_flags_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
			requires_money, requires_money_below, requires_coins, requires_coins_below, requires_player_facing,
			sets_flags, actions, warp_to_map_id, warp_to_x, warp_to_y
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16, $17::jsonb, $18::jsonb, $19, $20, $21)
		ON CONFLICT (script_label) DO UPDATE SET
			map_name = EXCLUDED.map_name,
			trigger_type = EXCLUDED.trigger_type,
			trigger_label = EXCLUDED.trigger_label,
			requires_flag = EXCLUDED.requires_flag,
			requires_flag_absent = EXCLUDED.requires_flag_absent,
			requires_flags = EXCLUDED.requires_flags,
			requires_flags_absent = EXCLUDED.requires_flags_absent,
			requires_item_id = EXCLUDED.requires_item_id,
			requires_item_absent_id = EXCLUDED.requires_item_absent_id,
			requires_pokedex_caught = EXCLUDED.requires_pokedex_caught,
			requires_money = EXCLUDED.requires_money,
			requires_money_below = EXCLUDED.requires_money_below,
			requires_coins = EXCLUDED.requires_coins,
			requires_coins_below = EXCLUDED.requires_coins_below,
			requires_player_facing = EXCLUDED.requires_player_facing,
			sets_flags = EXCLUDED.sets_flags,
			actions = EXCLUDED.actions,
			warp_to_map_id = EXCLUDED.warp_to_map_id,
			warp_to_x = EXCLUDED.warp_to_x,
			warp_to_y = EXCLUDED.warp_to_y`
	_, err := db.Exec(query,
		row.ScriptLabel, row.MapName, row.TriggerType, nullableString(row.TriggerLabel),
		nullableString(row.RequiresFlag), nullableString(row.RequiresFlagAbsent),
		string(row.RequiresFlagsJSON), string(row.RequiresFlagsAbstJSON),
		row.RequiresItemID, row.RequiresItemAbsentID, row.RequiresPokedexCaught, row.RequiresMoney, row.RequiresMoneyBelow,
		row.RequiresCoins, row.RequiresCoinsBelow, nullableString(row.RequiresPlayerFacing),
		string(row.SetsFlagsJSON), string(row.ActionsJSON),
		row.WarpToMapID, row.WarpToX, row.WarpToY)
	if err != nil {
		return fmt.Errorf("upsert script %s: %w", row.ScriptLabel, err)
	}
	return nil
}

func ownedCoordinates(events []EventFile) map[string][]EventCoordinate {
	result := make(map[string][]EventCoordinate)
	for _, event := range events {
		if event.Trigger.Type != "coord" || event.Trigger.Label == "" || len(event.Trigger.Coordinates) == 0 {
			continue
		}
		rows := append([]EventCoordinate(nil), event.Trigger.Coordinates...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].MapID != rows[j].MapID {
				return rows[i].MapID < rows[j].MapID
			}
			if rows[i].X != rows[j].X {
				return rows[i].X < rows[j].X
			}
			return rows[i].Y < rows[j].Y
		})
		result[event.Trigger.Label] = append(result[event.Trigger.Label], rows...)
	}
	return result
}

func syncOwnedCoordinates(db pokebattle.DBTX, coordsByLabel map[string][]EventCoordinate) (bool, error) {
	target := flattenCoordinates(coordsByLabel)
	existing, err := loadCoordinateRows(db, sortedKeys(coordsByLabel))
	if err != nil {
		return false, err
	}
	if equalCoordinates(existing, target) {
		return false, nil
	}

	labels := sortedKeys(coordsByLabel)
	if err := deleteCoordinateLabels(db, labels); err != nil {
		return false, err
	}
	for _, row := range target {
		if _, err := db.Exec(`
				INSERT INTO phaser_coordinate_triggers (map_name, map_id, label, x, y)
				VALUES ($1, $2, $3, $4, $5)`,
			row.MapName, row.MapID, row.Label, row.X, row.Y); err != nil {
			return false, fmt.Errorf("insert coordinate trigger %s (%d,%d): %w", row.Label, row.X, row.Y, err)
		}
	}
	return true, nil
}

type coordinateRow struct {
	MapName string
	MapID   int
	Label   string
	X       int
	Y       int
}

func flattenCoordinates(coordsByLabel map[string][]EventCoordinate) []coordinateRow {
	rows := []coordinateRow{}
	for label, coords := range coordsByLabel {
		for _, coord := range coords {
			rows = append(rows, coordinateRow{
				MapName: coord.MapName,
				MapID:   coord.MapID,
				Label:   label,
				X:       coord.X,
				Y:       coord.Y,
			})
		}
	}
	sortCoordinateRows(rows)
	return rows
}

func loadCoordinateRows(db pokebattle.DBTX, labels []string) ([]coordinateRow, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	query := fmt.Sprintf(`
		SELECT map_name, COALESCE(map_id, 0), label, x, y
		FROM phaser_coordinate_triggers
		WHERE label IN (%s)`, placeholders(len(labels)))
	args := make([]any, len(labels))
	for i, label := range labels {
		args[i] = label
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []coordinateRow{}
	for rows.Next() {
		var row coordinateRow
		if err := rows.Scan(&row.MapName, &row.MapID, &row.Label, &row.X, &row.Y); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortCoordinateRows(result)
	return result, nil
}

func deleteCoordinateLabels(db pokebattle.DBTX, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	query := fmt.Sprintf(`DELETE FROM phaser_coordinate_triggers WHERE label IN (%s)`, placeholders(len(labels)))
	args := make([]any, len(labels))
	for i, label := range labels {
		args[i] = label
	}
	if _, err := db.Exec(query, args...); err != nil {
		return err
	}
	return nil
}

func equalCoordinates(a, b []coordinateRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func syncObjectVisibility(db pokebattle.DBTX, rules []ObjectVisibilityRule) (bool, error) {
	existing, err := loadCustomVisibilityRows(db)
	if err != nil {
		return false, err
	}
	if equalVisibilityRules(existing, rules) {
		return false, nil
	}

	if _, err := db.Exec(`
			DELETE FROM phaser_event_object_visibility
			WHERE label NOT LIKE $1 OR label IS NULL`, sourceMissableLabel); err != nil {
		return false, fmt.Errorf("delete custom object visibility: %w", err)
	}
	for _, rule := range rules {
		if _, err := db.Exec(`
				INSERT INTO phaser_event_object_visibility (
					map_id, map_name, object_name, visible, requires_flag, requires_flag_absent, label
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			rule.MapID, rule.MapName, rule.ObjectName, rule.Visible,
			nullableString(rule.RequiresFlag), nullableString(rule.RequiresFlagAbsent), nullableString(rule.Label)); err != nil {
			return false, fmt.Errorf("insert object visibility %s/%s: %w", rule.MapName, rule.ObjectName, err)
		}
	}
	return true, nil
}

func loadCustomVisibilityRows(db pokebattle.DBTX) ([]ObjectVisibilityRule, error) {
	rows, err := db.Query(`
			SELECT map_id, map_name, object_name, visible, requires_flag, requires_flag_absent, COALESCE(label, '')
			FROM phaser_event_object_visibility
			WHERE label NOT LIKE $1 OR label IS NULL`, sourceMissableLabel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []ObjectVisibilityRule{}
	for rows.Next() {
		var rule ObjectVisibilityRule
		var requiresFlag, requiresFlagAbsent sql.NullString
		if err := rows.Scan(
			&rule.MapID,
			&rule.MapName,
			&rule.ObjectName,
			&rule.Visible,
			&requiresFlag,
			&requiresFlagAbsent,
			&rule.Label,
		); err != nil {
			return nil, err
		}
		rule.RequiresFlag = nullStringValue(requiresFlag)
		rule.RequiresFlagAbsent = nullStringValue(requiresFlagAbsent)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortVisibilityRules(rules)
	return rules, nil
}

func equalVisibilityRules(a, b []ObjectVisibilityRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func syncEventTileOverrides(db pokebattle.DBTX, rules []EventTileOverrideRule) (bool, error) {
	existing, err := loadEventTileOverrideRows(db)
	if err != nil {
		return false, err
	}
	if equalEventTileRules(existing, rules) {
		return false, nil
	}

	if _, err := db.Exec(`DELETE FROM phaser_event_tile_overrides`); err != nil {
		return false, fmt.Errorf("delete event tile overrides: %w", err)
	}
	for _, rule := range rules {
		if _, err := db.Exec(`
				INSERT INTO phaser_event_tile_overrides (
					map_id, map_name, x, y, tile_image_id, collision_type, requires_flag, requires_flag_absent, label
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			rule.MapID, rule.MapName, rule.X, rule.Y, rule.TileImageID, rule.CollisionType,
			nullableString(rule.RequiresFlag), nullableString(rule.RequiresFlagAbsent), nullableString(rule.Label)); err != nil {
			return false, fmt.Errorf("insert event tile %s (%d,%d): %w", rule.MapName, rule.X, rule.Y, err)
		}
	}
	return true, nil
}

func loadEventTileOverrideRows(db pokebattle.DBTX) ([]EventTileOverrideRule, error) {
	rows, err := db.Query(`
			SELECT map_id, map_name, x, y, tile_image_id, collision_type, requires_flag, requires_flag_absent, COALESCE(label, '')
			FROM phaser_event_tile_overrides
			ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []EventTileOverrideRule{}
	for rows.Next() {
		var rule EventTileOverrideRule
		var requiresFlag, requiresFlagAbsent sql.NullString
		if err := rows.Scan(
			&rule.MapID,
			&rule.MapName,
			&rule.X,
			&rule.Y,
			&rule.TileImageID,
			&rule.CollisionType,
			&requiresFlag,
			&requiresFlagAbsent,
			&rule.Label,
		); err != nil {
			return nil, err
		}
		rule.RequiresFlag = nullStringValue(requiresFlag)
		rule.RequiresFlagAbsent = nullStringValue(requiresFlagAbsent)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func equalEventTileRules(a, b []EventTileOverrideRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type conditionalDialogueDBRow struct {
	TextConstant       string
	Priority           int
	RequiresFlag       string
	RequiresFlagAbsent string
	RequiresFlagsJSON  []byte
	RequiresAbsentJSON []byte
	OverrideDialogue   string
	DialogueLabelsJSON []byte
}

func syncConditionalDialogue(db pokebattle.DBTX, rules []ConditionalDialogueRule) (bool, error) {
	targetRows, err := buildConditionalDialogueRows(db, rules)
	if err != nil {
		return false, err
	}
	existingRows, err := loadGeneratedConditionalDialogueRows(db)
	if err != nil {
		return false, err
	}
	if equalConditionalDialogueRows(existingRows, targetRows) {
		return false, nil
	}

	if _, err := db.Exec(`DELETE FROM phaser_conditional_dialogue WHERE source = $1`, extractorSource); err != nil {
		return false, fmt.Errorf("delete generated conditional dialogue: %w", err)
	}
	for _, row := range targetRows {
		if _, err := db.Exec(`
				INSERT INTO phaser_conditional_dialogue (
					text_constant, priority, requires_flag, requires_flag_absent,
					requires_flags, requires_flags_absent, override_dialogue,
					override_speaker, dialogue_labels, source
				)
				VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, NULL, $8::jsonb, $9)`,
			row.TextConstant,
			row.Priority,
			nullableString(row.RequiresFlag),
			nullableString(row.RequiresFlagAbsent),
			string(row.RequiresFlagsJSON),
			string(row.RequiresAbsentJSON),
			row.OverrideDialogue,
			string(row.DialogueLabelsJSON),
			extractorSource,
		); err != nil {
			return false, fmt.Errorf("insert generated conditional dialogue %s: %w", row.TextConstant, err)
		}
	}
	return true, nil
}

func buildConditionalDialogueRows(db pokebattle.DBTX, rules []ConditionalDialogueRule) ([]conditionalDialogueDBRow, error) {
	rows := make([]conditionalDialogueDBRow, 0, len(rules))
	for _, rule := range rules {
		requiresFlags := normalizedFlagList(appendFlag(rule.RequiresFlags, rule.RequiresFlag))
		requiresFlagsAbsent := normalizedFlagList(appendFlag(rule.RequiresFlagsAbsent, rule.RequiresFlagAbsent))
		requiresFlagsJSON, err := canonicalValue(requiresFlags)
		if err != nil {
			return nil, fmt.Errorf("%s requiresFlags: %w", rule.TextConstant, err)
		}
		requiresAbsentJSON, err := canonicalValue(requiresFlagsAbsent)
		if err != nil {
			return nil, fmt.Errorf("%s requiresFlagsAbsent: %w", rule.TextConstant, err)
		}
		labels := normalizedDialogueLabels(rule.DialogueLabels)
		labelsJSON, err := canonicalValue(labels)
		if err != nil {
			return nil, fmt.Errorf("%s dialogueLabels: %w", rule.TextConstant, err)
		}
		dialogue, err := resolveConditionalDialogueText(db, labels)
		if err != nil {
			return nil, fmt.Errorf("%s dialogue text: %w", rule.TextConstant, err)
		}
		rows = append(rows, conditionalDialogueDBRow{
			TextConstant:       strings.TrimSpace(rule.TextConstant),
			Priority:           rule.Priority,
			RequiresFlag:       "",
			RequiresFlagAbsent: "",
			RequiresFlagsJSON:  requiresFlagsJSON,
			RequiresAbsentJSON: requiresAbsentJSON,
			OverrideDialogue:   dialogue,
			DialogueLabelsJSON: labelsJSON,
		})
	}
	sortConditionalDialogueDBRows(rows)
	return rows, nil
}

func loadGeneratedConditionalDialogueRows(db pokebattle.DBTX) ([]conditionalDialogueDBRow, error) {
	rows, err := db.Query(`
			SELECT text_constant, priority, requires_flag, requires_flag_absent,
				requires_flags, requires_flags_absent, override_dialogue, dialogue_labels
			FROM phaser_conditional_dialogue
			WHERE source = $1
			ORDER BY text_constant, priority DESC, id`, extractorSource)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []conditionalDialogueDBRow{}
	for rows.Next() {
		var row conditionalDialogueDBRow
		var requiresFlag, requiresFlagAbsent sql.NullString
		var requiresFlagsRaw, requiresAbsentRaw, labelsRaw []byte
		if err := rows.Scan(
			&row.TextConstant,
			&row.Priority,
			&requiresFlag,
			&requiresFlagAbsent,
			&requiresFlagsRaw,
			&requiresAbsentRaw,
			&row.OverrideDialogue,
			&labelsRaw,
		); err != nil {
			return nil, err
		}
		row.RequiresFlag = nullStringValue(requiresFlag)
		row.RequiresFlagAbsent = nullStringValue(requiresFlagAbsent)
		var errJSON error
		row.RequiresFlagsJSON, errJSON = canonicalRawJSON(requiresFlagsRaw)
		if errJSON != nil {
			return nil, fmt.Errorf("canonical requires_flags for %s: %w", row.TextConstant, errJSON)
		}
		row.RequiresAbsentJSON, errJSON = canonicalRawJSON(requiresAbsentRaw)
		if errJSON != nil {
			return nil, fmt.Errorf("canonical requires_flags_absent for %s: %w", row.TextConstant, errJSON)
		}
		row.DialogueLabelsJSON, errJSON = canonicalRawJSON(labelsRaw)
		if errJSON != nil {
			return nil, fmt.Errorf("canonical dialogue_labels for %s: %w", row.TextConstant, errJSON)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortConditionalDialogueDBRows(result)
	return result, nil
}

func resolveConditionalDialogueText(db pokebattle.DBTX, labels []string) (string, error) {
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		dialogue, err := resolveDialogueLabel(db, label)
		if err != nil {
			return "", err
		}
		parts = append(parts, dialogue)
	}
	return strings.Join(parts, "\n"), nil
}

func resolveDialogueLabel(db pokebattle.DBTX, label string) (string, error) {
	candidates := []string{label}
	if strings.HasPrefix(label, "_") {
		candidates = append(candidates, strings.TrimPrefix(label, "_"))
	} else {
		candidates = append(candidates, "_"+label)
	}
	for _, candidate := range candidates {
		var dialogue string
		err := db.QueryRow(`SELECT dialogue FROM phaser_dialogue_text WHERE label = $1`, candidate).Scan(&dialogue)
		if err == nil {
			return dialogue, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	return "", fmt.Errorf("missing dialogue label %s", label)
}

func appendFlag(flags []string, flag string) []string {
	if strings.TrimSpace(flag) == "" {
		return flags
	}
	result := append([]string(nil), flags...)
	return append(result, flag)
}

func normalizedDialogueLabels(labels []string) []string {
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			result = append(result, label)
		}
	}
	return result
}

func sortConditionalDialogueRules(rules []ConditionalDialogueRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].TextConstant != rules[j].TextConstant {
			return rules[i].TextConstant < rules[j].TextConstant
		}
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		return strings.Join(rules[i].DialogueLabels, "\x00") < strings.Join(rules[j].DialogueLabels, "\x00")
	})
}

func sortConditionalDialogueDBRows(rows []conditionalDialogueDBRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TextConstant != rows[j].TextConstant {
			return rows[i].TextConstant < rows[j].TextConstant
		}
		if rows[i].Priority != rows[j].Priority {
			return rows[i].Priority > rows[j].Priority
		}
		return string(rows[i].DialogueLabelsJSON) < string(rows[j].DialogueLabelsJSON)
	})
}

func equalConditionalDialogueRows(a, b []conditionalDialogueDBRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].TextConstant != b[i].TextConstant ||
			a[i].Priority != b[i].Priority ||
			a[i].RequiresFlag != b[i].RequiresFlag ||
			a[i].RequiresFlagAbsent != b[i].RequiresFlagAbsent ||
			a[i].OverrideDialogue != b[i].OverrideDialogue ||
			!bytes.Equal(a[i].RequiresFlagsJSON, b[i].RequiresFlagsJSON) ||
			!bytes.Equal(a[i].RequiresAbsentJSON, b[i].RequiresAbsentJSON) ||
			!bytes.Equal(a[i].DialogueLabelsJSON, b[i].DialogueLabelsJSON) {
			return false
		}
	}
	return true
}

func findRootDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		for _, candidate := range []string{
			filepath.Join(dir, defaultRootDir),
			filepath.Join(dir, "server", defaultRootDir),
		} {
			if isScriptedEventsRoot(candidate) {
				return candidate, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", os.ErrNotExist
}

func isScriptedEventsRoot(path string) bool {
	info, err := os.Stat(filepath.Join(path, scriptsDirName))
	return err == nil && info.IsDir()
}

func canonicalRawArray(rawItems []json.RawMessage) ([]byte, error) {
	items := make([]any, 0, len(rawItems))
	for _, raw := range rawItems {
		v, err := decodeJSON(raw)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return json.Marshal(items)
}

func canonicalRawJSON(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte("null"), nil
	}
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func canonicalValue(v any) ([]byte, error) {
	return json.Marshal(v)
}

func decodeJSON(raw []byte) (any, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullIntValue(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func placeholders(count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(parts, ",")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizedFlagList(flags []string) []string {
	if len(flags) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(flags))
	normalized := make([]string, 0, len(flags))
	for _, flag := range flags {
		flag = strings.TrimSpace(flag)
		if flag == "" || seen[flag] {
			continue
		}
		seen[flag] = true
		normalized = append(normalized, flag)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

func sortCoordinateRows(rows []coordinateRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Label != rows[j].Label {
			return rows[i].Label < rows[j].Label
		}
		if rows[i].MapID != rows[j].MapID {
			return rows[i].MapID < rows[j].MapID
		}
		if rows[i].MapName != rows[j].MapName {
			return rows[i].MapName < rows[j].MapName
		}
		if rows[i].X != rows[j].X {
			return rows[i].X < rows[j].X
		}
		return rows[i].Y < rows[j].Y
	})
}

func sortVisibilityRules(rules []ObjectVisibilityRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].MapID != rules[j].MapID {
			return rules[i].MapID < rules[j].MapID
		}
		if rules[i].MapName != rules[j].MapName {
			return rules[i].MapName < rules[j].MapName
		}
		if rules[i].ObjectName != rules[j].ObjectName {
			return rules[i].ObjectName < rules[j].ObjectName
		}
		if rules[i].Label != rules[j].Label {
			return rules[i].Label < rules[j].Label
		}
		if rules[i].RequiresFlag != rules[j].RequiresFlag {
			return rules[i].RequiresFlag < rules[j].RequiresFlag
		}
		return rules[i].RequiresFlagAbsent < rules[j].RequiresFlagAbsent
	})
}

func sortEventTileRules(rules []EventTileOverrideRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].MapID != rules[j].MapID {
			return rules[i].MapID < rules[j].MapID
		}
		if rules[i].MapName != rules[j].MapName {
			return rules[i].MapName < rules[j].MapName
		}
		if rules[i].X != rules[j].X {
			return rules[i].X < rules[j].X
		}
		if rules[i].Y != rules[j].Y {
			return rules[i].Y < rules[j].Y
		}
		if rules[i].RequiresFlag != rules[j].RequiresFlag {
			return rules[i].RequiresFlag < rules[j].RequiresFlag
		}
		if rules[i].RequiresFlagAbsent != rules[j].RequiresFlagAbsent {
			return rules[i].RequiresFlagAbsent < rules[j].RequiresFlagAbsent
		}
		return rules[i].Label < rules[j].Label
	})
}

func eventTileRuleKey(rule EventTileOverrideRule) string {
	return fmt.Sprintf("%d|%s|%d|%d|%s|%s", rule.MapID, rule.MapName, rule.X, rule.Y, rule.RequiresFlag, rule.RequiresFlagAbsent)
}
