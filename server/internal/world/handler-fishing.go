package world

import (
	"encoding/json"
	"log"
	"math/rand"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type PokeFishingRequestPayload struct {
	ItemID    int32  `json:"itemId"` // Fallback rod item ID: 76=Old Rod, 77=Good Rod, 78=Super Rod
	RodType   string `json:"rodType,omitempty"`
	MapID     *int   `json:"mapId,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// HandlePokeFishing handles a fishing rod use request from the client.
// The client sends the rod item ID. The server checks if the player is facing
// water, selects an encounter, and starts a wild battle.
func HandlePokeFishing(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req PokeFishingRequestPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Fishing] Failed to unmarshal fishing request: %v", err)
		return false
	}

	charID := int64(ses.Client.CharData().ID)

	// Check if already in battle
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Already in a battle",
		}, opcodes.PokeFishingResponse)
		return false
	}

	// Determine rod type
	rodType := fishingRodType(req)
	if rodType == "" {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "That's not a fishing rod",
		}, opcodes.PokeFishingResponse)
		return false
	}

	charData := ses.Client.CharData()
	mapID, playerX, playerY := fishingPlayerPosition(ses, wh, req)
	direction := normalizeWarpDirection(req.Direction)
	if direction == "" && wh != nil && wh.PlayerMovement != nil {
		if currentDirection, ok := wh.PlayerMovement.GetDirection(int(charData.ID)); ok {
			direction = normalizeWarpDirection(currentDirection)
		}
	}
	if direction == "" {
		direction = directionFromCharacterHeading(charData.Heading)
	}
	if !isFacingFishableWater(wh, mapID, playerX, playerY, direction) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't fish here.",
		}, opcodes.PokeFishingResponse)
		return false
	}

	// Gen 1 fishing: 50% chance of "nothing bites" for Good Rod and Super Rod
	// Old Rod always hooks something
	if rodType != "old_rod" && rand.Intn(2) == 0 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": true,
			"hooked":  false,
			"message": "Not even a nibble!",
		}, opcodes.PokeFishingResponse)
		return false
	}

	myDB := db.GlobalWorldDB.DB

	// Select fishing encounter
	pokemonID, level, err := pokebattle.SelectFishingEncounter(myDB, mapID, rodType)
	if err != nil {
		log.Printf("[Fishing] No fishing encounters for map %d rod %s: %v", mapID, rodType, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": true,
			"hooked":  false,
			"message": "Not even a nibble!",
		}, opcodes.PokeFishingResponse)
		return false
	}

	// Build the wild Pokémon
	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, pokemonID, level)
	if err != nil {
		log.Printf("[Fishing] Failed to build wild pokemon %d: %v", pokemonID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to create encounter",
		}, opcodes.PokeFishingResponse)
		return false
	}

	// Load player's party. Oak's starter script is the source of truth for the
	// first Pokémon.
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[Fishing] No party for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "No Pokémon in party",
		}, opcodes.PokeFishingResponse)
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
		}, opcodes.PokeFishingResponse)
		return false
	}

	// Create battle
	battle := pokebattle.NewWildBattle(playerParty, wildPokemon)
	configureBattleObedience(battle, charID, wh.EventFlags)
	setBattle(charID, battle)

	log.Printf("[Fishing] %s hooked L%d %s with %s on map %d",
		charData.Name, level, wildPokemon.Name, rodType, mapID)

	// Send fishing success response (client shows "Oh! A bite!" then opens battle)
	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"hooked":  true,
		"message": "Oh! A bite!",
	}, opcodes.PokeFishingResponse)

	// Then send the battle start
	resp := buildBattleStateResponse(battle)
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)

	return false
}

func fishingRodType(req PokeFishingRequestPayload) string {
	switch strings.ToUpper(strings.TrimSpace(req.RodType)) {
	case "OLD_ROD", "OLD ROD", "OLD":
		return "old_rod"
	case "GOOD_ROD", "GOOD ROD", "GOOD":
		return "good_rod"
	case "SUPER_ROD", "SUPER ROD", "SUPER":
		return "super_rod"
	}
	switch req.ItemID {
	case 76:
		return "old_rod"
	case 77:
		return "good_rod"
	case 78:
		return "super_rod"
	default:
		return ""
	}
}

func fishingPlayerPosition(ses *session.Session, wh *WorldHandler, req PokeFishingRequestPayload) (mapID, x, y int) {
	charData := ses.Client.CharData()
	mapID = int(charData.MapID)
	x, y = int(charData.X), int(charData.Y)

	if req.MapID != nil {
		mapID = *req.MapID
	}
	if wh != nil && wh.PlayerMovement != nil {
		if currentX, currentY, currentMapID, ok := wh.PlayerMovement.GetPosition(int(charData.ID)); ok {
			x, y, mapID = currentX, currentY, currentMapID
		}
	}
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}
	return mapID, x, y
}

func isFacingFishableWater(wh *WorldHandler, mapID, playerX, playerY int, direction string) bool {
	if wh == nil || wh.ActorManager == nil {
		return false
	}
	targetX, targetY, ok := fishingTargetTile(playerX, playerY, direction)
	if !ok {
		return false
	}
	collisionType, exists := wh.ActorManager.CollisionTypeAt(mapID, targetX, targetY)
	return exists && collisionType == collisionWater
}

func fishingTargetTile(playerX, playerY int, direction string) (int, int, bool) {
	switch normalizeWarpDirection(direction) {
	case "UP":
		return playerX, playerY - 1, true
	case "DOWN":
		return playerX, playerY + 1, true
	case "LEFT":
		return playerX - 1, playerY, true
	case "RIGHT":
		return playerX + 1, playerY, true
	default:
		return playerX, playerY, false
	}
}

func directionFromCharacterHeading(heading float64) string {
	normalized := int(heading) % 360
	if normalized < 0 {
		normalized += 360
	}
	switch normalized {
	case 90:
		return "RIGHT"
	case 180:
		return "DOWN"
	case 270:
		return "LEFT"
	default:
		return "UP"
	}
}
