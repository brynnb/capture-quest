package client

import (
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
)

func TestClientEntryOptionsUseOwnedReadWithoutErrorDefaults(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name,options) VALUES(9,'client-reader','{"showNetworkStats":false,"rivalName":"Blue"}')`)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	client, err := NewClient(context.Background(), database, &model.CharacterData{ID: 9}, nil, nil, nil)
	if err != nil || client.Options().RivalName != "Blue" || client.ShowNetworkStatsEnabled() {
		t.Fatalf("owned options: %+v %v", client, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if client, err := NewClient(ctx, database, &model.CharacterData{ID: 9}, nil, nil, nil); client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled entry defaulted: %+v %v", client, err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET options='[]' WHERE id=9`)
	if client, err := NewClient(context.Background(), database, &model.CharacterData{ID: 9}, nil, nil, nil); client != nil || err == nil {
		t.Fatal("invalid options constructed client")
	}
}
