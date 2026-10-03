package world

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
)

type MapLoadEffect struct {
	MapID            int
	MapName          string
	SetFlags         []string
	ResetFlags       []string
	AffectedMapNames []string
}

func (e MapLoadEffect) Changed() bool {
	return len(e.SetFlags) > 0 || len(e.ResetFlags) > 0 || len(e.AffectedMapNames) > 0
}

func ApplyMapLoadScriptEffects(charID int64, mapID int, efm *EventFlagManager) (MapLoadEffect, error) {
	return commitStandaloneMapLoadEffects(context.Background(), charID, mapID, "", efm)
}

func ApplyMapLoadScriptEffectsForMapName(charID int64, mapName string, efm *EventFlagManager) (MapLoadEffect, error) {
	return commitStandaloneMapLoadEffects(context.Background(), charID, 0, mapName, efm)
}

func commitStandaloneMapLoadEffects(ctx context.Context, charID int64, mapID int, mapName string, efm *EventFlagManager) (MapLoadEffect, error) {
	if charID == 0 || efm == nil {
		return MapLoadEffect{}, nil
	}
	var effect MapLoadEffect
	err := db.Transaction(ctx, efm.db, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if mapName == "" {
			if err := tx.QueryRow(`SELECT name FROM phaser_maps WHERE id=$1`, mapID).Scan(&mapName); err != nil {
				return err
			}
		} else {
			if err := tx.QueryRow(`SELECT id FROM phaser_maps WHERE name=$1`, mapName).Scan(&mapID); err != nil {
				return err
			}
		}
		var err error
		effect, err = applyMapLoadScriptEffectsIn(tx, charID, mapID, mapName)
		return err
	})
	if err != nil {
		return MapLoadEffect{}, err
	}
	if err := efm.LoadFlagsContext(ctx, charID); err != nil {
		return effect, err
	}
	return effect, nil
}

func applyMapLoadScriptEffectsIn(tx db.DBTX, charID int64, mapID int, mapName string) (MapLoadEffect, error) {
	effect := MapLoadEffect{
		MapID:   mapID,
		MapName: mapName,
	}
	if err := db.RequireTransaction(tx); err != nil {
		return effect, err
	}
	flags := make(map[string]bool)
	rows, err := tx.Query(`SELECT flag_name FROM character_event_flags WHERE character_id=$1`, charID)
	if err != nil {
		return effect, err
	}
	for rows.Next() {
		var flag string
		if err := rows.Scan(&flag); err != nil {
			rows.Close()
			return effect, err
		}
		flags[flag] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return effect, err
	}

	switch effect.MapName {
	case "PALLET_TOWN":
		if !flags["EVENT_DAISY_WALKING"] &&
			flags["EVENT_GOT_TOWN_MAP"] &&
			flags["EVENT_ENTERED_BLUES_HOUSE"] {
			if err := writeEventFlag(tx, charID, "EVENT_DAISY_WALKING", true); err != nil {
				return effect, err
			}
			effect.SetFlags = append(effect.SetFlags, "EVENT_DAISY_WALKING")
			if err := setCharacterObjectVisibilityOverrideByName(tx, charID, "BluesHouse_NPC_1", false, "PalletTownDaisyScript"); err != nil {
				return effect, err
			}
			if err := setCharacterObjectVisibilityOverrideByName(tx, charID, "BluesHouse_NPC_2", true, "PalletTownDaisyScript"); err != nil {
				return effect, err
			}
			effect.AffectedMapNames = appendUniqueStrings(effect.AffectedMapNames, "BLUES_HOUSE")
		}
		if flags["EVENT_GOT_POKEBALLS_FROM_OAK"] &&
			!flags["EVENT_PALLET_AFTER_GETTING_POKEBALLS_2"] {
			if err := writeEventFlag(tx, charID, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2", true); err != nil {
				return effect, err
			}
			effect.SetFlags = append(effect.SetFlags, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2")
		}
	case "SEAFOAM_ISLANDS_1F":
		if err := writeEventFlag(tx, charID, "EVENT_IN_SEAFOAM_ISLANDS", true); err != nil {
			return effect, err
		}
		effect.SetFlags = append(effect.SetFlags, "EVENT_IN_SEAFOAM_ISLANDS")
	case "ROUTE_20":
		if !flags["EVENT_IN_SEAFOAM_ISLANDS"] {
			return effect, nil
		}
		if err := resetMapLoadFlags(tx, charID, "EVENT_IN_SEAFOAM_ISLANDS"); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags, "EVENT_IN_SEAFOAM_ISLANDS")
		if flags["EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE"] &&
			flags["EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"] {
			if err := setSeafoamRoute20ObjectOverrides(tx, charID, true,
				"SeafoamIslands1F_NPC_1",
				"SeafoamIslands1F_NPC_2",
			); err != nil {
				return effect, err
			}
			if err := setSeafoamRoute20ObjectOverrides(tx, charID, false,
				"SeafoamIslandsB1F_NPC_1",
				"SeafoamIslandsB1F_NPC_2",
				"SeafoamIslandsB2F_NPC_1",
				"SeafoamIslandsB2F_NPC_2",
				"SeafoamIslandsB3F_NPC_3",
				"SeafoamIslandsB3F_NPC_4",
			); err != nil {
				return effect, err
			}
			if err := clearBoulderPositionsForMaps(tx, charID, 192, 159, 160, 161); err != nil {
				return effect, err
			}
			effect.AffectedMapNames = appendUniqueStrings(effect.AffectedMapNames,
				"SEAFOAM_ISLANDS_1F",
				"SEAFOAM_ISLANDS_B1F",
				"SEAFOAM_ISLANDS_B2F",
				"SEAFOAM_ISLANDS_B3F",
			)
		}
		if flags["EVENT_SEAFOAM4_BOULDER1_DOWN_HOLE"] &&
			flags["EVENT_SEAFOAM4_BOULDER2_DOWN_HOLE"] {
			if err := setSeafoamRoute20ObjectOverrides(tx, charID, true,
				"SeafoamIslandsB3F_NPC_1",
				"SeafoamIslandsB3F_NPC_2",
			); err != nil {
				return effect, err
			}
			if err := setSeafoamRoute20ObjectOverrides(tx, charID, false,
				"SeafoamIslandsB4F_NPC_1",
				"SeafoamIslandsB4F_NPC_2",
			); err != nil {
				return effect, err
			}
			if err := clearBoulderPositionsForMaps(tx, charID, 161, 162); err != nil {
				return effect, err
			}
			effect.AffectedMapNames = appendUniqueStrings(effect.AffectedMapNames,
				"SEAFOAM_ISLANDS_B3F",
				"SEAFOAM_ISLANDS_B4F",
			)
		}
	case "ROUTE_23":
		if err := resetMapLoadFlags(tx, charID,
			"EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH1",
			"EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH2",
			"EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH1",
			"EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2",
		); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags,
			"EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH1",
			"EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH2",
			"EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH1",
			"EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2",
		)
		if err := clearBoulderPositionsForMaps(tx, charID, 194, 198); err != nil {
			return effect, err
		}
		effect.AffectedMapNames = append(effect.AffectedMapNames, "VICTORY_ROAD_2F", "VICTORY_ROAD_3F")
	case "VICTORY_ROAD_2F":
		if err := resetMapLoadFlags(tx, charID, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH"); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH")
		if err := clearBoulderPositionsForMaps(tx, charID, 108); err != nil {
			return effect, err
		}
		effect.AffectedMapNames = append(effect.AffectedMapNames, "VICTORY_ROAD_1F")
	case "INDIGO_PLATEAU_LOBBY":
		if err := resetMapLoadFlags(tx, charID, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH"); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH")
		if err := clearBoulderPositionsForMaps(tx, charID, 108); err != nil {
			return effect, err
		}
		effect.AffectedMapNames = append(effect.AffectedMapNames, "VICTORY_ROAD_1F")
	}
	return effect, nil
}

