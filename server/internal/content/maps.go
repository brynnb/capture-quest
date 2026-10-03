package content

import (
	"context"
	"database/sql"
	"time"

	"capturequest/internal/protocol"
)

func (s *Service) MapInfo(ctx context.Context, id int) (protocol.PhaserMapInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var m protocol.PhaserMapInfo
	err := s.database.QueryRowContext(ctx, `SELECT id,name,width,height,tileset_id,is_overworld FROM phaser_maps WHERE id=$1`, id).
		Scan(&m.ID, &m.Name, &m.Width, &m.Height, &m.TilesetID, &m.IsOverworld)
	return m, err
}

// OverworldInfo projects the catalog bounds. The runtime supplies its synthetic
// map ID; it is not a physical phaser_maps row or a catalog-local identity.
func (s *Service) OverworldInfo(ctx context.Context) (protocol.PhaserMapInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m := protocol.PhaserMapInfo{Name: "Unified Overworld", IsOverworld: 1}
	var minX, minY, maxX, maxY sql.NullInt64
	if err := s.database.QueryRowContext(ctx, `SELECT MIN(x),MIN(y),MAX(x),MAX(y) FROM phaser_tiles WHERE map_id IS NULL AND is_tile_erased=0`).
		Scan(&minX, &minY, &maxX, &maxY); err != nil {
		return m, err
	}
	if minX.Valid && minY.Valid && maxX.Valid && maxY.Valid {
		x0, y0, x1, y1 := int(minX.Int64), int(minY.Int64), int(maxX.Int64), int(maxY.Int64)
		m.TileMinX, m.TileMinY, m.TileMaxX, m.TileMaxY = &x0, &y0, &x1, &y1
		m.Width, m.Height = x1-x0+1, y1-y0+1
	}
	return m, nil
}

func (s *Service) OverworldMaps(ctx context.Context) ([]protocol.PhaserMapInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := s.database.QueryContext(ctx, `SELECT id,name,width,height,tileset_id,is_overworld FROM phaser_maps WHERE is_overworld=1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	maps := make([]protocol.PhaserMapInfo, 0)
	for rows.Next() {
		var m protocol.PhaserMapInfo
		if err := rows.Scan(&m.ID, &m.Name, &m.Width, &m.Height, &m.TilesetID, &m.IsOverworld); err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return maps, nil
}
