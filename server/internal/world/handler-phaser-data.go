package world

import (
	"database/sql"
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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

type PhaserPokemonDataRequest struct {
	PokemonID int `json:"pokemonId"`
}

type PhaserMoveDataRequest struct {
	MoveID int `json:"moveId"`
}

type PhaserMapScriptsRequest struct {
	MapName string `json:"mapName"` // CamelCase map name e.g. "PalletTown"
}

type PhaserLearnsetRequest struct {
	PokemonID int `json:"pokemonId"`
}

type PhaserItemDataRequest struct {
	ItemID int `json:"itemId"`
}

type PhaserHiddenObjectsRequest struct {
	MapID int `json:"mapId"`
}

type PhaserMapMusicRequest struct {
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

type PhaserPokemonFull struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	HP               int     `json:"hp"`
	Atk              int     `json:"atk"`
	Def              int     `json:"def"`
	Spd              int     `json:"spd"`
	Spc              int     `json:"spc"`
	Type1            string  `json:"type1"`
	Type2            *string `json:"type2"`
	CatchRate        int     `json:"catchRate"`
	BaseExp          int     `json:"baseExp"`
	DefaultMove1     *string `json:"defaultMove1Id"`
	DefaultMove2     *string `json:"defaultMove2Id"`
	DefaultMove3     *string `json:"defaultMove3Id"`
	DefaultMove4     *string `json:"defaultMove4Id"`
	BaseCry          *int    `json:"baseCry"`
	CryPitch         *int    `json:"cryPitch"`
	CryLength        *int    `json:"cryLength"`
	PokedexType      *string `json:"pokedexType"`
	Height           *string `json:"height"`
	Weight           *int    `json:"weight"`
	PokedexText      *string `json:"pokedexText"`
	EvolveLevel      *int    `json:"evolveLevel"`
	EvolvePokemon    *string `json:"evolvePokemon"`
	EvolvesFromTrade *int    `json:"evolvesFromTrade"`
	IconImage        *string `json:"iconImage"`
	PaletteType      *string `json:"paletteType"`
}

type PhaserMoveFull struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	ShortName       string  `json:"shortName"`
	Effect          *string `json:"effect"`
	Power           *int    `json:"power"`
	Type            *string `json:"type"`
	Accuracy        *int    `json:"accuracy"`
	PP              *int    `json:"pp"`
	BattleAnimation *string `json:"battleAnimation"`
	BattleSound     *string `json:"battleSound"`
	IsHM            int     `json:"isHm"`
	FieldMoveEffect *int    `json:"fieldMoveEffect"`
}

type PhaserItemFull struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ShortName     string `json:"shortName"`
	Price         *int   `json:"price"`
	IsUsable      int    `json:"isUsable"`
	UsesPartyMenu int    `json:"usesPartyMenu"`
	VendingPrice  *int   `json:"vendingPrice"`
	MoveID        *int   `json:"moveId"`
	IsGuardDrink  int    `json:"isGuardDrink"`
	IsKeyItem     int    `json:"isKeyItem"`
}

type PhaserLearnsetEntry struct {
	Level    int    `json:"level"`
	MoveName string `json:"moveName"`
	MoveID   *int   `json:"moveId"`
}

type PhaserTMHMEntry struct {
	TMHMName string `json:"tmHmName"`
	MoveName string `json:"moveName"`
	MoveID   *int   `json:"moveId"`
	IsHM     int    `json:"isHm"`
}

type PhaserMapScript struct {
	ScriptIndex    int     `json:"scriptIndex"`
	ScriptLabel    string  `json:"scriptLabel"`
	ScriptConstant string  `json:"scriptConstant"`
	RawASM         *string `json:"rawAsm"`
}

type PhaserEventFlag struct {
	FlagName     string  `json:"flagName"`
	Operation    string  `json:"operation"`
	ContextLabel *string `json:"contextLabel"`
}

type PhaserCoordinateTrigger struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
}

