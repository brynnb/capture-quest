package main

import (
	"database/sql"
	"fmt"
	"log"
)

func markUnresolvedLastMapWarpPlaceholdersPostgres(pg *sql.DB) error {
	result, err := pg.Exec(`
		UPDATE phaser_warps
		SET warp_type = 'inactive',
		    warp_direction = NULL
		WHERE COALESCE(warp_type, '') <> 'elevator'
		  AND (destination_map_id IS NULL OR destination_x IS NULL OR destination_y IS NULL)
		  AND EXISTS (
		      SELECT 1
		      FROM phaser_warp_events source_event
		      JOIN phaser_maps source_map
		        ON source_map.id = phaser_warps.source_map_id
		      WHERE (
		              source_event.map_id = phaser_warps.source_map_id
		              OR (
		                  source_event.map_id IS NULL
		                  AND LOWER(REPLACE(source_event.map_name, '_', '')) = LOWER(REPLACE(source_map.name, '_', ''))
		              )
		            )
		        AND source_event.x = phaser_warps.x
		        AND source_event.y = phaser_warps.y
		        AND UPPER(source_event.dest_map) = 'LAST_MAP'
		  )`)
	if err != nil {
		return fmt.Errorf("mark unresolved LAST_MAP warp placeholders: %w", err)
	}
	count, _ := result.RowsAffected()
	log.Printf("  -> Marked %d unresolved LAST_MAP warp placeholders inactive", count)
	return nil
}
