package world

import (
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

func configureBattleObedience(battle *pokebattle.BattleState, charID int64, efm *EventFlagManager) {
	if battle == nil {
		return
	}
	battle.PlayerTrainerID = charID
	battle.PlayerObedienceLevel = obedienceLevelForCharacter(charID, efm)
}

func obedienceLevelForCharacter(charID int64, efm *EventFlagManager) int {
	switch {
	case characterHasEventFlag(charID, "EVENT_GOT_EARTHBADGE", efm):
		return pokebattle.MaxObedienceLevel
	case characterHasEventFlag(charID, "EVENT_GOT_MARSHBADGE", efm):
		return 70
	case characterHasEventFlag(charID, "EVENT_GOT_RAINBOWBADGE", efm):
		return 50
	case characterHasEventFlag(charID, "EVENT_GOT_CASCADEBADGE", efm):
		return 30
	default:
		return pokebattle.BaseObedienceLevel
	}
}

func characterHasEventFlag(charID int64, flag string, efm *EventFlagManager) bool {
	if charID <= 0 || flag == "" {
		return false
	}
	if efm != nil && efm.CheckFlag(charID, flag) {
		return true
	}
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return false
	}

	var one int
	return db.GlobalWorldDB.DB.QueryRow(
		`SELECT 1 FROM character_event_flags WHERE character_id = $1 AND flag_name = $2 LIMIT 1`,
		charID, flag,
	).Scan(&one) == nil
}

// Transactional preparation reads flags through the owning transaction so a
// badge granted earlier in the same script is immediately authoritative.
func configureBattleObedienceFromDB(database db.DBTX, battle *pokebattle.BattleState, charID int64) error {
	battle.PlayerTrainerID = charID
	battle.PlayerObedienceLevel = pokebattle.BaseObedienceLevel
	for _, badge := range []struct {
		flag  string
		level int
	}{{"EVENT_GOT_EARTHBADGE", pokebattle.MaxObedienceLevel}, {"EVENT_GOT_MARSHBADGE", 70}, {"EVENT_GOT_RAINBOWBADGE", 50}, {"EVENT_GOT_CASCADEBADGE", 30}} {
		on, err := queryEventFlag(database, charID, badge.flag)
		if err != nil {
			return err
		}
		if on {
			battle.PlayerObedienceLevel = badge.level
			break
		}
	}
	return nil
}
