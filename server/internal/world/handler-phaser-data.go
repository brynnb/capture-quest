package world

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// --- Request types ---

type PhaserDialogueRequest struct {
	TextConstant string `json:"textConstant"` // e.g. "TEXT_PALLETTOWN_FISHER"
}

type PhaserWildEncountersRequest struct {
	MapID int `json:"mapId"`
}

type PhaserTrainerDataRequest struct {
	TrainerClass      string `json:"trainerClass"`      // e.g. "BUG_CATCHER"
	TrainerPartyIndex int    `json:"trainerPartyIndex"` // party index within the class
}

type PhaserHiddenObjectsRequest struct {
	MapID int `json:"mapId"`
}

// --- Response types ---

type PhaserDialogueEntry struct {
	Label      string  `json:"label"`
	SourceFile string  `json:"sourceFile"`
	Dialogue   string  `json:"dialogue"`
	IsTrainer  int     `json:"isTrainer"`
	MapName    *string `json:"mapName"`
}

type PhaserWildEncounter struct {
	ID            int    `json:"id"`
	MapName       string `json:"mapName"`
	EncounterType string `json:"encounterType"`
	EncounterRate int    `json:"encounterRate"`
	SlotIndex     int    `json:"slotIndex"`
	PokemonName   string `json:"pokemonName"`
	Level         int    `json:"level"`
	Version       string `json:"version"`
}

type PhaserEncounterSlot struct {
	SlotIndex             int     `json:"slotIndex"`
	Probability           float64 `json:"probability"`
	CumulativeProbability float64 `json:"cumulativeProbability"`
}

type PhaserTrainerClass struct {
	ID           int    `json:"id"`
	ConstantName string `json:"constantName"`
	DisplayName  string `json:"displayName"`
	BaseMoney    int    `json:"baseMoney"`
	IsGymLeader  int    `json:"isGymLeader"`
	IsEliteFour  int    `json:"isEliteFour"`
	IsRival      int    `json:"isRival"`
}

type PhaserTrainerPartyPokemon struct {
	SlotIndex   int    `json:"slotIndex"`
	PokemonName string `json:"pokemonName"`
	Level       int    `json:"level"`
}

type PhaserTrainerHeader struct {
	EventFlag            *string `json:"eventFlag"`
	SightRange           *int    `json:"sightRange"`
	BattleTextLabel      *string `json:"battleTextLabel"`
	EndBattleTextLabel   *string `json:"endBattleTextLabel"`
	AfterBattleTextLabel *string `json:"afterBattleTextLabel"`
}

type PhaserTrainerDataResponse struct {
	Class   PhaserTrainerClass          `json:"class"`
	Party   []PhaserTrainerPartyPokemon `json:"party"`
	Header  *PhaserTrainerHeader        `json:"header"`
	Success bool                        `json:"success"`
}

type PhaserHiddenItem struct {
	ID          int    `json:"id"`
	MapConstant string `json:"mapConstant"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
}

type PhaserHiddenObject struct {
	ID              int     `json:"id"`
	MapConstant     string  `json:"mapConstant"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	ItemOrDirection *string `json:"itemOrDirection"`
	Routine         *string `json:"routine"`
	ObjectType      *string `json:"objectType"`
}

// --- Handlers ---

// HandlePhaserDialogueRequest resolves a text constant to dialogue text
func HandlePhaserDialogueRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserDialogueRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid DialogueRequest: %v", err)
		return false
	}

	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}
	var efm *EventFlagManager
	if wh != nil {
		efm = wh.EventFlags
	}
	entries, err := resolvePhaserDialogueEntries(req.TextConstant, charID, efm)
	if err != nil {
		log.Printf("[Phaser] Error querying dialogue for %s: %v", req.TextConstant, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserDialogueResponse)
		return false
	}

	res := map[string]interface{}{
		"success":         true,
		"textConstant":    req.TextConstant,
		"dialogueEntries": StructToMap(entries),
	}

	// Check for branching dialogue (YES/NO choices) with event flag gating.
	if bd := branchingDialogueForResponse(req.TextConstant, charID, wh); bd != nil {
		res["hasBranching"] = true
		res["branchingPrompt"] = bd.PromptText
	}

	ses.SendStreamJSON(res, opcodes.PhaserDialogueResponse)
	log.Printf("[Phaser] Sent %d dialogue entries for %s", len(entries), req.TextConstant)
	return false
}

