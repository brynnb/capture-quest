package world

import (
	"encoding/json"
	"log"
	"math/rand"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type PokeSurfingRequestPayload struct {
	TargetX   *int   `json:"targetX,omitempty"`
	TargetY   *int   `json:"targetY,omitempty"`
	MapID     *int   `json:"mapId,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// HandlePokeSurfing handles a surf request from the client.
// Surfing triggers water encounters using encounter_type = 'water' in
// phaser_wild_encounters. Each surf action has a Gen 1 style encounter
// rate check (rand(256) < encounterRate). If no encounter triggers,
// the player surfs safely.
func HandlePokeSurfing(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req PokeSurfingRequestPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   "Invalid SURF request.",
			}, opcodes.PokeSurfingResponse)
			return false
		}
	}

	charData := ses.Client.CharData()
	if charData == nil {
		return false
	}
	charID := int64(charData.ID)

	// Check if already in battle
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Already in a battle",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	mapID := int(charData.MapID)
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}
	playerX, playerY := int(charData.X), int(charData.Y)
	if wh != nil && wh.PlayerMovement != nil {
		if x, y, movementMapID, ok := wh.PlayerMovement.GetPosition(int(charData.ID)); ok {
			playerX, playerY, mapID = x, y, movementMapID
		}
	}

	myDB := db.GlobalWorldDB.DB
	mapName := ""
	var efm *EventFlagManager
	if wh != nil && wh.Cutscenes != nil {
		mapName = wh.Cutscenes.MapNameForID(mapID)
	}
	if wh != nil {
		efm = wh.EventFlags
	}

	permission := CanUseFieldMove(charID, "SURF", efm)
	if !permission.Allowed {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   permission.Message,
		}, opcodes.PokeSurfingResponse)
		return false
	}

	if SeafoamSurfBlocked(charID, mapName, playerX, playerY, efm) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "The current is much too fast!",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	if req.TargetX != nil && req.TargetY != nil {
		return handlePokeSurfingTarget(ses, wh, charID, mapID, playerX, playerY, req)
	}

	// Check if water encounters exist for this map
	encounterRate := pokebattle.GetEncounterRate(myDB, mapID, "water")
	if encounterRate == 0 {
		// No water encounters on this map — just surf safely
		ses.SendStreamJSON(map[string]interface{}{
			"success":   true,
			"encounter": false,
			"message":   "You're surfing!",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	// Gen 1 encounter rate check: rand(256) < encounterRate
	roll := rand.Intn(256)
	if roll >= encounterRate {
		// No encounter this surf step
		ses.SendStreamJSON(map[string]interface{}{
			"success":   true,
			"encounter": false,
		}, opcodes.PokeSurfingResponse)
		return false
	}

	// Select a water encounter
	pokemonID, level, err := pokebattle.SelectWildEncounter(myDB, mapID, "water")
	if err != nil {
		log.Printf("[Surfing] No water encounters for map %d: %v", mapID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success":   true,
			"encounter": false,
		}, opcodes.PokeSurfingResponse)
		return false
	}

	// Build the wild Pokémon
	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, pokemonID, level)
	if err != nil {
		log.Printf("[Surfing] Failed to build wild pokemon %d: %v", pokemonID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to create encounter",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	// Load player's party. Oak's starter script is the source of truth for the
	// first Pokémon.
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[Surfing] No party for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "No Pokémon in party",
		}, opcodes.PokeSurfingResponse)
		return false
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
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "All your Pokémon have fainted",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	// Create battle
	battle := pokebattle.NewWildBattle(playerParty, wildPokemon)
	configureBattleObedience(battle, charID, wh.EventFlags)
	setBattle(charID, battle)

	log.Printf("[Surfing] %s encountered L%d %s while surfing on map %d",
		charData.Name, level, wildPokemon.Name, mapID)

	// Send surfing encounter response
	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"encounter": true,
		"message":   "A wild Pokémon appeared!",
	}, opcodes.PokeSurfingResponse)

	// Then send the battle start
	resp := buildBattleStateResponse(battle)
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)

	return false
}

func handlePokeSurfingTarget(
	ses *session.Session,
	wh *WorldHandler,
	charID int64,
	currentMapID int,
	playerX int,
	playerY int,
	req PokeSurfingRequestPayload,
) bool {
	if wh == nil || wh.ActorManager == nil || wh.PlayerMovement == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	targetMapID := currentMapID
	if req.MapID != nil {
		targetMapID = *req.MapID
	}
	if wh.ActorManager.IsOverworld(targetMapID) {
		targetMapID = UnifiedOverworldMapID
	}
	if targetMapID != currentMapID {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	targetX, targetY := *req.TargetX, *req.TargetY
	if abs(targetX-playerX)+abs(targetY-playerY) != 1 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You need to be next to the water.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	if !isSurfableWaterTile(wh, targetMapID, targetX, targetY) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	direction := normalizeWarpDirection(req.Direction)
	if direction == "" {
		direction = directionFromAdjacentTiles(playerX, playerY, targetX, targetY)
	}

	charIDInt := int(charID)
	if _, _, _, ok := wh.PlayerMovement.GetPosition(charIDInt); !ok {
		wh.PlayerMovement.RegisterPlayer(ses, charIDInt, playerX, playerY, currentMapID, direction)
	}
	if !wh.PlayerMovement.MovePlayerTo(charIDInt, targetX, targetY, targetMapID, direction, true) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	encounter := false
	if wh.WildEncounter != nil {
		encounter = wh.WildEncounter.CheckPlayerStep(charID, targetX, targetY, targetMapID, ses)
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"encounter": encounter,
		"message":   "You're surfing!",
		"mapId":     targetMapID,
		"x":         targetX,
		"y":         targetY,
		"direction": direction,
	}, opcodes.PokeSurfingResponse)
	return false
}

func directionFromAdjacentTiles(fromX, fromY, toX, toY int) string {
	switch {
	case toY < fromY:
		return "UP"
	case toY > fromY:
		return "DOWN"
	case toX < fromX:
		return "LEFT"
	case toX > fromX:
		return "RIGHT"
	default:
		return "DOWN"
	}
}
