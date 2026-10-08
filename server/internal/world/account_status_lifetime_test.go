package world

import (
	"capturequest/internal/db"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAccountStatusReadUsesInjectedDatabaseAndOwnerDeadline(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO account(id,name,status) VALUES(7,'status-reader',255)`)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	status, err := getAccountStatus(context.Background(), database, 7)
	if err != nil || status != 255 {
		t.Fatalf("injected status=%d: %v", status, err)
	}
	if _, err := getAccountStatus(context.Background(), database, 8); err == nil {
		t.Fatal("missing account silently defaulted status")
	}
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = getAccountStatus(ctx, database, 7)
	if !errors.Is(err, context.DeadlineExceeded) || database.Stats().WaitCount == before {
		t.Fatalf("status read escaped pool-wait deadline: %v", err)
	}
}

func TestSlashAuthorizationAndTileAuthorityShareOwnedStatusRead(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO account(id,name,status) VALUES(7,'status-reader',0)`)
	ses.AccountID = 7
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	called := false
	const name = "test_owned_permission"
	RegisterChatCommand(&ChatCommand{Name: name, MinStatus: 1, Handler: func(*session.Session, string, *WorldHandler) { called = true }})
	t.Cleanup(func() { delete(chatCommandRegistry, name) })
	messages.streams = nil
	HandleChatCommand(ses, "/help", wh)
	var help ChatMessageBroadcast
	if len(messages.streams) != 1 {
		t.Fatal("missing help response")
	}
	if err := json.Unmarshal(messages.streams[0].payload, &help); err != nil || !strings.Contains(help.Text, "/help") || strings.Contains(help.Text, name) {
		t.Fatalf("help exposed unavailable command: %+v %v", help, err)
	}
	HandleChatCommand(ses, "/"+name, wh)
	if called || canAdminEditWorldTiles(ses, wh) {
		t.Fatal("ordinary account obtained privileged authority")
	}
	ses.Client.CharData().Gm = 1
	if !canAdminEditWorldTiles(ses, wh) {
		t.Fatal("established character GM policy was removed")
	}
	ses.Client.CharData().Gm = 0
	testdb.Exec(t, database, `UPDATE account SET status=1 WHERE id=7`)
	HandleChatCommand(ses, "/"+name, wh)
	if !called || !canAdminEditWorldTiles(ses, wh) {
		t.Fatal("injected account authority unavailable")
	}
	called = false
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ses.ExecuteCommand(ctx, func() {
		HandleChatCommand(ses, "/"+name, wh)
		if canAdminEditWorldTiles(ses, wh) {
			t.Error("expired tile authority allowed")
		}
	}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("command deadline: %v", err)
	}
	if called {
		t.Fatal("cancelled permission read executed slash command")
	}
	// A character-level GM remains an established alternative permission, but
	// cancellation must retire that authority too, rather than fall through.
	ses.Client.CharData().Gm = 1
	before := database.Stats().WaitCount
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if err := ses.ExecuteCommand(ctx2, func() {
		if canAdminEditWorldTiles(ses, wh) {
			t.Error("cancelled account read fell through to cached GM")
		}
	}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("tile owner deadline: %v", err)
	}
	if database.Stats().WaitCount == before {
		t.Fatal("tile permission query did not exercise pool wait")
	}
}
