package world

import (
	"database/sql"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"

	"capturequest/internal/pokebattle"
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

func parseStringJSONList(raw []byte, scriptLabel, field string) []string {
	if strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		log.Printf("[CutsceneManager] Error parsing %s for %s: %v", field, scriptLabel, err)
		return nil
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
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

// CutsceneManager loads cutscene scripts from the database and provides
// lookups by map + trigger type. Cutscene eligibility is checked per-player
// using their event flags.
type CutsceneManager struct {
	db pokebattle.DBTX
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
func NewCutsceneManager(db pokebattle.DBTX) *CutsceneManager {
	return &CutsceneManager{
		db:             db,
		byMap:          make(map[string][]*CutsceneScript),
		byLabel:        make(map[string]*CutsceneScript),
		byTriggerLabel: make(map[string][]*CutsceneScript),
		mapIDToName:    make(map[int]string),
	}
}

// Load reads all cutscene scripts from the database into memory.
func (m *CutsceneManager) Load() {
	// Load map ID -> name mapping
	mapRows, err := m.db.Query(`SELECT id, name FROM phaser_maps`)
	if err != nil {
		log.Printf("[CutsceneManager] Failed to load map names: %v", err)
	} else {
		defer mapRows.Close()
		idToName := make(map[int]string)
		for mapRows.Next() {
			var id int
			var name string
			if err := mapRows.Scan(&id, &name); err == nil {
				idToName[id] = name
			}
		}
		m.mu.Lock()
		m.mapIDToName = idToName
		m.mu.Unlock()
	}

	rows, queryShape, err := m.queryCutsceneRows()
	if err != nil {
		log.Printf("[CutsceneManager] Failed to load: %v", err)
		return
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

		if queryShape.hasRequiresFlagArrays && queryShape.hasTriggerLabel && queryShape.hasRequiresItemID && queryShape.hasRequiresItemAbsentID && queryShape.hasRequiresCaught && queryShape.hasRequiresMoney && queryShape.hasRequiresCoins && queryShape.hasRequiresPlayerFacing {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqFlagsJSON, &reqFlagsAbsentJSON, &reqItemID, &reqItemAbsentID, &reqCaught, &reqMoney, &reqMoneyBelow, &reqCoins, &reqCoinsBelow, &reqPlayerFacing,
				&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel && queryShape.hasRequiresItemID && queryShape.hasRequiresItemAbsentID && queryShape.hasRequiresCaught && queryShape.hasRequiresMoney && queryShape.hasRequiresCoins && queryShape.hasRequiresPlayerFacing {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqItemID, &reqItemAbsentID, &reqCaught, &reqMoney, &reqMoneyBelow, &reqCoins, &reqCoinsBelow, &reqPlayerFacing,
				&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel && queryShape.hasRequiresItemID && queryShape.hasRequiresItemAbsentID && queryShape.hasRequiresCaught && queryShape.hasRequiresMoney && queryShape.hasRequiresCoins {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqItemID, &reqItemAbsentID, &reqCaught, &reqMoney, &reqMoneyBelow, &reqCoins, &reqCoinsBelow,
				&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel && queryShape.hasRequiresItemID && queryShape.hasRequiresItemAbsentID && queryShape.hasRequiresCaught && queryShape.hasRequiresMoney {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqItemID, &reqItemAbsentID, &reqCaught, &reqMoney, &reqMoneyBelow,
				&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel && queryShape.hasRequiresItemID && queryShape.hasRequiresItemAbsentID && queryShape.hasRequiresCaught {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqItemID, &reqItemAbsentID, &reqCaught,
				&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel && queryShape.hasRequiresItemID && queryShape.hasRequiresCaught {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqItemID, &reqCaught,
				&setsFlagsJSON, &actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel && queryShape.hasRequiresItemID {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &reqItemID, &setsFlagsJSON,
				&actionsJSON, &warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else if queryShape.hasTriggerLabel {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&triggerLabel, &reqFlag, &reqFlagAbsent, &setsFlagsJSON, &actionsJSON,
				&warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		} else {
			if err := rows.Scan(&cs.ID, &cs.ScriptLabel, &cs.MapName, &cs.TriggerType,
				&reqFlag, &reqFlagAbsent, &setsFlagsJSON, &actionsJSON,
				&warpMapID, &warpX, &warpY); err != nil {
				log.Printf("[CutsceneManager] Error scanning row: %v", err)
				continue
			}
		}
		cs.TriggerLabel = triggerLabel
		cs.RequiresFlag = reqFlag
		cs.RequiresFlagAbst = reqFlagAbsent
		cs.RequiresFlags = parseStringJSONList(reqFlagsJSON, cs.ScriptLabel, "requires_flags")
		cs.RequiresFlagsAbst = parseStringJSONList(reqFlagsAbsentJSON, cs.ScriptLabel, "requires_flags_absent")
		cs.RequiresItemID = reqItemID
		cs.RequiresItemAbst = reqItemAbsentID
		cs.RequiresCaught = reqCaught
		cs.RequiresMoney = reqMoney
		cs.RequiresMoneyBelow = reqMoneyBelow
		cs.RequiresCoins = reqCoins
		cs.RequiresCoinsBelow = reqCoinsBelow
		cs.RequiresPlayerFacing = reqPlayerFacing
		cs.Actions = actionsJSON
		cs.WarpToMapID = warpMapID
		cs.WarpToX = warpX
		cs.WarpToY = warpY

		// Parse sets_flags JSON array
		if setsFlagsJSON != nil {
			if err := json.Unmarshal(setsFlagsJSON, &cs.SetsFlags); err != nil {
				log.Printf("[CutsceneManager] Error parsing sets_flags for %s: %v", cs.ScriptLabel, err)
			}
		}

		byMap[cs.MapName] = append(byMap[cs.MapName], &cs)
		byLabel[cs.ScriptLabel] = &cs
		if cs.TriggerLabel != nil && *cs.TriggerLabel != "" {
			byTriggerLabel[*cs.TriggerLabel] = append(byTriggerLabel[*cs.TriggerLabel], &cs)
		}
		count++
	}

	m.mu.Lock()
	m.byMap = byMap
	m.byLabel = byLabel
	m.byTriggerLabel = byTriggerLabel
	m.mu.Unlock()

	log.Printf("[CutsceneManager] Loaded %d cutscene scripts across %d maps", count, len(byMap))
}

type cutsceneQueryShape struct {
	hasRequiresFlagArrays   bool
	hasTriggerLabel         bool
	hasRequiresItemID       bool
	hasRequiresItemAbsentID bool
	hasRequiresCaught       bool
	hasRequiresMoney        bool
	hasRequiresCoins        bool
	hasRequiresPlayerFacing bool
}

func (m *CutsceneManager) queryCutsceneRows() (*sql.Rows, cutsceneQueryShape, error) {
	rows, err := m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_flags, requires_flags_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
			requires_money, requires_money_below, requires_coins, requires_coins_below, requires_player_facing, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{
			hasRequiresFlagArrays: true,
			hasTriggerLabel:       true, hasRequiresItemID: true, hasRequiresItemAbsentID: true,
			hasRequiresCaught: true, hasRequiresMoney: true, hasRequiresCoins: true,
			hasRequiresPlayerFacing: true,
		}, nil
	}

	log.Printf("[CutsceneManager] requires_flags columns unavailable, using scalar-flag cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
			requires_money, requires_money_below, requires_coins, requires_coins_below, requires_player_facing, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{
			hasTriggerLabel: true, hasRequiresItemID: true, hasRequiresItemAbsentID: true,
			hasRequiresCaught: true, hasRequiresMoney: true, hasRequiresCoins: true,
			hasRequiresPlayerFacing: true,
		}, nil
	}

	log.Printf("[CutsceneManager] requires_player_facing column unavailable, using coin-gated cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
			requires_money, requires_money_below, requires_coins, requires_coins_below, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{hasTriggerLabel: true, hasRequiresItemID: true, hasRequiresItemAbsentID: true, hasRequiresCaught: true, hasRequiresMoney: true, hasRequiresCoins: true}, nil
	}

	log.Printf("[CutsceneManager] requires_coins columns unavailable, using money-gated cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught,
			requires_money, requires_money_below, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{hasTriggerLabel: true, hasRequiresItemID: true, hasRequiresItemAbsentID: true, hasRequiresCaught: true, hasRequiresMoney: true}, nil
	}

	log.Printf("[CutsceneManager] requires_money columns unavailable, using caught/item-absent cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_item_id, requires_item_absent_id, requires_pokedex_caught, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{hasTriggerLabel: true, hasRequiresItemID: true, hasRequiresItemAbsentID: true, hasRequiresCaught: true}, nil
	}

	log.Printf("[CutsceneManager] requires_item_absent_id column unavailable, using caught-gated cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_item_id, requires_pokedex_caught, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{hasTriggerLabel: true, hasRequiresItemID: true, hasRequiresCaught: true}, nil
	}

	log.Printf("[CutsceneManager] requires_pokedex_caught column unavailable, using item-gated cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, requires_item_id, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{hasTriggerLabel: true, hasRequiresItemID: true}, nil
	}

	log.Printf("[CutsceneManager] requires_item_id column unavailable, using trigger-label cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type, trigger_label,
			requires_flag, requires_flag_absent, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	if err == nil {
		return rows, cutsceneQueryShape{hasTriggerLabel: true}, nil
	}

	log.Printf("[CutsceneManager] trigger_label column unavailable, using legacy cutscene query: %v", err)
	rows, err = m.db.Query(`
		SELECT id, script_label, map_name, trigger_type,
			requires_flag, requires_flag_absent, sets_flags, actions,
			warp_to_map_id, warp_to_x, warp_to_y
		FROM phaser_cutscene_scripts
		ORDER BY id`)
	return rows, cutsceneQueryShape{}, err
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
	m.mu.RLock()
	mapped := append([]*CutsceneScript(nil), m.byTriggerLabel[trigger.Label]...)
	direct := m.byLabel[trigger.Label]
	m.mu.RUnlock()

	sortCutscenesBySpecificity(mapped)
	for _, cs := range mapped {
		if m.checkEligibleCoordinateCutscene(cs, trigger, charID, efm, playerFacing...) {
			return cs
		}
	}
	if m.checkEligibleCoordinateCutscene(direct, trigger, charID, efm, playerFacing...) {
		return direct
	}
	return nil
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
	if efm == nil {
		return false
	}
	if cs.RequiresPlayerFacing != nil && *cs.RequiresPlayerFacing != "" {
		if len(playerFacing) == 0 || normalizeCutsceneFacing(playerFacing[0]) != normalizeCutsceneFacing(*cs.RequiresPlayerFacing) {
			return false
		}
	}
	// Must have requires_flag if specified
	if cs.RequiresFlag != nil && *cs.RequiresFlag != "" {
		if !efm.CheckFlag(charID, *cs.RequiresFlag) {
			return false
		}
	}
	for _, flag := range cs.RequiresFlags {
		if flag != "" && !efm.CheckFlag(charID, flag) {
			return false
		}
	}
	// Must NOT have requires_flag_absent if specified
	if cs.RequiresFlagAbst != nil && *cs.RequiresFlagAbst != "" {
		if efm.CheckFlag(charID, *cs.RequiresFlagAbst) {
			return false
		}
	}
	for _, flag := range cs.RequiresFlagsAbst {
		if flag != "" && efm.CheckFlag(charID, flag) {
			return false
		}
	}
	if cs.RequiresItemID != nil && *cs.RequiresItemID > 0 {
		if !m.characterHasInventoryItem(charID, *cs.RequiresItemID) {
			return false
		}
	}
	if cs.RequiresItemAbst != nil && *cs.RequiresItemAbst > 0 {
		if m.characterHasInventoryItem(charID, *cs.RequiresItemAbst) {
			return false
		}
	}
	if cs.RequiresCaught != nil && *cs.RequiresCaught > 0 {
		if m.characterPokedexCaughtCount(charID) < *cs.RequiresCaught {
			return false
		}
	}
	if cs.RequiresMoney != nil && *cs.RequiresMoney > 0 {
		if m.characterMoney(charID) < *cs.RequiresMoney {
			return false
		}
	}
	if cs.RequiresMoneyBelow != nil && *cs.RequiresMoneyBelow > 0 {
		if m.characterMoney(charID) >= *cs.RequiresMoneyBelow {
			return false
		}
	}
	if cs.RequiresCoins != nil && *cs.RequiresCoins > 0 {
		if m.characterCoins(charID) < *cs.RequiresCoins {
			return false
		}
	}
	if cs.RequiresCoinsBelow != nil && *cs.RequiresCoinsBelow > 0 {
		if m.characterCoins(charID) >= *cs.RequiresCoinsBelow {
			return false
		}
	}
	return true
}

func normalizeCutsceneFacing(direction string) string {
	return normalizeWarpDirection(direction)
}

func (m *CutsceneManager) characterHasInventoryItem(charID int64, itemID int) bool {
	var quantity int
	err := m.db.QueryRow(`
		SELECT COALESCE(SUM(ii.quantity), 0)
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		WHERE ci.character_id = $1 AND ii.item_id = $2`, charID, itemID).Scan(&quantity)
	return err == nil && quantity > 0
}

func (m *CutsceneManager) characterPokedexCaughtCount(charID int64) int {
	var count int
	if err := m.db.QueryRow(`
		SELECT COALESCE(SUM(caught), 0)
		FROM character_pokedex
		WHERE character_id = $1`, charID).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (m *CutsceneManager) characterMoney(charID int64) int {
	var money int
	if err := m.db.QueryRow(`
		SELECT COALESCE(pokedollars, 0)
		FROM character_wallet
		WHERE character_id = $1`, charID).Scan(&money); err != nil {
		return 0
	}
	return money
}

func (m *CutsceneManager) characterCoins(charID int64) int {
	var coins int
	if err := m.db.QueryRow(`
		SELECT COALESCE(coins, 0)
		FROM character_coins
		WHERE character_id = $1`, charID).Scan(&coins); err != nil {
		return 0
	}
	return coins
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
