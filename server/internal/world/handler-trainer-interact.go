package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/session"
)

type TrainerInteractRequest struct {
	ActorID int `json:"actorId"`
}

type TrainerInteractResponse struct {
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
	TrainerActorID int    `json:"trainerActorId,omitempty"`
	TrainerName    string `json:"trainerName,omitempty"`
	TrainerClass   string `json:"trainerClass,omitempty"`
	Dialogue       string `json:"dialogue,omitempty"`
	ShouldBattle   bool   `json:"shouldBattle"`
	Defeated       bool   `json:"defeated"`
}

type TrainerBattleStartRequest struct {
	TrainerActorID int `json:"trainerActorId"`
}

func HandleTrainerInteractRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req TrainerInteractRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(TrainerInteractResponse{
			Success: false,
			Error:   "invalid request",
		}, opcodes.TrainerInteractResponse)
		return false
	}

	trainer, err := trainerDataForRuntimeActor(wh, req.ActorID)
	if err != nil {
		log.Printf("[TrainerInteract] Failed to load trainer actor %d: %v", req.ActorID, err)
		ses.SendStreamJSON(TrainerInteractResponse{
			Success: false,
			Error:   "trainer not found",
		}, opcodes.TrainerInteractResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	suppressedByGymLeaderDefeat := trainerBattleSuppressedByGymLeaderDefeat(charID, trainer, wh)
	defeated := suppressedByGymLeaderDefeat || trainerDefeatedForCharacter(charID, trainer, wh)
	shouldBattle := !defeated || (!suppressedByGymLeaderDefeat && trainerRebattleAllowed(charID))

	label := trainer.BattleTextLabel
	if defeated && !shouldBattle {
		label = trainer.AfterBattleTextLabel
	}
	dialogue, err := trainerDialogueByLabel(label)
	if err != nil && label != "" {
		log.Printf("[TrainerInteract] Missing dialogue %s for trainer %s: %v", label, trainer.Name, err)
	}

	ses.SendStreamJSON(TrainerInteractResponse{
		Success:        true,
		TrainerActorID: req.ActorID,
		TrainerName:    trainer.Name,
		TrainerClass:   trainer.TrainerClass,
		Dialogue:       dialogue,
		ShouldBattle:   shouldBattle,
		Defeated:       defeated,
	}, opcodes.TrainerInteractResponse)
	return false
}

func HandleTrainerBattleStartRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req TrainerBattleStartRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid request",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	trainer, err := trainerDataForRuntimeActor(wh, req.TrainerActorID)
	if err != nil {
		log.Printf("[TrainerInteract] Failed to start trainer actor %d: %v", req.TrainerActorID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "trainer not found",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	if trainerBattleSuppressedByGymLeaderDefeat(charID, trainer, wh) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "gym leader already defeated",
		}, opcodes.PokeBattleStartResponse)
		return false
	}
	if trainerDefeatedForCharacter(charID, trainer, wh) && !trainerRebattleAllowed(charID) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "trainer already defeated",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	playerX, playerY := trainerInteractionPlayerPosition(ses, wh, charID)
	postWinMapName, postWinActions := pokemonTower7FPostWinActions(trainer, playerX, playerY)
	battle, events, err := StartScriptedTrainerBattle(charID, ScriptedTrainerBattleSpec{
		TrainerClass:    trainer.TrainerClass,
		PartyIndex:      trainer.PartyIndex,
		TrainerObjectID: trainer.ObjectID,
		WinFlag:         trainer.EventFlag,
		PostWinMapName:  postWinMapName,
		PostWinActions:  postWinActions,
	})
	if err != nil {
		log.Printf("[TrainerInteract] Failed to start battle for trainer %s (%s/%d): %v",
			trainer.Name, trainer.TrainerClass, trainer.PartyIndex, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	resp := buildBattleStateResponse(battle)
	resp["trainerClass"] = trainer.TrainerClass
	resp["trainerName"] = trainerDisplayName(trainer.TrainerClass)
	resp["events"] = events
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
	return false
}

func trainerInteractionPlayerPosition(ses *session.Session, wh *WorldHandler, charID int64) (int, int) {
	if wh != nil && wh.PlayerMovement != nil {
		if x, y, _, ok := wh.PlayerMovement.GetPosition(int(charID)); ok {
			return x, y
		}
	}
	if ses == nil {
		return 0, 0
	}
	x, y := int(ses.X), int(ses.Y)
	if ses.Client != nil && (x != 0 || y != 0) {
		return x, y
	}
	if ses.Client != nil {
		if char := ses.Client.CharData(); char != nil {
			return int(char.X), int(char.Y)
		}
	}
	return x, y
}

func trainerDataForRuntimeActor(wh *WorldHandler, actorID int) (*trainerSightData, error) {
	if wh == nil || wh.ActorRegistry == nil {
		return nil, fmt.Errorf("actor registry unavailable")
	}
	objectID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, actorID)
	if objectID == 0 {
		return nil, fmt.Errorf("unknown actor %d", actorID)
	}
	return trainerDataForObjectID(objectID)
}

