package main

import (
	"database/sql"
	"fmt"
	"log"
)

func markDynamicElevatorWarpPlaceholdersPostgres(pg *sql.DB) error {
	result, err := pg.Exec(`
		UPDATE phaser_warps
		SET warp_type = 'elevator',
		    warp_direction = NULL
		WHERE destination_map_id IN (
		      SELECT id
		      FROM phaser_maps
		      WHERE name = 'UNUSED_MAP_ED'
		  )
		  AND destination_x IS NULL
		  AND destination_y IS NULL
		  AND EXISTS (
		      SELECT 1
		      FROM phaser_elevator_floors ef
		      WHERE ef.elevator_map_id = phaser_warps.source_map_id
		  )`)
	if err != nil {
		return fmt.Errorf("mark dynamic elevator warp placeholders: %w", err)
	}
	count, _ := result.RowsAffected()
	log.Printf("  -> Marked %d dynamic elevator warp placeholders", count)
	return nil
}