type PhaserNPCMovement struct {
	Label     string `json:"label"`
	Movements string `json:"movements"` // JSON array string
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
		if override := checkConditionalDialogue(textConstant, charID, efm); override != nil {
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
	var req PhaserPokemonDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid PokemonDataRequest: %v", err)
		return false
	}

	var p PhaserPokemonFull
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name, hp, atk, def, spd, spc, type_1, type_2, catch_rate, base_exp,
			default_move_1_id, default_move_2_id, default_move_3_id, default_move_4_id,
			base_cry, cry_pitch, cry_length, pokedex_type, height, weight, pokedex_text,
			evolve_level, evolve_pokemon, evolves_from_trade, icon_image, palette_type
		FROM phaser_pokemon WHERE id = $1`, req.PokemonID).Scan(
		&p.ID, &p.Name, &p.HP, &p.Atk, &p.Def, &p.Spd, &p.Spc, &p.Type1, &p.Type2, &p.CatchRate, &p.BaseExp,
		&p.DefaultMove1, &p.DefaultMove2, &p.DefaultMove3, &p.DefaultMove4,
		&p.BaseCry, &p.CryPitch, &p.CryLength, &p.PokedexType, &p.Height, &p.Weight, &p.PokedexText,
		&p.EvolveLevel, &p.EvolvePokemon, &p.EvolvesFromTrade, &p.IconImage, &p.PaletteType)
	if err != nil {
		log.Printf("[Phaser] Pokémon not found: %d: %v", req.PokemonID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "pokemon not found"}, opcodes.PhaserPokemonDataResponse)
		return false
	}

	res := StructToMap(p).(map[string]interface{})
	res["success"] = true
	ses.SendStreamJSON(res, opcodes.PhaserPokemonDataResponse)
	log.Printf("[Phaser] Sent Pokémon data for #%d %s", p.ID, p.Name)
	return false
}

// HandlePhaserMoveDataRequest returns move data by ID
func HandlePhaserMoveDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserMoveDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid MoveDataRequest: %v", err)
		return false
	}

	var m PhaserMoveFull
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name, short_name, effect, power, type, accuracy, pp, battle_animation, battle_sound, is_hm, field_move_effect
		FROM phaser_moves WHERE id = $1`, req.MoveID).Scan(
		&m.ID, &m.Name, &m.ShortName, &m.Effect, &m.Power, &m.Type, &m.Accuracy, &m.PP,
		&m.BattleAnimation, &m.BattleSound, &m.IsHM, &m.FieldMoveEffect)
	if err != nil {
		log.Printf("[Phaser] Move not found: %d: %v", req.MoveID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "move not found"}, opcodes.PhaserMoveDataResponse)
		return false
	}

	res := StructToMap(m).(map[string]interface{})
	res["success"] = true
	ses.SendStreamJSON(res, opcodes.PhaserMoveDataResponse)
	log.Printf("[Phaser] Sent move data for #%d %s", m.ID, m.Name)
	return false
}

// HandlePhaserMapScriptsRequest returns map scripts, event flags, coordinate triggers, and NPC movements for a map
func HandlePhaserMapScriptsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserMapScriptsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid MapScriptsRequest: %v", err)
		return false
	}

	// Map scripts
	scriptRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT script_index, script_label, script_constant, raw_asm
		FROM phaser_map_scripts WHERE map_name = $1 ORDER BY script_index`, req.MapName)
	if err != nil {
		log.Printf("[Phaser] Error querying map scripts for %s: %v", req.MapName, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserMapScriptsResponse)
		return false
	}
	defer scriptRows.Close()

	var scripts []PhaserMapScript
	for scriptRows.Next() {
		var s PhaserMapScript
		if err := scriptRows.Scan(&s.ScriptIndex, &s.ScriptLabel, &s.ScriptConstant, &s.RawASM); err != nil {
			continue
		}
		scripts = append(scripts, s)
	}

	// Event flags
	flagRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT flag_name, operation, context_label
		FROM phaser_event_flags WHERE map_name = $1`, req.MapName)
	if err != nil {
		log.Printf("[Phaser] Error querying event flags for %s: %v", req.MapName, err)
	}
	var flags []PhaserEventFlag
	if flagRows != nil {
		defer flagRows.Close()
		for flagRows.Next() {
			var f PhaserEventFlag
			if err := flagRows.Scan(&f.FlagName, &f.Operation, &f.ContextLabel); err != nil {
				continue
			}
			flags = append(flags, f)
		}
	}

	// Coordinate triggers
	triggerRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT label, x, y
		FROM phaser_coordinate_triggers WHERE map_name = $1`, req.MapName)
	if err != nil {
		log.Printf("[Phaser] Error querying coordinate triggers for %s: %v", req.MapName, err)
	}
	var triggers []PhaserCoordinateTrigger
	if triggerRows != nil {
		defer triggerRows.Close()
		for triggerRows.Next() {
			var t PhaserCoordinateTrigger
			if err := triggerRows.Scan(&t.Label, &t.X, &t.Y); err != nil {
				continue
			}
			triggers = append(triggers, t)
		}
	}

	// NPC movement data
	movementRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT label, movements
		FROM phaser_npc_movement_data WHERE map_name = $1`, req.MapName)
	if err != nil {
		log.Printf("[Phaser] Error querying NPC movements for %s: %v", req.MapName, err)
	}
	var movements []PhaserNPCMovement
	if movementRows != nil {
		defer movementRows.Close()
		for movementRows.Next() {
			var m PhaserNPCMovement
			if err := movementRows.Scan(&m.Label, &m.Movements); err != nil {
				continue
			}
			movements = append(movements, m)
		}
	}

	res := map[string]interface{}{
		"success":            true,
		"mapName":            req.MapName,
		"scripts":            StructToMap(scripts),
		"eventFlags":         StructToMap(flags),
		"coordinateTriggers": StructToMap(triggers),
		"npcMovements":       StructToMap(movements),
	}
	ses.SendStreamJSON(res, opcodes.PhaserMapScriptsResponse)
	log.Printf("[Phaser] Sent map scripts for %s (%d scripts, %d flags, %d triggers, %d movements)",
		req.MapName, len(scripts), len(flags), len(triggers), len(movements))

	if ses.HasValidClient() && wh.Cutscenes != nil && wh.EventFlags != nil {
		charID := int64(ses.Client.CharData().ID)
		playerFacing := ""
		if wh.PlayerMovement != nil {
			playerFacing, _ = wh.PlayerMovement.GetDirection(int(charID))
		}
		if cs := wh.Cutscenes.FindEligibleMapScriptCutscene(req.MapName, charID, wh.EventFlags, playerFacing); cs != nil {
			SendCutsceneToPlayer(ses, cs, wh)
		}
	}
	return false
}

