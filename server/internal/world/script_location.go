package world

import (
	"context"
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
	x, y, mapID := wh.scriptPlayerPosition(ses)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if mapID != UnifiedOverworldMapID {
		var name string
		var overworld int
		if err := wh.database.QueryRowContext(ctx, `SELECT name, is_overworld FROM phaser_maps WHERE id = $1`, mapID).Scan(&name, &overworld); err != nil {
			return "", err
		}
		if overworld == 0 {
			return name, nil
		}
	}
	var name string
	var count int
	err := wh.database.QueryRowContext(ctx, `
		SELECT COALESCE(MIN(pm.name), ''), COUNT(DISTINCT pm.id)
		FROM phaser_tiles pt
		JOIN phaser_maps pm ON pm.id = COALESCE(pt.original_source_map_id, pt.source_map_id)
		WHERE pt.map_id IS NULL AND pt.x = $1 AND pt.y = $2
		  AND pt.is_original_tile_location = 1 AND pm.is_overworld = 1`, x, y).Scan(&name, &count)
	if err != nil {
		return "", err
	}
	// Never choose an arbitrary map when the source identity is missing or ambiguous.
	if count != 1 {
		return "", fmt.Errorf("script location at %d,%d has %d native maps", x, y, count)
	}
	return name, nil
}

func (wh *WorldHandler) scriptPlayerPosition(ses *session.Session) (x, y, mapID int) {
	char := ses.Client.CharData()
	x, y, mapID = int(char.X), int(char.Y), int(char.MapID)
	if wh.PlayerMovement != nil {
		if mx, my, mm, ok := wh.PlayerMovement.GetPosition(int(char.ID)); ok {
			x, y, mapID = mx, my, mm
		}
	}
	return x, y, mapID
}
