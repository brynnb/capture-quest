package world

import (
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

const (
	pcBoxCount = 12 // Gen 1: 12 boxes
	pcBoxSize  = 20 // Gen 1: 20 Pokémon per box
)

// HandlePokemonPCOpen opens the PC and sends the current box contents + party.
func HandlePokemonPCOpen(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	// Get current box
	currentBox := 0
	myDB.QueryRow(`SELECT current_box FROM character_pc_state WHERE character_id = $1`, charID).Scan(&currentBox)

	// Load box contents
	boxPokemon, err := pokebattle.LoadBox(myDB, charID, currentBox)
	if err != nil {
		log.Printf("[PC] Failed to load box %d for char %d: %v", currentBox, charID, err)
	}

	// Load party
	party, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		log.Printf("[PC] Failed to load party for char %d: %v", charID, err)
	}

	boxDTOs := make([]PokemonDTO, 0, len(boxPokemon))
	for _, p := range boxPokemon {
		boxDTOs = append(boxDTOs, pokemonToDTO(p))
	}
	partyDTOs := make([]PokemonDTO, 0, len(party))
	for _, p := range party {
		partyDTOs = append(partyDTOs, pokemonToDTO(p))
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"currentBox": currentBox,
		"boxCount":   pcBoxCount,
		"boxSize":    pcBoxSize,
		"box":        boxDTOs,
		"party":      partyDTOs,
	}, opcodes.PokemonPCOpenResponse)

	return false
}

// HandlePokemonPCDeposit deposits a party Pokémon into the current PC box.
func HandlePokemonPCDeposit(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		PartySlot int `json:"partySlot"`
		Box       int `json:"box"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.PokemonPCDepositResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	// Must keep at least 1 Pokémon in party
	var partySize int
	myDB.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = $1 AND box = -1`, charID).Scan(&partySize)
	if partySize <= 1 {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Can't deposit your last Pokémon!"}, opcodes.PokemonPCDepositResponse)
		return false
	}

	if req.Box < 0 || req.Box >= pcBoxCount {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Invalid box"}, opcodes.PokemonPCDepositResponse)
		return false
	}

	boxSlot, err := pokebattle.DepositToPC(myDB, charID, req.PartySlot, req.Box)
	if err != nil {
		log.Printf("[PC] Deposit failed for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PokemonPCDepositResponse)
		return false
	}

	log.Printf("[PC] Char %d deposited party slot %d to box %d slot %d", charID, req.PartySlot, req.Box, boxSlot)

	// Send updated box + party
	sendPCUpdate(ses, charID, req.Box)

	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"boxSlot": boxSlot,
	}, opcodes.PokemonPCDepositResponse)

	return false
}

// HandlePokemonPCWithdraw withdraws a PC Pokémon to the party.
func HandlePokemonPCWithdraw(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		Box     int `json:"box"`
		BoxSlot int `json:"boxSlot"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.PokemonPCWithdrawResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	partySlot, err := pokebattle.WithdrawFromPC(myDB, charID, req.Box, req.BoxSlot)
	if err != nil {
		log.Printf("[PC] Withdraw failed for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PokemonPCWithdrawResponse)
		return false
	}

	log.Printf("[PC] Char %d withdrew box %d slot %d to party slot %d", charID, req.Box, req.BoxSlot, partySlot)

	// Send updated box + party
	sendPCUpdate(ses, charID, req.Box)

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"partySlot": partySlot,
	}, opcodes.PokemonPCWithdrawResponse)

	return false
}

// HandlePokemonPCRelease releases a Pokémon from PC storage permanently.
func HandlePokemonPCRelease(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		Box     int `json:"box"`
		BoxSlot int `json:"boxSlot"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.PokemonPCReleaseResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	err := pokebattle.ReleasePokemon(myDB, charID, req.Box, req.BoxSlot)
	if err != nil {
		log.Printf("[PC] Release failed for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PokemonPCReleaseResponse)
		return false
	}

	log.Printf("[PC] Char %d released Pokémon from box %d slot %d", charID, req.Box, req.BoxSlot)

	// Send updated box
	sendPCUpdate(ses, charID, req.Box)

	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
	}, opcodes.PokemonPCReleaseResponse)

	return false
}

// HandlePokemonPCSwitchBox switches to a different PC box and sends its contents.
func HandlePokemonPCSwitchBox(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		Box int `json:"box"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.PokemonPCSwitchBoxResponse)
		return false
	}

	if req.Box < 0 || req.Box >= pcBoxCount {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Invalid box"}, opcodes.PokemonPCSwitchBoxResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	// Save current box preference
	myDB.Exec(`INSERT INTO character_pc_state (character_id, current_box) VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET current_box = EXCLUDED.current_box`, charID, req.Box)

	// Load box contents
	boxPokemon, _ := pokebattle.LoadBox(myDB, charID, req.Box)
	boxDTOs := make([]PokemonDTO, 0, len(boxPokemon))
	for _, p := range boxPokemon {
		boxDTOs = append(boxDTOs, pokemonToDTO(p))
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"currentBox": req.Box,
		"box":        boxDTOs,
	}, opcodes.PokemonPCSwitchBoxResponse)

	return false
}

// sendPCUpdate sends updated box contents and party to the client after a PC operation.
func sendPCUpdate(ses *session.Session, charID int64, box int) {
	myDB := db.GlobalWorldDB.DB

	boxPokemon, _ := pokebattle.LoadBox(myDB, charID, box)
	boxDTOs := make([]PokemonDTO, 0, len(boxPokemon))
	for _, p := range boxPokemon {
		boxDTOs = append(boxDTOs, pokemonToDTO(p))
	}

	party, _ := pokebattle.LoadParty(myDB, charID)
	partyDTOs := make([]PokemonDTO, 0, len(party))
	for _, p := range party {
		partyDTOs = append(partyDTOs, pokemonToDTO(p))
	}

	// Push updated party to the party store
	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"party":   partyDTOs,
	}, opcodes.PokemonPartyResponse)

	// Push updated box + party to the PC store via switch box response
	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"currentBox": box,
		"box":        boxDTOs,
		"party":      partyDTOs,
	}, opcodes.PokemonPCSwitchBoxResponse)
}
