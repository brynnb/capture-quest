package world

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

func (m *TrainerEncounterManager) requireEncounterSchema(ctx context.Context) error {
	if m.wh == nil || m.wh.database == nil {
		return fmt.Errorf("trainer preload requires a database")
	}
	rows, err := m.wh.database.QueryContext(ctx, `SELECT character_id,version,encounter_token,trainer_object_id,trainer_map_id,trainer_class,party_index,map_id,x,y,resolution FROM character_trainer_encounters LIMIT 0`)
	if err != nil {
		return fmt.Errorf("trainer encounter schema: %w", err)
	}
	return rows.Close()
}

func savePendingTrainerIn(tx db.DBTX, charID int64, mapID, x, y int, t *trainerSightData) (*pendingEncounter, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return nil, err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	enc := &pendingEncounter{TrainerData: t, CharID: charID, PlayerX: x, PlayerY: y, MapID: mapID, Token: hex.EncodeToString(token[:]), Resolution: "pending"}
	_, err := tx.Exec(`INSERT INTO character_trainer_encounters(character_id,version,encounter_token,trainer_object_id,trainer_map_id,trainer_class,party_index,map_id,x,y,resolution) VALUES($1,1,$2,$3,$4,$5,$6,$7,$8,$9,'pending') ON CONFLICT(character_id) DO UPDATE SET version=1,encounter_token=EXCLUDED.encounter_token,trainer_object_id=EXCLUDED.trainer_object_id,trainer_map_id=EXCLUDED.trainer_map_id,trainer_class=EXCLUDED.trainer_class,party_index=EXCLUDED.party_index,map_id=EXCLUDED.map_id,x=EXCLUDED.x,y=EXCLUDED.y,resolution='pending',updated_at=CURRENT_TIMESTAMP`, charID, enc.Token, t.ObjectID, t.MapID, t.TrainerClass, t.PartyIndex, mapID, x, y)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

func (m *TrainerEncounterManager) loadEncounterIn(q db.DBTX, charID int64) (*pendingEncounter, error) {
	enc := &pendingEncounter{CharID: charID, TrainerData: &trainerSightData{}}
	var version int
	t := enc.TrainerData
	err := q.QueryRow(`SELECT version,encounter_token,trainer_object_id,trainer_map_id,trainer_class,party_index,map_id,x,y,resolution FROM character_trainer_encounters WHERE character_id=$1`, charID).Scan(&version, &enc.Token, &t.ObjectID, &t.MapID, &t.TrainerClass, &t.PartyIndex, &enc.MapID, &enc.PlayerX, &enc.PlayerY, &enc.Resolution)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if version != 1 || !validMovementToken(enc.Token) {
		return nil, fmt.Errorf("invalid trainer encounter for character %d version=%d", charID, version)
	}
	if enc.Resolution != "pending" {
		return enc, nil
	}
	// Runtime actor IDs are rebuilt on startup. Resolve the stable catalog identity
	// and reject a changed native map/class/party rather than guessing a replacement.
	for _, current := range m.byMap[enc.MapID] {
		if current.ObjectID == t.ObjectID && current.MapID == t.MapID && current.TrainerClass == t.TrainerClass && current.PartyIndex == t.PartyIndex {
			enc.TrainerData = current
			return enc, nil
		}
	}
	return nil, fmt.Errorf("pending trainer identity missing or changed: character=%d object=%d native_map=%d class=%s party=%d", charID, t.ObjectID, t.MapID, t.TrainerClass, t.PartyIndex)
}

func trainerEligibleIn(q db.DBTX, charID int64, t *trainerSightData, flags *EventFlagManager) (bool, error) {
	if meta, ok := gymLeaderMetadataForMap(t.MapID); ok && gymLeaderDefeatedForCharacter(charID, meta, flags) {
		return false, nil
	}
	var defeated bool
	if err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_defeated_trainers WHERE character_id=$1 AND trainer_object_id=$2)`, charID, t.ObjectID).Scan(&defeated); err != nil {
		return false, err
	}
	if !defeated {
		return true, nil
	}
	opts := db_character.DefaultOptions()
	var raw sql.NullString
	if err := q.QueryRow(`SELECT options FROM character_data WHERE id=$1`, charID).Scan(&raw); err != nil {
		return false, err
	}
	if raw.Valid && raw.String != "" {
		if err := json.Unmarshal([]byte(raw.String), opts); err != nil {
			return false, fmt.Errorf("trainer options for %d: %w", charID, err)
		}
	}
	return opts.AllowTrainerRebattles, nil
}

func (m *TrainerEncounterManager) resumePendingEncounter(ses *session.Session, wh *WorldHandler) (bool, error) {
	charID := int64(ses.Client.CharData().ID)
	var enc *pendingEncounter
	err := db.Transaction(ses.CommandContext(), wh.database, func(q db.DBTX) (err error) {
		enc, err = m.loadEncounterIn(q, charID)
		return err
	})
	if err != nil || enc == nil || enc.Resolution != "pending" {
		return false, err
	}
	x, y, mapID := wh.ownedPlayerPosition(ses)
	if mapID != enc.MapID || x != enc.PlayerX || y != enc.PlayerY {
		return false, fmt.Errorf("pending trainer source changed for character %d", charID)
	}
	if getBattle(charID) != nil {
		return false, nil
	}
	m.publishPositionEncounter(enc, ses)
	return true, nil
}

type trainerReadyResult struct {
	Battle      *pokebattle.BattleState
	Blackout    *BlackoutResult
	Party       []*pokebattle.Pokemon
	DisplayName string
	Replayed    bool
}

func (m *TrainerEncounterManager) resolveEncounter(ctx context.Context, charID int64, req protocol.TrainerEncounterReadyRequest) (trainerReadyResult, error) {
	var result trainerReadyResult
	err := db.Transaction(ctx, m.wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		enc, err := m.loadEncounterIn(tx, charID)
		if err != nil {
			return err
		}
		if enc == nil || enc.Token != req.EncounterToken || enc.Resolution == "cancelled" {
			return fmt.Errorf("trainer encounter authorization unavailable")
		}
		t := enc.TrainerData
		if enc.Resolution != "pending" {
			result.Replayed = true
			// Resend only the current saved battle belonging to this trainer. A
			// completed/closed result cannot recreate a battle or repeat blackout.
			if enc.Resolution == "battle" {
				current, err := pokebattle.LoadBattleState(tx, charID)
				if err != nil {
					return err
				}
				if current != nil && current.Trainer != nil && current.Trainer.TrainerObjectID == t.ObjectID {
					if err := current.RestoreParty(tx, charID); err != nil {
						return err
					}
					result.Battle, result.DisplayName = current, current.Trainer.Name
				}
			}
			return nil
		}
		if req.TrainerActorID != t.RuntimeActorID {
			return fmt.Errorf("trainer actor mismatch")
		}
		var mapID, x, y int
		if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1`, charID).Scan(&mapID, &x, &y); err != nil {
			return err
		}
		if mapID != enc.MapID || x != enc.PlayerX || y != enc.PlayerY {
			return fmt.Errorf("trainer encounter source changed")
		}
		if m.wh.PlayerMovement != nil {
			if ownedX, ownedY, ownedMap, ok := m.wh.PlayerMovement.GetPosition(int(charID)); ok && (ownedMap != enc.MapID || ownedX != enc.PlayerX || ownedY != enc.PlayerY) {
				return fmt.Errorf("trainer owned source changed")
			}
		}
		active, err := pokebattle.LoadBattleState(tx, charID)
		if err != nil {
			return err
		}
		if active != nil && (!active.IsOver() || active.PendingMoveLearn != nil) {
			return pokebattle.ErrBattleConflict
		}
		flags, err := eventFlagSnapshotIn(tx, charID)
		if err != nil {
			return err
		}
		eligible, err := trainerEligibleIn(tx, charID, t, flags)
		if err != nil {
			return err
		}
		if !eligible {
			return fmt.Errorf("trainer is no longer eligible")
		}
		trainerParty, err := pokebattle.BuildTrainerParty(tx, t.TrainerClass, t.PartyIndex)
		if err != nil {
			return err
		}
		if len(trainerParty) == 0 {
			return fmt.Errorf("trainer party absent: class=%s party=%d", t.TrainerClass, t.PartyIndex)
		}
		playerParty, err := pokebattle.LoadParty(tx, charID)
		if err != nil {
			return err
		}
		resolution := "battle"
		if len(playerParty) == 0 || playerParty[pokebattle.FirstAlivePartyIndex(playerParty)].IsFainted() {
			blackout, party, err := CommitStandaloneBlackout(ctx, tx, charID)
			if err != nil {
				return err
			}
			result.Blackout, result.Party = &blackout, party
			resolution = "blackout"
		} else {
			var displayName string
			var baseMoney int
			err := tx.QueryRow(`SELECT display_name,base_money FROM phaser_trainer_classes WHERE constant_name=$1`, t.TrainerClass).Scan(&displayName, &baseMoney)
			if err != nil {
				return err
			}
			if displayName == "" {
				displayName = t.TrainerClass
			} // Existing display policy when the catalog row has an empty name.
			result.DisplayName, err = trainerNameForCharacterFromDB(tx, charID, t.TrainerClass, displayName)
			if err != nil {
				return err
			}
			battle := pokebattle.NewTrainerBattle(playerParty, trainerParty)
			battle.Trainer = &pokebattle.TrainerMeta{ClassName: t.TrainerClass, Name: result.DisplayName, PrizeMoney: trainerPrizeMoneyFromBase(baseMoney, trainerParty), TrainerObjectID: t.ObjectID, WinFlag: t.EventFlag}
			applyPokemonTower7FPostWinMetadata(battle.Trainer, t, enc.PlayerX, enc.PlayerY)
			result.Battle, err = pokebattle.StartBattleInTransaction(tx, charID, battle, func(q db.DBTX, next *pokebattle.BattleState) error {
				if err := configureBattleObedienceFromDB(q, next, charID); err != nil {
					return err
				}
				return markPokemonSeen(q, charID, next.GetEnemyPokemon().ID)
			})
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(`UPDATE character_trainer_encounters SET resolution=$1,updated_at=CURRENT_TIMESTAMP WHERE character_id=$2`, resolution, charID)
		return err
	})
	if err != nil {
		return trainerReadyResult{}, err
	}
	return result, nil
}
