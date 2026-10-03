package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"context"
	"database/sql"
	"fmt"
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

	database *sql.DB
}

// NewWildEncounterManager creates and initializes the wild encounter manager.
func NewWildEncounterManager(wh *WorldHandler, database *sql.DB) *WildEncounterManager {
	return &WildEncounterManager{
		wh:        wh,
		areas:     make(map[int]*encounterAreaData),
		tileCache: make(map[[3]int]int),
		database:  database,
	}
}

// Load preloads all encounter areas and their slots before world timers start.
func (m *WildEncounterManager) Load(ctx context.Context) error {
	if m.wh == nil || m.wh.database == nil {
		return fmt.Errorf("wild encounter preload requires a database")
	}
	myDB := m.wh.database
	// Readiness must reject a code-only activation without its durable effect
	// schema; otherwise every encounter step would fail after serving traffic.
	probe, err := m.database.QueryContext(ctx, `SELECT character_id,steps_left FROM character_repels LIMIT 0`)
	if err != nil {
		return fmt.Errorf("repel state schema: %w", err)
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close repel schema probe: %w", err)
	}

	areas := make(map[int]*encounterAreaData)
	// Load encounter areas
	rows, err := myDB.QueryContext(ctx, `SELECT id, name, encounter_rate FROM phaser_encounter_areas`)
	if err != nil {
		return fmt.Errorf("[WildEncounter] Failed to load encounter areas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var area encounterAreaData
		if err := rows.Scan(&area.ID, &area.Name, &area.EncounterRate); err != nil {
			return fmt.Errorf("[WildEncounter] Error scanning encounter area: %w", err)
		}
		areas[area.ID] = &area
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read encounter areas: %w", err)
	}
	// Load slots for all areas
	slotRows, err := myDB.QueryContext(ctx, `
		SELECT encounter_area_id, pokemon_id, level, probability
		FROM phaser_encounter_area_slots
		ORDER BY encounter_area_id, slot_index`)
	if err != nil {
		return fmt.Errorf("[WildEncounter] Failed to load encounter slots: %w", err)
	}
	defer slotRows.Close()

	for slotRows.Next() {
		var areaID int
		var slot encounterSlot
		if err := slotRows.Scan(&areaID, &slot.PokemonID, &slot.Level, &slot.Probability); err != nil {
			return fmt.Errorf("scan encounter slot: %w", err)
		}
		if area, ok := areas[areaID]; ok {
			area.Slots = append(area.Slots, slot)
		} else {
			return fmt.Errorf("encounter slot references absent area %d", areaID)
		}
	}

	// Preload the tile encounter cache for all tiles that have encounter areas
	if err := slotRows.Err(); err != nil {
		return fmt.Errorf("read encounter slots: %w", err)
	}
	tiles, err := m.loadTileCache(ctx, areas)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.tileCacheMu.Lock()
	m.areas, m.tileCache, m.cacheLoaded = areas, tiles, true
	m.tileCacheMu.Unlock()

	log.Printf("[WildEncounter] Loaded %d encounter areas, tile cache has %d encounter tiles",
		len(areas), len(tiles))
	return nil
}

// loadTileCache stages tile references without modifying the published cache.
func (m *WildEncounterManager) loadTileCache(ctx context.Context, areas map[int]*encounterAreaData) (map[[3]int]int, error) {
	myDB := m.wh.database

	rows, err := myDB.QueryContext(ctx, `
		SELECT map_id, x, y, encounter_area_id
		FROM phaser_tiles
		WHERE encounter_area_id IS NOT NULL
		  AND is_tile_erased = 0`)
	if err != nil {
		return nil, fmt.Errorf("[WildEncounter] Failed to preload tile cache: %w", err)
	}
	defer rows.Close()

	tiles := make(map[[3]int]int)

	for rows.Next() {
		var x, y, areaID int
		var mapID sql.NullInt64
		if err := rows.Scan(&mapID, &x, &y, &areaID); err != nil {
			return nil, fmt.Errorf("scan encounter tile: %w", err)
		}
		if _, ok := areas[areaID]; !ok {
			return nil, fmt.Errorf("encounter tile map %v (%d,%d) references absent area %d", mapID, x, y, areaID)
		}
		if mapID.Valid {
			mid := int(mapID.Int64)
			tiles[[3]int{mid, x, y}] = areaID
			// For overworld maps, also index under the unified overworld map ID (9999)
			if m.wh.ActorManager.IsOverworld(mid) {
				tiles[[3]int{UnifiedOverworldMapID, x, y}] = areaID
			}
		} else {
			// Overworld tiles (map_id IS NULL) — index under unified overworld ID
			tiles[[3]int{UnifiedOverworldMapID, x, y}] = areaID
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read encounter tiles: %w", err)
	}
	return tiles, nil
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
	repel, err := m.tickRepel(charID, ses)
	if err != nil {
		log.Printf("[Repel] Step failed for character %d: %v", charID, err)
		return false
	}

	// Gen 1 encounter rate check: roll rand(256) < encounterRate
	roll := rand.Intn(256)
	if roll >= area.EncounterRate {
		return false // No encounter this step
	}

	// Select which Pokémon would appear
	pokemonID, level := m.selectEncounterPokemon(area)

	// Repel check: if repel is active and wild level < lead party level, suppress
	if repel.Active {
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
	cacheLoaded := m.cacheLoaded
	m.tileCacheMu.RUnlock()

	if ok {
		return areaID
	}

	// Cache miss (shouldn't happen after preload, but handle gracefully)
	if cacheLoaded {
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
			  AND is_tile_erased = 0
			LIMIT 1`, x, y).Scan(&dbAreaID)
	} else {
		err = myDB.QueryRow(`
			SELECT encounter_area_id FROM phaser_tiles
			WHERE map_id = $1 AND x = $2 AND y = $3
			  AND is_tile_erased = 0
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

func (m *WildEncounterManager) InvalidateTiles(mapID int, tiles []TileEdit) {
	if m == nil || len(tiles) == 0 {
		return
	}
	m.tileCacheMu.Lock()
	defer m.tileCacheMu.Unlock()
	for _, tile := range tiles {
		delete(m.tileCache, [3]int{mapID, tile.X, tile.Y})
		if mapID == UnifiedOverworldMapID || (m.wh != nil && m.wh.ActorManager != nil && m.wh.ActorManager.IsOverworld(mapID)) {
			delete(m.tileCache, [3]int{UnifiedOverworldMapID, tile.X, tile.Y})
		}
	}
	m.cacheLoaded = false
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
	battle, err = startBattle(m.wh.database, charID, battle)
	if err != nil {
		log.Printf("[PokeBattle] Start failed for character %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not start battle. Please reconnect."}, opcodes.PokeBattleStartResponse)
		return
	}

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
	battle, err = startBattle(m.wh.database, charID, battle)
	if err != nil {
		log.Printf("[PokeBattle] Start failed for character %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not start battle. Please reconnect."}, opcodes.PokeBattleStartResponse)
		return
	}

	log.Printf("[WildEncounter] %s started wild battle: L%d %s vs L%d %s",
		ses.Client.CharData().Name, playerParty[0].Level, playerParty[0].Name,
		wildPokemon.Level, wildPokemon.Name)

	resp := buildBattleStateResponse(battle)
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
}

// --- Repel system ---

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

func (m *WildEncounterManager) RepelStatus(charID int64) (RepelStatus, error) {
	return loadRepelStatus(context.Background(), m.database, charID)
}

func (m *WildEncounterManager) SetRepelSteps(charID int64, stepsLeft int) error {
	return setRepelSteps(context.Background(), m.database, charID, stepsLeft)
}

func (m *WildEncounterManager) AdvanceRepelStep(charID int64) (bool, error) {
	_, wore, err := advanceRepelStep(context.Background(), m.database, charID)
	return wore, err
}

func (m *WildEncounterManager) tickRepel(charID int64, ses *session.Session) (RepelStatus, error) {
	status, wore, err := advanceRepelStep(context.Background(), m.database, charID)
	if err != nil {
		return RepelStatus{}, err
	}
	if wore {
		ses.SendStreamJSON(map[string]interface{}{"message": "REPEL's effect wore off!"}, opcodes.RepelWoreOffNotify)
	}
	return status, nil
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
