package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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

	result, err := readTrainerInteraction(ses.CommandContext(), ses, wh, req.ActorID)
	if err != nil {
		if !errors.Is(err, errScriptInteractionDenied) {
			log.Printf("[TrainerInteract] Failed to read trainer actor %d: %v", req.ActorID, err)
		}
		ses.SendStreamJSON(TrainerInteractResponse{Error: "trainer interaction unavailable"}, opcodes.TrainerInteractResponse)
		return false
	}
	ses.SendStreamJSON(result, opcodes.TrainerInteractResponse)
	return false
}

// Runtime reach is checked before the database snapshot. This read never issues
// a battle: the separate battle command must recheck reach and mutation policy.
func readTrainerInteraction(ctx context.Context, ses *session.Session, wh *WorldHandler, actorID int) (TrainerInteractResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	trainer, err := trainerDataForRuntimeActorContext(ctx, ses, wh, actorID)
	if err != nil {
		return TrainerInteractResponse{}, err
	}
	charID := int64(ses.Client.CharData().ID)
	return db.ReadSnapshot(ctx, wh.database, func(ctx context.Context, q db.ReadDBTX) (TrainerInteractResponse, error) {
		flags, err := eventFlagSnapshotIn(q, charID)
		if err != nil {
			return TrainerInteractResponse{}, err
		}
		// Reload catalog labels and status on the same retained snapshot; cached
		// defeated sets and session flags can lag a committed reward or healing reset.
		trainer, err := trainerDataForObjectIDContext(ctx, q, trainer.ObjectID)
		if err != nil {
			return TrainerInteractResponse{}, err
		}
		defeated, suppressed, err := directTrainerDefeatStatusIn(q, charID, trainer, flags)
		if err != nil {
			return TrainerInteractResponse{}, err
		}
		shouldBattle := !defeated || (!suppressed && trainerRebattleAllowed(ses))
		label := trainer.BattleTextLabel
		if defeated && !shouldBattle {
			label = trainer.AfterBattleTextLabel
		}
		dialogue, err := trainerDialogueByLabelContext(ctx, q, label)
		if err != nil {
			return TrainerInteractResponse{}, err
		}
		return TrainerInteractResponse{Success: true, TrainerActorID: actorID, TrainerName: trainer.Name, TrainerClass: trainer.TrainerClass, Dialogue: dialogue, ShouldBattle: shouldBattle, Defeated: defeated}, nil
	})
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

	trainer, err := trainerDataForRuntimeActor(ses, wh, req.TrainerActorID)
	if err != nil {
		if !errors.Is(err, errScriptInteractionDenied) {
			log.Printf("[TrainerInteract] Failed to start trainer actor %d: %v", req.TrainerActorID, err)
		}
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "trainer not found",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	playerX, playerY := trainerInteractionPlayerPosition(ses, wh, charID)
	postWinMapName, postWinActions := pokemonTower7FPostWinActions(trainer, playerX, playerY)
	battle, events, err := startScriptedTrainerBattle(ses.CommandContext(), wh.database, charID, ScriptedTrainerBattleSpec{
		TrainerClass:    trainer.TrainerClass,
		PartyIndex:      trainer.PartyIndex,
		TrainerObjectID: trainer.ObjectID,
		WinFlag:         trainer.EventFlag,
		PostWinMapName:  postWinMapName,
		PostWinActions:  postWinActions,
	}, func(tx db.DBTX) error {
		flags, err := eventFlagSnapshotIn(tx, charID)
		if err != nil {
			return err
		}
		defeated, suppressed, err := directTrainerDefeatStatusIn(tx, charID, trainer, flags)
		if err != nil {
			return err
		}
		if suppressed || (defeated && !trainerRebattleAllowed(ses)) {
			return errDirectTrainerIneligible
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, errDirectTrainerIneligible) {
			log.Printf("[TrainerInteract] Failed to start battle for trainer %s (%s/%d): %v",
				trainer.Name, trainer.TrainerClass, trainer.PartyIndex, err)
		}
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	resp := buildBattleStateResponse(battle)
	resp["trainerClass"] = trainer.TrainerClass
	resp["trainerName"] = battle.Trainer.Name
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

func trainerDataForRuntimeActor(ses *session.Session, wh *WorldHandler, actorID int) (*trainerSightData, error) {
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	return trainerDataForRuntimeActorContext(ctx, ses, wh, actorID)
}

func trainerDataForRuntimeActorContext(ctx context.Context, ses *session.Session, wh *WorldHandler, actorID int) (*trainerSightData, error) {
	if wh == nil || wh.ActorRegistry == nil {
		return nil, fmt.Errorf("actor registry unavailable")
	}
	objectID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, actorID)
	if objectID == 0 {
		return nil, fmt.Errorf("unknown actor %d", actorID)
	}
	actor, _, err := wh.scriptInteractionTargetContext(ctx, ses, objectID)
	if err != nil {
		return nil, err
	}
	trainer, err := trainerDataForObjectIDContext(ctx, wh.database, objectID)
	if err != nil {
		return nil, err
	}
	trainer.X, trainer.Y = *actor.X, *actor.Y
	return trainer, nil
}

func trainerDataForObjectID(objectID int) (*trainerSightData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return trainerDataForObjectIDContext(ctx, db.GlobalWorldDB.DB, objectID)
}

func trainerDataForObjectIDContext(ctx context.Context, database db.ContextDBTX, objectID int) (*trainerSightData, error) {
	var t trainerSightData
	var globalX, globalY sql.NullInt64
	var isGymLeader sql.NullInt64
	var direction, trainerClass, name sql.NullString
	var eventFlag, battleTextLabel, endBattleTextLabel, afterBattleTextLabel sql.NullString
	var sightRange sql.NullInt64

	err := database.QueryRowContext(ctx, `
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return trainerDialogueByLabelContext(ctx, db.GlobalWorldDB.DB, label)
}

func trainerDialogueByLabelContext(ctx context.Context, database db.ContextDBTX, label string) (string, error) {
	if label == "" {
		return "", nil
	}
	var dialogue string
	if err := database.QueryRowContext(ctx,
		`SELECT dialogue FROM phaser_dialogue_text WHERE label = $1`,
		label,
	).Scan(&dialogue); err != nil {
		return "", err
	}
	return dialogue, nil
}

func trainerRebattleAllowed(ses *session.Session) bool {
	opts := ses.Client.Options()
	return opts != nil && opts.AllowTrainerRebattles
}

var errDirectTrainerIneligible = errors.New("trainer already defeated")

// Direct clicks allow explicit rebattles; sight encounters retain their distinct
// policy. Both use the same authoritative defeat-history query.
func directTrainerDefeatStatusIn(q db.DBTX, charID int64, trainer *trainerSightData, flags *EventFlagManager) (bool, bool, error) {
	history, err := trainerDefeatedIn(q, charID, trainer.ObjectID)
	if err != nil {
		return false, false, err
	}
	suppressed := trainerBattleSuppressedByGymLeaderFlags(charID, trainer, flags)
	return suppressed || history || (trainer.EventFlag != "" && flags.CheckFlag(charID, trainer.EventFlag)), suppressed, nil
}
