package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"database/sql"
	"log"
	"math/rand"
	"sync"
)

// encounterAreaData holds preloaded encounter area info.
type encounterAreaData struct {
	ID            int
	Name          string
	EncounterRate int // 0-255, Gen 1 style: rand(256) < rate
	Slots         []encounterSlot
}

type encounterSlot struct {
	PokemonID   int
	Level       int
	Probability float64
}

// Repel step durations (Gen 1)
const (
	RepelSteps       = 100
	SuperRepelSteps  = 200
	MaxRepelSteps    = 250
	RepelItemID      = 30
	SuperRepelItemID = 56
	MaxRepelItemID   = 57
)

// repelState tracks active repel for a player.
type repelState struct {
	StepsLeft int
}

type RepelStatus struct {
	Active    bool
	StepsLeft int
}

// WildEncounterManager handles wild encounter checks during player movement.
// Uses the phaser_tiles.encounter_area_id column to determine encounter eligibility
// per tile, eliminating the need for map ID resolution on the overworld.
type WildEncounterManager struct {
	wh *WorldHandler

	// Encounter area data keyed by area ID
	areas map[int]*encounterAreaData

	// Tile encounter cache: (mapID, x, y) → encounter_area_id (0 = none)
	tileCache   map[[3]int]int
	tileCacheMu sync.RWMutex
	cacheLoaded bool

	// Repel tracking per player
	repels   map[int64]*repelState
	repelsMu sync.RWMutex
}

// NewWildEncounterManager creates and initializes the wild encounter manager.
func NewWildEncounterManager(wh *WorldHandler) *WildEncounterManager {
	return &WildEncounterManager{
		wh:        wh,
		areas:     make(map[int]*encounterAreaData),
		tileCache: make(map[[3]int]int),
		repels:    make(map[int64]*repelState),
	}
}

// Load preloads all encounter areas and their slots from the DB.
func (m *WildEncounterManager) Load() {
	myDB := db.GlobalWorldDB.DB

	// Load encounter areas
	rows, err := myDB.Query(`SELECT id, name, encounter_rate FROM phaser_encounter_areas`)
	if err != nil {
		log.Printf("[WildEncounter] Failed to load encounter areas: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var area encounterAreaData
		if err := rows.Scan(&area.ID, &area.Name, &area.EncounterRate); err != nil {
			log.Printf("[WildEncounter] Error scanning encounter area: %v", err)
			continue
		}
		m.areas[area.ID] = &area
	}

	// Load slots for all areas
	slotRows, err := myDB.Query(`
		SELECT encounter_area_id, pokemon_id, level, probability
		FROM phaser_encounter_area_slots
		ORDER BY encounter_area_id, slot_index`)
	if err != nil {
		log.Printf("[WildEncounter] Failed to load encounter slots: %v", err)
		return
	}
	defer slotRows.Close()

	for slotRows.Next() {
		var areaID int
		var slot encounterSlot
		if err := slotRows.Scan(&areaID, &slot.PokemonID, &slot.Level, &slot.Probability); err != nil {
			continue
		}
		if area, ok := m.areas[areaID]; ok {
			area.Slots = append(area.Slots, slot)
		}
	}

	// Preload the tile encounter cache for all tiles that have encounter areas
	m.preloadTileCache()

	log.Printf("[WildEncounter] Loaded %d encounter areas, tile cache has %d encounter tiles",
		len(m.areas), len(m.tileCache))
}

// preloadTileCache loads all tiles with encounter_area_id into the global cache.
func (m *WildEncounterManager) preloadTileCache() {
	myDB := db.GlobalWorldDB.DB

	rows, err := myDB.Query(`
		SELECT map_id, x, y, encounter_area_id
		FROM phaser_tiles
		WHERE encounter_area_id IS NOT NULL`)
	if err != nil {
		log.Printf("[WildEncounter] Failed to preload tile cache: %v", err)
		return
	}
	defer rows.Close()

	m.tileCacheMu.Lock()
	defer m.tileCacheMu.Unlock()

	for rows.Next() {
		var x, y, areaID int
		var mapID sql.NullInt64
		if err := rows.Scan(&mapID, &x, &y, &areaID); err != nil {
			continue
		}
		if mapID.Valid {
			mid := int(mapID.Int64)
			m.tileCache[[3]int{mid, x, y}] = areaID
			// For overworld maps, also index under the unified overworld map ID (9999)
			if m.wh.ActorManager.IsOverworld(mid) {
				m.tileCache[[3]int{UnifiedOverworldMapID, x, y}] = areaID
			}
		} else {
			// Overworld tiles (map_id IS NULL) — index under unified overworld ID
			m.tileCache[[3]int{UnifiedOverworldMapID, x, y}] = areaID
		}
	}
	m.cacheLoaded = true
}

// CheckPlayerStep checks if a wild encounter should trigger when a player steps on a tile.
// Uses (mapID, x, y) to look up the encounter area for the tile.
// Returns true if an encounter was triggered (caller should stop player movement).
func (m *WildEncounterManager) CheckPlayerStep(charID int64, x, y, mapID int, ses *session.Session) bool {
	// Check if player is already in a battle
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		return false
	}

	// Look up encounter area for this tile by (mapID, x, y)
	areaID := m.getEncounterAreaID(mapID, x, y)
	if areaID == 0 {
		return false // No encounters on this tile
	}

	area, ok := m.areas[areaID]
	if !ok || area.EncounterRate == 0 || len(area.Slots) == 0 {
		return false
	}

	// Tick repel step counter (even if no encounter triggers)
	m.tickRepel(charID, ses)

	// Gen 1 encounter rate check: roll rand(256) < encounterRate
	roll := rand.Intn(256)
	if roll >= area.EncounterRate {
		return false // No encounter this step
	}

	// Select which Pokémon would appear
	pokemonID, level := m.selectEncounterPokemon(area)

	// Repel check: if repel is active and wild level < lead party level, suppress
	if m.isRepelActive(charID) {
		leadLevel := m.getLeadPokemonLevel(charID)
		if level < leadLevel {
			return false // Repel suppresses this encounter
		}
	}

	// Encounter triggered!
	log.Printf("[WildEncounter] Encounter triggered for char %d at (%d,%d) area=%s rate=%d/256",
		charID, x, y, area.Name, area.EncounterRate)

	m.startWildBattleWithPokemon(charID, pokemonID, level, ses)
	return true
}