func trainerDataForObjectID(objectID int) (*trainerSightData, error) {
	var t trainerSightData
	var globalX, globalY sql.NullInt64
	var isGymLeader sql.NullInt64
	var direction, trainerClass, name sql.NullString
	var eventFlag, battleTextLabel, endBattleTextLabel, afterBattleTextLabel sql.NullString
	var sightRange sql.NullInt64

	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT
			po.id,
			po.map_id,
			COALESCE(po.x, po.local_x) AS global_x,
			COALESCE(po.y, po.local_y) AS global_y,
			po.action_direction,
			po.trainer_class,
			po.trainer_party_index,
			po.name,
			COALESCE(tc.is_gym_leader, 0) AS is_gym_leader,
			th.event_flag,
			th.sight_range,
			th.battle_text_label,
			th.end_battle_text_label,
			th.after_battle_text_label
		FROM phaser_objects po
		LEFT JOIN phaser_maps pm
			ON pm.id = po.map_id
		LEFT JOIN phaser_text_pointers tp
			ON tp.text_constant = po.text
			AND tp.is_trainer = 1
		LEFT JOIN phaser_trainer_headers th
			ON th.header_index = (
				SELECT COUNT(*) - 1
				FROM phaser_text_pointers tp_rank
				WHERE tp_rank.is_trainer = 1
				  AND tp_rank.pointer_index <= tp.pointer_index
				  AND LOWER(REPLACE(tp_rank.map_name, '_', '')) = LOWER(REPLACE(tp.map_name, '_', ''))
			)
			AND (
				th.map_id = po.map_id
				OR LOWER(REPLACE(th.map_name, '_', '')) = LOWER(REPLACE(pm.name, '_', ''))
				OR LOWER(REPLACE(th.map_name, '_', '')) = LOWER(REPLACE(tp.map_name, '_', ''))
			)
		LEFT JOIN phaser_trainer_classes tc
			ON tc.constant_name = po.trainer_class
		WHERE po.id = $1
			AND po.trainer_class IS NOT NULL
			AND po.trainer_class != ''
		LIMIT 1`, objectID).Scan(
		&t.ObjectID,
		&t.MapID,
		&globalX,
		&globalY,
		&direction,
		&trainerClass,
		&t.PartyIndex,
		&name,
		&isGymLeader,
		&eventFlag,
		&sightRange,
		&battleTextLabel,
		&endBattleTextLabel,
		&afterBattleTextLabel,
	)
	if err != nil {
		return nil, err
	}

	if globalX.Valid {
		t.X = int(globalX.Int64)
	}
	if globalY.Valid {
		t.Y = int(globalY.Int64)
	}
	t.Direction = "DOWN"
	if direction.Valid && direction.String != "" {
		t.Direction = direction.String
	}
	if trainerClass.Valid {
		t.TrainerClass = trainerClass.String
	}
	if name.Valid {
		t.Name = name.String
	}
	t.IsGymLeader = isGymLeader.Valid && isGymLeader.Int64 != 0
	if eventFlag.Valid {
		t.EventFlag = eventFlag.String
	}
	if sightRange.Valid {
		t.SightRange = int(sightRange.Int64)
	}
	if battleTextLabel.Valid {
		t.BattleTextLabel = battleTextLabel.String
	}
	if endBattleTextLabel.Valid {
		t.EndBattleTextLabel = endBattleTextLabel.String
	}
	if afterBattleTextLabel.Valid {
		t.AfterBattleTextLabel = afterBattleTextLabel.String
	}
	applyGymLeaderBattleMetadata(&t)
	if !trainerHasRuntimeBattleMetadata(&t) {
		return nil, sql.ErrNoRows
	}
	return &t, nil
}

func trainerHasRuntimeBattleMetadata(t *trainerSightData) bool {
	if t == nil {
		return false
	}
	if t.EventFlag == "" {
		return false
	}
	return t.BattleTextLabel != "" || t.AfterBattleTextLabel != "" || t.IsGymLeader
}

func trainerDialogueByLabel(label string) (string, error) {
	if label == "" {
		return "", nil
	}
	var dialogue string
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT dialogue FROM phaser_dialogue_text WHERE label = $1`,
		label,
	).Scan(&dialogue); err != nil {
		return "", err
	}
	return dialogue, nil
}

func trainerDefeatedForCharacter(charID int64, trainer *trainerSightData, wh *WorldHandler) bool {
	if trainer == nil {
		return false
	}
	if wh != nil && wh.TrainerEncounter != nil && wh.TrainerEncounter.IsTrainerDefeated(charID, trainer.ObjectID) {
		return true
	}
	return trainer.EventFlag != "" && wh != nil && wh.EventFlags != nil && wh.EventFlags.CheckFlag(charID, trainer.EventFlag)
}

func trainerRebattleAllowed(charID int64) bool {
	opts, err := db_character.LoadOptions(context.Background(), int32(charID))
	return err == nil && opts != nil && opts.AllowTrainerRebattles
}
