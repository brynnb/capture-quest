package world

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/session"
)

type tileCoord struct {
	X int
	Y int
}

type worldTileMutationContext struct {
	MapID        int
	StorageMapID any
	CharID       int64
	Source       string
}

type worldTileMutationResult struct {
	Edits   []TileEdit
	Changed int
	MapID   int
}

func canAdminEditWorldTiles(ses *session.Session) bool {
	if ses == nil || !ses.Authenticated {
		return false
	}
	if getAccountStatus(ses.AccountID) > 0 {
		return true
	}
	if ses.HasValidClient() {
		charData := ses.Client.CharData()
		return charData != nil && charData.Gm > 0
	}
	return false
}

func characterIDForTileMutation(ses *session.Session) int64 {
	if ses != nil && ses.HasValidClient() {
		if charData := ses.Client.CharData(); charData != nil {
			return int64(charData.ID)
		}
	}
	return 0
}

func newWorldTileMutationContext(ses *session.Session, wh *WorldHandler, mapID int, source string) (worldTileMutationContext, error) {
	normalizedMapID, err := normalizeWorldTileMapID(wh, mapID)
	if err != nil {
		return worldTileMutationContext{}, err
	}
	var storageMapID any = normalizedMapID
	if normalizedMapID == UnifiedOverworldMapID {
		storageMapID = nil
	}
	if source == "" {
		source = "unknown"
	}
	return worldTileMutationContext{
		MapID:        normalizedMapID,
		StorageMapID: storageMapID,
		CharID:       characterIDForTileMutation(ses),
		Source:       source,
	}, nil
}

func normalizeWorldTileMapID(wh *WorldHandler, mapID int) (int, error) {
	if mapID == UnifiedOverworldMapID {
		return UnifiedOverworldMapID, nil
	}
	if mapID <= 0 {
		return 0, fmt.Errorf("invalid map")
	}
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		return UnifiedOverworldMapID, nil
	}

	var isOverworld int
	err := db.GlobalWorldDB.DB.QueryRow(`SELECT is_overworld FROM phaser_maps WHERE id = $1`, mapID).Scan(&isOverworld)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("unknown map")
		}
		return 0, fmt.Errorf("lookup map: %w", err)
	}
	if isOverworld == 1 {
		return UnifiedOverworldMapID, nil
	}
	return mapID, nil
}

