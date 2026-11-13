package content

import (
	"capturequest/internal/db"
	"capturequest/internal/staticdata"
	"context"
	"database/sql"
)

// StaticData shares the same coherent, bounded aggregate read as the other
// content projections. No process-global cache can retain a cancelled first load.
func (s *Service) StaticData(ctx context.Context) (*staticdata.StaticData, error) {
	return db.ReadSnapshot(ctx, s.database, func(ctx context.Context, q db.ReadDBTX) (*staticdata.StaticData, error) {
		data := &staticdata.StaticData{}
		var err error
		data.Classes, err = collect(ctx, q, `SELECT id,name,class_type,lore FROM poke_classes ORDER BY id`, func(rows *sql.Rows, c *staticdata.ClassInfo) error {
			var kind, lore sql.NullString
			if err := rows.Scan(&c.ID, &c.Name, &kind, &lore); err != nil {
				return err
			}
			c.ClassType = kind.String
			c.Lore = lore.String
			return nil
		})
		if err != nil {
			return nil, err
		}
		data.Factions, err = collect(ctx, q, `SELECT id,name,short_name,lore,is_playable,is_starting FROM poke_factions ORDER BY id`, func(rows *sql.Rows, f *staticdata.FactionInfo) error {
			var short, lore sql.NullString
			var playable, starting sql.NullInt64
			if err := rows.Scan(&f.ID, &f.Name, &short, &lore, &playable, &starting); err != nil {
				return err
			}
			f.ShortName = short.String
			f.Lore = lore.String
			f.IsPlayable = playable.Valid && playable.Int64 == 1
			f.IsStarting = starting.Valid && starting.Int64 == 1
			return nil
		})
		if err != nil {
			return nil, err
		}
		data.Maps, err = collect(ctx, q, `SELECT id,name,width,height,tileset_id,is_overworld,north_connection,south_connection,west_connection,east_connection FROM phaser_maps ORDER BY id`, func(rows *sql.Rows, m *staticdata.MapInfo) error {
			var overworld sql.NullInt64
			if err := rows.Scan(&m.ID, &m.Name, &m.Width, &m.Height, &m.TilesetID, &overworld, &m.NorthConnection, &m.SouthConnection, &m.WestConnection, &m.EastConnection); err != nil {
				return err
			}
			m.IsOverworld = overworld.Valid && overworld.Int64 == 1
			return nil
		})
		if err != nil {
			return nil, err
		}
		data.StartCities, err = collect(ctx, q, `SELECT id,map_id,name,spawn_x,spawn_y,description,sort_order FROM poke_start_cities ORDER BY sort_order,id`, func(rows *sql.Rows, c *staticdata.StartCityInfo) error {
			var description sql.NullString
			if err := rows.Scan(&c.ID, &c.MapID, &c.Name, &c.SpawnX, &c.SpawnY, &description, &c.SortOrder); err != nil {
				return err
			}
			c.Description = description.String
			return nil
		})
		if err != nil {
			return nil, err
		}
		return data, nil
	})
}
