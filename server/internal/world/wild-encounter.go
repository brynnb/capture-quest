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

	database      *sql.DB
	encounterRoll func(int) int
}

// NewWildEncounterManager creates and initializes the wild encounter manager.
func NewWildEncounterManager(wh *WorldHandler, database *sql.DB) *WildEncounterManager {
	return &WildEncounterManager{
		wh:            wh,
		areas:         make(map[int]*encounterAreaData),
		tileCache:     make(map[[3]int]int),
		database:      database,
		encounterRoll: rand.Intn,
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

// A wild step returns private committed-effect candidates for its movement caller.
type wildStepResult struct {
	Battle       *pokebattle.BattleState
	Blackout     *BlackoutResult
	Party        []*pokebattle.Pokemon
	RepelWoreOff bool
}

func (m *WildEncounterManager) encounterAreaIn(q db.DBTX, mapID, x, y int) (*encounterAreaData, error) {
	m.tileCacheMu.RLock()
	id, known := m.tileCache[[3]int{mapID, x, y}]
	loaded := m.cacheLoaded
	m.tileCacheMu.RUnlock()
	if !known && !loaded {
		var raw sql.NullInt64
		err := q.QueryRow(`SELECT encounter_area_id FROM phaser_tiles WHERE (($4 AND map_id IS NULL) OR (NOT $4 AND map_id=$1)) AND x=$2 AND y=$3 AND is_tile_erased=0 LIMIT 1`, mapID, x, y, mapID == UnifiedOverworldMapID).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if raw.Valid {
			id = int(raw.Int64)
		}
	}
	if id == 0 {
		return nil, nil
	}
	area := m.areas[id]
	if area == nil {
		return nil, fmt.Errorf("encounter tile map %d (%d,%d) references absent area %d", mapID, x, y, id)
	}
	if area.EncounterRate == 0 || len(area.Slots) == 0 {
		return nil, nil
	}
	return area, nil
}

// Prepare durable counter/battle/blackout effects without publishing or mutating
// the active battle registry. A movement caller supplies its owning transaction.
func (m *WildEncounterManager) prepareWildStepIn(ctx context.Context, tx db.DBTX, charID int64, x, y, mapID int) (wildStepResult, error) {
	var result wildStepResult
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		return result, nil
	}
	area, err := m.encounterAreaIn(tx, mapID, x, y)
	if err != nil || area == nil {
		return result, err
	}
	repel, wore, err := advanceRepelStepIn(tx, charID)
	if err != nil {
		return result, err
	}
	result.RepelWoreOff = wore
	if m.encounterRoll(256) >= area.EncounterRate {
		return result, nil
	}
	pokemonID, level := m.selectEncounterPokemon(area)
	party, err := pokebattle.LoadParty(tx, charID)
	if err != nil {
		return wildStepResult{}, err
	}
	if repel.Active {
		for _, p := range party {
			if p.CurHP > 0 {
				if level < p.Level {
					return result, nil
				}
				break
			}
		}
	}
	wild, err := pokebattle.BuildWildPokemon(tx, pokemonID, level)
	if err != nil {
		return wildStepResult{}, err
	}
	if len(party) == 0 || party[pokebattle.FirstAlivePartyIndex(party)].IsFainted() {
		blackout, healed, err := CommitStandaloneBlackout(ctx, tx, charID)
		if err != nil {
			return wildStepResult{}, err
		}
		result.Blackout = &blackout
		result.Party = healed
		return result, nil
	}
	battle := pokebattle.NewWildBattle(party, wild)
	result.Battle, err = pokebattle.StartBattleInTransaction(tx, charID, battle, func(q db.DBTX, next *pokebattle.BattleState) error {
		if err := configureBattleObedienceFromDB(q, next, charID); err != nil {
			return err
		}
		return markPokemonSeen(q, charID, next.GetEnemyPokemon().ID)
	})
	if err != nil {
		return wildStepResult{}, err
	}
	return result, nil
}

func (m *WildEncounterManager) publishWildStep(ses *session.Session, charID int64, result wildStepResult) {
	if result.RepelWoreOff {
		ses.SendStreamJSON(map[string]interface{}{"message": "REPEL's effect wore off!"}, opcodes.RepelWoreOffNotify)
	}
	if result.Battle != nil {
		setBattle(charID, result.Battle)
		ses.SendStreamJSON(buildBattleStateResponse(result.Battle), opcodes.PokeBattleStartResponse)
	}
	if result.Blackout != nil {
		publishStandaloneBlackout(ses, m.wh, charID, *result.Blackout, result.Party)
	}
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

func (m *WildEncounterManager) RepelStatus(ctx context.Context, charID int64) (RepelStatus, error) {
	return loadRepelStatus(ctx, m.database, charID)
}

func (m *WildEncounterManager) SetRepelSteps(ctx context.Context, charID int64, stepsLeft int) error {
	return setRepelSteps(ctx, m.database, charID, stepsLeft)
}

func (m *WildEncounterManager) AdvanceRepelStep(ctx context.Context, charID int64) (bool, error) {
	_, wore, err := advanceRepelStep(ctx, m.database, charID)
	return wore, err
}

func (m *WildEncounterManager) tickRepel(charID int64, ses *session.Session) (RepelStatus, error) {
	status, wore, err := advanceRepelStep(ses.CommandContext(), m.database, charID)
	if err != nil {
		return RepelStatus{}, err
	}
	if wore {
		ses.SendStreamJSON(map[string]interface{}{"message": "REPEL's effect wore off!"}, opcodes.RepelWoreOffNotify)
	}
	return status, nil
}
