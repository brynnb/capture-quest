package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// Gameplay recovery reads current authority, never a historical notification.
// Explicit JSON names also drive the generated TypeScript contract.
type GameplayStateRequest struct {
	RequestID string `json:"requestId"`
	MapID     int    `json:"mapId,omitempty"`
	Current   bool   `json:"current,omitempty"`
}
type GameplayBattleState struct {
	NeedsDismissal  bool                 `json:"needsDismissal,omitempty"`
	BattleID        string               `json:"battleId"`
	Revision        int64                `json:"revision"`
	Phase           string               `json:"phase"`
	TurnNumber      int                  `json:"turnNumber"`
	PlayerPokemon   PokemonDTO           `json:"playerPokemon"`
	EnemyPokemon    PokemonDTO           `json:"enemyPokemon"`
	PlayerParty     []PokemonDTO         `json:"playerParty"`
	PlayerActive    int                  `json:"playerActive"`
	BattleType      string               `json:"battleType"`
	AllowedActions  []string             `json:"allowedActions"`
	GuaranteedCatch bool                 `json:"guaranteedCatch"`
	TrainerClass    string               `json:"trainerClass"`
	TrainerName     string               `json:"trainerName"`
	PendingMove     *GameplayPendingMove `json:"pendingMove"`
}
type GameplayPendingMove struct {
	MoveID       int    `json:"moveId"`
	MoveName     string `json:"moveName"`
	PokemonIndex int    `json:"pokemonIndex"`
}
type SafariRecoveryPokemon struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	HP    int    `json:"hp"`
	MaxHP int    `json:"maxHp"`
}
type SafariRecoveryState struct {
	Active    bool                   `json:"active"`
	BallsLeft int                    `json:"ballsLeft"`
	StepsLeft int                    `json:"stepsLeft"`
	Pokemon   *SafariRecoveryPokemon `json:"pokemon"`
}
type GameplayStateResponse struct {
	Success   bool                                    `json:"success" tstype:"true"`
	RequestID string                                  `json:"requestId"`
	Position  protocol.OwnedPlayerPositionResponse    `json:"position" tstype:"import(\"./protocol\").OwnedPlayerPositionResponse"`
	Battle    *GameplayBattleState                    `json:"battle" tstype:"GameplayBattleState | null"`
	Safari    *SafariRecoveryState                    `json:"safari" tstype:"SafariRecoveryState | null"`
	Trainer   *protocol.TrainerEncounterNotifyPayload `json:"trainer" tstype:"import(\"./protocol\").TrainerEncounterNotifyPayload | null"`
	Cutscene  *protocol.CutsceneStartNotify           `json:"cutscene" tstype:"import(\"./protocol\").CutsceneStartNotify | null"`
}

func HandleGameplayStateRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req GameplayStateRequest
	if err := decodePlayerMovement(payload, &req); err != nil || req.RequestID == "" || len(req.RequestID) > 64 {
		sendOwnedPlayerError(ses, wh, req.RequestID, opcodes.GameplayStateResponse, "Invalid gameplay recovery request.")
		return false
	}
	result, battle, err := readGameplayState(ses.CommandContext(), ses, wh, req)
	if err != nil {
		log.Printf("[GameplayRecovery] Character %d: %v", ses.Client.CharData().ID, err)
		sendOwnedPlayerError(ses, wh, req.RequestID, opcodes.GameplayStateResponse, "Could not recover current gameplay. Please reconnect.")
		return false
	}
	// Publish only the fully validated snapshot; a partial read must not erase
	// active ownership or independently deliver a trainer/battle notification.
	charID := int64(ses.Client.CharData().ID)
	if battle == nil {
		forgetBattle(charID, getBattle(charID))
	} else {
		setBattle(charID, battle)
	}
	ses.SendStreamJSON(result, opcodes.GameplayStateResponse)
	return false
}

