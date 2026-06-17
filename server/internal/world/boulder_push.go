package world

import (
	"database/sql"
	"strings"

	"capturequest/internal/db"
)

const (
	BoulderNeedsStrengthMessage = "This requires STRENGTH to move!"
	boulderNoBoulderMessage     = "There is no boulder to push."
)

type BoulderPushResult struct {
	Success      bool
	Message      string
	ObjectID     int
	ObjectName   string
	MapID        int
	MapName      string
	Direction    string
	FromX        int
	FromY        int
	ToX          int
	ToY          int
	Dropped      bool
	FlagSet      string
	AffectedMaps []string
	StrengthUsed *FieldMoveUseResult
}

type BoulderObjectState struct {
	ObjectID int
	MapID    int
	Name     string
	Text     string
	X        int
	Y        int
	Visible  bool
	Label    string
}

func TryPushBoulder(charID int64, mapID int, playerX, playerY int, direction string, activateStrength bool, efm *EventFlagManager) (BoulderPushResult, error) {
	result := BoulderPushResult{
		MapID:     mapID,
		MapName:   mapNameForBoulderMapID(mapID),
		Direction: normalizeBoulderDirection(direction),
	}
	if result.Direction == "" {
		result.Message = "Unknown push direction."
		return result, nil
	}

	dx, dy := boulderDirectionDelta(result.Direction)
	frontX, frontY := playerX+dx, playerY+dy
	targetX, targetY := frontX+dx, frontY+dy

	boulders, err := BoulderObjectsForCharacter(charID, mapID, efm)
	if err != nil {
		return result, err
	}
	boulder, ok := visibleBoulderAt(boulders, frontX, frontY, 0)
	if !ok {
		result.Message = boulderNoBoulderMessage
		return result, nil
	}
	result.ObjectID = boulder.ObjectID
	result.ObjectName = boulder.Name
	result.FromX = boulder.X
	result.FromY = boulder.Y
	result.ToX = targetX
	result.ToY = targetY

	if activateStrength {
		used := TryUseFieldMove(charID, mapID, "STRENGTH", efm)
		result.StrengthUsed = &used
		if !used.Success {
			result.Message = used.Message
			return result, nil
		}
	}
	if !IsFieldMoveActive(charID, mapID, "STRENGTH") {
		result.Message = BoulderNeedsStrengthMessage
		return result, nil
	}

	if !boulderTargetWalkable(charID, mapID, targetX, targetY, efm) {
		result.Message = "The boulder won't budge."
		return result, nil
	}
	if _, occupied := visibleBoulderAt(boulders, targetX, targetY, boulder.ObjectID); occupied {
		result.Message = "The boulder won't budge."
		return result, nil
	}

	if err := setCharacterObjectPosition(charID, boulder.ObjectID, mapID, targetX, targetY); err != nil {
		return result, err
	}

	if hole, ok := SeafoamBoulderHoleAt(result.MapName, targetX, targetY); ok {
		outcome, err := HandleSeafoamBoulderHole(charID, result.MapName, hole.HoleIndex, efm)
		if err != nil {
			return result, err
		}
		_ = clearCharacterObjectPosition(charID, boulder.ObjectID)
		result.Dropped = true
		result.FlagSet = outcome.Hole.Flag
		result.AffectedMaps = append(result.AffectedMaps, outcome.Hole.DestinationMapName)
	}
	if target, ok := VictoryRoadBoulderTargetAt(result.MapName, targetX, targetY); ok {
		outcome, err := HandleVictoryRoadBoulderTarget(charID, target, efm)
		if err != nil {
			return result, err
		}
		if outcome.Target.DropsThroughHole {
			_ = clearCharacterObjectPosition(charID, boulder.ObjectID)
			result.Dropped = true
		}
		result.FlagSet = outcome.Target.Flag
		if outcome.Target.DestinationMapName != "" {
			result.AffectedMaps = append(result.AffectedMaps, outcome.Target.DestinationMapName)
		}
	}

	result.Success = true
	result.Message = "The boulder moved."
	return result, nil
}

