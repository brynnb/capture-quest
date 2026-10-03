package world

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"capturequest/internal/session"
)

// nativeScriptMap resolves script identity from owned server state, never from
// metadata requested by the client. Overworld coordinates use the original tile
// provenance even when the current tile was edited or erased.
func (wh *WorldHandler) nativeScriptMap(ses *session.Session) (string, error) {
	if wh == nil || wh.database == nil || ses == nil || !ses.HasValidClient() || ses.IsClosed() {
		return "", fmt.Errorf("script location unavailable")
	}
	x, y, mapID := wh.ownedPlayerPosition(ses)
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	return nativeScriptMapAt(func(query string, args ...any) *sql.Row { return wh.database.QueryRowContext(ctx, query, args...) }, mapID, x, y)
}

// The same provenance query serves ordinary reads and transactional issuance.
func nativeScriptMapAt(queryRow func(string, ...any) *sql.Row, mapID, x, y int) (string, error) {
	if mapID != UnifiedOverworldMapID {
		var name string
		var overworld int
		if err := queryRow(`SELECT name, is_overworld FROM phaser_maps WHERE id = $1`, mapID).Scan(&name, &overworld); err != nil {
			return "", err
		}
		if overworld == 0 {
			return name, nil
		}
	}
	_, name, err := nativeOverworldMapAt(queryRow, x, y)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", fmt.Errorf("script location at %d,%d has no native map", x, y)
	}
	return name, nil
}

// Original source identity survives edited/erased art. Pure user-added locations
// have no native effects; broken or ambiguous original identity must fail closed.
func nativeOverworldMapAt(queryRow func(string, ...any) *sql.Row, x, y int) (int, string, error) {
	var id, originals, matched, maps int
	var name string
	err := queryRow(`SELECT COALESCE(MIN(pm.id),0), COALESCE(MIN(pm.name),''),
        COUNT(*), COUNT(pm.id), COUNT(DISTINCT pm.id)
        FROM phaser_tiles pt
        LEFT JOIN phaser_maps pm ON pm.id=COALESCE(pt.original_source_map_id,pt.source_map_id) AND pm.is_overworld=1
        WHERE pt.map_id IS NULL AND pt.x=$1 AND pt.y=$2 AND pt.is_original_tile_location=1`, x, y).Scan(&id, &name, &originals, &matched, &maps)
	if err != nil {
		return 0, "", err
	}
	if originals == 0 {
		return 0, "", nil
	}
	if matched != originals || maps != 1 || name == "" {
		return 0, "", fmt.Errorf("native map at %d,%d: %d original tiles, %d matched tiles, %d maps", x, y, originals, matched, maps)
	}
	return id, name, nil
}
