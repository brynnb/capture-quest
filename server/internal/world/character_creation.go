package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"capturequest/internal/config"
	"capturequest/internal/db"
)

const maximumAccountCharacters = 8

// CharacterCreate owns one new identity, its profile, binds and optional local
// fixtures. Repeating a name is a conflict, never permission to reset an existing
// character. The account row serializes creation admission across connections.
func CharacterCreate(ctx context.Context, database *sql.DB, accountID int64, req CharCreateRequest) (int64, error) {
	if accountID <= 0 || req.FactionID == 0 || req.FactionID > 32767 || req.CharClass == 0 || req.CharClass > 255 || req.Gender > 2 || req.HomeTownMapID < 0 {
		return 0, fmt.Errorf("invalid character creation selection")
	}
	profile := &CharacterCreateProfile{Binds: make([]BindPoint, 5)}
	InitializeNewCharacterProfile(profile, req)
	ApplyDefaultSpawn(profile)
	for i := range profile.Binds {
		profile.Binds[i] = BindPoint{MapID: profile.MapID, X: profile.X, Y: profile.Y, Z: profile.Z, Heading: profile.Heading}
	}
	optionsJSON, err := json.Marshal(profile.Options)
	if err != nil {
		return 0, err
	}
	seedFixtures := false
	if localDevFixturesEnabled() {
		cfg, err := config.Get()
		if err != nil {
			return 0, err
		}
		seedFixtures = cfg.Local
	}

	var characterID int64
	err = db.Transaction(ctx, database, func(tx db.DBTX) error {
		var lockedAccount int64
		if err := tx.QueryRow(`SELECT id FROM account WHERE id=$1 FOR UPDATE`, accountID).Scan(&lockedAccount); err != nil {
			return fmt.Errorf("lock creation account: %w", err)
		}
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM character_data WHERE account_id=$1 AND deleted_at IS NULL`, accountID).Scan(&count); err != nil {
			return err
		}
		if count >= maximumAccountCharacters {
			return fmt.Errorf("account already has %d active characters", count)
		}
		valid, message, err := ValidateNameContext(ctx, tx.(db.ContextDBTX), req.Name)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("invalid character name: %s", message)
		}
		var faction, class, hometown bool
		if err := tx.QueryRow(`SELECT
			EXISTS(SELECT 1 FROM poke_factions WHERE id=$1 AND is_playable=1 AND is_starting=1),
			EXISTS(SELECT 1 FROM poke_classes WHERE id=$2),
			EXISTS(SELECT 1 FROM poke_start_cities WHERE map_id=$3)`,
			req.FactionID, req.CharClass, req.HomeTownMapID).Scan(&faction, &class, &hometown); err != nil {
			return err
		}
		if !faction || !class || !hometown {
			return fmt.Errorf("character creation selection is unavailable")
		}
		if err := tx.QueryRow(`INSERT INTO character_data
			(account_id,name,gm,map_id,x,y,z,heading,faction_id,class,gender,last_login,birthday,options)
			VALUES($1,$2,0,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)
			RETURNING id`,
			accountID, profile.Name, profile.MapID, profile.X, profile.Y, profile.Z, profile.Heading,
			profile.FactionID, profile.CharClass, profile.Gender, profile.LastLogin, profile.Birthday, string(optionsJSON)).Scan(&characterID); err != nil {
			return fmt.Errorf("insert new character: %w", err)
		}
		for slot, bind := range profile.Binds {
			if _, err := tx.Exec(`INSERT INTO character_bind(id,map_id,x,y,z,heading,slot) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				characterID, bind.MapID, bind.X, bind.Y, bind.Z, bind.Heading, slot); err != nil {
				return fmt.Errorf("create character bind %d: %w", slot, err)
			}
		}
		if seedFixtures {
			if err := seedLocalDevCreationFixturesIn(tx, characterID); err != nil {
				return fmt.Errorf("create local character fixtures: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return characterID, nil
}
