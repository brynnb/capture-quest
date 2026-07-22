package main

import (
	"database/sql"
	"fmt"
	"log"
)

// markOrphanedBlockedCarpetWarpsPostgres disables generated fixed-destination
// carpet/directional rows that point out from blocked source tiles without
// nearby inverse or paired-source evidence that they are intended entrances.
func markOrphanedBlockedCarpetWarpsPostgres(pg *sql.DB) error {
	result, err := pg.Exec(`
		UPDATE phaser_warps
		SET warp_type = 'inactive',
		    warp_direction = NULL
		WHERE destination_map_id IS NOT NULL
		  AND destination_x IS NOT NULL
		  AND destination_y IS NOT NULL
		  AND COALESCE(warp_type, 'door') IN ('carpet', 'directional')
		  AND EXISTS (
		      SELECT 1
		      FROM phaser_maps source_map
		      JOIN phaser_tiles source_tile
		        ON source_tile.x = phaser_warps.x
		       AND source_tile.y = phaser_warps.y
		       AND (
		            (source_map.is_overworld = 1 AND source_tile.map_id IS NULL)
		            OR (source_map.is_overworld = 0 AND source_tile.map_id = source_map.id)
		       )
		      WHERE source_map.id = phaser_warps.source_map_id
		        AND COALESCE(source_tile.collision_type, 0) = 0
		  )
		  AND NOT EXISTS (
		      SELECT 1
		      FROM phaser_warps inverse_warp
		      WHERE inverse_warp.source_map_id = phaser_warps.destination_map_id
		        AND inverse_warp.destination_map_id = phaser_warps.source_map_id
		        AND inverse_warp.destination_x IS NOT NULL
		        AND inverse_warp.destination_y IS NOT NULL
		        AND ABS(inverse_warp.x - phaser_warps.destination_x) <= 2
		        AND ABS(inverse_warp.y - phaser_warps.destination_y) <= 2
		        AND ABS(inverse_warp.destination_x - phaser_warps.x) <= 2
		        AND ABS(inverse_warp.destination_y - phaser_warps.y) <= 2
		  )
		  AND NOT EXISTS (
		      SELECT 1
		      FROM phaser_warps source_pair
		      WHERE source_pair.id <> phaser_warps.id
		        AND source_pair.source_map_id = phaser_warps.source_map_id
		        AND source_pair.destination_map_id = phaser_warps.destination_map_id
		        AND source_pair.destination_x = phaser_warps.destination_x
		        AND source_pair.destination_y = phaser_warps.destination_y
		        AND ABS(source_pair.x - phaser_warps.x) <= 2
		        AND ABS(source_pair.y - phaser_warps.y) <= 2
		  )`)
	if err != nil {
		return fmt.Errorf("mark orphaned blocked carpet warps inactive: %w", err)
	}
	count, _ := result.RowsAffected()
	log.Printf("  -> Marked %d orphaned blocked carpet warps inactive", count)
	return nil
}