func readGameplayState(ctx context.Context, ses *session.Session, wh *WorldHandler, req GameplayStateRequest) (GameplayStateResponse, *pokebattle.BattleState, error) {
	result := GameplayStateResponse{Success: true, RequestID: req.RequestID, Position: wh.ownedPlayerSnapshot(ses, req.RequestID)}
	// Current recovery follows authority after a possibly committed teleport.
	// Scene-bound reads retain their expected map; never infer current mode from
	// an omitted or stale map, and reject contradictory selectors.
	if req.Current && req.MapID != 0 {
		return GameplayStateResponse{}, nil, fmt.Errorf("current recovery cannot supply a view map")
	}
	if !req.Current && req.MapID != result.Position.MapID {
		return GameplayStateResponse{}, nil, fmt.Errorf("recovery view map %d differs from owned map %d", req.MapID, result.Position.MapID)
	}
	charID := int64(ses.Client.CharData().ID)
	var battle *pokebattle.BattleState
	err := db.Transaction(ctx, wh.database, func(tx db.DBTX) error {
		// SELECT takes the same row lock as mutations without firing UPDATE triggers.
		// This holds party, battle, Safari and pending plans at one character boundary.
		var mapID, x, y int
		if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1 FOR UPDATE`, charID).Scan(&mapID, &x, &y); err != nil {
			return err
		}
		if mapID != result.Position.MapID || x != result.Position.X || y != result.Position.Y {
			return fmt.Errorf("saved recovery source differs from owned source")
		}
		var err error
		battle, err = pokebattle.LoadBattleState(tx, charID)
		if err != nil {
			return err
		}
		if battle != nil {
			if err := battle.RestoreParty(tx, charID); err != nil {
				return err
			}
			if err := configureBattleObedienceFromDB(tx, battle, charID); err != nil {
				return err
			}
			result.Battle = gameplayBattleSnapshot(battle)
		} else {
			battle = nil
		}
		safari, err := safariSessionIn(tx, charID)
		if err != nil {
			return err
		}
		if safari != nil && safari.Active {
			result.Safari = &SafariRecoveryState{Active: true, BallsLeft: safari.BallsLeft, StepsLeft: safari.StepsLeft}
			if safari.Battle != nil {
				p := safari.Battle.WildPokemon
				result.Safari.Pokemon = &SafariRecoveryPokemon{ID: p.ID, Name: p.Name, Level: p.Level, HP: p.CurHP, MaxHP: p.MaxHP}
			}
		}
		if result.Battle != nil && result.Safari != nil && result.Safari.Pokemon != nil {
			return fmt.Errorf("ordinary battle conflicts with safari battle")
		}
		if wh.TrainerEncounter != nil {
			plan, err := wh.TrainerEncounter.loadEncounterIn(tx, charID)
			if err != nil {
				return err
			}
			if plan != nil && plan.Resolution == "pending" {
				if plan.MapID != mapID || plan.PlayerX != x || plan.PlayerY != y {
					return fmt.Errorf("pending trainer source differs from recovery source")
				}
				if result.Battle != nil || (result.Safari != nil && result.Safari.Pokemon != nil) {
					return fmt.Errorf("pending trainer conflicts with active battle")
				}
				payload := wh.TrainerEncounter.encounterPayload(plan)
				result.Trainer = &payload
			}
		}
		var token string
		err = tx.QueryRow(`SELECT completion_token FROM character_cutscene_plans WHERE character_id=$1 AND resolution='pending' ORDER BY sequence LIMIT 1`, charID).Scan(&token)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		plan, err := loadCutscenePlanIn(tx, charID, token)
		if err != nil {
			return err
		}
		if plan.Event.MapID != mapID || plan.Event.X != x || plan.Event.Y != y {
			return fmt.Errorf("pending cutscene source differs from recovery source")
		}
		// Battle presentation takes priority; its post-battle plan can resume
		// later. Do not ask the browser to play a script over an active battle.
		if result.Battle != nil || (result.Safari != nil && result.Safari.Pokemon != nil) || result.Trainer != nil {
			return nil
		}
		script := plan.Event.Script
		actions, err := DecodeCutsceneActions(script.Actions)
		if err != nil {
			return err
		}
		if wh.ActorRegistry != nil {
			if _, err := annotateCutsceneActionListForClient(tx, actions, script.MapName, wh); err != nil {
				return err
			}
		}
		raw, err := json.Marshal(actions)
		if err != nil {
			return err
		}
		result.Cutscene = &protocol.CutsceneStartNotify{ScriptLabel: script.ScriptLabel, CompletionToken: token, MapName: script.MapName, Actions: raw}
		return nil
	})
	if err != nil {
		return GameplayStateResponse{}, nil, err
	}
	return result, battle, nil
}

func gameplayBattleSnapshot(b *pokebattle.BattleState) *GameplayBattleState {
	result := &GameplayBattleState{NeedsDismissal: b.IsOver() && b.PendingMoveLearn == nil, BattleID: b.BattleID, Revision: b.Revision, Phase: phaseToString(b.Phase), TurnNumber: b.TurnNumber, PlayerPokemon: pokemonToDTO(b.GetPlayerPokemon()), EnemyPokemon: pokemonToDTO(b.GetEnemyPokemon()), PlayerParty: battlePartyDTOs(b), PlayerActive: b.PlayerActive, BattleType: battleTypeToString(b.BattleType), GuaranteedCatch: b.GuaranteedCatch, AllowedActions: []string{}}
	for _, action := range b.AllowedActions {
		result.AllowedActions = append(result.AllowedActions, string(action))
	}
	if b.Trainer != nil {
		result.TrainerClass, result.TrainerName = b.Trainer.ClassName, b.Trainer.Name
	}
	if b.PendingMoveLearn != nil {
		result.Phase = "move_learn_prompt"
		result.PendingMove = &GameplayPendingMove{MoveID: b.PendingMoveLearn.MoveID, MoveName: b.PendingMoveLearn.MoveName, PokemonIndex: b.PendingMoveLearn.PokemonIndex}
	}
	return result
}
