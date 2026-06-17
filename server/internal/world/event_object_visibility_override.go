package world

import (
	"database/sql"
	"fmt"
	"strings"

	"capturequest/internal/db"
)

type objectVisibilityOverride struct {
	Visible bool
	Source  string
}

func SetCharacterObjectVisibilityOverrideByName(charID int64, objectName string, visible bool, source string) error {
	objectName = strings.TrimSpace(objectName)
	if charID == 0 || objectName == "" {
		return nil
	}
	var objectID int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id FROM phaser_objects WHERE name = $1 LIMIT 1`,
		objectName,
	).Scan(&objectID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("object visibility override unknown object %s", objectName)
		}
		return err
	}
	return SetCharacterObjectVisibilityOverride(charID, objectID, visible, source)
}

func SetCharacterObjectVisibilityOverride(charID int64, objectID int, visible bool, source string) error {
	if charID == 0 || objectID == 0 {
		return nil
	}
	if strings.TrimSpace(source) == "" {
		source = "CharacterObjectVisibilityOverride"
	}
	_, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_object_visibility_overrides
			(character_id, object_id, visible, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (character_id, object_id) DO UPDATE SET
			visible = EXCLUDED.visible,
			source = EXCLUDED.source,
			updated_at = CURRENT_TIMESTAMP`,
		charID,
		objectID,
		visible,
		source,
	)
	return err
}

func objectVisibilityOverridesForCharacter(charID int64) (map[int]objectVisibilityOverride, error) {
	overrides := make(map[int]objectVisibilityOverride)
	if charID == 0 {
		return overrides, nil
	}
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT object_id, visible, source
		FROM character_object_visibility_overrides
		WHERE character_id = $1`,
		charID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var objectID int
		var visible bool
		var source string
		if err := rows.Scan(&objectID, &visible, &source); err != nil {
			return nil, err
		}
		overrides[objectID] = objectVisibilityOverride{
			Visible: visible,
			Source:  source,
		}
	}
	return overrides, rows.Err()
}

func applyObjectVisibilityOverride(objectID int, visible bool, label string, overrides map[int]objectVisibilityOverride) (bool, string) {
	override, ok := overrides[objectID]
	if !ok {
		return visible, label
	}
	return override.Visible, override.Source
}
