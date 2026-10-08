package world

import (
	"capturequest/internal/db"
	"context"
	"errors"
	"testing"
	"time"
)

func TestTileQueryUsesInjectedPoolAndOwnerCancellation(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- ses.ExecuteCommand(ctx, func() { HandlePhaserTilesRequest(ses, []byte(`{"mapId":9999,"requestId":"tile:cancel"}`), wh) })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("tile owner deadline: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		lease.Close()
		<-done
		t.Fatal("tile query escaped owner deadline")
	}
	if database.Stats().WaitCount == before {
		t.Fatal("tile read did not wait on injected pool")
	}
}