// getEncounterAreaID returns the encounter_area_id for a (mapID, x, y) position.
func (m *WildEncounterManager) getEncounterAreaID(mapID, x, y int) int {
	key := [3]int{mapID, x, y}

	m.tileCacheMu.RLock()
	areaID, ok := m.tileCache[key]
	m.tileCacheMu.RUnlock()

	if ok {
		return areaID
	}

	// Cache miss (shouldn't happen after preload, but handle gracefully)
	if m.cacheLoaded {
		return 0 // Not in cache = no encounter area
	}

	// Fallback: query DB
	myDB := db.GlobalWorldDB.DB
	var dbAreaID sql.NullInt64
	var err error
	if mapID == UnifiedOverworldMapID {
		err = myDB.QueryRow(`
			SELECT encounter_area_id FROM phaser_tiles
			WHERE map_id IS NULL AND x = $1 AND y = $2
			LIMIT 1`, x, y).Scan(&dbAreaID)
	} else {
		err = myDB.QueryRow(`
			SELECT encounter_area_id FROM phaser_tiles
			WHERE map_id = $1 AND x = $2 AND y = $3
			LIMIT 1`, mapID, x, y).Scan(&dbAreaID)
	}
	if err != nil || !dbAreaID.Valid {
		return 0
	}

	// Store in cache
	m.tileCacheMu.Lock()
	m.tileCache[key] = int(dbAreaID.Int64)
	m.tileCacheMu.Unlock()

	return int(dbAreaID.Int64)
}

// selectEncounterPokemon picks a random Pokémon from an encounter area's slot table.
func (m *WildEncounterManager) selectEncounterPokemon(area *encounterAreaData) (pokemonID, level int) {
	roll := rand.Float64() * 100.0
	cumulative := 0.0
	for _, slot := range area.Slots {
		cumulative += slot.Probability
		if roll < cumulative {
			return slot.PokemonID, slot.Level
		}
	}
	// Fallback to last slot
	last := area.Slots[len(area.Slots)-1]
	return last.PokemonID, last.Level
}

// startWildBattle initiates a wild battle for the player using encounter area data.
func (m *WildEncounterManager) startWildBattle(charID int64, area *encounterAreaData, ses *session.Session) {
	myDB := db.GlobalWorldDB.DB

	// Select a wild Pokémon from the area's slots
	pokemonID, level := m.selectEncounterPokemon(area)

	// Build the wild Pokémon
	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, pokemonID, level)
	if err != nil {
		log.Printf("[WildEncounter] Failed to build wild pokemon %d: %v", pokemonID, err)
		return
	}

	// Load player's party from DB. Oak's starter script is the source of truth
	// for the first Pokémon.
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[WildEncounter] No party for char %d (err: %v), triggering blackout", charID, err)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return
	}

	// Check if any party Pokémon can battle
	hasAlive := false
	for _, p := range playerParty {
		if p.CurHP > 0 {
			hasAlive = true
			break
		}
	}
	if !hasAlive {
		log.Printf("[WildEncounter] All pokemon fainted for char %d, triggering blackout", charID)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return
	}

	// Create battle
	battle := pokebattle.NewWildBattle(playerParty, wildPokemon)
	configureBattleObedience(battle, charID, m.wh.EventFlags)
	setBattle(charID, battle)

	log.Printf("[WildEncounter] %s started wild battle: L%d %s vs L%d %s",
		ses.Client.CharData().Name, playerParty[0].Level, playerParty[0].Name,
		wildPokemon.Level, wildPokemon.Name)

	resp := buildBattleStateResponse(battle)
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
}

