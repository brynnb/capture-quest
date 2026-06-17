package world

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db"
)

const (
	cardKeyItemID           = 48
	silphCardKeyScriptLabel = "SilphCardKeyDoor"
)

type SilphCardKeyDoor struct {
	MapName         string
	FloorLabel      string
	DoorIndex       int
	TextConstant    string
	Flag            string
	BlockX          int
	BlockY          int
	OpenTileImageID int
}

type SilphCardKeyOutcome struct {
	Door        SilphCardKeyDoor
	Dialogue    []string
	HasCardKey  bool
	Opened      bool
	AlreadyOpen bool
	Changed     bool
}

var silphCardKeyDoors = []SilphCardKeyDoor{
	silphCardKeyDoor("SILPH_CO_2F", "2F", 1, "EVENT_SILPH_CO_2_UNLOCKED_DOOR1", 2, 2, 758),
	silphCardKeyDoor("SILPH_CO_2F", "2F", 2, "EVENT_SILPH_CO_2_UNLOCKED_DOOR2", 2, 5, 758),
	silphCardKeyDoor("SILPH_CO_3F", "3F", 1, "EVENT_SILPH_CO_3_UNLOCKED_DOOR1", 4, 4, 758),
	silphCardKeyDoor("SILPH_CO_3F", "3F", 2, "EVENT_SILPH_CO_3_UNLOCKED_DOOR2", 8, 4, 758),
	silphCardKeyDoor("SILPH_CO_4F", "4F", 1, "EVENT_SILPH_CO_4_UNLOCKED_DOOR1", 2, 6, 758),
	silphCardKeyDoor("SILPH_CO_4F", "4F", 2, "EVENT_SILPH_CO_4_UNLOCKED_DOOR2", 6, 4, 758),
	silphCardKeyDoor("SILPH_CO_5F", "5F", 1, "EVENT_SILPH_CO_5_UNLOCKED_DOOR1", 3, 2, 758),
	silphCardKeyDoor("SILPH_CO_5F", "5F", 2, "EVENT_SILPH_CO_5_UNLOCKED_DOOR2", 3, 6, 758),
	silphCardKeyDoor("SILPH_CO_5F", "5F", 3, "EVENT_SILPH_CO_5_UNLOCKED_DOOR3", 7, 5, 758),
	silphCardKeyDoor("SILPH_CO_6F", "6F", 1, "EVENT_SILPH_CO_6_UNLOCKED_DOOR", 2, 6, 758),
	silphCardKeyDoor("SILPH_CO_7F", "7F", 1, "EVENT_SILPH_CO_7_UNLOCKED_DOOR1", 5, 3, 758),
	silphCardKeyDoor("SILPH_CO_7F", "7F", 2, "EVENT_SILPH_CO_7_UNLOCKED_DOOR2", 10, 2, 758),
	silphCardKeyDoor("SILPH_CO_7F", "7F", 3, "EVENT_SILPH_CO_7_UNLOCKED_DOOR3", 10, 6, 758),
	silphCardKeyDoor("SILPH_CO_8F", "8F", 1, "EVENT_SILPH_CO_8_UNLOCKED_DOOR", 3, 4, 758),
	silphCardKeyDoor("SILPH_CO_9F", "9F", 1, "EVENT_SILPH_CO_9_UNLOCKED_DOOR1", 1, 4, 758),
	silphCardKeyDoor("SILPH_CO_9F", "9F", 2, "EVENT_SILPH_CO_9_UNLOCKED_DOOR2", 9, 2, 758),
	silphCardKeyDoor("SILPH_CO_9F", "9F", 3, "EVENT_SILPH_CO_9_UNLOCKED_DOOR3", 9, 5, 758),
	silphCardKeyDoor("SILPH_CO_9F", "9F", 4, "EVENT_SILPH_CO_9_UNLOCKED_DOOR4", 5, 6, 758),
	silphCardKeyDoor("SILPH_CO_10F", "10F", 1, "EVENT_SILPH_CO_10_UNLOCKED_DOOR", 5, 4, 758),
	silphCardKeyDoor("SILPH_CO_11F", "11F", 1, "EVENT_SILPH_CO_11_UNLOCKED_DOOR", 3, 6, 123),
}

var silphCardKeyDoorsByText = func() map[string]SilphCardKeyDoor {
	doors := make(map[string]SilphCardKeyDoor, len(silphCardKeyDoors))
	for _, door := range silphCardKeyDoors {
		doors[door.TextConstant] = door
	}
	return doors
}()

func SilphCardKeyScriptLabel() string {
	return silphCardKeyScriptLabel
}

func IsSilphCardKeyDoorTextConstant(textConstant string) bool {
	_, ok := SilphCardKeyDoorForTextConstant(textConstant)
	return ok
}

func SilphCardKeyDoorForTextConstant(textConstant string) (SilphCardKeyDoor, bool) {
	door, ok := silphCardKeyDoorsByText[textConstant]
	return door, ok
}

func HandleSilphCardKeyDoor(charID int64, textConstant string, efm *EventFlagManager) (*SilphCardKeyOutcome, error) {
	door, ok := SilphCardKeyDoorForTextConstant(textConstant)
	if !ok {
		return nil, fmt.Errorf("unknown Silph Card Key door text constant %q", textConstant)
	}

	hasCardKey, err := characterHasCQItem(charID, cardKeyItemID)
	if err != nil {
		return nil, err
	}
	if !hasCardKey {
		return &SilphCardKeyOutcome{
			Door:       door,
			Dialogue:   []string{"Darn! It needs a CARD KEY!"},
			HasCardKey: false,
		}, nil
	}

	alreadyOpen := efm != nil && efm.CheckFlag(charID, door.Flag)
	if !alreadyOpen && efm != nil {
		if err := efm.SetFlag(charID, door.Flag); err != nil {
			return nil, err
		}
	}

	return &SilphCardKeyOutcome{
		Door:        door,
		Dialogue:    []string{"Bingo!", "The CARD KEY opened the door!"},
		HasCardKey:  true,
		Opened:      true,
		AlreadyOpen: alreadyOpen,
		Changed:     !alreadyOpen,
	}, nil
}

func silphCardKeyDoor(mapName, floorLabel string, doorIndex int, flag string, blockX, blockY, openTileImageID int) SilphCardKeyDoor {
	return SilphCardKeyDoor{
		MapName:         mapName,
		FloorLabel:      floorLabel,
		DoorIndex:       doorIndex,
		TextConstant:    fmt.Sprintf("TEXT_SILPH_CARD_KEY_DOOR_%s_%d", floorLabel, doorIndex),
		Flag:            flag,
		BlockX:          blockX,
		BlockY:          blockY,
		OpenTileImageID: openTileImageID,
	}
}

func characterHasCQItem(charID int64, itemID int) (bool, error) {
	var quantity sql.NullInt64
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT SUM(ii.quantity)
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		WHERE ci.character_id = $1 AND ii.item_id = $2`,
		charID, itemID).Scan(&quantity); err != nil {
		return false, fmt.Errorf("check inventory item %d for char %d: %w", itemID, charID, err)
	}
	return quantity.Valid && quantity.Int64 > 0, nil
}
