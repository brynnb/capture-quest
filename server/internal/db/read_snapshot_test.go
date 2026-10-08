package db_test

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"testing"
	"time"
)

func TestReadSnapshotBoundsNestedContextlessQuery(t *testing.T) {
	database := testdb.Postgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	value, err := db.ReadSnapshot(ctx, database, func(_ context.Context, q db.ReadDBTX) (int, error) {
		var ignored any
		// Legacy domain repositories use QueryRow without a context argument. The
		// snapshot must still supply its owned deadline rather than Background.
		err := q.QueryRow(`SELECT pg_sleep(5)`).Scan(&ignored)
		return 99, err
	})
	if err == nil || value != 0 || ctx.Err() != context.DeadlineExceeded || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("nested query escaped snapshot: value=%d error=%v duration=%v", value, err, time.Since(started))
	}
}