// startWildBattleWithPokemon initiates a wild battle with a specific pokemonID and level.
func (m *WildEncounterManager) startWildBattleWithPokemon(charID int64, pokemonID, level int, ses *session.Session) {
	myDB := db.GlobalWorldDB.DB

	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, pokemonID, level)
	if err != nil {
		log.Printf("[WildEncounter] Failed to build wild pokemon %d: %v", pokemonID, err)
		return
	}

	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[WildEncounter] No party for char %d (err: %v), triggering blackout", charID, err)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return
	}

	hasAlive := false
	for _, p := range playerParty {
		if p.CurHP > 0 {
			hasAlive = true
			break
		}
	}
	if !hasAlive {
		log.Printf("[WildEncounter] All pokemon fainted for char %d, triggering blackout", charID)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return
	}

	battle := pokebattle.NewWildBattle(playerParty, wildPokemon)
	configureBattleObedience(battle, charID, m.wh.EventFlags)
	setBattle(charID, battle)

	log.Printf("[WildEncounter] %s started wild battle: L%d %s vs L%d %s",
		ses.Client.CharData().Name, playerParty[0].Level, playerParty[0].Name,
		wildPokemon.Level, wildPokemon.Name)

	resp := buildBattleStateResponse(battle)
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
}

// --- Repel system ---

// ActivateRepel starts a repel effect for the given player.
func (m *WildEncounterManager) ActivateRepel(charID int64, itemID int) {
	steps, ok := RepelStepsForItem(itemID)
	if !ok {
		return
	}
	m.repelsMu.Lock()
	m.repels[charID] = &repelState{StepsLeft: steps}
	m.repelsMu.Unlock()
	log.Printf("[Repel] Activated for char %d: %d steps", charID, steps)
}

func RepelStepsForItem(itemID int) (int, bool) {
	switch itemID {
	case RepelItemID:
		return RepelSteps, true
	case SuperRepelItemID:
		return SuperRepelSteps, true
	case MaxRepelItemID:
		return MaxRepelSteps, true
	default:
		return 0, false
	}
}

// isRepelActive returns true if the player has an active repel.
func (m *WildEncounterManager) isRepelActive(charID int64) bool {
	m.repelsMu.RLock()
	defer m.repelsMu.RUnlock()
	r := m.repels[charID]
	return r != nil && r.StepsLeft > 0
}

func (m *WildEncounterManager) RepelStatus(charID int64) RepelStatus {
	m.repelsMu.RLock()
	defer m.repelsMu.RUnlock()
	r := m.repels[charID]
	if r == nil || r.StepsLeft <= 0 {
		return RepelStatus{}
	}
	return RepelStatus{
		Active:    true,
		StepsLeft: r.StepsLeft,
	}
}

func (m *WildEncounterManager) SetRepelSteps(charID int64, stepsLeft int) {
	m.repelsMu.Lock()
	defer m.repelsMu.Unlock()
	if stepsLeft <= 0 {
		delete(m.repels, charID)
		return
	}
	m.repels[charID] = &repelState{StepsLeft: stepsLeft}
}

// tickRepel decrements the repel counter and notifies when it wears off.
func (m *WildEncounterManager) tickRepel(charID int64, ses *session.Session) {
	wore := m.AdvanceRepelStep(charID)
	if wore {
		log.Printf("[Repel] Wore off for char %d", charID)
		ses.SendStreamJSON(map[string]interface{}{
			"message": "REPEL's effect wore off!",
		}, opcodes.RepelWoreOffNotify)
	}
}

func (m *WildEncounterManager) AdvanceRepelStep(charID int64) bool {
	m.repelsMu.Lock()
	r := m.repels[charID]
	if r == nil || r.StepsLeft <= 0 {
		m.repelsMu.Unlock()
		return false
	}
	r.StepsLeft--
	wore := r.StepsLeft <= 0
	if wore {
		delete(m.repels, charID)
	}
	m.repelsMu.Unlock()
	return wore
}

// getLeadPokemonLevel returns the level of the player's lead (first non-fainted) Pokémon.
func (m *WildEncounterManager) getLeadPokemonLevel(charID int64) int {
	myDB := db.GlobalWorldDB.DB
	party, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(party) == 0 {
		return 0
	}
	for _, p := range party {
		if p.CurHP > 0 {
			return p.Level
		}
	}
	return party[0].Level
}

// ClearPlayer removes all tracking data for a player (on disconnect).
func (m *WildEncounterManager) ClearPlayer(charID int64) {
	m.repelsMu.Lock()
	delete(m.repels, charID)
	m.repelsMu.Unlock()
}
