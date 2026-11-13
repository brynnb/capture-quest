package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/session"
)

type movementRoute struct {
	MapID, X, Y int
	Path        []PathNode
	Surfing     bool
}
type routePoint struct {
	X *int `json:"x"`
	Y *int `json:"y"`
}
type storedMovementRoute struct {
	Version int          `json:"version"`
	Surfing *bool        `json:"surfing"`
	Path    []routePoint `json:"path"`
}

func loadMovementRouteIn(tx db.DBTX, charID int64) (*movementRoute, error) {
	var route movementRoute
	var encoded string
	err := tx.QueryRow(`SELECT map_id,x,y,path_json FROM character_movement_routes WHERE character_id=$1`, charID).Scan(&route.MapID, &route.X, &route.Y, &encoded)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stored storedMovementRoute
	if err := decodePlayerMovement([]byte(encoded), &stored); err != nil {
		return nil, fmt.Errorf("movement route for character %d: %w", charID, err)
	}
	if stored.Version != 1 || len(stored.Path) == 0 || stored.Surfing == nil {
		return nil, fmt.Errorf("invalid movement route for character %d: version=%d points=%d", charID, stored.Version, len(stored.Path))
	}
	route.Surfing = *stored.Surfing
	x, y := route.X, route.Y
	for _, point := range stored.Path {
		if point.X == nil || point.Y == nil || abs(*point.X-x)+abs(*point.Y-y) != 1 {
			return nil, fmt.Errorf("invalid movement route point for character %d from (%d,%d)", charID, x, y)
		}
		x, y = *point.X, *point.Y
		route.Path = append(route.Path, PathNode{X: x, Y: y})
	}
	return &route, nil
}

func saveMovementRouteIn(tx db.DBTX, charID int64, mapID, x, y int, path []PathNode, surfing bool) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	if len(path) == 0 {
		_, err := tx.Exec(`DELETE FROM character_movement_routes WHERE character_id=$1`, charID)
		return err
	}
	stored := storedMovementRoute{Version: 1, Surfing: &surfing}
	fromX, fromY := x, y
	for _, point := range path {
		if abs(point.X-fromX)+abs(point.Y-fromY) != 1 {
			return fmt.Errorf("movement route for character %d is not adjacent from (%d,%d) to (%d,%d)", charID, fromX, fromY, point.X, point.Y)
		}
		px, py := point.X, point.Y
		stored.Path = append(stored.Path, routePoint{X: &px, Y: &py})
		fromX, fromY = px, py
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO character_movement_routes(character_id,map_id,x,y,path_json) VALUES($1,$2,$3,$4,$5) ON CONFLICT(character_id) DO UPDATE SET map_id=EXCLUDED.map_id,x=EXCLUDED.x,y=EXCLUDED.y,path_json=EXCLUDED.path_json`, charID, mapID, x, y, string(encoded))
	return err
}

// Restore only the current selected registration, under its command gate. The
// stored cursor must match saved and runtime source; malformed data fails entry.
func (m *PlayerMovementManager) restoreMovementRoute(ses *session.Session) error {
	charID := int64(ses.Client.CharData().ID)
	var route *movementRoute
	var savedMap, x, y int
	var surfing bool
	err := db.Transaction(ses.CommandContext(), m.wh.database, func(tx db.DBTX) error {
		if err := tx.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=$1 FOR UPDATE`, charID).Scan(&savedMap, &x, &y); err != nil {
			return err
		}
		var err error
		route, err = loadMovementRouteIn(tx, charID)
		if err != nil {
			return err
		}
		if route == nil {
			// Prepare entry state before presence/spawn publication, never from
			// an independent SQL read during actor presentation.
			surfing, err = isSurfableWaterTileIn(ses.CommandContext(), tx.(db.ContextDBTX), m.wh, normalizedVisiblePlayerMapID(m.wh, savedMap), x, y)
			return err
		}
		if route.MapID != normalizedVisiblePlayerMapID(m.wh, savedMap) || route.X != x || route.Y != y {
			return fmt.Errorf("movement route source for character %d differs from saved position", charID)
		}
		if err := validateClientDestinationIn(tx, route.MapID, route.X, route.Y); err != nil {
			return err
		}
		if err := requireNoOwnedBattleIn(tx, charID); err != nil {
			if errors.Is(err, errBattleOwnership) {
				route = nil
				return saveMovementRouteIn(tx, charID, 0, 0, 0, nil, false)
			}
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[int(charID)]
	if state == nil || state.SessionID != ses.SessionID || state.MapID != normalizedVisiblePlayerMapID(m.wh, savedMap) || state.CurrentX != x || state.CurrentY != y {
		return fmt.Errorf("movement route owner changed for character %d", charID)
	}
	if err := ses.CommandContext().Err(); err != nil {
		return err
	}
	if route != nil {
		state.Path = append([]PathNode(nil), route.Path...)
		surfing = route.Surfing
	}
	state.IsSurfing = surfing
	m.applyBicycleMapRules(state)
	state.LastMoveTime = time.Now()
	return nil
}

// Used by tests and transaction callers; does not publish or install a route.
func readMovementRoute(ctx context.Context, database *sql.DB, charID int64) (*movementRoute, error) {
	var route *movementRoute
	err := db.Transaction(ctx, database, func(tx db.DBTX) error { var err error; route, err = loadMovementRouteIn(tx, charID); return err })
	return route, err
}