func validateWorldTileCoordinates(ctx worldTileMutationContext, coords []tileCoord) error {
	if len(coords) == 0 {
		return fmt.Errorf("invalid tile count")
	}
	if ctx.MapID == UnifiedOverworldMapID {
		return nil
	}

	var width, height int
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT width, height FROM phaser_maps WHERE id = $1`, ctx.MapID).Scan(&width, &height); err != nil {
		return fmt.Errorf("lookup map bounds: %w", err)
	}
	for _, coord := range coords {
		if coord.X < 0 || coord.Y < 0 || coord.X >= width || coord.Y >= height {
			return fmt.Errorf("tile out of bounds")
		}
	}
	return nil
}

func validateTileImagesExist(tileImageIDs []int) error {
	seen := make(map[int]bool, len(tileImageIDs))
	for _, tileImageID := range tileImageIDs {
		if tileImageID <= 0 {
			return fmt.Errorf("invalid tile image")
		}
		if seen[tileImageID] {
			continue
		}
		seen[tileImageID] = true
		var exists int
		if err := db.GlobalWorldDB.DB.QueryRow(`SELECT 1 FROM phaser_tile_images WHERE id = $1`, tileImageID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("unknown tile image")
			}
			return fmt.Errorf("lookup tile image: %w", err)
		}
	}
	return nil
}

func applyWorldTilePlacements(
	ses *session.Session,
	wh *WorldHandler,
	mapID int,
	tiles []TileEdit,
	source string,
) (worldTileMutationResult, error) {
	if len(tiles) == 0 || len(tiles) > maxTilesPerRequest {
		return worldTileMutationResult{}, fmt.Errorf("invalid tile count")
	}

	ctx, err := newWorldTileMutationContext(ses, wh, mapID, source)
	if err != nil {
		return worldTileMutationResult{}, err
	}

	coords := make([]tileCoord, 0, len(tiles))
	tileImageIDs := make([]int, 0, len(tiles))
	for _, tile := range tiles {
		coords = append(coords, tileCoord{X: tile.X, Y: tile.Y})
		tileImageIDs = append(tileImageIDs, tile.TileImageID)
	}
	if err := validateWorldTileCoordinates(ctx, coords); err != nil {
		return worldTileMutationResult{}, err
	}
	if err := validateTileImagesExist(tileImageIDs); err != nil {
		return worldTileMutationResult{}, err
	}

	propertiesCache := make(map[int]tileRuntimeProperties)
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return worldTileMutationResult{}, fmt.Errorf("begin tile mutation: %w", err)
	}
	defer tx.Rollback()

	result := worldTileMutationResult{Edits: make([]TileEdit, 0, len(tiles)), MapID: ctx.MapID}
	for _, tile := range tiles {
		props, ok := propertiesCache[tile.TileImageID]
		if !ok {
			props = tileRuntimePropertiesForTileImage(tile.TileImageID)
			propertiesCache[tile.TileImageID] = props
		}
		if err := upsertWorldTilePlacement(tx, ctx, tile, props); err != nil {
			return worldTileMutationResult{}, err
		}
		result.Edits = append(result.Edits, TileEdit{
			X:             tile.X,
			Y:             tile.Y,
			TileImageID:   tile.TileImageID,
			CollisionType: props.CollisionType,
			RawFootTileID: props.RawFootTileID,
			TalkOverTile:  props.TalkOverTile,
			Erased:        false,
		})
		result.Changed++
	}

	if err := tx.Commit(); err != nil {
		return worldTileMutationResult{}, fmt.Errorf("commit tile mutation: %w", err)
	}
	invalidateWorldTileCaches(wh, ctx.MapID, result.Edits)
	return result, nil
}

func upsertWorldTilePlacement(tx *sql.Tx, ctx worldTileMutationContext, tile TileEdit, props tileRuntimeProperties) error {
	if _, err := tx.Exec(`
		INSERT INTO phaser_tiles (
			x, y, tile_image_id, map_id, collision_type, raw_foot_tile_id, talk_over_tile,
			local_x, local_y, source_map_id, encounter_area_id,
			is_native_game_data, coordinate_origin, content_origin,
			is_original_tile_location, has_tile_edit, is_tile_erased,
			is_user_placed, placed_by_char_id, placed_at,
			last_edited_by_char_id, last_edited_at, last_edit_source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, NULL, NULL, FALSE, 'user', 'user', 0, 1, 0, 1, $8, CURRENT_TIMESTAMP, $8, CURRENT_TIMESTAMP, $9)
		ON CONFLICT (x, y, COALESCE(map_id, -1)) DO UPDATE SET
			tile_image_id = EXCLUDED.tile_image_id,
			collision_type = EXCLUDED.collision_type,
			raw_foot_tile_id = EXCLUDED.raw_foot_tile_id,
			talk_over_tile = EXCLUDED.talk_over_tile,
			encounter_area_id = NULL,
			content_origin = 'user',
			has_tile_edit = 1,
			is_tile_erased = 0,
			is_user_placed = 1,
			placed_by_char_id = EXCLUDED.placed_by_char_id,
			placed_at = CURRENT_TIMESTAMP,
			last_edited_by_char_id = EXCLUDED.last_edited_by_char_id,
			last_edited_at = CURRENT_TIMESTAMP,
			last_edit_source = EXCLUDED.last_edit_source`,
		tile.X,
		tile.Y,
		tile.TileImageID,
		ctx.StorageMapID,
		props.CollisionType,
		props.RawFootTileID,
		props.TalkOverTile,
		ctx.CharID,
		ctx.Source,
	); err != nil {
		return fmt.Errorf("upsert tile (%d,%d): %w", tile.X, tile.Y, err)
	}
	return nil
}

func applyWorldTileErasures(
	ses *session.Session,
	wh *WorldHandler,
	mapID int,
	coords []tileCoord,
	source string,
) (worldTileMutationResult, error) {
	if len(coords) == 0 || len(coords) > maxTilesPerRequest {
		return worldTileMutationResult{}, fmt.Errorf("invalid tile count")
	}

	ctx, err := newWorldTileMutationContext(ses, wh, mapID, source)
	if err != nil {
		return worldTileMutationResult{}, err
	}
	if err := validateWorldTileCoordinates(ctx, coords); err != nil {
		return worldTileMutationResult{}, err
	}

	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return worldTileMutationResult{}, fmt.Errorf("begin tile erasure: %w", err)
	}
	defer tx.Rollback()

	result := worldTileMutationResult{Edits: make([]TileEdit, 0, len(coords)), MapID: ctx.MapID}
	for _, coord := range coords {
		changed, err := eraseWorldTile(tx, ctx, coord)
		if err != nil {
			return worldTileMutationResult{}, err
		}
		if !changed {
			continue
		}
		result.Edits = append(result.Edits, TileEdit{
			X:           coord.X,
			Y:           coord.Y,
			TileImageID: 0,
			Erased:      true,
		})
		result.Changed++
	}

	if err := tx.Commit(); err != nil {
		return worldTileMutationResult{}, fmt.Errorf("commit tile erasure: %w", err)
	}
	if result.Changed > 0 {
		invalidateWorldTileCaches(wh, ctx.MapID, result.Edits)
	}
	return result, nil
}

func eraseWorldTile(tx *sql.Tx, ctx worldTileMutationContext, coord tileCoord) (bool, error) {
	var isOriginal, isErased int
	var query string
	var args []any
	if ctx.MapID == UnifiedOverworldMapID {
		query = `SELECT is_original_tile_location, is_tile_erased FROM phaser_tiles WHERE map_id IS NULL AND x = $1 AND y = $2`
		args = []any{coord.X, coord.Y}
	} else {
		query = `SELECT is_original_tile_location, is_tile_erased FROM phaser_tiles WHERE map_id = $1 AND x = $2 AND y = $3`
		args = []any{ctx.MapID, coord.X, coord.Y}
	}
	if err := tx.QueryRow(query, args...).Scan(&isOriginal, &isErased); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("lookup tile for erase (%d,%d): %w", coord.X, coord.Y, err)
	}
	if isOriginal == 1 {
		if isErased == 1 {
			return false, nil
		}
		if ctx.MapID == UnifiedOverworldMapID {
			_, err := tx.Exec(`
				UPDATE phaser_tiles
				SET has_tile_edit = 1,
					is_tile_erased = 1,
					encounter_area_id = NULL,
					content_origin = 'user',
					is_user_placed = 1,
					placed_by_char_id = $3,
					placed_at = CURRENT_TIMESTAMP,
					last_edited_by_char_id = $3,
					last_edited_at = CURRENT_TIMESTAMP,
					last_edit_source = $4
				WHERE map_id IS NULL AND x = $1 AND y = $2`,
				coord.X, coord.Y, ctx.CharID, ctx.Source)
			if err != nil {
				return false, fmt.Errorf("mark original tile erased (%d,%d): %w", coord.X, coord.Y, err)
			}
		} else {
			_, err := tx.Exec(`
				UPDATE phaser_tiles
				SET has_tile_edit = 1,
					is_tile_erased = 1,
					encounter_area_id = NULL,
					content_origin = 'user',
					is_user_placed = 1,
					placed_by_char_id = $4,
					placed_at = CURRENT_TIMESTAMP,
					last_edited_by_char_id = $4,
					last_edited_at = CURRENT_TIMESTAMP,
					last_edit_source = $5
				WHERE map_id = $1 AND x = $2 AND y = $3`,
				ctx.MapID, coord.X, coord.Y, ctx.CharID, ctx.Source)
			if err != nil {
				return false, fmt.Errorf("mark original tile erased (%d,%d): %w", coord.X, coord.Y, err)
			}
		}
		return true, nil
	}

	var err error
	if ctx.MapID == UnifiedOverworldMapID {
		_, err = tx.Exec(`DELETE FROM phaser_tiles WHERE map_id IS NULL AND x = $1 AND y = $2`, coord.X, coord.Y)
	} else {
		_, err = tx.Exec(`DELETE FROM phaser_tiles WHERE map_id = $1 AND x = $2 AND y = $3`, ctx.MapID, coord.X, coord.Y)
	}
	if err != nil {
		return false, fmt.Errorf("delete placed tile (%d,%d): %w", coord.X, coord.Y, err)
	}
	return true, nil
}

func currentWorldTileImageID(mapID, x, y int) (int, bool, error) {
	var tileImageID int
	var err error
	if mapID == UnifiedOverworldMapID {
		err = db.GlobalWorldDB.DB.QueryRow(`
			SELECT tile_image_id
			FROM phaser_tiles
			WHERE map_id IS NULL AND x = $1 AND y = $2 AND is_tile_erased = 0`,
			x, y).Scan(&tileImageID)
	} else {
		err = db.GlobalWorldDB.DB.QueryRow(`
			SELECT tile_image_id
			FROM phaser_tiles
			WHERE map_id = $1 AND x = $2 AND y = $3 AND is_tile_erased = 0`,
			mapID, x, y).Scan(&tileImageID)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return tileImageID, true, nil
}

func invalidateWorldTileCaches(wh *WorldHandler, mapID int, edits []TileEdit) {
	if wh == nil {
		return
	}
	if wh.ActorManager != nil {
		wh.ActorManager.InvalidateCollisionMap(mapID)
	}
	if wh.WildEncounter != nil {
		wh.WildEncounter.InvalidateTiles(mapID, edits)
	}
}
