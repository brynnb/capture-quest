package db_currency

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestOwnedWalletReadHonorsPoolWaitCancellation(t *testing.T) {
	database := testdb.Postgres(t)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	wallet, err := GetCharacterWalletContext(ctx, database, 42)
	if !errors.Is(err, context.DeadlineExceeded) || wallet != nil {
		t.Fatalf("cancelled wallet read became success: wallet=%v error=%v", wallet, err)
	}
}
