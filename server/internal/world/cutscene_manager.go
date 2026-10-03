package world

import (
	"capturequest/internal/db"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"context"
)

// CutsceneScript represents a cutscene loaded from the database.
type CutsceneScript struct {
	ID                   int
	ScriptLabel          string
	MapName              string
	TriggerType          string // "coord", "map_script", "npc_click"
	TriggerLabel         *string
	RequiresFlag         *string
	RequiresFlagAbst     *string
	RequiresFlags        []string
	RequiresFlagsAbst    []string
	RequiresItemID       *int
	RequiresItemAbst     *int
	RequiresCaught       *int
	RequiresMoney        *int
	RequiresMoneyBelow   *int
	RequiresCoins        *int
	RequiresCoinsBelow   *int
	RequiresPlayerFacing *string
	SetsFlags            []string
	Actions              json.RawMessage // Raw JSON array of action objects
	WarpToMapID          *int            // Optional: warp player to this map after cutscene
	WarpToX              *int
	WarpToY              *int
}

func parseStringJSONList(raw []byte, scriptLabel, field string) ([]string, error) {
	if strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("cutscene %s field %s: %w", scriptLabel, field, err)
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("cutscene %s field %s entry %d is empty or null", scriptLabel, field, i)
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result, nil
}

func validateCutsceneActionTypes(actions []CutsceneAction, path string) error {
	for i, action := range actions {
		at := fmt.Sprintf("%s[%d]", path, i)
		if strings.TrimSpace(action.Type) == "" {
			return fmt.Errorf("%s has no action type", at)
		}
		for name, children := range map[string][]CutsceneAction{"actions": action.Actions, "postWinActions": action.PostWinActions, "postLoseActions": action.PostLoseActions} {
			if err := validateCutsceneActionTypes(children, at+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}

// CutsceneManager loads cutscene scripts from the database and provides
// lookups by map + trigger type. Cutscene eligibility is checked per-player
// using their event flags.
type CutsceneManager struct {
	db *sql.DB
	mu sync.RWMutex
	// byMap indexes cutscenes by map_name -> list of scripts
	byMap map[string][]*CutsceneScript
	// byLabel indexes cutscenes by script_label for direct lookup
	byLabel map[string]*CutsceneScript
	// byTriggerLabel indexes cutscenes by source trigger labels from the extracted data.
	byTriggerLabel map[string][]*CutsceneScript
	// mapIDToName maps phaser_maps.id -> phaser_maps.name for resolving map IDs
	mapIDToName map[int]string
}

// NewCutsceneManager creates a new CutsceneManager.
func NewCutsceneManager(db *sql.DB) *CutsceneManager {
	return &CutsceneManager{
		db:             db,
		byMap:          make(map[string][]*CutsceneScript),
		byLabel:        make(map[string]*CutsceneScript),
		byTriggerLabel: make(map[string][]*CutsceneScript),
		mapIDToName:    make(map[int]string),
	}
}

// Load reads all cutscene scripts from the database into memory.
func (m *CutsceneManager) Load(ctx context.Context) error {
	if m.db == nil {
		return fmt.Errorf("cutscene preload requires a database")
	}
	if err := requireCutsceneIssuanceSchema(ctx, m.db); err != nil {
		return err
	}
	mapRows, err := m.db.QueryContext(ctx, `SELECT id, name FROM phaser_maps`)
	if err != nil {
		return fmt.Errorf("load cutscene map names: %w", err)
	}
	defer mapRows.Close()
	idToName := make(map[int]string)
	for mapRows.Next() {
		var id int
		var name string
		if err := mapRows.Scan(&id, &name); err != nil {
			return fmt.Errorf("scan cutscene map: %w", err)
		}
		idToName[id] = name
	}
	if err := mapRows.Err(); err != nil {
		return fmt.Errorf("read cutscene maps: %w", err)
	}
	// Runtime requires the current canonical schema. Never retry a weaker
	// query that drops prerequisite fields after a database error.
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_flags, requires_flags_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
			requires_money, requires_money_below, requires_coins, requires_coins_below, requires_player_facing, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("load cutscenes: %w", err)
	}
	defer rows.Close()

	count := 0
	byMap := make(map[string][]*CutsceneScript)
	byLabel := make(map[string]*CutsceneScript)
	byTriggerLabel := make(map[string][]*CutsceneScript)
	for rows.Next() {
		var cs CutsceneScript
		var triggerLabel, reqFlag, reqFlagAbsent, reqPlayerFacing *string
		var reqItemID, reqItemAbsentID, reqCaught, reqMoney, reqMoneyBelow, reqCoins, reqCoinsBelow *int
		var reqFlagsJSON, reqFlagsAbsentJSON []byte
		var setsFlagsJSON, actionsJSON []byte
		var warpMapID, warpX, warpY *int

		if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
			&triggerLabel, &reqFlag, &reqFlagAbsent, &reqFlagsJSON, &reqFlagsAbsentJSON, &reqItemID, &reqItemAbsentID, &reqCaught, &reqMoney, &reqMoneyBelow, &reqCoins, &reqCoinsBelow, &reqPlayerFacing,
			&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
			return fmt.Errorf("scan cutscene row: %w", err)
		}

		cs.TriggerLabel = triggerLabel
		cs.RequiresFlag = reqFlag
		cs.RequiresFlagAbst = reqFlagAbsent
		cs.RequiresFlags, err = parseStringJSONList(reqFlagsJSON, cs.ScriptLabel, "requires_flags")
		if err != nil {
			return err
		}
		cs.RequiresFlagsAbst, err = parseStringJSONList(reqFlagsAbsentJSON, cs.ScriptLabel, "requires_flags_absent")
		if err != nil {
			return err
		}
		cs.RequiresItemID = reqItemID
		cs.RequiresItemAbst = reqItemAbsentID
		cs.RequiresCaught = reqCaught
		cs.RequiresMoney = reqMoney
		cs.RequiresMoneyBelow = reqMoneyBelow
		cs.RequiresCoins = reqCoins
		cs.RequiresCoinsBelow = reqCoinsBelow
		cs.RequiresPlayerFacing = reqPlayerFacing
		var actions []CutsceneAction
		if err := json.Unmarshal(actionsJSON, &actions); err != nil {
			return fmt.Errorf("cutscene %s actions: %w", cs.ScriptLabel, err)
		}
		if actions == nil {
			return fmt.Errorf("cutscene %s actions must be an array", cs.ScriptLabel)
		}
		if err := validateCutsceneActionTypes(actions, "cutscene "+cs.ScriptLabel+" actions"); err != nil {
			return err
		}
		cs.Actions = actionsJSON
		cs.WarpToMapID = warpMapID
		cs.WarpToX = warpX
		cs.WarpToY = warpY

		cs.SetsFlags, err = parseStringJSONList(setsFlagsJSON, cs.ScriptLabel, "sets_flags")
		if err != nil {
			return err
		}

		byMap[cs.MapName] = append(byMap[cs.MapName], &cs)
		byLabel[cs.ScriptLabel] = &cs
		if cs.TriggerLabel != nil && *cs.TriggerLabel != "" {
			byTriggerLabel[*cs.TriggerLabel] = append(byTriggerLabel[*cs.TriggerLabel], &cs)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read cutscene rows: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	m.mapIDToName = idToName
	m.byMap = byMap
	m.byLabel = byLabel
	m.byTriggerLabel = byTriggerLabel
	m.mu.Unlock()

	log.Printf("[CutsceneManager] Loaded %d cutscene scripts across %d maps", count, len(byMap))
	return nil
}

// GetByLabel returns a cutscene script by its label, or nil.
func (m *CutsceneManager) GetByLabel(label string) *CutsceneScript {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byLabel[label]
}

// HasClickCutsceneForTriggerLabel reports whether a text/object trigger is owned
// by a scripted click cutscene, regardless of whether the current player is
// eligible for that script. This lets ordinary dialogue avoid surfacing older
// branching-dialogue rows for events now handled by file-backed cutscenes.
func (m *CutsceneManager) HasClickCutsceneForTriggerLabel(label string) bool {
	if m == nil || label == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if direct := m.byLabel[label]; direct != nil && direct.TriggerType == "npc_click" {
		return true
	}
	for _, cs := range m.byTriggerLabel[label] {
		if cs != nil && cs.TriggerType == "npc_click" {
			return true
		}
	}
	return false
}

// FindEligibleCoordCutsceneForTrigger resolves an extracted coordinate trigger
// to a cutscene. Prefer explicit trigger_label mappings, then fall back to
// legacy direct script_label matches for older data.
func (m *CutsceneManager) FindEligibleCoordCutsceneForTrigger(trigger CoordinateTrigger, charID int64, efm *EventFlagManager, playerFacing ...string) *CutsceneScript {
	cs, err := m.findEligibleCoordCutsceneIn(m.db, trigger, charID, efm, playerFacing...)
	if err != nil {
		log.Printf("[Cutscene] Coordinate eligibility: %v", err)
		return nil
	}
	return cs
}

func (m *CutsceneManager) findEligibleCoordCutsceneIn(q db.DBTX, trigger CoordinateTrigger, charID int64, flags *EventFlagManager, playerFacing ...string) (*CutsceneScript, error) {
	m.mu.RLock()
	mapped := append([]*CutsceneScript(nil), m.byTriggerLabel[trigger.Label]...)
	direct := m.byLabel[trigger.Label]
	m.mu.RUnlock()
	sortCutscenesBySpecificity(mapped)
	for _, cs := range append(mapped, direct) {
		if cs == nil || (cs.TriggerType != "coord" && cs.TriggerType != "npc_click") {
			continue
		}
		if trigger.MapName != "" && cs.MapName != "" && !sameMapName(cs.MapName, trigger.MapName) {
			continue
		}
		eligible, err := checkCutsceneEligibleIn(q, cs, charID, flags, playerFacing...)
		if err != nil {
			return nil, err
		}
		if eligible {
			return cs, nil
		}
	}
	return nil, nil
}

func (m *CutsceneManager) checkEligibleCoordinateCutscene(cs *CutsceneScript, trigger CoordinateTrigger, charID int64, efm *EventFlagManager, playerFacing ...string) bool {
	if cs == nil {
		return false
	}
	if cs.TriggerType != "coord" && cs.TriggerType != "npc_click" {
		return false
	}
	if trigger.MapName != "" && cs.MapName != "" && !sameMapName(cs.MapName, trigger.MapName) {
		return false
	}
	return m.CheckEligible(cs, charID, efm, playerFacing...)
}

func sameMapName(a, b string) bool {
	return normalizeCutsceneMapName(a) == normalizeCutsceneMapName(b)
}

func normalizeCutsceneMapName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "-", "")
	return strings.ToUpper(name)
}

// FindEligibleClickCutscene resolves an actor/object click to a cutscene.
// Click triggers are keyed by extracted text constants, object names, or
// explicit labels such as object:<phaser_objects.id>.
func (m *CutsceneManager) FindEligibleClickCutscene(mapName string, triggerKeys []string, charID int64, efm *EventFlagManager, playerFacing ...string) *CutsceneScript {
	seen := make(map[string]bool)
	for _, key := range triggerKeys {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true

		m.mu.RLock()
		mapped := append([]*CutsceneScript(nil), m.byTriggerLabel[key]...)
		direct := m.byLabel[key]
		m.mu.RUnlock()

		sortCutscenesBySpecificity(mapped)
		for _, cs := range mapped {
			if m.checkEligibleClickCutscene(cs, mapName, charID, efm, playerFacing...) {
				return cs
			}
		}
		if direct != nil && m.checkEligibleClickCutscene(direct, mapName, charID, efm, playerFacing...) {
			return direct
		}
	}
	return nil
}

func (m *CutsceneManager) checkEligibleClickCutscene(cs *CutsceneScript, mapName string, charID int64, efm *EventFlagManager, playerFacing ...string) bool {
	if cs.TriggerType != "npc_click" {
		return false
	}
	if mapName != "" && cs.MapName != mapName {
		return false
	}
	return m.CheckEligible(cs, charID, efm, playerFacing...)
}

func sortCutscenesBySpecificity(cutscenes []*CutsceneScript) {
	sort.SliceStable(cutscenes, func(i, j int) bool {
		return cutsceneSpecificity(cutscenes[i]) > cutsceneSpecificity(cutscenes[j])
	})
}

func cutsceneSpecificity(cs *CutsceneScript) int {
	if cs == nil {
		return 0
	}
	score := 0
	if cs.RequiresFlag != nil && *cs.RequiresFlag != "" {
		score++
	}
	if cs.RequiresFlagAbst != nil && *cs.RequiresFlagAbst != "" {
		score++
	}
	score += len(cs.RequiresFlags) + len(cs.RequiresFlagsAbst)
	if cs.RequiresItemID != nil && *cs.RequiresItemID > 0 {
		score++
	}
	if cs.RequiresItemAbst != nil && *cs.RequiresItemAbst > 0 {
		score++
	}
	if cs.RequiresCaught != nil && *cs.RequiresCaught > 0 {
		score++
	}
	if cs.RequiresMoney != nil && *cs.RequiresMoney > 0 {
		score++
	}
	if cs.RequiresMoneyBelow != nil && *cs.RequiresMoneyBelow > 0 {
		score++
	}
	if cs.RequiresCoins != nil && *cs.RequiresCoins > 0 {
		score++
	}
	if cs.RequiresCoinsBelow != nil && *cs.RequiresCoinsBelow > 0 {
		score++
	}
	if cs.RequiresPlayerFacing != nil && *cs.RequiresPlayerFacing != "" {
		score++
	}
	return score
}

// GetForMap returns all cutscene scripts for a given map.
func (m *CutsceneManager) GetForMap(mapName string) []*CutsceneScript {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byMap[mapName]
}

// CheckEligible returns true if the player meets the requirements for this cutscene.
func (m *CutsceneManager) CheckEligible(cs *CutsceneScript, charID int64, efm *EventFlagManager, playerFacing ...string) bool {
	eligible, err := checkCutsceneEligibleIn(m.db, cs, charID, efm, playerFacing...)
	if err != nil {
		log.Printf("[Cutscene] Eligibility for %s: %v", cs.ScriptLabel, err)
	}
	return err == nil && eligible
}

func checkCutsceneEligibleIn(q db.DBTX, cs *CutsceneScript, charID int64, efm *EventFlagManager, playerFacing ...string) (bool, error) {
	if efm == nil {
		return false, nil
	}
	if cs.RequiresPlayerFacing != nil && *cs.RequiresPlayerFacing != "" {
		if len(playerFacing) == 0 || normalizeCutsceneFacing(playerFacing[0]) != normalizeCutsceneFacing(*cs.RequiresPlayerFacing) {
			return false, nil
		}
	}
	// Must have requires_flag if specified
	if cs.RequiresFlag != nil && *cs.RequiresFlag != "" {
		if !efm.CheckFlag(charID, *cs.RequiresFlag) {
			return false, nil
		}
	}
	for _, flag := range cs.RequiresFlags {
		if flag != "" && !efm.CheckFlag(charID, flag) {
			return false, nil
		}
	}
	// Must NOT have requires_flag_absent if specified
	if cs.RequiresFlagAbst != nil && *cs.RequiresFlagAbst != "" {
		if efm.CheckFlag(charID, *cs.RequiresFlagAbst) {
			return false, nil
		}
	}
	for _, flag := range cs.RequiresFlagsAbst {
		if flag != "" && efm.CheckFlag(charID, flag) {
			return false, nil
		}
	}
	for _, condition := range []struct {
		id     *int
		absent bool
	}{{cs.RequiresItemID, false}, {cs.RequiresItemAbst, true}} {
		if condition.id == nil || *condition.id <= 0 {
			continue
		}
		var quantity int
		if err := q.QueryRow(`SELECT COALESCE(SUM(ii.quantity),0) FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=$1 AND ii.item_id=$2`, charID, *condition.id).Scan(&quantity); err != nil {
			return false, err
		}
		if (quantity > 0) == condition.absent {
			return false, nil
		}
	}
	for _, condition := range []struct {
		threshold *int
		query     string
		below     bool
	}{
		{cs.RequiresCaught, `SELECT COALESCE(SUM(caught),0) FROM character_pokedex WHERE character_id=$1`, false},
		{cs.RequiresMoney, `SELECT COALESCE(pokedollars,0) FROM character_wallet WHERE character_id=$1`, false},
		{cs.RequiresMoneyBelow, `SELECT COALESCE(pokedollars,0) FROM character_wallet WHERE character_id=$1`, true},
		{cs.RequiresCoins, `SELECT COALESCE(coins,0) FROM character_coins WHERE character_id=$1`, false},
		{cs.RequiresCoinsBelow, `SELECT COALESCE(coins,0) FROM character_coins WHERE character_id=$1`, true},
	} {
		if condition.threshold == nil || *condition.threshold <= 0 {
			continue
		}
		var value int
		if err := q.QueryRow(condition.query, charID).Scan(&value); err != nil && err != sql.ErrNoRows {
			return false, err
		}
		if (value >= *condition.threshold) == condition.below {
			return false, nil
		}
	}
	return true, nil
}

func normalizeCutsceneFacing(direction string) string {
	return normalizeWarpDirection(direction)
}

// FindEligibleCoordCutscene checks if any coord-triggered cutscene should fire
// for the given player at the given map position.
func (m *CutsceneManager) FindEligibleCoordCutscene(mapName string, charID int64, efm *EventFlagManager, playerFacing ...string) *CutsceneScript {
	m.mu.RLock()
	scripts := m.byMap[mapName]
	m.mu.RUnlock()

	for _, cs := range scripts {
		if cs.TriggerType != "coord" {
			continue
		}
		if m.CheckEligible(cs, charID, efm, playerFacing...) {
			return cs
		}
	}
	return nil
}

// MapNameForID resolves a phaser_maps.id to its name string.
func (m *CutsceneManager) MapNameForID(mapID int) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapIDToName[mapID]
}

// FindEligibleMapScriptCutscene checks if any map_script-triggered cutscene should fire.
func (m *CutsceneManager) FindEligibleMapScriptCutscene(mapName string, charID int64, efm *EventFlagManager, playerFacing ...string) *CutsceneScript {
	m.mu.RLock()
	scripts := m.byMap[mapName]
	m.mu.RUnlock()

	for _, cs := range scripts {
		if cs.TriggerType != "map_script" {
			continue
		}
		if m.CheckEligible(cs, charID, efm, playerFacing...) {
			return cs
		}
	}
	return nil
}