// HandlePhaserLearnsetRequest returns learnset + TM/HM compatibility for a Pokémon
func HandlePhaserLearnsetRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserLearnsetRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid LearnsetRequest: %v", err)
		return false
	}

	// Level-up learnset
	learnRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT level, move_name, move_id
		FROM phaser_pokemon_learnset WHERE pokemon_id = $1 ORDER BY level`, req.PokemonID)
	if err != nil {
		log.Printf("[Phaser] Error querying learnset for pokemon %d: %v", req.PokemonID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserLearnsetResponse)
		return false
	}
	defer learnRows.Close()

	var learnset []PhaserLearnsetEntry
	for learnRows.Next() {
		var e PhaserLearnsetEntry
		if err := learnRows.Scan(&e.Level, &e.MoveName, &e.MoveID); err != nil {
			continue
		}
		learnset = append(learnset, e)
	}

	// TM/HM compatibility
	tmhmRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT tm_hm_name, move_name, move_id, is_hm
		FROM phaser_pokemon_tmhm WHERE pokemon_id = $1 ORDER BY tm_hm_name`, req.PokemonID)
	if err != nil {
		log.Printf("[Phaser] Error querying TM/HM for pokemon %d: %v", req.PokemonID, err)
	}
	var tmhm []PhaserTMHMEntry
	if tmhmRows != nil {
		defer tmhmRows.Close()
		for tmhmRows.Next() {
			var e PhaserTMHMEntry
			if err := tmhmRows.Scan(&e.TMHMName, &e.MoveName, &e.MoveID, &e.IsHM); err != nil {
				continue
			}
			tmhm = append(tmhm, e)
		}
	}

	res := map[string]interface{}{
		"success":   true,
		"pokemonId": req.PokemonID,
		"learnset":  StructToMap(learnset),
		"tmhm":      StructToMap(tmhm),
	}
	ses.SendStreamJSON(res, opcodes.PhaserLearnsetResponse)
	log.Printf("[Phaser] Sent learnset for pokemon %d (%d level-up, %d TM/HM)", req.PokemonID, len(learnset), len(tmhm))
	return false
}

// HandlePhaserItemDataRequest returns item data by ID
func HandlePhaserItemDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserItemDataRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid ItemDataRequest: %v", err)
		return false
	}

	var item PhaserItemFull
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name, short_name, price, is_usable, uses_party_menu, vending_price, move_id, is_guard_drink, is_key_item
		FROM phaser_items WHERE id = $1`, req.ItemID).Scan(
		&item.ID, &item.Name, &item.ShortName, &item.Price, &item.IsUsable, &item.UsesPartyMenu,
		&item.VendingPrice, &item.MoveID, &item.IsGuardDrink, &item.IsKeyItem)
	if err != nil {
		log.Printf("[Phaser] Item not found: %d: %v", req.ItemID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "item not found"}, opcodes.PhaserItemDataResponse)
		return false
	}

	res := StructToMap(item).(map[string]interface{})
	res["success"] = true
	ses.SendStreamJSON(res, opcodes.PhaserItemDataResponse)
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

// HandlePhaserMapMusicRequest returns the music constant for a map
func HandlePhaserMapMusicRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserMapMusicRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid MapMusicRequest: %v", err)
		return false
	}

	var musicConstant string
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT music_constant FROM phaser_map_music WHERE map_id = $1`, req.MapID).Scan(&musicConstant)
	if err != nil {
		log.Printf("[Phaser] No music found for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "no music for map"}, opcodes.PhaserMapMusicResponse)
		return false
	}

	res := map[string]interface{}{
		"success":       true,
		"mapId":         req.MapID,
		"musicConstant": musicConstant,
	}
	ses.SendStreamJSON(res, opcodes.PhaserMapMusicResponse)
	log.Printf("[Phaser] Sent music for map %d: %s", req.MapID, musicConstant)
	return false
}