func BoulderObjectsForCharacter(charID int64, mapID int, efm *EventFlagManager) ([]BoulderObjectState, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id, po.map_id, po.name, COALESCE(po.text, ''),
		       COALESCE(cop.x, po.x, po.local_x) AS x,
		       COALESCE(cop.y, po.y, po.local_y) AS y
		FROM phaser_objects po
		LEFT JOIN character_object_positions cop
		  ON cop.character_id = $1 AND cop.object_id = po.id
		WHERE po.map_id = $2 AND po.sprite_name = 'SPRITE_BOULDER'
		ORDER BY po.id`,
		charID,
		mapID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules, err := eventObjectVisibilityForMap(mapID)
	if err != nil {
		return nil, err
	}
	overrides, err := objectVisibilityOverridesForCharacter(charID)
	if err != nil {
		return nil, err
	}

	boulders := []BoulderObjectState{}
	for rows.Next() {
		var boulder BoulderObjectState
		if err := rows.Scan(
			&boulder.ObjectID,
			&boulder.MapID,
			&boulder.Name,
			&boulder.Text,
			&boulder.X,
			&boulder.Y,
		); err != nil {
			return nil, err
		}
		boulder.Visible, boulder.Label = currentEventObjectVisibility(charID, efm, boulder.Name, rules)
		boulder.Visible, boulder.Label = applyObjectVisibilityOverride(boulder.ObjectID, boulder.Visible, boulder.Label, overrides)
		boulders = append(boulders, boulder)
	}
	return boulders, rows.Err()
}

func ApplyCharacterObjectPositions(charID int64, actors []PhaserActor) []PhaserActor {
	if len(actors) == 0 || charID == 0 {
		return actors
	}
	positions, err := characterObjectPositions(charID)
	if err != nil {
		return actors
	}
	if len(positions) == 0 {
		return actors
	}
	for i := range actors {
		pos, ok := positions[actors[i].DbID]
		if !ok {
			continue
		}
		x, y := pos.X, pos.Y
		actors[i].X = &x
		actors[i].Y = &y
	}
	return actors
}

type objectPosition struct {
	X int
	Y int
}

func characterObjectPositions(charID int64) (map[int]objectPosition, error) {
	rows, err := db.GlobalWorldDB.DB.Query(
		`SELECT object_id, x, y FROM character_object_positions WHERE character_id = $1`,
		charID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := make(map[int]objectPosition)
	for rows.Next() {
		var objectID int
		var pos objectPosition
		if err := rows.Scan(&objectID, &pos.X, &pos.Y); err != nil {
			return nil, err
		}
		positions[objectID] = pos
	}
	return positions, rows.Err()
}

func setCharacterObjectPosition(charID int64, objectID, mapID, x, y int) error {
	_, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_object_positions (character_id, object_id, map_id, x, y)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (character_id, object_id) DO UPDATE SET
			map_id = EXCLUDED.map_id,
			x = EXCLUDED.x,
			y = EXCLUDED.y`,
		charID,
		objectID,
		mapID,
		x,
		y,
	)
	return err
}

func clearCharacterObjectPosition(charID int64, objectID int) error {
	_, err := db.GlobalWorldDB.DB.Exec(
		`DELETE FROM character_object_positions WHERE character_id = $1 AND object_id = $2`,
		charID,
		objectID,
	)
	return err
}

func visibleBoulderAt(boulders []BoulderObjectState, x, y, excludeObjectID int) (BoulderObjectState, bool) {
	for _, boulder := range boulders {
		if !boulder.Visible || boulder.ObjectID == excludeObjectID {
			continue
		}
		if boulder.X == x && boulder.Y == y {
			return boulder, true
		}
	}
	return BoulderObjectState{}, false
}

func boulderTargetWalkable(charID int64, mapID, x, y int, efm *EventFlagManager) bool {
	state, err := baseEventTileState(mapID, x, y)
	if err != nil {
		return false
	}
	if overrides := EventTileCollisionOverrides(charID, mapID, efm); len(overrides) > 0 {
		if collisionType, ok := overrides[tileKey(x, y)]; ok {
			return collisionType > 0
		}
	}
	return state.CollisionType > 0
}

func mapNameForBoulderMapID(mapID int) string {
	var name sql.NullString
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM phaser_maps WHERE id = $1`, mapID).Scan(&name); err != nil {
		return ""
	}
	if !name.Valid {
		return ""
	}
	return name.String
}

func normalizeBoulderDirection(direction string) string {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "UP", "DOWN", "LEFT", "RIGHT":
		return strings.ToUpper(strings.TrimSpace(direction))
	default:
		return ""
	}
}

func boulderDirectionDelta(direction string) (int, int) {
	switch direction {
	case "UP":
		return 0, -1
	case "DOWN":
		return 0, 1
	case "LEFT":
		return -1, 0
	case "RIGHT":
		return 1, 0
	default:
		return 0, 0
	}
}