func resolvePhaserDialogueEntries(textConstant string, charID int64, efm *EventFlagManager) ([]PhaserDialogueEntry, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT dt.label, dt.source_file, dt.dialogue, tp.is_trainer, tp.map_name
		FROM phaser_text_pointers tp
		LEFT JOIN phaser_dialogue_text dt ON dt.label = tp.dialogue_label
		WHERE tp.text_constant = $1`, textConstant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []PhaserDialogueEntry
	for rows.Next() {
		var label, sourceFile, dialogue, mapName sql.NullString
		var isTrainer sql.NullInt64
		if err := rows.Scan(&label, &sourceFile, &dialogue, &isTrainer, &mapName); err != nil {
			log.Printf("[Phaser] Error scanning dialogue: %v", err)
			continue
		}
		if !label.Valid || !sourceFile.Valid || !dialogue.Valid {
			continue
		}
		e := PhaserDialogueEntry{
			Label:      label.String,
			SourceFile: sourceFile.String,
			Dialogue:   dialogue.String,
			IsTrainer:  int(isTrainer.Int64),
		}
		if mapName.Valid {
			mapNameString := mapName.String
			e.MapName = &mapNameString
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		entries = resolveScriptDialogueFallbackEntries(textConstant, charID, efm)
	}

	if charID > 0 && efm != nil {
		override, err := checkConditionalDialogue(textConstant, charID, efm)
		if err != nil {
			return nil, err
		}
		if override != nil {
			entries = []PhaserDialogueEntry{{
				Label:    override.label,
				Dialogue: override.dialogue,
			}}
		}
	}
	return entries, nil
}

func branchingDialogueForResponse(textConstant string, charID int64, wh *WorldHandler) *BranchingDialogue {
	if bd := checkInGameTradeBranchingDialogue(textConstant, charID); bd != nil {
		return bd
	}

	var (
		cutscenes *CutsceneManager
		efm       *EventFlagManager
	)
	if wh != nil {
		cutscenes = wh.Cutscenes
		efm = wh.EventFlags
	}
	if cutscenes != nil && cutscenes.HasClickCutsceneForTriggerLabel(textConstant) {
		return nil
	}
	return CheckForBranchingDialogueWithFlags(textConstant, charID, efm)
}

// HandlePhaserWildEncountersRequest returns wild encounters + slot probabilities for a map
func HandlePhaserWildEncountersRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserWildEncountersRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid WildEncountersRequest: %v", err)
		return false
	}

	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, map_name, encounter_type, encounter_rate, slot_index, pokemon_name, level, version
		FROM phaser_wild_encounters WHERE map_id = $1`, req.MapID)
	if err != nil {
		log.Printf("[Phaser] Error querying wild encounters for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserWildEncountersResponse)
		return false
	}
	defer rows.Close()

	var encounters []PhaserWildEncounter
	for rows.Next() {
		var e PhaserWildEncounter
		if err := rows.Scan(&e.ID, &e.MapName, &e.EncounterType, &e.EncounterRate, &e.SlotIndex, &e.PokemonName, &e.Level, &e.Version); err != nil {
			log.Printf("[Phaser] Error scanning encounter: %v", err)
			continue
		}
		encounters = append(encounters, e)
	}

	// Also fetch encounter slot probabilities
	slotRows, err := db.GlobalWorldDB.DB.Query(`SELECT slot_index, probability, cumulative_probability FROM phaser_encounter_slots ORDER BY slot_index`)
	if err != nil {
		log.Printf("[Phaser] Error querying encounter slots: %v", err)
	}
	var slots []PhaserEncounterSlot
	if slotRows != nil {
		defer slotRows.Close()
		for slotRows.Next() {
			var s PhaserEncounterSlot
			if err := slotRows.Scan(&s.SlotIndex, &s.Probability, &s.CumulativeProbability); err != nil {
				continue
			}
			slots = append(slots, s)
		}
	}

	res := map[string]interface{}{
		"success":    true,
		"mapId":      req.MapID,
		"encounters": StructToMap(encounters),
		"slots":      StructToMap(slots),
	}
	ses.SendStreamJSON(res, opcodes.PhaserWildEncountersResponse)
	log.Printf("[Phaser] Sent %d wild encounters for map %d", len(encounters), req.MapID)
	return false
}

