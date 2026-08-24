package db

import (
	"database/sql"
	"fmt"
)

// EnsureWorldTileMutationSchema upgrades an existing runtime database before
// any tile consumers start querying it. Fresh databases receive the same
// columns from postgres_runtime_schema.sql; this path keeps long-lived local
// and deployed databases compatible without requiring a destructive import.
func EnsureWorldTileMutationSchema(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("world database is nil")
	}

	statements := []string{
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS is_native_game_data boolean NOT NULL DEFAULT false`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS coordinate_origin varchar(16) NOT NULL DEFAULT 'user'`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS content_origin varchar(16) NOT NULL DEFAULT 'user'`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS is_original_tile_location smallint NOT NULL DEFAULT 0`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS has_tile_edit smallint NOT NULL DEFAULT 0`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS is_tile_erased smallint NOT NULL DEFAULT 0`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_tile_image_id integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_collision_type integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_raw_foot_tile_id integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_talk_over_tile boolean`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_encounter_area_id integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_local_x integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_local_y integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_source_map_id integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS last_edited_by_char_id integer`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS last_edited_at timestamp`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS last_edit_source varchar(32)`,
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin world tile schema upgrade: %w", err)
	}
	defer tx.Rollback()

	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("upgrade world tile schema: %w", err)
		}
	}

	if _, err := tx.Exec(`
		UPDATE phaser_tiles
		SET
			is_original_tile_location = 1,
			is_native_game_data = TRUE,
			coordinate_origin = 'native',
			content_origin = CASE WHEN has_tile_edit = 1 OR is_user_placed = 1 THEN 'user' ELSE 'native' END,
			original_tile_image_id = COALESCE(original_tile_image_id, tile_image_id),
			original_collision_type = COALESCE(original_collision_type, collision_type),
			original_raw_foot_tile_id = COALESCE(original_raw_foot_tile_id, raw_foot_tile_id),
			original_talk_over_tile = COALESCE(original_talk_over_tile, talk_over_tile),
			original_encounter_area_id = COALESCE(original_encounter_area_id, encounter_area_id),
			original_local_x = COALESCE(original_local_x, local_x),
			original_local_y = COALESCE(original_local_y, local_y),
			original_source_map_id = COALESCE(original_source_map_id, source_map_id)
		WHERE source_map_id IS NOT NULL OR is_native_game_data = TRUE OR is_original_tile_location = 1`); err != nil {
		return fmt.Errorf("backfill world tile originals: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit world tile schema upgrade: %w", err)
	}
	return nil
}
