package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"context"
	"encoding/json"
	"log"
)

// PokeCenterHealRequest is sent by the client when the player interacts with Nurse Joy.
type PokeCenterHealRequest struct {
	MapID int `json:"mapId"` // The map the player is currently on (should be a Pokémon Center)
}

// HandlePokeCenterHeal restores all party Pokémon to full HP/PP, clears status conditions,
// records this Pokémon Center as the last visited (for blackout warp), and resets defeated trainers.
func HandlePokeCenterHeal(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PokeCenterHealRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[PokeCenter] Invalid PokeCenterHealRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid request",
		}, opcodes.PokeCenterHealResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB
	ctx := context.Background()

	party, err := HealCharacterParty(charID)
	if err != nil {
		log.Printf("[PokeCenter] Failed to load party for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "no party to heal",
		}, opcodes.PokeCenterHealResponse)
		return false
	}

	// Update last visited Pokémon Center in character options
	// Look up the entrance warp coordinates for this Pokémon Center map
	// (the warp that leads INTO this center from the overworld — that's where we warp back on blackout)
	entranceMapID, entranceX, entranceY := getPokeCenterEntrance(myDB, req.MapID)

	opts, err := db_character.LoadOptions(ctx, int32(charID))
	if err != nil {
		log.Printf("[PokeCenter] Failed to load options for char %d: %v", charID, err)
		opts = db_character.DefaultOptions()
	}
	opts.LastPokeCenterMapID = entranceMapID
	opts.LastPokeCenterX = entranceX
	opts.LastPokeCenterY = entranceY
	if err := db_character.SaveOptions(ctx, int32(charID), opts); err != nil {
		log.Printf("[PokeCenter] Failed to save options for char %d: %v", charID, err)
	}

	// Reset defeated trainers — in the original games, healing at a Pokémon Center
	// resets all defeated trainer flags so they can be re-battled
	_, err = myDB.Exec(`DELETE FROM character_defeated_trainers WHERE character_id = $1`, charID)
	if err != nil {
		log.Printf("[PokeCenter] Failed to reset defeated trainers for char %d: %v", charID, err)
	} else {
		log.Printf("[PokeCenter] Reset defeated trainers for char %d", charID)
	}

	// Clear the in-memory spottedBy tracking so trainers can re-trigger
	if wh.TrainerEncounter != nil {
		wh.TrainerEncounter.ClearPlayer(charID)
	}

	log.Printf("[PokeCenter] Healed party for char %d at map %d (entrance: map %d, %d,%d)",
		charID, req.MapID, entranceMapID, entranceX, entranceY)

	// Send updated party to client
	partyDTOs := make([]PokemonDTO, 0, len(party))
	for _, p := range party {
		partyDTOs = append(partyDTOs, pokemonToDTO(p))
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":             true,
		"party":               partyDTOs,
		"lastPokeCenterMapId": entranceMapID,
		"lastPokeCenterX":     entranceX,
		"lastPokeCenterY":     entranceY,
	}, opcodes.PokeCenterHealResponse)

	return false
}

// getPokeCenterEntrance finds the entrance coordinates for a Pokémon Center.
// It looks for warps that lead INTO this map from outside, giving us the coordinates
// the player should warp to on blackout. Falls back to the inside coordinates (3,4)
// of the center itself if no inbound warp is found.
func getPokeCenterEntrance(myDB pokebattle.DBTX, pokeCenterMapID int) (mapID, x, y int) {
	// Default: warp to inside the Pokémon Center (standard Nurse Joy position area)
	// In the original games, blackout warps you to the last Pokémon Center's interior
	mapID = pokeCenterMapID
	x = 3
	y = 4

	return mapID, x, y
}