func resetMapLoadFlags(tx db.DBTX, charID int64, flags ...string) error {
	for _, flag := range flags {
		if err := writeEventFlag(tx, charID, flag, false); err != nil {
			return err
		}
	}
	return nil
}

func setSeafoamRoute20ObjectOverrides(tx db.DBTX, charID int64, visible bool, objectNames ...string) error {
	for _, objectName := range objectNames {
		if err := setCharacterObjectVisibilityOverrideByName(tx, charID, objectName, visible, "Route20SeafoamBoulderReset"); err != nil {
			return err
		}
	}
	return nil
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, addition := range additions {
		if addition == "" || seen[addition] {
			continue
		}
		values = append(values, addition)
		seen[addition] = true
	}
	return values
}

func clearBoulderPositionsForMaps(tx db.DBTX, charID int64, mapIDs ...int) error {
	for _, mapID := range mapIDs {
		if _, err := tx.Exec(`DELETE FROM character_object_positions
   WHERE character_id=$1 AND map_id=$2
   AND object_id IN(SELECT id FROM phaser_objects WHERE sprite_name='SPRITE_BOULDER')`,
			charID,
			mapID,
		); err != nil {
			return fmt.Errorf("clear boulder positions for map %d: %w", mapID, err)
		}
	}
	return nil
}

// mapLoadArrival is an accepted destination, not a client-provided effect identity.
type mapLoadArrival struct {
	MapID, X, Y                                  int
	WritePosition, ValidateCatalog, ApplyEffects bool
}

// Position, Safari transition and all map-load mutations share the same commit.
func commitMapLoad(ctx context.Context, database *sql.DB, charID int64, arrival mapLoadArrival) (MapLoadEffect, error) {
	var effect MapLoadEffect
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if arrival.WritePosition {
			if arrival.ValidateCatalog {
				if err := validateClientDestinationIn(tx, arrival.MapID, arrival.X, arrival.Y); err != nil {
					return err
				}
			}
			if err := saveFieldDestinationIn(tx, charID, arrival.MapID, arrival.X, arrival.Y); err != nil {
				return err
			}
		}
		if !arrival.ApplyEffects {
			return nil
		}
		effectMapID := arrival.MapID
		var effectMapName string
		if arrival.MapID == UnifiedOverworldMapID {
			var err error
			effectMapID, effectMapName, err = nativeOverworldMapAt(tx.QueryRow, arrival.X, arrival.Y)
			if err != nil {
				return err
			}
			if effectMapName == "" {
				return nil
			}
		} else {
			if err := tx.QueryRow(`SELECT name FROM phaser_maps WHERE id=$1`, arrival.MapID).Scan(&effectMapName); err != nil {
				return err
			}
		}
		var err error
		effect, err = applyMapLoadScriptEffectsIn(tx, charID, effectMapID, effectMapName)
		return err
	})
	if err != nil {
		return MapLoadEffect{}, err
	}
	return effect, nil
}