// HandlePhaserTrainerDataRequest returns trainer class, party, and header for a trainer NPC
func HandlePhaserTrainerDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserTrainerDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid TrainerDataRequest: %v", err)
		return false
	}

	// Get trainer class
	var tc PhaserTrainerClass
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, constant_name, display_name, base_money, is_gym_leader, is_elite_four, is_rival
		FROM phaser_trainer_classes WHERE constant_name = $1`, req.TrainerClass).Scan(
		&tc.ID, &tc.ConstantName, &tc.DisplayName, &tc.BaseMoney, &tc.IsGymLeader, &tc.IsEliteFour, &tc.IsRival)
	if err != nil {
		log.Printf("[Phaser] Trainer class not found: %s: %v", req.TrainerClass, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "trainer class not found"}, opcodes.PhaserTrainerDataResponse)
		return false
	}

	// Get party Pokémon
	partyRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT tpp.slot_index, tpp.pokemon_name, tpp.level
		FROM phaser_trainer_party_pokemon tpp
		JOIN phaser_trainer_parties tp ON tpp.trainer_party_id = tp.id
		WHERE tp.trainer_class_id = $1 AND tp.party_index = $2
		ORDER BY tpp.slot_index`, tc.ID, req.TrainerPartyIndex)
	if err != nil {
		log.Printf("[Phaser] Error querying trainer party: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserTrainerDataResponse)
		return false
	}
	defer partyRows.Close()

	var party []PhaserTrainerPartyPokemon
	for partyRows.Next() {
		var p PhaserTrainerPartyPokemon
		if err := partyRows.Scan(&p.SlotIndex, &p.PokemonName, &p.Level); err != nil {
			continue
		}
		party = append(party, p)
	}

	// Get trainer header (optional — may not exist for all trainer/party combos)
	var header *PhaserTrainerHeader
	headerRow := db.GlobalWorldDB.DB.QueryRow(`
		SELECT event_flag, sight_range, battle_text_label, end_battle_text_label, after_battle_text_label
		FROM phaser_trainer_headers
		WHERE header_label LIKE CONCAT('%', $1, '%')
		LIMIT 1`, req.TrainerClass)
	var th PhaserTrainerHeader
	if err := headerRow.Scan(&th.EventFlag, &th.SightRange, &th.BattleTextLabel, &th.EndBattleTextLabel, &th.AfterBattleTextLabel); err == nil {
		header = &th
	}

	resp := PhaserTrainerDataResponse{
		Class:   tc,
		Party:   party,
		Header:  header,
		Success: true,
	}
	ses.SendStreamJSON(StructToMap(resp), opcodes.PhaserTrainerDataResponse)
	log.Printf("[Phaser] Sent trainer data for %s party %d (%d Pokémon)", req.TrainerClass, req.TrainerPartyIndex, len(party))
	return false
}

// HandlePhaserPokemonDataRequest returns full Pokémon data by ID
func HandlePhaserPokemonDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserPokemonDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid PokemonDataRequest: %v", err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "invalid pokemon data request"}, opcodes.PhaserPokemonDataResponse)
		return false
	}

	p, err := wh.Content.Pokemon(ses.CommandContext(), req.PokemonID)
	if err != nil {
		log.Printf("[Phaser] Pokémon query failed: %d: %v", req.PokemonID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: contentQueryError("pokemon", err)}, opcodes.PhaserPokemonDataResponse)
		return false
	}

	ses.SendStreamJSON(protocol.PhaserPokemonDataResponse{PhaserPokemonFull: p, Success: true}, opcodes.PhaserPokemonDataResponse)
	log.Printf("[Phaser] Sent Pokémon data for #%d %s", p.ID, p.Name)
	return false
}

