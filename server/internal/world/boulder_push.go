package world

import (
	"context"
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

// Simulator callers retain the public entry point; runtime supplies its owned
// context and database directly. Both use the same transaction/domain operation.
func TryPushBoulder(charID int64, mapID, playerX, playerY int, direction string, activateStrength bool, efm *EventFlagManager) (BoulderPushResult, error) {
	return pushBoulder(context.Background(), db.GlobalWorldDB.DB, charID, mapID, playerX, playerY, direction, activateStrength, efm)
}

func BoulderObjectsForCharacter(charID int64, mapID int, efm *EventFlagManager) ([]BoulderObjectState, error) {
	return boulderObjectsForCharacterContext(context.Background(), db.GlobalWorldDB.DB, charID, mapID, efm)
}

func boulderObjectsForCharacterContext(ctx context.Context, database db.ContextDBTX, charID int64, mapID int, efm *EventFlagManager) ([]BoulderObjectState, error) {
	rows, err := database.QueryContext(ctx, `
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
		boulders = append(boulders, boulder)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rules, err := eventObjectVisibilityForMapContext(ctx, database, mapID)
	if err != nil {
		return nil, err
	}
	overrides, err := objectVisibilityOverridesForCharacterContext(ctx, database, charID)
	if err != nil {
		return nil, err
	}

	for i := range boulders {
		boulder := &boulders[i]
		boulder.Visible, boulder.Label = currentEventObjectVisibility(charID, efm, boulder.Name, rules)
		boulder.Visible, boulder.Label = applyObjectVisibilityOverride(boulder.ObjectID, boulder.Visible, boulder.Label, overrides)
	}
	return boulders, nil
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
	return characterObjectPositionsContext(context.Background(), db.GlobalWorldDB.DB, charID)
}

func characterObjectPositionsContext(ctx context.Context, database db.ContextDBTX, charID int64) (map[int]objectPosition, error) {
	rows, err := database.QueryContext(ctx,
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
