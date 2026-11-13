package content

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestMapCatalogProjections(t *testing.T) {
	database := testdb.Postgres(t)
	s := New(database)
	info, err := s.OverworldInfo(context.Background())
	if err != nil || info.TileMinX != nil || info.Width != 0 {
		t.Fatalf("empty catalog must not invent bounds: %+v %v", info, err)
	}
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,tileset_id,is_overworld) VALUES
 (9,'NINE',20,30,4,1),(2,'TWO',10,12,NULL,1),(38,'ROOM',8,8,NULL,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,is_tile_erased) VALUES
 (NULL,-10,-20,1,0),(NULL,5,7,1,0),(NULL,-100,-100,1,1),(38,100,100,1,0);`)
	room, err := s.MapInfo(context.Background(), 38)
	if err != nil || room.Name != "ROOM" || room.IsOverworld != 0 || room.TileMinX != nil {
		t.Fatalf("interior: %+v %v", room, err)
	}
	info, err = s.OverworldInfo(context.Background())
	if err != nil || info.TileMinX == nil || *info.TileMinX != -10 || *info.TileMinY != -20 || *info.TileMaxX != 5 || *info.TileMaxY != 7 || info.Width != 16 || info.Height != 28 {
		t.Fatalf("active NULL-map bounds: %+v %v", info, err)
	}
}

func TestMapCatalogCancellationDeadlineAndRetry(t *testing.T) {
	database := testdb.Postgres(t)
	s := New(database)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(2,'TWO',10,12,1)`)
	cases := []struct {
		name, table string
		load        func(context.Context) error
	}{
		{"map", "phaser_maps", func(ctx context.Context) error { _, err := s.MapInfo(ctx, 2); return err }},
		{"bounds", "phaser_tiles", func(ctx context.Context) error { _, err := s.OverworldInfo(ctx); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := tc.load(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
			lock, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback()
			if _, err := lock.Exec(`LOCK TABLE ` + tc.table + ` IN ACCESS EXCLUSIVE MODE`); err != nil {
				t.Fatal(err)
			}
			ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			started := time.Now()
			if err := tc.load(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline: %v", err)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("ignored caller deadline")
			}
			if err := lock.Rollback(); err != nil {
				t.Fatal(err)
			}
			if err := tc.load(context.Background()); err != nil {
				t.Fatalf("retry: %v", err)
			}
		})
	}
}