// HandlePhaserMoveDataRequest returns move data by ID
func HandlePhaserMoveDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserMoveDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid MoveDataRequest: %v", err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "invalid move data request"}, opcodes.PhaserMoveDataResponse)
		return false
	}

	m, err := wh.Content.Move(ses.CommandContext(), req.MoveID)
	if err != nil {
		log.Printf("[Phaser] Move query failed: %d: %v", req.MoveID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: contentQueryError("move", err)}, opcodes.PhaserMoveDataResponse)
		return false
	}

	ses.SendStreamJSON(protocol.PhaserMoveDataResponse{PhaserMoveFull: m, Success: true}, opcodes.PhaserMoveDataResponse)
	log.Printf("[Phaser] Sent move data for #%d %s", m.ID, m.Name)
	return false
}

// HandlePhaserMapScriptsRequest returns map scripts, event flags, coordinate triggers, and NPC movements for a map
func HandlePhaserMapScriptsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserMapScriptsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "invalid map scripts request"}, opcodes.PhaserMapScriptsResponse)
		return false
	}
	res, err := wh.Content.MapScripts(ses.CommandContext(), req.MapName)
	if err != nil {
		log.Printf("[Phaser] Map script query failed for %s: %v", req.MapName, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "failed to load map scripts"}, opcodes.PhaserMapScriptsResponse)
		return false
	}
	if err := ses.SendStreamJSON(res, opcodes.PhaserMapScriptsResponse); err != nil {
		return false
	}

	if ses.HasValidClient() && wh.Cutscenes != nil && wh.EventFlags != nil {
		mapName, err := wh.nativeScriptMap(ses)
		if err != nil {
			log.Printf("[Cutscene] Resolve map-ready location: %v", err)
			return false
		}
		if mapName != req.MapName {
			return false
		}
		charID := int64(ses.Client.CharData().ID)
		if battle := getBattle(charID); battle != nil && (!battle.IsOver() || battle.PendingMoveLearn != nil) {
			return false
		}
		if wh.TrainerEncounter != nil {
			resumed, err := wh.TrainerEncounter.resumePendingEncounter(ses, wh)
			if err != nil {
				log.Printf("[TrainerEncounter] Resume failed for %d: %v", charID, err)
				return false
			}
			if resumed {
				return false
			}
		}
		resumed, err := resumePendingCutscene(ses, wh)
		if err != nil {
			log.Printf("[Cutscene] Resume character %d: %v", charID, err)
			return false
		}
		if resumed {
			return false
		}
		playerFacing := ""
		if wh.PlayerMovement != nil {
			playerFacing, _ = wh.PlayerMovement.GetDirection(int(charID))
		}
		if cs := wh.Cutscenes.FindEligibleMapScriptCutscene(mapName, charID, wh.EventFlags, playerFacing); cs != nil {
			SendCutsceneToPlayer(ses, cs, wh)
		}
	}
	return false
}

// HandlePhaserLearnsetRequest returns learnset + TM/HM compatibility for a Pokémon
func HandlePhaserLearnsetRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserLearnsetRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "invalid learnset request"}, opcodes.PhaserLearnsetResponse)
		return false
	}
	res, err := wh.Content.Learnset(ses.CommandContext(), req.PokemonID)
	if err != nil {
		log.Printf("[Phaser] Learnset query failed for pokemon %d: %v", req.PokemonID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "failed to load learnset"}, opcodes.PhaserLearnsetResponse)
		return false
	}
	ses.SendStreamJSON(res, opcodes.PhaserLearnsetResponse)
	return false
}

