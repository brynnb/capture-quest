package content

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestContentQueryCancellationDeadlineAndRetry(t *testing.T) {
	database := testdb.Postgres(t)
	queries := New(database)
	loaders := map[string]func(context.Context) error{
		"phaser_pokemon": func(ctx context.Context) error { _, err := queries.Pokemon(ctx, 25); return err },
		"phaser_moves":   func(ctx context.Context) error { _, err := queries.Move(ctx, 150); return err },
		"phaser_items":   func(ctx context.Context) error { _, err := queries.Item(ctx, 1); return err },
	}
	testdb.Exec(t, database, `INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp) VALUES(25,'PIKACHU','ELECTRIC',35,55,30,90,50,190,82);
  INSERT INTO phaser_moves(id,constant_name,name,short_name) VALUES(150,'SPLASH','SPLASH','SPLASH');
  INSERT INTO phaser_items(id,name,short_name) VALUES(1,'POTION','POTION')`)
	for table, load := range loaders {
		t.Run(table, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := load(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled query=%v", err)
			}
			lock, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback()
			if _, err := lock.Exec(`LOCK TABLE ` + table + ` IN ACCESS EXCLUSIVE MODE`); err != nil {
				t.Fatal(err)
			}
			deadline, cancelDeadline := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancelDeadline()
			started := time.Now()
			if err := load(deadline); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("locked query=%v", err)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("query ignored caller deadline")
			}
			if err := lock.Rollback(); err != nil {
				t.Fatal(err)
			}
			if err := load(context.Background()); err != nil {
				t.Fatalf("retry=%v", err)
			}
		})
	}
}

func TestContentQueryBoundsBackgroundCaller(t *testing.T) {
	database := testdb.Postgres(t)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE phaser_pokemon IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = New(database).Pokemon(context.Background(), 25)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unbounded caller query=%v", err)
	}
	if elapsed := time.Since(started); elapsed > 7*time.Second {
		t.Fatalf("query exceeded five-second limit and driver allowance: %v", elapsed)
	}
}
