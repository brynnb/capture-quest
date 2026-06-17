package scriptsim

import (
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/world"
)

func seedObjectPosition(charID int64, defaultMapName string, pos FixtureObjectPosition) error {
	mapName := pos.MapName
	if mapName == "" {
		mapName = defaultMapName
	}
	mapID, err := mapIDForName(mapName)
	if err != nil {
		return err
	}

	objectID, err := fixtureObjectPositionID(mapName, pos)
	if err != nil {
		return err
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_object_positions (character_id, object_id, map_id, x, y)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (character_id, object_id) DO UPDATE SET
			map_id = EXCLUDED.map_id,
			x = EXCLUDED.x,
			y = EXCLUDED.y`,
		charID,
		objectID,
		mapID,
		pos.X,
		pos.Y,
	); err != nil {
		return fmt.Errorf("seed object position %d: %w", objectID, err)
	}
	return nil
}

func fixtureObjectPositionID(mapName string, pos FixtureObjectPosition) (int, error) {
	if pos.ObjectKey != "" {
		ids, err := world.ResolveCutsceneObjectKey(mapName, pos.ObjectKey)
		if err != nil {
			return 0, err
		}
		if len(ids) == 0 {
			return 0, fmt.Errorf("object key %s did not resolve on %s", pos.ObjectKey, mapName)
		}
		return ids[0], nil
	}
	if pos.ObjectName == "" {
		return 0, fmt.Errorf("objectPositions requires objectKey or objectName")
	}
	var objectID int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT po.id
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE pm.name = $1 AND po.name = $2
		LIMIT 1`,
		mapName,
		pos.ObjectName,
	).Scan(&objectID); err != nil {
		return 0, fmt.Errorf("lookup object %s/%s: %w", mapName, pos.ObjectName, err)
	}
	return objectID, nil
}