// HandlePhaserItemDataRequest returns item data by ID
func HandlePhaserItemDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserItemDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid ItemDataRequest: %v", err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "invalid item data request"}, opcodes.PhaserItemDataResponse)
		return false
	}

	item, err := wh.Content.Item(ses.CommandContext(), req.ItemID)
	if err != nil {
		log.Printf("[Phaser] Item query failed: %d: %v", req.ItemID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: contentQueryError("item", err)}, opcodes.PhaserItemDataResponse)
		return false
	}

	ses.SendStreamJSON(protocol.PhaserItemDataResponse{PhaserItemFull: item, Success: true}, opcodes.PhaserItemDataResponse)
	log.Printf("[Phaser] Sent item data for #%d %s", item.ID, item.Name)
	return false
}

// HandlePhaserHiddenObjectsRequest returns hidden items, coins, and objects for a map
func HandlePhaserHiddenObjectsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserHiddenObjectsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid HiddenObjectsRequest: %v", err)
		return false
	}

	// Hidden items
	itemRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, map_constant, x, y FROM phaser_hidden_items WHERE map_id = $1`, req.MapID)
	if err != nil {
		log.Printf("[Phaser] Error querying hidden items for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserHiddenObjectsResponse)
		return false
	}
	defer itemRows.Close()

	var hiddenItems []PhaserHiddenItem
	for itemRows.Next() {
		var h PhaserHiddenItem
		if err := itemRows.Scan(&h.ID, &h.MapConstant, &h.X, &h.Y); err != nil {
			continue
		}
		hiddenItems = append(hiddenItems, h)
	}

	// Hidden coins
	coinRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, map_constant, x, y FROM phaser_hidden_coins WHERE map_id = $1`, req.MapID)
	if err != nil {
		log.Printf("[Phaser] Error querying hidden coins for map %d: %v", req.MapID, err)
	}
	var hiddenCoins []PhaserHiddenItem
	if coinRows != nil {
		defer coinRows.Close()
		for coinRows.Next() {
			var h PhaserHiddenItem
			if err := coinRows.Scan(&h.ID, &h.MapConstant, &h.X, &h.Y); err != nil {
				continue
			}
			hiddenCoins = append(hiddenCoins, h)
		}
	}

	// Hidden objects
	objRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, map_constant, x, y, item_or_direction, routine, object_type
		FROM phaser_hidden_objects WHERE map_id = $1`, req.MapID)
	if err != nil {
		log.Printf("[Phaser] Error querying hidden objects for map %d: %v", req.MapID, err)
	}
	var hiddenObjects []PhaserHiddenObject
	if objRows != nil {
		defer objRows.Close()
		for objRows.Next() {
			var h PhaserHiddenObject
			if err := objRows.Scan(&h.ID, &h.MapConstant, &h.X, &h.Y, &h.ItemOrDirection, &h.Routine, &h.ObjectType); err != nil {
				continue
			}
			hiddenObjects = append(hiddenObjects, h)
		}
	}

	res := map[string]interface{}{
		"success":       true,
		"mapId":         req.MapID,
		"hiddenItems":   StructToMap(hiddenItems),
		"hiddenCoins":   StructToMap(hiddenCoins),
		"hiddenObjects": StructToMap(hiddenObjects),
	}
	ses.SendStreamJSON(res, opcodes.PhaserHiddenObjectsResponse)
	log.Printf("[Phaser] Sent hidden objects for map %d (%d items, %d coins, %d objects)",
		req.MapID, len(hiddenItems), len(hiddenCoins), len(hiddenObjects))
	return false
}

// This opcode remains reserved for stale clients. Runtime music comes from the
// canonical generated browser manifest; never restore a second database path.
func HandlePhaserMapMusicRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	ses.SendStreamJSON(protocol.ErrorResponse{Error: "Map music query is no longer supported; reload the client."}, opcodes.PhaserMapMusicResponse)
	return false
}

func contentQueryError(kind string, err error) string {
	if errors.Is(err, sql.ErrNoRows) {
		return kind + " not found"
	}
	return "failed to load " + kind + " data"
}
