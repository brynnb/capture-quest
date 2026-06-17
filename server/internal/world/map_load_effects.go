package world

import (
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
	return applyMapLoadScriptEffects(charID, mapID, mapNameForBoulderMapID(mapID), efm)
}

func ApplyMapLoadScriptEffectsForMapName(charID int64, mapName string, efm *EventFlagManager) (MapLoadEffect, error) {
	mapID := 0
	if id, err := mapIDForBoulderMapName(mapName); err == nil {
		mapID = id
	}
	return applyMapLoadScriptEffects(charID, mapID, mapName, efm)
}

func OverworldMapLoadNameForPosition(x, y int) string {
	// Route 20 is stored in the unified overworld at local_x + 20, local_y + 108.
	if x >= 20 && x <= 69 && y >= 108 && y <= 116 {
		return "ROUTE_20"
	}
	return ""
}

func applyMapLoadScriptEffects(charID int64, mapID int, mapName string, efm *EventFlagManager) (MapLoadEffect, error) {
	effect := MapLoadEffect{
		MapID:   mapID,
		MapName: mapName,
	}
	if charID == 0 || efm == nil {
		return effect, nil
	}

	switch effect.MapName {
	case "PALLET_TOWN":
		if !efm.CheckFlag(charID, "EVENT_DAISY_WALKING") &&
			efm.CheckFlag(charID, "EVENT_GOT_TOWN_MAP") &&
			efm.CheckFlag(charID, "EVENT_ENTERED_BLUES_HOUSE") {
			if err := efm.SetFlag(charID, "EVENT_DAISY_WALKING"); err != nil {
				return effect, err
			}
			effect.SetFlags = append(effect.SetFlags, "EVENT_DAISY_WALKING")
			if err := SetCharacterObjectVisibilityOverrideByName(charID, "BluesHouse_NPC_1", false, "PalletTownDaisyScript"); err != nil {
				return effect, err
			}
			if err := SetCharacterObjectVisibilityOverrideByName(charID, "BluesHouse_NPC_2", true, "PalletTownDaisyScript"); err != nil {
				return effect, err
			}
			effect.AffectedMapNames = appendUniqueStrings(effect.AffectedMapNames, "BLUES_HOUSE")
		}
		if efm.CheckFlag(charID, "EVENT_GOT_POKEBALLS_FROM_OAK") &&
			!efm.CheckFlag(charID, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2") {
			if err := efm.SetFlag(charID, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2"); err != nil {
				return effect, err
			}
			effect.SetFlags = append(effect.SetFlags, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2")
		}
	case "SEAFOAM_ISLANDS_1F":
		if err := efm.SetFlag(charID, "EVENT_IN_SEAFOAM_ISLANDS"); err != nil {
			return effect, err
		}
		effect.SetFlags = append(effect.SetFlags, "EVENT_IN_SEAFOAM_ISLANDS")
	case "ROUTE_20":
		if !efm.CheckFlag(charID, "EVENT_IN_SEAFOAM_ISLANDS") {
			return effect, nil
		}
		if err := resetMapLoadFlags(charID, efm, "EVENT_IN_SEAFOAM_ISLANDS"); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags, "EVENT_IN_SEAFOAM_ISLANDS")
		if efm.CheckFlag(charID, "EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE") &&
			efm.CheckFlag(charID, "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE") {
			if err := setSeafoamRoute20ObjectOverrides(charID, true,
				"SeafoamIslands1F_NPC_1",
				"SeafoamIslands1F_NPC_2",
			); err != nil {
				return effect, err
			}
			if err := setSeafoamRoute20ObjectOverrides(charID, false,
				"SeafoamIslandsB1F_NPC_1",
				"SeafoamIslandsB1F_NPC_2",
				"SeafoamIslandsB2F_NPC_1",
				"SeafoamIslandsB2F_NPC_2",
				"SeafoamIslandsB3F_NPC_3",
				"SeafoamIslandsB3F_NPC_4",
			); err != nil {
				return effect, err
			}
			if err := clearBoulderPositionsForMaps(charID, 192, 159, 160, 161); err != nil {
				return effect, err
			}
			effect.AffectedMapNames = appendUniqueStrings(effect.AffectedMapNames,
				"SEAFOAM_ISLANDS_1F",
				"SEAFOAM_ISLANDS_B1F",
				"SEAFOAM_ISLANDS_B2F",
				"SEAFOAM_ISLANDS_B3F",
			)
		}
		if efm.CheckFlag(charID, "EVENT_SEAFOAM4_BOULDER1_DOWN_HOLE") &&
			efm.CheckFlag(charID, "EVENT_SEAFOAM4_BOULDER2_DOWN_HOLE") {
			if err := setSeafoamRoute20ObjectOverrides(charID, true,
				"SeafoamIslandsB3F_NPC_1",
				"SeafoamIslandsB3F_NPC_2",
			); err != nil {
				return effect, err
			}
			if err := setSeafoamRoute20ObjectOverrides(charID, false,
				"SeafoamIslandsB4F_NPC_1",
				"SeafoamIslandsB4F_NPC_2",
			); err != nil {
				return effect, err
			}
			if err := clearBoulderPositionsForMaps(charID, 161, 162); err != nil {
				return effect, err
			}
			effect.AffectedMapNames = appendUniqueStrings(effect.AffectedMapNames,
				"SEAFOAM_ISLANDS_B3F",
				"SEAFOAM_ISLANDS_B4F",
			)
		}
	case "ROUTE_23":
		if err := resetMapLoadFlags(charID, efm,
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
		if err := clearBoulderPositionsForMaps(charID, 194, 198); err != nil {
			return effect, err
		}
		effect.AffectedMapNames = append(effect.AffectedMapNames, "VICTORY_ROAD_2F", "VICTORY_ROAD_3F")
	case "VICTORY_ROAD_2F":
		if err := resetMapLoadFlags(charID, efm, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH"); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH")
		if err := clearBoulderPositionsForMaps(charID, 108); err != nil {
			return effect, err
		}
		effect.AffectedMapNames = append(effect.AffectedMapNames, "VICTORY_ROAD_1F")
	case "INDIGO_PLATEAU_LOBBY":
		if err := resetMapLoadFlags(charID, efm, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH"); err != nil {
			return effect, err
		}
		effect.ResetFlags = append(effect.ResetFlags, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH")
		if err := clearBoulderPositionsForMaps(charID, 108); err != nil {
			return effect, err
		}
		effect.AffectedMapNames = append(effect.AffectedMapNames, "VICTORY_ROAD_1F")
	}
	return effect, nil
}

func resetMapLoadFlags(charID int64, efm *EventFlagManager, flags ...string) error {
	for _, flag := range flags {
		if err := efm.ResetFlag(charID, flag); err != nil {
			return err
		}
	}
	return nil
}

func setSeafoamRoute20ObjectOverrides(charID int64, visible bool, objectNames ...string) error {
	for _, objectName := range objectNames {
		if err := SetCharacterObjectVisibilityOverrideByName(charID, objectName, visible, "Route20SeafoamBoulderReset"); err != nil {
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

func clearBoulderPositionsForMaps(charID int64, mapIDs ...int) error {
	for _, mapID := range mapIDs {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			DELETE FROM character_object_positions cop
			USING phaser_objects po
			WHERE cop.character_id = $1
			  AND cop.map_id = $2
			  AND po.id = cop.object_id
			  AND po.sprite_name = 'SPRITE_BOULDER'`,
			charID,
			mapID,
		); err != nil {
			return fmt.Errorf("clear boulder positions for map %d: %w", mapID, err)
		}
	}
	return nil
}
